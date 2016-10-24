package nasa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIVersion = "v1"

	apiBaseURL = "https://api.nasa.gov"

	defaultUserAgent = "go-nasa-fetcher"
)

var (
	apiKey = os.Getenv("NASA_API_KEY")

	defaultAPIKey = "DEMO_KEY"
)

type Client struct {
	apiKey    string
	version   string
	client    *http.Client
	userAgent string
	mu        sync.RWMutex
}

func (c *Client) SetAPIKey(apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.apiKey = apiKey
}

func (c *Client) SetVersion(version string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.version = version
}

func (c *Client) Version() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	version := c.version
	if version == "" {
		version = defaultAPIVersion
	}
	return version
}

func (c *Client) APIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	apiKey := c.apiKey
	if apiKey == "" {
		apiKey = defaultAPIKey
	}
	return apiKey
}

type MarsPhotos struct {
	Photos []*MarsPhoto `json:"photos,omitempty"`
}

type MarsPhoto struct {
	Id        int      `json:"id"`
	SOL       int      `json:"sol"`
	Camera    *Camera  `json:"camera,omitempty"`
	Rover     *Rover   `json:"rover,omitempty"`
	EarthDate *YMDTime `json:"earth_date,omitempty"`
	ImageURL  string   `json:"img_src,omitempty"`
}

type YMDTime time.Time

func (ymd *YMDTime) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	splits := strings.Split(str, "-")
	if len(splits) < 3 {
		return fmt.Errorf("expecting atleast 3 values separated by -, got %q", str)
	}
	values := []int{}
	for _, split := range splits {
		v, err := strconv.ParseInt(split, 10, 32)
		if err != nil {
			return err
		}
		values = append(values, int(v))
	}

	*ymd = YMDTime(time.Date(values[0], time.Month(values[1]), values[2], 0, 0, 0, 0, time.UTC))
	return nil
}

func (ymd YMDTime) String() string {
	year, month, day := time.Time(ymd).Date()
	return fmt.Sprintf("%d-%d-%d", year, month, day)
}

func (ymd *YMDTime) MarshalJSON() ([]byte, error) {
	return []byte(ymd.String()), nil
}

type Camera struct {
	Id        int      `json:"id,omitempty"`
	ShortName string   `json:"name,omitempty"`
	RoverId   int      `json:"rover_id,omitempty"`
	FullName  string   `json:"full_name,omitempty"`
	EarthDate *YMDTime `json:"earth_date,omitempty"`
}

type Rover struct {
	Id      int       `json:"id"`
	Name    string    `json:"name"`
	MaxSOL  int       `json:"max_sol"`
	MaxDate *YMDTime  `json:"max_date"`
	Status  Status    `json:"status"`
	Cameras []*Camera `json:"camera"`

	LandingDate *YMDTime `json:"landing_date"`
	LaunchDate  *YMDTime `json:"launch_date"`
	TotalPhotos uint64   `json:"total_photos"`
}

func (c *Client) queryParamsForPhotos(earthDate *time.Time) string {
	if earthDate == nil {
		now := time.Now()
		earthDate = &now
	}
	ymd := YMDTime(*earthDate)
	args := []string{fmt.Sprintf("earth_date=%s", ymd)}
	if apiKey := c.APIKey(); apiKey != "" {
		args = append(args, fmt.Sprintf("api_key=%s", apiKey))
	}

	return strings.Join(args, "&")
}

type Option interface {
	Apply(*Client)
}

type WithAPIKey string

func (key WithAPIKey) Apply(c *Client) {
	c.SetAPIKey(string(key))
}

type WithHTTPClient struct {
	Client *http.Client
}

func (whc WithHTTPClient) Apply(c *Client) {
	c.client = whc.Client
}

func New(options ...Option) (*Client, error) {
	c := new(Client)
	for _, opt := range options {
		opt.Apply(c)
	}
	return c, nil
}

type WithUserAgent string

func (wua WithUserAgent) Apply(c *Client) {
	c.userAgent = string(wua)
}

type requestDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func (c *Client) UserAgent() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ua := c.userAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	return ua
}

func (c *Client) SetUserAgent(ua string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userAgent = ua
}

func (c *Client) httpClient() requestDoer {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil {
		return http.DefaultClient
	}
	return c.client
}

func (c *Client) MarsPhotosToday() (*MarsPhotos, error) {
	return c.MarsPhotos(nil)
}

func (c *Client) MarsPhotos(t *time.Time) (*MarsPhotos, error) {
	// https://api.nasa.gov/mars-photos/api/v1/rovers/curiosity/photos?earth_date=2016-10-23&api_key=DEMO_KEY
	// https://api.nasa.gov/mars-photos/api/v1/rovers/curiosity/photos?earth_date=2014-10-25&api_key=DEMO_KEY
	path := fmt.Sprintf("%s/rovers/curiosity/photos", c.Version())
	if queryParamsStr := c.queryParamsForPhotos(t); queryParamsStr != "" {
		path = fmt.Sprintf("%s?%s", path, queryParamsStr)
	}

	apiURL := fmt.Sprintf("%s/mars-photos/api/%s", apiBaseURL, path)
	log.Printf("apiURL: %q\n", apiURL)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Add("User-Agent", c.UserAgent())

	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("%d %s", res.StatusCode, res.Status)
	}

	blob, err := ioutil.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		return nil, err
	}

	marsPhotos := new(MarsPhotos)
	if err := json.Unmarshal(blob, marsPhotos); err != nil {
		return nil, err
	}

	return marsPhotos, nil
}

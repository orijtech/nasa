package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/odeke-em/nasa"
)

var roverClient *nasa.Client

func init() {
	var err error
	roverClient, err = nasa.New()
	if err != nil {
		panic(err)
	}
}

type request struct {
	Date      *nasa.YMDTime `json:"date"`
	PastHours float64       `json:"hours"`
}

func parseRequest(req *http.Request) (*request, error) {
	slurp, err := ioutil.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(slurp) < 2 { // at bare minimum {}
		return &request{}, nil
	}

	preq := new(request)
	if err := json.Unmarshal(slurp, preq); err != nil {
		return nil, err
	}
	return preq, nil
}

func photos(rw http.ResponseWriter, req *http.Request) {
	preq, err := parseRequest(req)
	if err != nil {
		sendError(rw, err, http.StatusBadRequest)
		return
	}
	t := (*time.Time)(preq.Date)
	requestForPhotosAtDate(rw, t)
}

func requestForPhotosAtDate(rw http.ResponseWriter, t *time.Time) {
	marsPhotos, err := roverClient.MarsPhotos(t)
	if err != nil {
		sendError(rw, err, http.StatusInternalServerError)
		return
	}
	blob, err := json.MarshalIndent(marsPhotos, "", " ")
	if err != nil {
		sendError(rw, err, http.StatusInternalServerError)
		return
	}
	rw.Write(blob)
}

func sendError(rw http.ResponseWriter, err error, istatus int) {
	if err == nil {
		return
	}

	var status int = istatus
	if statuser, ok := err.(interface {
		Status() int
	}); ok {
		status = statuser.Status()
	}

	var resp interface{} = err
	if _, ok := err.(json.Marshaler); !ok {
		resp = map[string]string{"error": err.Error()}
	}

	blob, _ := json.MarshalIndent(resp, "", " ")
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	rw.Write(blob)
}

func parsePastHours(req *http.Request) (float64, error) {
	switch req.Method {
	case "POST":
		slurp, _ := ioutil.ReadAll(req.Body)
		_ = req.Body.Close()
		rreq := new(request)
		if err := json.Unmarshal(slurp, rreq); err != nil {
			return 0, err
		}
		return rreq.PastHours, nil
	default: // Fallback to the query string for all the others
		query := req.URL.Query()
		return strconv.ParseFloat(query.Get("h"), 0)
	}
}

func pastHours(rw http.ResponseWriter, req *http.Request) {
	hourDuration, err := parsePastHours(req)
	if err != nil {
		sendError(rw, err, http.StatusBadRequest)
		return
	}
	if hourDuration <= -0.0 {
		hourDuration *= -1
	}

	floorHour := int(hourDuration)
	minuteDuration := int(60 * (hourDuration - float64(floorHour)))
	totalDuration := (time.Duration(floorHour) * time.Hour) + (time.Duration(minuteDuration) * time.Minute)

	date := time.Now().Add(-1 * totalDuration)
	requestForPhotosAtDate(rw, &date)
}

func portFromEnv() string {
	port := "8080"
	p := strings.TrimSpace(os.Getenv("MARS_ROVER_SERVER_PORT"))
	if p == "" {
		p = port
	}
	return fmt.Sprintf(":%s", strings.TrimPrefix(p, ":"))
}

func main() {
	http.HandleFunc("/", photos)
	http.HandleFunc("/past", pastHours)
	server := http.Server{
		Addr: portFromEnv(),
	}
	if err := server.ListenAndServe(); err != nil {
	}
}

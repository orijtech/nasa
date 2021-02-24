package nasa_test

import (
	"fmt"
	"log"
	"time"

	"github.com/orijtech/nasa"
)

func Example_clientMarsPhotoTody() {
	client, err := nasa.New()
	if err != nil {
		log.Fatal(err)
	}

	marsPhotos, err := client.MarsPhotosToday()
	if err != nil {
		log.Fatal(err)
	}

	for i, photo := range marsPhotos.Photos {
		fmt.Printf("#%d: id:%d earthDate: %s imageURL: %s\n", i, photo.Id, photo.EarthDate, photo.ImageURL)
	}
}

func Example_clientMarsPhoto() {
	client, err := nasa.New()
	if err != nil {
		log.Fatal(err)
	}

	tenHoursAgo := time.Now().Add(-10 * time.Hour)
	marsPhotos, err := client.MarsPhotos(&tenHoursAgo)
	if err != nil {
		log.Fatal(err)
	}

	for i, photo := range marsPhotos.Photos {
		fmt.Printf("#%d: id:%d earthDate: %s imageURL: %s\n", i, photo.Id, photo.EarthDate, photo.ImageURL)
	}
}

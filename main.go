// stravauploader project main.go
package main

import (
	"encoding/json"
	"flag"
	"github.com/strava/go.strava"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var lastSync time.Time
var accessToken string
var client *strava.Client
var uploadService *strava.UploadsService
var athleteService *strava.CurrentAthleteService

func init() {
	flag.StringVar(&accessToken, "token", "", "user access_token from Strava")
	flag.Parse()

	if accessToken == "" {
		log.Println("\nPlease provide an access_token, one can be found at https://www.strava.com/settings/api")
		flag.Usage()
		os.Exit(1)
	}

	lastSync = time.Now().Add(-160 * 24 * time.Hour)
	client = strava.NewClient(accessToken)
	uploadService = strava.NewUploadsService(client)
	athleteService = strava.NewCurrentAthleteService(client)
}

func main() {
	const DEVICE_NAME = "GARMIN"

	basePath := getDevicePath(DEVICE_NAME)
	if len(basePath) == 0 {
		log.Fatalf("Device %s not found", DEVICE_NAME)
	} else {
		activitiesPath := filepath.Join(basePath, "Garmin", "Activities")

		uploadedActivities := getUploadedActivities(lastSync)
		activityFiles := getActivityFiles(activitiesPath, lastSync)

		for _, file := range activityFiles {
			found := false
			for _, activity := range uploadedActivities {
				if activity == file.Name() {
					found = true
				}
			}
			if !found {
				uploadData(activitiesPath, file)
			}
		}
	}
}

func getActivityFiles(path string, lastSync time.Time) []os.FileInfo {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal("Activity files cannot be found", err)
	}
	var fileInfos []os.FileInfo
	for _, fileInfo := range files {
		strDate := fileInfo.Name()[:len(fileInfo.Name())-4]
		date, err := time.Parse("2006-01-02-15-04-05", strDate)
		if err != nil {
			log.Fatal(err)
		}
		if date.After(lastSync) {
			fileInfos = append(fileInfos, fileInfo)
		}
	}
	return fileInfos
}

func getUploadedActivities(lastSync time.Time) []string {
	activities, err := athleteService.ListActivities().After(int(lastSync.Unix())).Do()
	log.Printf("Found %d uploaded activities after %s \n", len(activities), lastSync)
	if err != nil {
		log.Fatal(err)
	}
	activityNames := make([]string, len(activities))
	for i := 0; i < len(activities); i++ {
		activityNames[i] = activities[i].ExternalId
	}
	return activityNames
}

func uploadData(basePath string, file os.FileInfo) {
	log.Printf("Uploading file %s \n", file.Name())

	filePath := filepath.Join(basePath, file.Name())
	reader, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Failed to read file "+filePath, err)
	}
	upload, err := uploadService.
		Create(strava.FileDataTypes.FIT, file.Name(), reader).
		Private().
		Do()
	if err != nil {
		if e, ok := err.(strava.Error); ok && e.Message == "Authorization Error" {
			log.Printf("Make sure your token has 'write' permissions. You'll need implement the oauth process to get one")
		}

		log.Fatal("Error sending file ", err)
	}

	log.Printf("Upload Complete...")
	jsonForDisplay, _ := json.Marshal(upload)
	log.Printf(string(jsonForDisplay))

	log.Printf("Waiting a 5 seconds so the upload will finish (might not)")
	time.Sleep(5 * time.Second)

	uploadSummary, err := uploadService.Get(upload.Id).Do()
	jsonForDisplay, _ = json.Marshal(uploadSummary)
	log.Printf(string(jsonForDisplay))

	log.Printf("Your new activity is id %d", uploadSummary.ActivityId)
	log.Printf("You can view it at http://www.strava.com/activities/%d", uploadSummary.ActivityId)
}

func getDevicePath(name string) string {
	var err error
	var bytes []byte

	if bytes, err = ioutil.ReadFile("/proc/mounts"); err != nil {
		log.Fatal(err)
	}

	data := string(bytes)

	var path string
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		tokens := strings.Split(line, " ")
		if len(tokens) > 1 && strings.Contains(tokens[1], name) {
			path = tokens[1]
		}
	}

	return path
}

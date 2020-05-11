package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/taliesin-insa/lib-auth"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

//////////////////// CONSTS ////////////////////
var DatabaseAPI string
var DatabasePassword string
var FileServerURL string

const (
	LaiaDaemonAPI string = "http://raoh.educ.insa:12191"

	NbOfImagesToSend int = 25

	RecoAnnotatorId string = "$taliesin_recognizer"
)

//////////////////// STRUCTS ////////////////////

// DATABASE STRUCTS

/* PiFF file representation */
type Meta struct {
	Type string
	URL  string
}

type Location struct {
	Type    string
	Polygon [][2]int
	Id      string
}

type Data struct {
	Type       string
	LocationId string
	Value      string
	Id         string
}

type PiFFStruct struct {
	Meta     Meta
	Location []Location
	Data     []Data
	Children []int
	Parent   int
}

/* Struct of a database entry */
type Picture struct {
	// Id in db
	Id []byte `json:"Id"`
	// Piff
	PiFF     PiFFStruct `json:"PiFF"`
	Url      string     `json:"Url"`      //The URL on our fileserver
	Filename string     `json:"Filename"` //The original name of the file
	// Flags
	Annotated  bool `json:"Annotated"`
	Corrected  bool `json:"Corrected"`
	SentToReco bool `json:"SentToReco"`
	Unreadable bool `json:"Unreadable"`
	//
	Annotator string `json:"Annotator"`
}

// LAIA DAEMON STRUCT

/* Images sent to recognizer */
type LineImg struct {
	Id  []byte
	Url string
}

//////////////////// INTERMEDIATE REQUESTS ////////////////////

func checkPermission(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("ReqFromCron") != "" { // request from cron
		log.Printf("[INFO] Request from CRON")

		if r.Header.Get("Authorization") == DatabasePassword {
			return true
		} else {
			log.Printf("[DEBUG] CRON with bad header, received [%v], expected [%v]", r.Header.Get("Authorization"), DatabasePassword)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("[MICRO-RECO] Incorrect authorization header received from cron"))
			return false
		}
	} else { // request from GUI
		log.Printf("[INFO] Request from GUI")

		user, err, authStatusCode := lib_auth.AuthenticateUser(r)

		// check if there was an error during the authentication or if the user wasn't authenticated
		if err != nil {
			log.Printf("[ERROR] Check authentication: %v", err.Error())
			w.WriteHeader(authStatusCode)
			w.Write([]byte("[MICRO-RECO] Couldn't verify identity"))
			return false
		}

		// check if the authenticated user has sufficient permissions to call this endpoint
		if user.Role != lib_auth.RoleAdmin {
			log.Printf("[ERROR] Insufficient permission: want %v, was %v", lib_auth.RoleAdmin, user.Role)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("[MICRO-RECO] Insufficient permissions to call Laia"))
			return false
		}
		return true
	}
}

/* Request to retrieve a given number of pictures from the database */
func getPictures(client *http.Client) ([]Picture, error) {

	request, err := http.NewRequest(http.MethodGet, DatabaseAPI+"/db/retrieve/recognizer/"+strconv.Itoa(NbOfImagesToSend), nil)
	if err != nil {
		log.Printf("[ERROR] Create GET request to DB: %v", err.Error())
		return nil, err
	}

	request.Header.Set("Authorization", DatabasePassword)

	response, err := client.Do(request)
	if err != nil {
		log.Printf("[ERROR] Execute GET request to DB: %v", err.Error())
		return nil, err
	}

	// check that received body isn't empty
	if response.Body == nil {
		log.Printf("[ERROR] Database: received body is empty")
		return nil, err
	}

	// check whether there was an error during request
	if response.StatusCode != http.StatusOK {
		var body, _ = ioutil.ReadAll(response.Body)
		log.Printf("[ERROR] Error during GET request to DB: %d, %v", response.StatusCode, string(body))
		return nil, errors.New("bad status")
	}

	// get body of returned data
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("[ERROR] Couldn't read received data: %v", err.Error())
		return nil, err
	}

	// transform json into struct
	var pictures []Picture
	err = json.Unmarshal(body, &pictures)
	if err != nil {
		log.Printf("[ERROR] Database: unmarshal data: %v", err.Error())
		return nil, err
	}

	return pictures, nil
}

/* Request that sends images to the recognizer and gets in return a suggestion of transcription for each image */
func getSuggestionsFromReco(lineImgs []LineImg, client *http.Client) (io.ReadCloser, error) {
	// transform the request body into JSON
	reqBodyJSON, err := json.Marshal(lineImgs)
	if err != nil {
		log.Printf("[ERROR] Fail marshalling request body to JSON:\n%v", err.Error())
		return nil, err
	}

	// create and send request to recognizer
	request, err := http.NewRequest(http.MethodGet, LaiaDaemonAPI+"/laiaDaemon/recognizeImgs", bytes.NewBuffer(reqBodyJSON))
	if err != nil {
		log.Printf("[ERROR] Create GET request to recognizer: %v", err.Error())
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		log.Printf("[ERROR] Execute GET request to recognizer: %v", err.Error())
		return nil, err
	}

	// check that received body isn't empty
	if response.Body == nil {
		log.Printf("[ERROR] Recognizer: received body is empty")
		return nil, err
	}

	// check whether there was an error during request
	if response.StatusCode != http.StatusOK {
		var body, _ = ioutil.ReadAll(response.Body)
		log.Printf("[ERROR] Error during GET request to recognizer: %v", string(body))
		return nil, errors.New("bad status")
	}

	return response.Body, nil
}

/* Request to send suggestions made by the recognizer to the database */
func updatePictures(reqBody io.ReadCloser, client *http.Client) error {

	// send recognizer's suggestions to database, identified with a unique annotator's id
	request, err := http.NewRequest(http.MethodPut, DatabaseAPI+"/db/update/value/"+RecoAnnotatorId, reqBody)
	if err != nil {
		log.Printf("[ERROR] Create PUT request to DB: %v", err.Error())
		return err
	}

	request.Header.Set("Authorization", DatabasePassword)

	response, err := client.Do(request)
	if err != nil {
		log.Printf("[ERROR] Execute PUT request to DB: %v", err.Error())
		return err
	}

	// check whether there was an error during requestGetPictures
	if response.StatusCode != http.StatusNoContent {
		var body, _ = ioutil.ReadAll(response.Body)
		if response.StatusCode < 200 || response.StatusCode > 300 {
			log.Printf("[ERROR] Error during PUT request to DB: %d, %v", response.StatusCode, string(body))
			return errors.New("bad status")
		} else {
			log.Printf("[WARNING] Minor error during PUT request to DB, status received= %d, expected=204, body=%v", response.StatusCode, string(body))
		}
	}

	return nil
}

//////////////////// API FUNCTIONS ////////////////////
func home(w http.ResponseWriter, r *http.Request) {
	log.Printf("HomeLink joined")
	fmt.Fprint(w, "[MICRO-RECOGNIZER] HomeLink joined")
}

func sendImgsToRecognizer(w http.ResponseWriter, r *http.Request) {

	log.Printf("[INFO] sendImgs joined")

	hasPermission := checkPermission(w, r)

	if !hasPermission {
		log.Printf("[INFO] Received request didn't have sufficient permissions")
		return
	} else {
		// we send a response directly, to avoid blocking the caller while we annotate images with the recognizer
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("[MICRO-RECOGNIZER] Request accepted"))

		client := &http.Client{}

		// we repeat the operation until there isn't anymore images to translate with the recognizer

		var receivedPictures = NbOfImagesToSend
		var count = 1
		// golang version of a while
		for receivedPictures == NbOfImagesToSend {
			log.Printf("[INFO] ===== Turn %d =====", count)

			pictures, err := getPictures(client)
			if err != nil {
				return
			}

			log.Printf("[INFO] Pictures received")

			receivedPictures = len(pictures)
			if receivedPictures == 0 {
				log.Printf("[INFO] No more images to send to recognizer (0 received)\nsendImgs finished")
				return
			}

			// create body to send to recognizer
			var lineImgs []LineImg
			for _, picture := range pictures {
				lineImgs = append(lineImgs, LineImg{
					Id:  picture.Id,
					Url: FileServerURL + picture.Url,
				})
			}

			resBody, err := getSuggestionsFromReco(lineImgs, client)
			if err != nil {
				return
			}

			log.Printf("[INFO] Suggestions received")

			err = updatePictures(resBody, client)
			if err != nil {
				return
			}

			log.Printf("[INFO] Pictures updated")
			count++
		}

		log.Printf("[INFO] sendImgs finished")
		return
	}
}

//////////////////// MAIN ////////////////////
func main() {
	// get environment variables
	dbEnvVal, dbEnvExists := os.LookupEnv("DATABASE_API_URL")

	if dbEnvExists {
		DatabaseAPI = dbEnvVal
	} else {
		DatabaseAPI = "http://database-api.gitlab-managed-apps.svc.cluster.local:8080"
	}

	DatabasePassword = os.Getenv("CLUSTER_INTERNAL_PASSWORD")

	fileServerEnvVal, fileServerEnvExists := os.LookupEnv("FILESERVER_URL")

	if fileServerEnvExists {
		FileServerURL = fileServerEnvVal
	} else {
		FileServerURL = "https://inky.local:9501"
	}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/recognizer", home)

	router.HandleFunc("/recognizer/sendImgs", sendImgsToRecognizer).Methods("POST")

	log.Fatal(http.ListenAndServe(":8080", router))

}

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

//////////////////// CONSTS ////////////////////
var DatabaseAPI string

const (
	LaiaDaemonAPI string = "http://raoh.fr:12191"

	NbOfImagesToSend int = 200

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
	Id  string
	Url string
}

type ValueUpdate struct {
	Id    string
	Value string
}

//////////////////// INTERMEDIATE REQUESTS ////////////////////

/* Request to retrieve a given number of pictures from the database */
func getPictures(client *http.Client) ([]Picture, error) {

	request, err := http.NewRequest(http.MethodGet, DatabaseAPI+"/db/retrieve/snippets/"+strconv.Itoa(NbOfImagesToSend), nil)
	if err != nil {
		log.Printf("[ERROR] Create GET request to DB: %v", err.Error())
		return nil, err
	}

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
func getSuggestionsFromReco(lineImgs []LineImg) ([]byte, error) {

	var valuesUpdate []ValueUpdate

	for _, element := range lineImgs {
		valuesUpdate = append(valuesUpdate, ValueUpdate{
			Id:    element.Id,
			Value: "MOCK text from reco",
		})
	}

	log.Printf("[INFO] returned data: %v", &valuesUpdate)

	// transform the request body into JSON
	reqBodyJSON, err := json.Marshal(valuesUpdate)
	if err != nil {
		log.Printf("[ERROR] Fail marshalling request body to JSON:\n%v", err.Error())
		return nil, err
	}

	return reqBodyJSON, nil
}

/* Request to send suggestions made by the recognizer to the database */
func updatePictures(reqBody []byte, client *http.Client) error {

	// send recognizer's suggestions to database, identified with a unique annotator's id
	request, err := http.NewRequest(http.MethodPut, DatabaseAPI+"/db/update/value/"+RecoAnnotatorId, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("[ERROR] Create PUT request to DB: %v", err.Error())
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		log.Printf("[ERROR] Execute PUT request to DB: %v", err.Error())
		return err
	}

	// check whether there was an error during requestGetPictures
	if response.StatusCode != http.StatusOK {
		var body, _ = ioutil.ReadAll(response.Body)
		log.Printf("[ERROR] Error during PUT request to DB: %v", string(body))
		return errors.New("bad status")
	}

	return nil
}

//////////////////// API FUNCTIONS ////////////////////
func home(w http.ResponseWriter, r *http.Request) {
	log.Printf("HomeLink joined")
	fmt.Fprint(w, "[MICRO-RECOGNIZER] HomeLink joined")
}

func sendImgsToRecognizer(w http.ResponseWriter, r *http.Request) {

	log.Printf("sendImgs joined")

	// we send a response directly, to avoid blocking the caller while we annotate images with the recognizer
	w.WriteHeader(http.StatusAccepted)

	client := &http.Client{}

	// we repeat the operation until there isn't anymore images to translate with the recognizer

	var receivedPictures = 200
	// golang version of a while
	for receivedPictures == 200 {
		pictures, err := getPictures(client)
		if err != nil {
			return
		}

		receivedPictures = len(pictures)
		if receivedPictures == 0 {
			return
		}

		// create body to send to recognizer
		var lineImgs []LineImg
		for _, picture := range pictures {
			lineImgs = append(lineImgs, LineImg{
				Id:  string(picture.Id),
				Url: picture.Url,
			})
		}

		resBody, err := getSuggestionsFromReco(lineImgs)
		if err != nil {
			return
		}

		err = updatePictures(resBody, client)
		if err != nil {
			return
		}
	}

	log.Printf("sendImgs finished")

}

//////////////////// MAIN ////////////////////
func main() {
	dbEnvVal, dbEnvExists := os.LookupEnv("DATABASE_API_URL")

	if dbEnvExists {
		DatabaseAPI = dbEnvVal
	} else {
		DatabaseAPI = "http://database-api.gitlab-managed-apps.svc.cluster.local:8080"
	}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/recognizer", home)

	router.HandleFunc("/recognizer/sendImgs", sendImgsToRecognizer).Methods("POST")

	log.Fatal(http.ListenAndServe(":8080", router))

}

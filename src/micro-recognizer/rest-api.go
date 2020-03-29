package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

//////////////////// CONSTS ////////////////////
var DatabaseAPI string

const LaiaDaemonAPI string = "http://raoh.fr:12191"

const NbOfImagesToSend int = 200

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
	PiFF PiFFStruct `json:"PiFF"`
	// Url in fileserver
	Url string `json:"Url"`
	// Flags
	Annotated  bool `json:"Annotated"`
	Corrected  bool `json:"Corrected"`
	SentToReco bool `json:"SentToReco"`
	Unreadable bool `json:"Unreadable"`
}

// LAIA DAEMON STRUCTS

/* Images sent to recognizer */
type LineImg struct {
	Id  string
	Url string
}

//////////////////// INTERMEDIATE REQUESTS ////////////////////

/* Request to retrieve a given number of pictures from the database */
func getPictures(client *http.Client, w http.ResponseWriter) ([]Picture, error) {

	request, err := http.NewRequest(http.MethodGet, DatabaseAPI+"/db/retrieve/snippets/"+string(NbOfImagesToSend), nil)
	if err != nil {
		log.Printf("[ERROR] Get request to DB: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't make GET request to database"))
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		log.Printf("[ERROR] Error executing GET request to DB: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't retrieve images from database"))
		return nil, err
	}

	// check that received body isn't empty
	if response.Body == nil {
		log.Printf("[ERROR] Database: received body is empty")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Empty data received from database"))
		return nil, err
	}

	// check whether there was an error during request
	if response.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Error during GET request to DB: %v", response.Body)
		w.WriteHeader(response.StatusCode)
		w.Write([]byte("[MICRO-RECOGNIZER] Error while contacting database"))
		return nil, errors.New("bad status")
	}

	// get body of returned data
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("[ERROR] Read data: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't read data from database"))
		return nil, err
	}

	// transform json into struct
	var pictures []Picture
	err = json.Unmarshal(body, &pictures)
	if err != nil {
		log.Printf("[ERROR] Database: unmarshal data: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't unmarshal data received from database"))
		return nil, err
	}

	return pictures, nil
}

/* Request that sends images to the recognizer and gets in return a suggestion of transcription for each image */
func getSuggestionsFromReco(lineImgs []LineImg, client *http.Client, w http.ResponseWriter) (io.ReadCloser, error) {
	// transform the request body into JSON
	reqBodyJSON, err := json.Marshal(lineImgs)
	if err != nil {
		log.Printf("[ERROR] Fail marshalling request body to JSON:\n%v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error formatting request for recognizer"))
		return nil, err
	}

	// create and send request to recognizer
	request, err := http.NewRequest(http.MethodGet, LaiaDaemonAPI+"/laiaDaemon/recognizeImgs", bytes.NewBuffer(reqBodyJSON))
	if err != nil {
		log.Printf("[ERROR] Get request: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't make GET request to recognizer"))
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		log.Printf("[ERROR] Error executing GET request to recognizer: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-EXPORT] Couldn't contact the recognizer"))
		return nil, err
	}

	// check that received body isn't empty
	if response.Body == nil {
		log.Printf("[ERROR] Recognizer: received body is empty")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Empty data received from recognizer"))
		return nil, err
	}

	// check whether there was an error during request
	if response.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Error during GET request to recognizer: %v", response.Body)
		w.WriteHeader(response.StatusCode)
		w.Write([]byte("[MICRO-RECOGNIZER] Error while contacting recognizer"))
		return nil, errors.New("bad status")
	}

	return response.Body, nil
}

/* Request to send suggestions made by the recognizer to the database */
func updatePictures(reqBody io.ReadCloser, client *http.Client, w http.ResponseWriter) error {
	// send recognizer's suggestions to database
	request, err := http.NewRequest(http.MethodPut, DatabaseAPI+"/db/update/value/", reqBody)
	if err != nil {
		log.Printf("[ERROR] PUT request to DB: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't make PUT request to database"))
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		log.Printf("[ERROR] Error executing PUT request to DB: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't send suggestions to database"))
		return err
	}

	// check whether there was an error during requestGetPictures
	if response.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Error during PUT request to DB: %v", response.Body)
		w.WriteHeader(response.StatusCode)
		w.Write([]byte("[MICRO-RECOGNIZER] Error while contacting database"))
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

	client := &http.Client{}

	pictures, err := getPictures(client, w)
	if err != nil {
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

	resBody, err := getSuggestionsFromReco(lineImgs, client, w)
	if err != nil {
		return
	}

	err = updatePictures(resBody, client, w)
	if err != nil {
		return
	}

	// everything went fine, we send back a response
	w.WriteHeader(http.StatusOK)
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

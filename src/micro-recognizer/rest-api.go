package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
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

/* Parse PiFF files */
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

/* Our working structure */
type Picture struct {
	// Id in db
	Id []byte `json:"Id"`
	// Piff
	PiFF PiFFStruct `json:"PiFF"`
	// Url fileserver
	Url string `json:"Url"`
	// Flags
	Annotated  bool `json:"Annotated"`
	Corrected  bool `json:"Corrected"`
	SentToReco bool `json:"SentToReco"`
	Unreadable bool `json:"Unreadable"`
}

// LAIA DAEMON STRUCTS

/* Request sent */
type LineImg struct {
	Url string
	Id  string
}

type ReqBody struct {
	Images []LineImg
}

/* Response received */
type ImgValue struct {
	Id    string
	Value string
}

type ReqRes struct {
	Images []ImgValue
}

//////////////////// INTERMEDIATE REQUESTS ////////////////////

//////////////////// API FUNCTIONS ////////////////////
func home(w http.ResponseWriter, r *http.Request) {
	log.Printf("HomeLink joined")
	fmt.Fprint(w, "[MICRO-RECOGNIZER] HomeLink joined")
}

func sendImgsToRecognizer(w http.ResponseWriter, r *http.Request) {
	// get Pictures to send to recognizer from database
	client := &http.Client{}
	requestGetPictures, err := http.NewRequest(http.MethodGet, DatabaseAPI+"/db/retrieve/snippets/"+string(NbOfImagesToSend), nil)
	if err != nil {
		log.Printf("[ERROR] Get request to DB: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't make GET request to database"))
		return
	}

	responseGetPictures, err := client.Do(requestGetPictures)
	if err != nil {
		log.Printf("[ERROR] Error executing GET request to DB: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't retrieve images from database"))
		return
	}

	// check that received body isn't empty
	if responseGetPictures.Body == nil {
		log.Printf("[ERROR] Database: received body is empty")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Empty data received from database"))
		return
	}

	// check whether there was an error during requestGetPictures
	if responseGetPictures.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Error during GET request to DB: %v", responseGetPictures.Body)
		w.WriteHeader(responseGetPictures.StatusCode)
		w.Write([]byte("[MICRO-RECOGNIZER] Error while contacting database"))
		return
	}

	// get body of returned data
	body, err := ioutil.ReadAll(responseGetPictures.Body)
	if err != nil {
		log.Printf("[ERROR] Read data: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't read data from database"))
		return
	}

	// transform json into struct
	var pictures []Picture
	err = json.Unmarshal(body, &pictures)
	if err != nil {
		log.Printf("[ERROR] Database: unmarshal data: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't unmarshal data received from database"))
		return
	}

	// create requestGetPictures body
	var reqBodyReco ReqBody
	for _, picture := range pictures {
		reqBodyReco.Images = append(reqBodyReco.Images, LineImg{
			Url: picture.Url,
			Id:  string(picture.Id),
		})
	}

	// transform the request body into JSON
	reqBodyRecoJSON, err := json.Marshal(reqBodyReco)
	if err != nil {
		log.Printf("[ERROR] Fail marshalling request body to JSON:\n%v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error formatting request to recognizer"))
		return
	}

	// create and send request to recognizer
	requestReco, err := http.NewRequest(http.MethodGet, LaiaDaemonAPI+"/laiaDaemon/recognizeImgs", bytes.NewBuffer(reqBodyRecoJSON))
	if err != nil {
		log.Printf("[ERROR] Get requestReco: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't make GET request to recognizer"))
		return
	}

	responseReco, err := client.Do(requestReco)
	if err != nil {
		log.Printf("[ERROR] Error executing GET request to recognizer: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-EXPORT] Couldn't contact the recognizer"))
		return
	}

	// check that received body isn't empty
	if responseReco.Body == nil {
		log.Printf("[ERROR] Recognizer: received body is empty")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Empty data received from recognizer"))
		return
	}

	// check whether there was an error during request
	if responseReco.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Error during GET requestGetPictures to DB: %v", responseReco.Body)
		w.WriteHeader(responseGetPictures.StatusCode)
		w.Write([]byte("[MICRO-RECOGNIZER] Error while contacting database"))
		return
	}

	// send recognizer's suggestions to database
	requestUpdatePictures, err := http.NewRequest(http.MethodPut, DatabaseAPI+"/db/update/value/", responseReco.Body)
	if err != nil {
		log.Printf("[ERROR] PUT request to DB: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't make PUT request to database"))
		return
	}

	responseUpdatePictures, err := client.Do(requestUpdatePictures)
	if err != nil {
		log.Printf("[ERROR] Error executing PUT request to DB: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[MICRO-RECOGNIZER] Couldn't send suggestions to database"))
		return
	}

	// check whether there was an error during requestGetPictures
	if responseUpdatePictures.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Error during PUT request to DB: %v", responseUpdatePictures.Body)
		w.WriteHeader(responseUpdatePictures.StatusCode)
		w.Write([]byte("[MICRO-RECOGNIZER] Error while contacting database"))
		return
	}

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

package main

import (
	"cloud.google.com/go/compute/metadata"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	port     = os.Getenv("PORT")
	version  = os.Getenv("VERSION")
	kind     = os.Getenv("KIND")
	backendA = os.Getenv("BE_A")
	backendB = os.Getenv("BE_B")
	backendC = os.Getenv("BE_C")
)

type commonResponse struct {
	Kind     string `json:"kind"`    // backend, backend-b, backend-c
	Version  string `json:"version"` // v1, v2, v3
	Region   string `json:"region"`
	Cluster  string `json:"cluster"`
	Hostname string `json:"hostname"`
}

type bffResponse struct {
	Bff      commonResponse `json:"bff"`
	BackendA commonResponse `json:"backend_a"`
	BackendB commonResponse `json:"backend_b"`
	BackendC commonResponse `json:"backend_c"`
}

func resolveRegion() string {
	if !metadata.OnGCE() {
		log.Println("This app is not running on GCE")
	} else {
		zone, err := metadata.Zone()
		if err != nil {
			log.Printf("could not get zone info: %v", err)
			return "unknown"
		}
		region := zone[:strings.LastIndex(zone, "-")]
		return region
	}
	return "unknown"
}

func resolveCluster() string {
	if !metadata.OnGCE() {
		log.Println("This app is not running on GCE")
	} else {
		cluster, err := metadata.Get("/instance/attributes/cluster-name")
		if err != nil {
			log.Printf("could not get cluster name: %v", err)
			return "unknown"
		}
		return cluster
	}
	return "unknown"
}

func resolveHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("could not get hostname: %v", err)
		return "unknown"
	}
	return hostname
}

func fetchBackend(target string, path string) *commonResponse {
	var backendRes commonResponse
	client := &http.Client{}
	client.Timeout = time.Second * 2
	req, err := http.NewRequest("GET", target+path, nil)
	if err != nil {
		log.Printf("could not make a new request: %v", err)
		return &backendRes
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("could not feach backend: %v", err)
		return &backendRes
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("could not read response body: %v", err)
	}
	if err := json.Unmarshal(body, &backendRes); err != nil {
		log.Printf("could not json.Unmarshal: %v", err)
	}
	return &backendRes
}

func fetchRootResponse(w http.ResponseWriter, r *http.Request) {
	responseBody, err := json.Marshal(&commonResponse{
		Version:  version,
		Kind:     kind,
		Region:   resolveRegion(),
		Cluster:  resolveCluster(),
		Hostname: resolveHostname(),
	})
	if err != nil {
		log.Printf("could not json.Marshal: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.Write(responseBody)
}

func fetchBffResponse(w http.ResponseWriter, r *http.Request) {
	bffInfo := commonResponse{
		Kind:     kind,
		Version:  version,
		Region:   resolveRegion(),
		Cluster:  resolveCluster(),
		Hostname: resolveHostname(),
	}

	backendARes := fetchBackend(backendA, "")
	backendBRes := fetchBackend(backendB, "")
	backendCRes := fetchBackend(backendC, "")

	res := bffResponse{
		Bff:      bffInfo,
		BackendA: *backendARes,
		BackendB: *backendBRes,
		BackendC: *backendCRes,
	}

	responseBody, err := json.Marshal(res)
	if err != nil {
		log.Printf("could not json.Marshal: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBody)
}

func main() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", fetchRootResponse).Methods("GET")
	router.HandleFunc("/bff", fetchBffResponse).Methods("GET")
	err := http.ListenAndServe(":"+port, router)
	if err != nil {
		log.Fatal("ListenAndServer: ", err)
	}
}

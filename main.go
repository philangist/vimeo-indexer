package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	// "log"
	"io"
	"net/http"
	// _ "net/http/pprof"
	"os"
	// "runtime/pprof"
	"strings"
	"sync"
	"time"
)

const (
	MEMPROF = "perf/memprof-%s"
	CPUPROF = "perf/cpuprof-%s"
)

var (
	TOTAL_REQUESTS = 0
	STATUS_CODES = []int{}
)

type User struct {
	ID int `json:"id"`
	FullName string `json:"fullName"`
	Email string `json:"email"`
	Country string `json:"country"`
	Language string `json:"language"`
	LastIP string `json:"lastIp"`
}

type UserResponse struct {
	Data User `json:"data"`
}

type Video struct {
	ID int `json:"id"`
	Title string `json:"title"`
	Caption string `json:"caption"`
	Privacy string `json:"privacy"`
	FrameRate string `json:"frameRate"`
	VideoCodec string `json:"videoCodec"`
	AudioCodec string `json:"audioCodec"`
	AudioSampleRate string `json:"audioSampleRate"`
}

type VideoResponse struct {
	Data Video `json:"data"`
}

type Index struct {
	User User `json:"user"`
	Video Video `json:"video"`
}

func main(){
	start := time.Now()
	httpClient := &http.Client{Timeout: time.Second * 10}
	inputStream := make(chan [2]string, 1)
	wg := &sync.WaitGroup{}

	validateLine := func(line [2]string) (string, string, bool) {
		if line == [2]string{}{
			return "", "", false
		}
		return line[0], line[1], true
	}

	handler := func(wg *sync.WaitGroup){
		for {
			select{
			case line := <- inputStream:
				userID, videoID, valid := validateLine(line)
				if !valid {
					// fmt.Printf("Line: %v is invalid. Skipping...\n", line)
					continue
				}
				_, err := fetchUserVideoData(userID, videoID, httpClient)
				if err != nil {
					wg.Add(1)
					go func(){
						inputStream <- line
						wg.Done()
					}()
				}
			case <- time.After(5 * time.Second):
				wg.Done()
			}
		}
	}

	for i := 0; i < 1; i++ {
		wg.Add(1)
		go handler(wg)
	}
	parseCSVStream(bufio.NewScanner(os.Stdin), inputStream)
	func(){
		wg.Wait()
		defer close(inputStream)
	}()

	fmt.Println("Elapsed time was: ", time.Since(start))
	fmt.Println("TOTAL_REQUESTS is: ", TOTAL_REQUESTS)
	fmt.Println("len(STATUS_CODES) is: ", len(STATUS_CODES))
	fmt.Println("STATUS_CODES is: ", STATUS_CODES)
}


func parseCSVStream(scanner *bufio.Scanner, inputStream chan [2]string) {
        for scanner.Scan(){
 		line := strings.Split(scanner.Text(), ",")

		inputStream <- [2]string{line[0], line[1]}
		// fmt.Printf("Sending line %v to inputStream\n", line)
	}
}

func fetchUserVideoData(userID, videoID string, httpClient *http.Client) (userIndex Index, err error) {
	var userResponse UserResponse
	var videoResponse VideoResponse

	// fmt.Printf("fetching userID: %s and videoID: %s\n", userID, videoID)

	err = getUserData(httpClient, userID, &userResponse)
	if err == nil {
		userIndex.User = userResponse.Data
	} else {
		// fmt.Printf(
		// 	"Get user failed for userID %s\n", userID)
		return userIndex, err
	}

	err = getVideoData(httpClient, videoID, &videoResponse)
	if err == nil {
		userIndex.Video = videoResponse.Data
	} else {
		// fmt.Printf(
		// 	"Get video failed for userID %s\n", userID)
		return userIndex, err
	}

	err = postIndexData(httpClient, userIndex)
	return userIndex, err
}

func postIndexData(httpClient *http.Client, userIndex Index) error {
	indexURL := "http://localhost:8002/index"

	serializedUserIndex, err := json.Marshal(userIndex)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", indexURL, bytes.NewBuffer(serializedUserIndex))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf(
			"Index service returned unexpected status code %d ", response.StatusCode)
	}

	response.Body.Close()

	TOTAL_REQUESTS += 1
	STATUS_CODES = append(STATUS_CODES, response.StatusCode)
	return nil
}

func getUserData(httpClient *http.Client, userID string, userResponse *UserResponse) error {
	userURL := fmt.Sprintf("http://localhost:8000/users/%s", userID)

	// fmt.Println("Requesting URL", userURL)
	request, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		// fmt.Printf("Error %s preparing request for URL %s\n", err, userURL)
		return err
	}

	// request.Header.Add("Accept-Encoding", "json")
	response, err := httpClient.Do(request)
	if err != nil {
		// fmt.Printf("Error %s requesting URL %s\n", err, userURL)
		return err
	}

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			return err
		}
		defer reader.Close()
	default:
		reader = response.Body
	}

	err = json.NewDecoder(reader).Decode(userResponse)
	if err != nil {
		// fmt.Printf("Error %s decoding %v for %s\n", err, reader, userURL)
		return err
	}
	return nil
}

func getVideoData(httpClient *http.Client, videoID string, videoResponse *VideoResponse) error {
	videoURL := fmt.Sprintf("http://localhost:8001/videos/%s", videoID)

	request, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		return err
	}

	// request.Header.Add("Accept-Encoding", "json")
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			return err
		}
		defer reader.Close()
	default:
		reader = response.Body
	}

	err = json.NewDecoder(reader).Decode(&videoResponse)
	if err != nil {
		// fmt.Printf("Error %s decoding %v for %s\n", err, reader, videoURL)
		return err
	}
	return nil
}

package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	wg := &sync.WaitGroup{}
	inputStream := make(chan [2]string, 1)
	defer close(inputStream)

	handler := func(wg *sync.WaitGroup){
		for {
			select{
			case line := <- inputStream:
				userID, videoID := line[0], line[1]
				_, err := fetchUserVideoData(userID, videoID, httpClient)
				if err != nil {
					wg.Add(1)
					go func(){
						inputStream <- line
						wg.Done()
					}()
				}
			case <- time.After(5 * time.Second): // TIMEOUT should be an ENV VAR
				wg.Done()
			}
		}
	}

	for i := 0; i < 1; i++ {  // NUM_THREADS should be an ENV VAR
		wg.Add(1)
		go handler(wg)
	}
	parseCSVStream(bufio.NewScanner(os.Stdin), inputStream)
	wg.Wait()
	fmt.Println("Elapsed time was: ", time.Since(start))
}

func parseCSVStream(scanner *bufio.Scanner, inputStream chan [2]string) {
        for scanner.Scan(){
 		line := strings.Split(scanner.Text(), ",")
		valid := validateCSVLine(line)
		if !valid {
			continue
		}
		inputStream <- [2]string{line[0], line[1]}
	}
}

func validateCSVLine(line []string) bool {
	if line == nil {
		return false
	}
	return true
}

func fetchUserVideoData(userID, videoID string, httpClient *http.Client) (userIndex Index, err error) {
	var userResponse UserResponse
	var videoResponse VideoResponse

	err = getUserData(httpClient, userID, &userResponse)
	if err == nil {
		userIndex.User = userResponse.Data
	} else {
		return userIndex, err
	}

	err = getVideoData(httpClient, videoID, &videoResponse)
	if err == nil {
		userIndex.Video = videoResponse.Data
	} else {
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
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		// fmt.Printf(
		//	"Index service returned unexpected status code %d for userID %d and videoID %d\n",
		//	response.StatusCode, userIndex.User.ID, userIndex.Video.ID)
		return fmt.Errorf(
			"Index service returned unexpected status code %d", response.StatusCode)
	}

	return nil
}

func getUserData(httpClient *http.Client, userID string, userResponse *UserResponse) error {
	userURL := fmt.Sprintf("http://localhost:8000/users/%s", userID)

	request, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		return err
	}

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

	err = json.NewDecoder(reader).Decode(userResponse)
	if err != nil {
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
		return err
	}
	return nil
}

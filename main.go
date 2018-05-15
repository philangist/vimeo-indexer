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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MEMPROF = "perf/memprof-%s"
	CPUPROF = "perf/cpuprof-%s"
	USERS_URL = "http://localhost:8000/users/"
	VIDEOS_URL = "http://localhost:8001/videos/"
	INDEX_URL = "http://localhost:8002/index"
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
				_, err := FetchUserVideoData(userID, videoID, httpClient)
				if err != nil {
					wg.Add(1)
					go func(){
						inputStream <- line
						wg.Done()
					}()
				}
			case <- time.After(1 * time.Second): // TIMEOUT should be an ENV VAR
				wg.Done()
			}
		}
	}

	for i := 0; i < 1; i++ {  // NUM_THREADS should be an ENV VAR
		wg.Add(1)
		go handler(wg)
	}
	ParseCSVStream(bufio.NewScanner(os.Stdin), inputStream)
	wg.Wait()
	fmt.Println("Elapsed time was: ", time.Since(start))
}

func ParseCSVStream(scanner *bufio.Scanner, inputStream chan [2]string) {
        for scanner.Scan(){
 		line := strings.Split(scanner.Text(), ",")
		valid := ValidateCSVLine(line)
		if !valid {
			continue
		}
		inputStream <- [2]string{line[0], line[1]}
	}
}

func ValidateCSVLine(line []string) bool {
	if line == nil {
		return false
	}
	if len(line) != 2 {
		return false
	}

	userID := strings.TrimSpace(line[0])
	videoID := strings.TrimSpace(line[1])

	if (len(userID) == 0 || len(videoID) == 0) {
		return false
	}

	_, err := strconv.ParseInt(userID, 10, 32)
	if err != nil {
		return false
	}

	_, err = strconv.ParseInt(videoID, 10, 32)
	if err != nil {
		return false
	}

	return true
}

func FetchUserVideoData(userID, videoID string, httpClient *http.Client) (userIndex Index, err error) {
	userResponse, err := GetUserData(httpClient, USERS_URL, userID)
	if err == nil {
		userIndex.User = userResponse.Data
	} else {
		return userIndex, err
	}

	videoResponse, err := GetVideoData(httpClient, VIDEOS_URL, videoID)
	if err == nil {
		userIndex.Video = videoResponse.Data
	} else {
		return userIndex, err
	}

	err = PostIndexData(httpClient, INDEX_URL, userIndex)
	return userIndex, err
}

func PostIndexData(httpClient *http.Client, indexURL string, userIndex Index) error {

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
		return fmt.Errorf(
			"Index service returned unexpected status code %d", response.StatusCode)
	}

	return nil
}

func GetUserData(httpClient *http.Client, usersUrl, userID string) (*UserResponse, error) {
	userResponse := &UserResponse{}
	usersURL := fmt.Sprintf("%s/%s", usersUrl, userID)

	request, err := http.NewRequest("GET", usersURL, nil)
	if err != nil {
		return userResponse, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return userResponse, err
	}
	if response.StatusCode != http.StatusOK {
		return userResponse, fmt.Errorf(
			"Users service returned unexpected status code %d", response.StatusCode)
	}

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			return userResponse, err
		}
		defer reader.Close()
	default:
		reader = response.Body
	}

	err = json.NewDecoder(reader).Decode(userResponse)
	if err != nil {
		return userResponse, err
	}
	return userResponse, nil
}

func GetVideoData(httpClient *http.Client, videosUrl, videoID string) (*VideoResponse, error) {
	// these urls are attributes on a container struct
	// GET, POST, ETC, are methods on it
	videoResponse := &VideoResponse{}
	videosURL := fmt.Sprintf("%s/%s", videosUrl, videoID)

	request, err := http.NewRequest("GET", videosURL, nil)
	if err != nil {
		return videoResponse, err
	}

	// request.Header.Add("Accept-Encoding", "json")
	response, err := httpClient.Do(request)
	if err != nil {
		return videoResponse, err
	}
	if response.StatusCode != http.StatusOK {
		return videoResponse, fmt.Errorf(
			"Videos service returned unexpected status code %d", response.StatusCode)
	}

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			return videoResponse, err
		}
		defer reader.Close()
	default:
		reader = response.Body
	}

	err = json.NewDecoder(reader).Decode(videoResponse)
	if err != nil {
		return videoResponse, err
	}
	return videoResponse, nil
}

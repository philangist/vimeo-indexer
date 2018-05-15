package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	// "runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	CPUPROF    = "perf/cpuprof-%s"  // add CPUPROF flag
	USERS_URL  = "http://localhost:8000/users"
	VIDEOS_URL = "http://localhost:8001/videos"
	INDEX_URL  = "http://localhost:8002/index"
)

type User struct {
	ID       int    `json:"id"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Country  string `json:"country"`
	Language string `json:"language"`
	LastIP   string `json:"lastIp"`
}

type UserResponse struct {
	Data User `json:"data"`
}

type Video struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Caption         string `json:"caption"`
	Privacy         string `json:"privacy"`
	FrameRate       string `json:"frameRate"`
	VideoCodec      string `json:"videoCodec"`
	AudioCodec      string `json:"audioCodec"`
	AudioSampleRate string `json:"audioSampleRate"`
}

type VideoResponse struct {
	Data Video `json:"data"`
}

// IndexRequest??
type IndexRequest struct {
	User  User  `json:"user"`
	Video Video `json:"video"`
}

type Config struct {
	UsersURL string
	VideosURL string
	IndexURL string
	Timeout time.Duration
	Threads int
}

func ReadConfigFromEnv() *Config {
	usersURL := os.Getenv("USERS_URL")
	if len(usersURL) == 0 {
		usersURL = USERS_URL
	}

	videosURL := os.Getenv("VIDEOS_URL")
	if len(videosURL) == 0 {
		videosURL = VIDEOS_URL
	}

	indexURL := os.Getenv("INDEX_URL")
	if len(indexURL) == 0 {
		indexURL = INDEX_URL
	}

	timeout, err := strconv.ParseInt(
		os.Getenv("TIMEOUT"), 10, 32)
	if err != nil {
		log.Panic("Invalid value set for TIMEOUT")
	}

	threads, err := strconv.ParseInt(
		os.Getenv("NUM_THREADS"), 10, 32)
	if err != nil {
		log.Panic("Invalid value set for NUM_THREADS")
	}

	return &Config{
		usersURL,
		videosURL,
		indexURL,
		time.Duration(timeout) * time.Second,
		int(threads),
	}
}

type IndexService struct {
	*Config
	Input       chan [2]string
	Timeout     chan bool
	Client      *http.Client
}

func NewIndexService(cfg *Config, client *http.Client) *IndexService {
	input := make(chan [2]string, 1)
	timeout := make(chan bool, 1)

	return &IndexService{
		cfg,
		input,
		timeout,
		client,
	}
}

func (service *IndexService) Execute(reader io.Reader) error {
	wg := &sync.WaitGroup{}
	multiplexer := func(wg *sync.WaitGroup) {
		for line := range service.Input {
			fmt.Println("read line", line)
			userID, videoID := line[0], line[1]
			err := service.IndexUserVideo(userID, videoID)
			if err == nil {
				service.Timeout <- true
				continue
			} else {
				wg.Add(1)
				go func() {
					service.Input <- line
					wg.Done()
				}()
			}
		}
	}

	timeout := func(wg *sync.WaitGroup) {
		for {
			select {
			case <- service.Timeout:
				// no-op
			case <- time.After(service.Config.Timeout):
				wg.Done()
			}
		}
	}

	wg.Add(1)
	go timeout(wg)
	for i := 0; i < service.Threads; i++ {
		go multiplexer(wg)
	}

	service.ParseCSVStream(bufio.NewScanner(reader))
	wg.Wait()
	return nil
}

func (service *IndexService) ParseCSVStream(scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := strings.Split(scanner.Text(), ",")
		valid := ValidateCSVLine(line)
		if !valid {
			continue
		}
		service.Input <- [2]string{line[0], line[1]}
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

	if len(userID) == 0 || len(videoID) == 0 {
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

func (service *IndexService) IndexUserVideo(userID, videoID string) error {
	var indexRequest IndexRequest

	fmt.Printf("IndexUserVideo.userID is %s\n", userID)
	fmt.Printf("IndexUserVideo.videoID is %s\n", videoID)

	userResponse, err := service.GetUser(userID)
	if err != nil {
		return err
	}

	videoResponse, err := service.GetVideo(videoID)
	if err != nil {
		return err
	}

	indexRequest.User = userResponse.Data
	indexRequest.Video = videoResponse.Data

	err = service.PostIndex(indexRequest)
	return err
}

func (service *IndexService) PostIndex (indexRequest IndexRequest) error {
	serializedIndexRequest, err := json.Marshal(indexRequest)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(
		"POST",
		service.IndexURL,
		bytes.NewBuffer(serializedIndexRequest),
	)  // why did i use bytes.newbuffer here?

	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := service.Client.Do(request)
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

func  (service *IndexService) GetUser(ID string) (*UserResponse, error){
	userResponse := &UserResponse{}
	usersURL := fmt.Sprintf("%s/%s", service.UsersURL, ID)

	b, err := service.JSONRequest(usersURL)
	if err != nil {
		return userResponse, err
	}

	json.Unmarshal(b, userResponse)
	return userResponse, nil
}

func (service *IndexService) GetVideo(ID string) (*VideoResponse, error){
	videoResponse := &VideoResponse{}
	videosURL := fmt.Sprintf("%s/%s", service.VideosURL, ID)

	b, err := service.JSONRequest(videosURL)
	if err != nil {
		return videoResponse, err
	}

	json.Unmarshal(b, videoResponse)
	return videoResponse, nil
}

func(service *IndexService) JSONRequest(URL string) ([]byte, error){
	var byteStream []byte

	request, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return byteStream, err
	}

	response, err := service.Client.Do(request)
	if err != nil {
		return byteStream, err
	}
	if response.StatusCode != http.StatusOK {
		return byteStream, fmt.Errorf(
			"URL: '%s' returned unexpected status code %d", URL, response.StatusCode)
	}

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			return byteStream, err
		}
		defer reader.Close()
	default:
		reader = response.Body
	}

	return ioutil.ReadAll(reader)
}

func(service *IndexService) Close (){
	close(service.Input)
	close(service.Timeout)
}

func main(){
	start := time.Now()

	service := NewIndexService(
		ReadConfigFromEnv(),
		&http.Client{Timeout: time.Second * 10},
	)
	defer service.Close()

	err := service.Execute(os.Stdin)
	fmt.Println("Elapsed time was: ", time.Since(start))
	if err != nil {
		log.Panic(err)
	}
}

package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
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
	CPUPROF    = "perf/cpuprof-%s"
	MEMPROF    = "perf/memprof-%s"
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
	Data *User `json:"data"`
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
	Data *Video `json:"data"`
}

type IndexRequest struct {
	User  *User  `json:"user"`
	Video *Video `json:"video"`
}

type Config struct {
	UsersURL  string
	VideosURL string
	IndexURL  string
	Timeout   time.Duration
	Threads   int
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

	return &Config{usersURL, videosURL, indexURL, time.Duration(timeout) * time.Second, int(threads)}
}

type Line [2]string

func NewLine(l []string) (*Line, error) {
	line := &Line{}
	if l == nil {
		return line, errors.New("Error: cannot create line with nil value")
	}

	if len(l) != 2 {
		return line, errors.New("Error: line must have exactly 2 values")
	}

	line[0] = l[0]
	line[1] = l[1]

	return line, nil
}

func (l *Line) Validate() bool {
	userID := l[0]
	videoID := l[1]

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

type IndexService struct {
	*Config
	Input   chan Line
	Timeout chan bool
	Client  *http.Client
}

func NewIndexService(cfg *Config, client *http.Client) *IndexService {
	input := make(chan Line, 1)
	timeout := make(chan bool, 1)

	return &IndexService{
		cfg,
		input,
		timeout,
		client,
	}
}

func (service *IndexService) Execute(reader io.Reader) {
	wg := &sync.WaitGroup{}
	multiplexer := func(wg *sync.WaitGroup) {
		for line := range service.Input {
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
			case <-service.Timeout:
				// no-op
			case <-time.After(service.Config.Timeout):
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
}

func (service *IndexService) ParseCSVStream(scanner *bufio.Scanner) {
	for scanner.Scan() {
		line, err := NewLine(strings.Split(scanner.Text(), ","))
		if err != nil {
			continue
		}
		valid := line.Validate()
		if !valid {
			continue
		}
		service.Input <- Line{line[0], line[1]}
	}
}

func (service *IndexService) IndexUserVideo(userID, videoID string) error {
	var indexRequest IndexRequest

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

func (service *IndexService) PostIndex(indexRequest IndexRequest) error {
	serializedIndexRequest, err := json.Marshal(indexRequest)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(
		"POST",
		service.IndexURL,
		bytes.NewBuffer(serializedIndexRequest),
	)

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

func (service *IndexService) GetUser(ID string) (*UserResponse, error) {
	userResponse := &UserResponse{}
	usersURL := fmt.Sprintf("%s/%s", service.UsersURL, ID)

	b, err := service.JSONRequest(usersURL)
	if err != nil {
		return userResponse, err
	}

	json.Unmarshal(b, userResponse)
	return userResponse, nil
}

func (service *IndexService) GetVideo(ID string) (*VideoResponse, error) {
	videoResponse := &VideoResponse{}
	videosURL := fmt.Sprintf("%s/%s", service.VideosURL, ID)

	b, err := service.JSONRequest(videosURL)
	if err != nil {
		return videoResponse, err
	}

	json.Unmarshal(b, videoResponse)
	return videoResponse, nil
}

func (service *IndexService) JSONRequest(URL string) ([]byte, error) {
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

func (service *IndexService) Close() {
	close(service.Input)
	close(service.Timeout)
}

func main() {
	start := time.Now()
	service := NewIndexService(ReadConfigFromEnv(), &http.Client{Timeout: time.Second * 5})
	defer service.Close()

	fmt.Printf(
		"Running on %d threads with a timeout of %d seconds\n",
		service.Threads, (service.Config.Timeout/time.Second))
	service.Execute(os.Stdin)
	fmt.Println("Elapsed time: ", time.Since(start))
}

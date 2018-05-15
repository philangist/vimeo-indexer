package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	TIMEOUT          = time.Second * 3
	HTTP_CREATED     = http.StatusCreated
	HTTP_UNAVAILABLE = http.StatusServiceUnavailable
)

// return a handler that writes a json serialized version of entity
func jsonHandler(entity interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		serialized, err := json.Marshal(entity)
		if err != nil {
			log.Panic(err)
		}
		w.Write(serialized)
	}
}

func statusHandler(status int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}
}

func createTestConfig(url string) *Config {
	return &Config{url, url, url, time.Duration(1), 1}
}

/*
-- input tests
*/

type csvTestCase struct {
	Tag      string
	Input    []string
	Expected bool
}

func TestValidateCSVLine(t *testing.T) {
	fmt.Println("Running TestValidateCSVLine...")
	cases := []csvTestCase{
		{
			Tag:      "case 1 - valid ids",
			Input:    []string{"111111", "111111"},
			Expected: true,
		},
		{
			Tag:      "case 2 - missing video id",
			Input:    []string{"99999", ""},
			Expected: false,
		},
		{
			Tag:      "case 3 - empty strings",
			Input:    []string{"  ", "  "},
			Expected: false,
		},
		{
			Tag:      "case 4 - non integer ids",
			Input:    []string{"foo", "bar"},
			Expected: false,
		},
	}

	for _, c := range cases {
		fmt.Println(c.Tag)
		actual := ValidateCSVLine(c.Input)
		if c.Expected != actual {
			t.Errorf("Actual value '%v' did not match expected value '%v'\n", actual, c.Expected)
		}
	}
}

type parseCSVStreamTestCase struct {
	Tag      string
	Input    string
	Expected [][2]string
}

func TestParseCSVStream(t *testing.T) {
	cases := []parseCSVStreamTestCase{
		/*{
			Tag:      "case 1 - valid ids",
			Input:    "111111,111111\n222222,222222\n",
			Expected: [][2]string{
				[2]string{"111111","111111"},
				[2]string{"222222","222222"},
			},

		},
		{
			Tag:      "case 2 - invalid ids",
			Input:    "111111,\nstring,456678\n\n",
			Expected: [][2]string{},

		},*/
		{
			Tag:      "case 3 - mixed valid and invalid ids",
			Input:    "333333,333333\n,\n567489,567489\nstring,322222\n\n",
			Expected: [][2]string{
				[2]string{"333333","333333"},
				[2]string{"567489","567489"},
			},

		},
	}

	service := NewIndexService(
		createTestConfig(""),
		&http.Client{},
	)
	defer service.Close()

	// This test is a little hacky. Need to research more on
	// testing channels
	for _, c := range cases {
		fmt.Println(c.Tag)
		done := make(chan bool)
		actual := [][2]string{}
		scanner := bufio.NewScanner(strings.NewReader(c.Input))
		go func(){
			for {
				select {
				case line := <- service.Input:
					actual = append(actual, line)
				case <- time.After(1 * time.Second) :
					done <- true
				}
			}
		}()
		service.ParseCSVStream(scanner)
		<- done
		if !reflect.DeepEqual(actual, c.Expected) {
			t.Errorf(
				"%s: Parsed CSV values '%v' did not match expected values '%v'\n",
				c.Tag, actual, c.Expected)
		}
	}

}

/*
-- networking tests
*/

type getUserTestCase struct {
	Tag      string
	ID       string
	Expected *UserResponse
}

func TestGetUsers(t *testing.T) {
	fmt.Println("Running TestGetUsers...")
	c := getUserTestCase{
		Tag: "Case 1 - Basic deserialization",
		ID:  "1000",
		Expected: &UserResponse{
			Data: User{
				ID:       1,
				FullName: "John Smith",
				Email:    "john.smith@gmail.com",
				Country:  "Antigua",
				Language: "Dutch",
				LastIP:   "10.10.10.10",
			},
		},
	}

	handler := jsonHandler(c.Expected)
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	service := NewIndexService(
		createTestConfig(tServer.URL),
		&http.Client{Timeout: TIMEOUT},
	)
	defer service.Close()

	actual, _ := service.GetUser(c.ID)
	if !reflect.DeepEqual(actual, c.Expected) {
		t.Errorf(
			"Unmarshalled value '%v' did not match expected value '%v'\n",
			actual, c.Expected)
	}
}

func TestGetUsersServerDown(t *testing.T) {
	fmt.Println("Running TestGetUsersServerDown...")
	handler := statusHandler(HTTP_UNAVAILABLE)
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	service := NewIndexService(
		createTestConfig(tServer.URL),
		&http.Client{Timeout: TIMEOUT},
	)
	defer service.Close()
	_, err := service.GetUser("1000")

	if err == nil {
		t.Errorf("Expected GetUser to err on 503 server response, receieved nil")
	}
}

type getVideoTestCase struct {
	Tag      string
	ID       string
	Expected *VideoResponse
}

func TestGetVideos(t *testing.T) {
	fmt.Println("Running TestGetVideos...")
	c := getVideoTestCase{
		Tag: "Case 1 - Basic deserialization",
		ID:  "1000",
		Expected: &VideoResponse{
			Data: Video{
				ID:              1,
				Title:           "Joe Rogan Experience #1114 - Matt Taibbi",
				Caption:         "Matt Taibbi is a journalist and author...",
				Privacy:         "public",
				FrameRate:       "60",
				VideoCodec:      "H.264",
				AudioCodec:      "AAC",
				AudioSampleRate: "128",
			},
		},
	}

	handler := jsonHandler(c.Expected)
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	service := NewIndexService(
		createTestConfig(tServer.URL),
		&http.Client{Timeout: TIMEOUT},
	)
	defer service.Close()

	actual, _ := service.GetVideo(c.ID)
	if !reflect.DeepEqual(actual, c.Expected) {
		t.Errorf(
			"Unmarshalled value '%v' did not match expected value '%v'\n",
			actual, c.Expected)
	}
}

func TestGetVideosServerDown(t *testing.T) {
	fmt.Println("Running TestGetVideosServerDown...")
	handler := statusHandler(HTTP_UNAVAILABLE)
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	service := NewIndexService(
		createTestConfig(tServer.URL),
		&http.Client{Timeout: TIMEOUT},
	)
	defer service.Close()

	_, err := service.GetVideo("1000")
	if err == nil {
		t.Errorf("Expected GetVideo to err on 503 server response, receieved nil")
	}
}

func TestPostIndex(t *testing.T) {
	fmt.Println("Running TestPostIndexData...")

	handler := statusHandler(HTTP_CREATED)
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	service := NewIndexService(
		createTestConfig(tServer.URL),
		&http.Client{Timeout: TIMEOUT},
	)
	defer service.Close()

	err := service.PostIndex(IndexRequest{})
	if err != nil {
		t.Errorf("Expected TestPostIndex to succeed on 201 server response, receieved err '%v' instead", err)
	}
}

func TestPostIndexServerDown(t *testing.T) {
	fmt.Println("Running TestPostIndexServerDown...")

	handler := statusHandler(HTTP_UNAVAILABLE)
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	service := NewIndexService(
		createTestConfig(tServer.URL),
		&http.Client{Timeout: TIMEOUT},
	)
	defer service.Close()

	err := service.PostIndex(IndexRequest{})
	if err == nil {
		t.Errorf("Expected PostIndexData to err on 503 server response, receieved nil")
	}
}

/*
-- concurrency tests
*/



func TestIndexServiceExecute(t *testing.T) {
	// Expected returned objects from /users/:id and /videos/:id
	user := &User{
		ID:       1,
		FullName: "John Smith",
		Email:    "john.smith@gmail.com",
		Country:  "Antigua",
		Language: "Dutch",
		LastIP:   "10.10.10.10",
	}

	video := Video{
		ID:              1,
		Title:           "Joe Rogan Experience #1114 - Matt Taibbi",
		Caption:         "Matt Taibbi is a journalist and author...",
		Privacy:         "public",
		FrameRate:       "60",
		VideoCodec:      "H.264",
		AudioCodec:      "AAC",
		AudioSampleRate: "128",
	}

	muxer := http.NewServeMux()
	tServer := httptest.NewServer(muxer)
	defer tServer.Close()

	usersURL := fmt.Sprintf("%s/users", tServer.URL)
	videosURL := fmt.Sprintf("%s/videos", tServer.URL)
	indexURL  := fmt.Sprintf("%s/index", tServer.URL)

	cfg := &Config{
		usersURL,
		videosURL,
		indexURL,
		1 * time.Second,
		1,
	}
	service := NewIndexService(
		cfg,
		&http.Client{Timeout: TIMEOUT},
	)
	defer service.Close()

	muxer.HandleFunc(
		fmt.Sprintf("/users/%d", user.ID),
		jsonHandler(user))

	muxer.HandleFunc(
		fmt.Sprintf("/videos/%d", video.ID),
		jsonHandler(video))

	muxer.HandleFunc("/index", statusHandler(HTTP_CREATED))

	err := service.Execute(strings.NewReader(
		fmt.Sprintf("%d,%d\n", user.ID, video.ID),
	))
	if err != nil {
		t.Errorf(
			`Expected IndexService.Execute to run to completion, receieved err '%v' instead`, err)
	}
}

func TestCloseChannels(t *testing.T) {}

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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

// return a handler that writes status and a json serialized version of entity
func mockHandler(status int, entity interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			w.WriteHeader(status)
		}

		if entity != nil {
			serialized, err := json.Marshal(entity)
			if err != nil {
				log.Panic(err)
			}
			w.Write(serialized)
		}
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
	Valid    bool
	Expected *Line
}

func TestValidateCSVLine(t *testing.T) {
	fmt.Println("Running TestValidateCSVLine...")
	cases := []csvTestCase{
		{
			Tag:      "case 1 - valid ids",
			Input:    []string{"111111", "111111"},
			Valid:    true,
			Expected: &Line{"111111", "111111"},
		},
		{
			Tag:      "case 2 - missing video id",
			Input:    []string{"99999", ""},
			Valid:    false,
			Expected: &Line{},
		},
		{
			Tag:      "case 3 - missing user id",
			Input:    []string{"", "99999"},
			Valid:    false,
			Expected: &Line{},
		},
		{
			Tag:      "case 4 - empty strings",
			Input:    []string{"  ", "  "},
			Valid:    false,
			Expected: &Line{},
		},
		{
			Tag:      "case 5 - nil input",
			Input:    nil,
			Valid:    false,
			Expected: &Line{},
		},
		{
			Tag:      "case 6 - non integer ids",
			Input:    []string{"foo", "bar"},
			Valid:    false,
			Expected: &Line{},
		},
	}

	for _, c := range cases {
		fmt.Println(c.Tag)
		line, err := NewLine(c.Input)
		valid := line.Validate()
		if c.Valid {
			if (err != nil) || (!valid) {
				t.Errorf("%s: Input %s was unexpectedly parsed as invalid", c.Tag, c.Input)
				continue
			}

			if !reflect.DeepEqual(line, c.Expected) {
				t.Errorf(
					"%s: Parsed Line values '%v' did not match expected values '%v'\n",
					c.Tag, line, c.Expected)
			}
		}
	}
}

type parseCSVStreamTestCase struct {
	Tag      string
	Input    string
	Expected []Line
}

func TestParseCSVStream(t *testing.T) {
	cases := []parseCSVStreamTestCase{
		/*{
			Tag:      "case 1 - valid ids",
			Input:    "111111,111111\n222222,222222\n",
			Expected: []Line{
				Line{"111111","111111"},
				Line{"222222","222222"},
			},

		},
		{
			Tag:      "case 2 - invalid ids",
			Input:    "111111,\nstring,456678\n\n",
			Expected: []Line{},

		},*/
		{
			Tag:   "case 3 - mixed valid and invalid ids",
			Input: "333333,333333\n,\n567489,567489\nstring,322222\n\n",
			Expected: []Line{
				Line{"333333", "333333"},
				Line{"567489", "567489"},
			},
		},
	}

	service := NewIndexService(
		createTestConfig(""),
		&http.Client{},
	)
	defer service.Close()

	// this test is a little hacky. Need to research more on
	// testing channels
	for _, c := range cases {
		fmt.Println(c.Tag)
		done := make(chan bool)
		actual := []Line{}
		scanner := bufio.NewScanner(strings.NewReader(c.Input))
		go func() {
			for {
				select {
				case line := <-service.Input:
					actual = append(actual, line)
				case <-time.After(time.Second):
					done <- true
				}
			}
		}()
		service.ParseCSVStream(scanner)
		<-done
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
			Data: &User{
				ID:       1,
				FullName: "John Smith",
				Email:    "john.smith@gmail.com",
				Country:  "Antigua",
				Language: "Dutch",
				LastIP:   "10.10.10.10",
			},
		},
	}

	handler := mockHandler(0, c.Expected)
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
	handler := mockHandler(HTTP_UNAVAILABLE, nil)
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
			Data: &Video{
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

	handler := mockHandler(0, c.Expected)
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
	handler := mockHandler(HTTP_UNAVAILABLE, nil)
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

	handler := mockHandler(HTTP_CREATED, nil)
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

	handler := mockHandler(HTTP_UNAVAILABLE, nil)
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

type mockable func(status int, value interface{}) func(http.ResponseWriter, *http.Request)
type handler func(http.ResponseWriter, *http.Request)

type Mock struct {
	Counter map[string]int
}

func (m *Mock) Add(name string, fn mockable, status int, param interface{}) handler {
	count, _ := m.Counter[name]
	m.Counter[name] = count + 1
	return fn(status, param)
}

func (m *Mock) Count(name string) int {
	count, _ := m.Counter[name]
	return count
}

func TestIndexServiceExecute(t *testing.T) {
	fmt.Println("Running TestIndexServiceExecute...")
	os.Setenv("TIMEOUT", "3") // increase codecov value
	os.Setenv("NUM_THREADS", "1")
	ReadConfigFromEnv()

	// expected returned objects from /users/:id and /videos/:id
	user := &User{
		ID:       1,
		FullName: "John Smith",
		Email:    "john.smith@gmail.com",
		Country:  "Antigua",
		Language: "Dutch",
		LastIP:   "10.10.10.10",
	}

	video := &Video{
		ID:              1,
		Title:           "Joe Rogan Experience #1114 - Matt Taibbi",
		Caption:         "Matt Taibbi is a journalist and author...",
		Privacy:         "public",
		FrameRate:       "60",
		VideoCodec:      "H.264",
		AudioCodec:      "AAC",
		AudioSampleRate: "128",
	}

	// set up test servers for users, videos, and index services
	muxer := http.NewServeMux()
	tServer := httptest.NewServer(muxer)
	defer tServer.Close()

	usersURL := fmt.Sprintf("%s/users", tServer.URL)
	videosURL := fmt.Sprintf("%s/videos", tServer.URL)
	indexURL := fmt.Sprintf("%s/index", tServer.URL)

	// create new IndexService instance
	cfg := &Config{
		usersURL,
		videosURL,
		indexURL,
		time.Second,
		1,
	}
	service := NewIndexService(
		cfg,
		&http.Client{Timeout: TIMEOUT},
	)
	defer service.Close()

	mock := &Mock{map[string]int{}}

	// register mock handlers for users, videos, and index services.
	// this will allow us to ensure that each service was called at least once
	muxer.HandleFunc(
		fmt.Sprintf("/users/%d", user.ID),
		mock.Add(
			"users-get",
			mockHandler,
			0,
			user,
		))

	muxer.HandleFunc(
		fmt.Sprintf("/videos/%d", video.ID),
		mock.Add(
			"videos-get",
			mockHandler,
			0,
			video,
		))

	muxer.HandleFunc(
		"/index",
		mock.Add(
			"index-post",
			mockHandler,
			HTTP_CREATED,
			nil,
		))

	// execute the entire workflow
	service.Execute(strings.NewReader(
		fmt.Sprintf("%d,%d\n", user.ID, video.ID),
	))

	// test that the expected http handlers are actually called
	expectations := map[string]int{
		"users-get":  1,
		"videos-get": 1,
		"index-post": 1,
	}

	for tag, count := range expectations {
		if mock.Count(tag) != count {
			t.Errorf(
				"Expected %s to be called %d time(s). Was actually called: %d time(s)",
				tag,
				count,
				mock.Count(tag),
			)
		}
	}
}

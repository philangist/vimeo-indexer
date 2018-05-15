package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"time"
	"testing"
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

// return a handler that mimics an internal server error
func status503Handler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

type csvTestCase struct {
	Tag string
	Input []string
	Expected bool
}

func TestValidateCSVLine(t *testing.T){
	fmt.Println("Running TestValidateCSVLine...")
	cases := []csvTestCase{
		{
			Tag: "case 1 - valid ids",
			Input: []string{"111111","111111"},
			Expected: true,
		},
		{
			Tag: "case 2 - missing video id",
			Input: []string{"99999",""},
			Expected: false,
		},
		{
			Tag: "case 3 - empty strings",
			Input: []string{"  ","  "},
			Expected: false,
		},
		{
			Tag: "case 4 - non integer ids",
			Input: []string{"foo","bar"},
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


type getUserTestCase struct {
	Tag string
	ID  string
	Expected *UserResponse
}

func TestGetUsers(t *testing.T){
	fmt.Println("Running TestGetUsers...")
	c := getUserTestCase{
		Tag: "Case 1 - Basic deserialization",
		ID:  "1000",
		Expected: &UserResponse{
			Data: User{
				ID: 1,
				FullName: "John Smith",
				Email: "john.smith@gmail.com",
				Country: "Antigua",
				Language: "Dutch",
				LastIP: "10.10.10.10",
			},
		},
	}

	httpClient := &http.Client{Timeout: time.Second * 10}

	handler := jsonHandler(c.Expected)
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	actual, _ := GetUserData(httpClient, tServer.URL, c.ID)
	if !reflect.DeepEqual(actual, c.Expected) {
		t.Errorf(
			"Unmarshalled value '%v' did not match expected value '%v'\n",
			actual, c.Expected)
	}
}

func TestGetUsersServerDown(t *testing.T){
	fmt.Println("Running TestGetUsersServerDown...")
	handler := status503Handler()
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	httpClient := &http.Client{Timeout: time.Second * 10}
	_, err := GetUserData(httpClient, tServer.URL, "1000")

	if err == nil {
		t.Errorf("Expected GetUserData to err on 503, receieved nil")
	}
}

type getVideoTestCase struct {
	Tag string
	ID  string
	Expected *VideoResponse
}

func TestGetVideosUp(t *testing.T){
	fmt.Println("Running TestGetVideosUp...")
	c := getVideoTestCase{
		Tag: "Case 1 - Basic deserialization",
		ID:  "1000",
		Expected: &VideoResponse{
			Data: Video{
				ID: 1,
				Title: "Joe Rogan Experience #1114 - Matt Taibbi",
				Caption: "Matt Taibbi is a journalist and author...",
				Privacy: "public",
				FrameRate: "60",
				VideoCodec: "H.264",
				AudioCodec: "AAC",
				AudioSampleRate: "128",
			},
		},
	}

	httpClient := &http.Client{Timeout: time.Second * 10}

	handler := jsonHandler(c.Expected)
	tServer := httptest.NewServer(http.HandlerFunc(handler))

	defer tServer.Close()		

	actual, _ := GetVideoData(httpClient, tServer.URL, c.ID)
	if !reflect.DeepEqual(actual, c.Expected) {
		t.Errorf(
			"Unmarshalled value '%v' did not match expected value '%v'\n",
			actual, c.Expected)
	}
}

func TestGetVideosServerDown(t *testing.T){
	fmt.Println("Running TestGetVideosServerDown...")
	handler := status503Handler()
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	httpClient := &http.Client{Timeout: time.Second * 10}
	_, err := GetVideoData(httpClient, tServer.URL, "1000")
	if err == nil {
		t.Errorf("Expected GetUserData to err on 503, receieved nil")
	}
}

func TestPostIndexData(t *testing.T){
	fmt.Println("Running TestPostIndexData...")
	if 1 == 2 {
		t.Errorf("PLACEHOLDER TEST")
	}
}

func TestPostIndexServerDown(t *testing.T){
	fmt.Println("Running TestPostIndexServerDown...")

	handler := status503Handler()
	tServer := httptest.NewServer(http.HandlerFunc(handler))
	defer tServer.Close()

	httpClient := &http.Client{Timeout: time.Second * 10}
	err := PostIndexData(httpClient, tServer.URL, Index{})

	if err == nil {
		t.Errorf("Expected TestPostIndexServerDown to err on 503, receieved nil")
	}
}

/*
func TestFetchUserVideoData(){}
*/

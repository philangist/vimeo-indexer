package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"time"
	"testing"
)

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
	cases := []getUserTestCase{
		{
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
		},
	}

	httpClient := &http.Client{Timeout: time.Second * 10}

	for _, c := range cases {
		tServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					serialized, _ := json.Marshal(c.Expected)
					w.Write(serialized)
				}))

		actual, _ := GetUserData(httpClient, tServer.URL, c.ID)
		if !reflect.DeepEqual(actual, c.Expected) {
			t.Errorf(
				"Unmarshalled value '%v' did not match expected value '%v'\n",
				actual, c.Expected)
		}

		tServer.Close()		
	}
}

func TestGetUsersServerDown(t *testing.T){
	fmt.Println("Running TestGetUsersServerDown...")
	tServer := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
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

func TestGetVideos(t *testing.T){
	fmt.Println("Running TestGetVideos...")
	cases := []getVideoTestCase{
		{
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
		},
	}

	httpClient := &http.Client{Timeout: time.Second * 10}

	for _, c := range cases {
		tServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					serialized, _ := json.Marshal(c.Expected)
					w.Write(serialized)
				}))

		actual, _ := GetVideoData(httpClient, tServer.URL, c.ID)
		if !reflect.DeepEqual(actual, c.Expected) {
			t.Errorf(
				"Unmarshalled value '%v' did not match expected value '%v'\n",
				actual, c.Expected)
		}

		tServer.Close()		
	}
}

func TestGetVideosServerDown(t *testing.T){
	fmt.Println("Running TestGetVideosServerDown...")
	tServer := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
	defer tServer.Close()

	httpClient := &http.Client{Timeout: time.Second * 10}
	_, err := GetVideoData(httpClient, tServer.URL, "1000")
	fmt.Println("err is ", err)
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
	tServer := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
	defer tServer.Close()

	httpClient := &http.Client{Timeout: time.Second * 10}
	err := PostIndexData(httpClient, tServer.URL, Index{})
	fmt.Println("err is ", err)
	if err == nil {
		t.Errorf("Expected TestPosttVideoDataServerDown to err on 503, receieved nil")
	}
}

/*
func TestFetchUserVideoData(){}
*/

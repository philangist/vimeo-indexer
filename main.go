package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
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

func ParseCSVStream(scanner *bufio.Scanner) (<-chan [2]string, error) {
	channel := make(chan [2]string)
	go func () {
		for scanner.Scan(){
			line := strings.Split(scanner.Text(), ",")
			channel <- [2]string{line[0], line[1]}
		}
		close(channel)
	}()

	return channel, nil
}

func main(){
	httpClient := &http.Client{Timeout: time.Second * 10}
	var userResponse UserResponse
	var videoResponse VideoResponse

	var userIndex Index
	// userIndexes := make(map[string][]Index)
	var userIndexes []Index

	failed := [][2]string{}

	dataStream, err := ParseCSVStream(bufio.NewScanner(os.Stdin))
	if err != nil {
		log.Panic(err)
	}

	for data := range dataStream {
		userID, videoID := data[0], data[1]
		appended := false

		err := getUserData(httpClient, userID, &userResponse)
		if err == nil {
			userIndex.User = userResponse.Data
		} else {
			appended = true
			failed = append(failed, data)
		}

		err = getVideoData(httpClient, videoID, &videoResponse)
		if err == nil {
			userIndex.Video = videoResponse.Data
		} else {
			if !appended {
				failed = append(failed, data)
			}
		}
		userIndexes = append(userIndexes, userIndex)
	}

	for _, userIndex := range userIndexes {
		err := postIndexData(httpClient, userIndex)
		if err != nil {
		}
	}

	fmt.Println("userIndexes are ", userIndexes)
	fmt.Printf("Failed requests were %s\n", failed)
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
	return nil
}

func getUserData(httpClient *http.Client, userID string, userResponse *UserResponse) error {
	userURL := fmt.Sprintf("http://localhost:8000/users/%s", userID)

	fmt.Println("Requesting URL", userURL)
	request, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		fmt.Printf("Error %s preparing request for URL %s", err, userURL)
	}
	// request.Header.Add("Accept-Encoding", "json")
	response, err := httpClient.Do(request)
	if err != nil {
		fmt.Printf("Error %s requesting URL %s", err, userURL)
	}

	// Check that the server actually sent compressed data
	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		defer reader.Close()
	default:
		reader = response.Body
	}

	err = json.NewDecoder(reader).Decode(userResponse)
	fmt.Printf("userResponse is %v\n", userResponse.Data)
	if err != nil {
		fmt.Printf("Error %s decoding %v\n", err, reader)
	}
	return err
}

func getVideoData(httpClient *http.Client, videoID string, videoResponse *VideoResponse) error {
	videoURL := fmt.Sprintf("http://localhost:8001/videos/%s", videoID)

	fmt.Println("Requesting URL", videoURL)
	request, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		fmt.Printf("Error %s preparing request for URL %s", err, videoURL)
	}
	// request.Header.Add("Accept-Encoding", "json")
	response, err := httpClient.Do(request)
	if err != nil {
		fmt.Printf("Error %s requesting URL %s", err, videoURL)
	}

	// Check that the server actually sent compressed data
	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		defer reader.Close()
	default:
		reader = response.Body
	}

	err = json.NewDecoder(reader).Decode(&videoResponse)
	fmt.Printf("VideoResponse is %v\n", videoResponse.Data)
	if err != nil {
		fmt.Printf("Error %s decoding %v\n", err, reader)
	}
	return err
}

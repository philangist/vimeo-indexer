package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"io"
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

func main(){
	start := time.Now()
	httpClient := &http.Client{Timeout: time.Second * 10}
	inputStream := make(chan [2]string, 1)
	userIndexes := make(chan Index, 1)

	go parseCSVStream(bufio.NewScanner(os.Stdin), inputStream)
	go fetchUsersVideosData(httpClient, inputStream, userIndexes)
	indexUsersVideosData(httpClient, userIndexes)

	elapsed := time.Since(start)
	log.Printf("Indexing took %s", elapsed)
}

func parseCSVStream(scanner *bufio.Scanner, channel chan [2]string) {
/*	inputStream := [][2]string{
		{"123","456"},
		{"456","123"},
	}

	for _, line := range inputStream {*/

        for scanner.Scan(){
		line := strings.Split(scanner.Text(), ",")
		scanner.Scan()
		fmt.Printf("sending line %v to inputStream channel\n", line)
		channel <- [2]string{line[0], line[1]}
	}
	defer close(channel)
	fmt.Println("Done parseCSVStream")
}

func fetchUsersVideosData(httpClient *http.Client, inputStream chan [2]string, userIndexes chan Index) {
	var userResponse UserResponse
	var videoResponse VideoResponse
	var userIndex Index

	for data, ok := <- inputStream; ok; data, ok = <- inputStream {
		failed := false
		userID, videoID := data[0], data[1]

		fmt.Printf("Reading line %v from inputStream channel\n", data)

		err := getUserData(httpClient, userID, &userResponse)
		if err == nil {
			userIndex.User = userResponse.Data
		} else {
			failed = true
		}

		err = getVideoData(httpClient, videoID, &videoResponse)
		if err == nil {
			userIndex.Video = videoResponse.Data
		} else {
			failed = failed || true
		}

		/*if failed {
			inputStream <- data
		}*/

		fmt.Println("data is ", data)
		userIndexes <- userIndex
	}
	defer close(userIndexes)
	fmt.Println("Done fetchUsersVideosData")
}

func indexUsersVideosData(httpClient *http.Client, userIndexes chan Index){
	for userIndex := range userIndexes {
		fmt.Printf("Posting userIndex %v to index service\n", userIndex)
		err := postIndexData(httpClient, userIndex)
		if err != nil {
			fmt.Println("error indexing user data", err)
		}
	}

	fmt.Println("Done indexUsersVideosData")
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

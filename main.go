package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	// "os"
	"time"
)

var userVideos = [][2]string{
	{"5433093","9745438"},
	{"9069083","2592427"},
	{"3098486","16187"},
	{"1359220","1456728"},
	{"1579325","6012200"},
	{"8915243","2523777"},
	{"8371832","4449674"},
	{"7993782","5083041"},
	{"2142853","4050035"},
	{"5244960","6805475"},
	{"5170385","3250572"},
	{"9346206","2634211"},
	{"3925649","2322726"},
	{"7082623","5616344"},
	{"3742176","2839349"},
	{"9927352","5633582"},
	{"3285161","8120300"},
	{"4155557","5233818"},
	{"5301863","3436964"},
	{"3858666","4234147"},
	{"8363202","9382368"},
	{"5579883","1845820"},
	{"9575200","5731143"},
	{"1598605","1691638"},
	{"2608055","2273662"},
	{"993542","7384371"},
	{"3339188","6603692"},
	{"1945506","4544641"},
	{"6548676","7434149"},
	{"1518448","1618282"},
	{"8211370","1426604"},
	{"7556236","1144090"},
	{"1673399","729021"},
	{"8291869","8468959"},
	{"1098225","2917854"},
	{"2725040","6865928"},
	{"9037693","4772606"},
	{"9234696","5019385"},
	{"8649968","150807"},
	{"4850804","4364447"},
	{"6925550","1827770"},
	{"5780141","3016280"},
	{"5339670","7522023"},
	{"3674960","4753105"},
	{"4975847","6250757"},
	{"8526319","9358479"},
	{"6482989","9210317"},
	{"822975","2308126"},
	{"6179454","8690966"},
	{"6817368","5761534"},
	{"468553","9450251"},
	{"4701266","7082205"},
	{"9383845","3915494"},
	{"6174099","317746"},
	{"4026183","7862861"},
	{"6509194","5284461"},
	{"4279180","975370"},
	{"2389741","5338729"},
	{"7642336","2416852"},
	{"8618073","2711734"},
	{"690486","6871507"},
	{"4616201","2387363"},
	{"1279899","4110938"},
	{"5342748","5921242"},
	{"6855036","3386148"},
	{"9648071","9737812"},
	{"2834520","6833490"},
	{"1208992","6859629"},
	{"8298754","3169811"},
	{"547352","327851"},
	{"7425635","3725380"},
	{"6900911","424147"},
	{"8315602","4339909"},
	{"1252685","6223665"},
	{"4090607","6116692"},
	{"7609507","8211410"},
	{"7739161","4515964"},
	{"1045746","8539904"},
	{"9504938","9244713"},
	{"6974536","3428991"},
	{"7458924","8862602"},
	{"3214993","6003198"},
	{"5637639","6036759"},
	{"4249175","2862131"},
	{"4087600","5135752"},
	{"5104833","3495899"},
	{"2832064","2833755"},
	{"7152769","1626846"},
	{"8207522","9022116"},
	{"7982151","6906863"},
	{"3572121","2254341"},
	{"4903980","1174465"},
	{"6097045","8342447"},
	{"9744952","1849239"},
	{"2249046","348988"},
	{"1582838","379057"},
	{"1069630","6965221"},
	{"4634658","3086744"},
	{"313019","6842309"},
	{"7667612","8796179"},
	{"7777579","2617993"},
	{"9185439","1842694"},
	{"8948147","7149746"},
	{"4830257","7618967"},
	{"5516628","567226"},
	{"69778","5058164"},
	{"52946","2354502"},
	{"8457705","1914678"},
	{"7970013","786889"},
	{"4317483","3788194"},
}

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
	httpClient := &http.Client{Timeout: time.Second * 10}
	var userResponse UserResponse
	var videoResponse VideoResponse

	var userIndex Index
	// userIndexes := make(map[string][]Index)
	var userIndexes []Index

	failed := [][2]string{}

	for _, data := range userVideos[:5] {
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

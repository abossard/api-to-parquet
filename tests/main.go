package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type TimeSeriesData struct {
	Timestamp       int64  `json:"timestamp"`
	TimeOffsetHours int8   `json:"timeOffsetHours"`
	PointId         string `json:"pointId"`
	Sequence        int32  `json:"sequence"`
	Project         string `json:"project"`
	Value           int    `json:"value"`
	Res             string `json:"res"`
	Quality         int32  `json:"quality"`
}

type InputRecord struct {
	Content       []TimeSeriesData `json:"content"`
	Id            string           `json:"id"`
	TimeGenerated int64            `json:"timeGenerated"`
	File          string           `json:"file"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <url>")
		return
	}
	url := os.Args[1]

	// Generate random data
	data := generateData()

	// Marshal the data to JSON
	payload, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	// Make the HTTP POST request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Error making HTTP request:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("HTTP response status:", resp.Status)
}

func generateData() InputRecord {
	// Generate a random ID and file name
	id := generateGUID()
	file := fmt.Sprintf("%d-%s", time.Now().Unix(), generateGUID())

	// Generate three random time series data points
	var content []TimeSeriesData
	for i := 0; i < 20000; i++ {
		content = append(content, TimeSeriesData{
			Timestamp:       time.Now().UnixNano() / int64(time.Millisecond),
			TimeOffsetHours: int8(rand.Intn(24)),
			PointId:         generateGUID(),
			Sequence:        int32(rand.Intn(100)),
			Project:         generateGUID(),
			Value:           rand.Intn(43) + 1,
			Res:             generateGUID(),
			Quality:         int32(rand.Intn(100)),
		})
	}

	// Create the input record
	return InputRecord{
		Content:       content,
		Id:            id,
		TimeGenerated: time.Now().UnixNano() / int64(time.Millisecond),
		File:          file,
	}
}

func generateGUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

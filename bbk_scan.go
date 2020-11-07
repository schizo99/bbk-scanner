package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type bbk struct {
	Start         time.Time
	Operator      string
	SupportID     string
	Latency       float64
	Download      float64
	Upload        float64
	MeasurementID string
}

//DATABASE URL
var DATABASE = os.Getenv("DATABASE")

func scanner(data string) bbk {

	space := regexp.MustCompile(`\s+`)
	var counter int
	const layout = "2006-01-02 15:04:05"

	var result bbk
	for _, row := range strings.Split(data, "\n") {
		line := space.ReplaceAllString(row, " ")
		sc := strings.Split(line, ":")
		split := strings.TrimLeft(strings.Join(sc[1:], ":"), " ")

		switch counter {
		case 0:
			loc, _ := time.LoadLocation("Europe/Stockholm")
			start, err := time.ParseInLocation(layout, split, loc)
			if err != nil {
				log.Printf("-E- Failed to parse location:\n %s", err)
			}
			result.Start = start
		case 1:
			result.Operator = split
		case 2:
			result.SupportID = split
		case 3:
			result.Latency, _ = strconv.ParseFloat(strings.Split(split, " ")[0], 64)
		case 4:
			result.Download, _ = strconv.ParseFloat(strings.Split(split, " ")[0], 64)
		case 5:
			result.Upload, _ = strconv.ParseFloat(strings.Split(split, " ")[0], 64)
		case 6:
			result.MeasurementID = split
		}
		counter++
	}
	return result
}

func runBbk() []byte {
	log.Println("-I- Running BBK Scan")
	out, err := exec.Command("/app/bbk_cli").Output()
	if err != nil {
		panic(err)
	}
	log.Printf("\nScanning results:\n%s\n", out)
	return out
}

func createDB(db, uri string) {
	resource := "/query"
	data := url.Values{}
	data.Set("q", "CREATE DATABASE "+db)

	u, _ := url.ParseRequestURI(uri)
	u.Path = resource
	urlStr := u.String() // "https://api.com/user/"

	client := &http.Client{}
	r, _ := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(data.Encode())) // URL-encoded payload
	r.Header.Add("Authorization", "auth_token=\"XXXXXXX\"")
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	client.Do(r)

}

func saveToDB(data bbk) {
	client := influxdb2.NewClient(DATABASE, "my-token")
	writeAPI := client.WriteAPIBlocking("bbk", "test")
	p := influxdb2.NewPoint("bbk",
		map[string]string{"operator": data.Operator, "supportid": data.SupportID, "measurementid": data.MeasurementID},
		map[string]interface{}{"upload": data.Upload, "download": data.Download, "latency": data.Latency},
		data.Start)

	writeAPI.WritePoint(context.Background(), p)

}

func sendMessage(data bbk) {
	TOKEN := os.Getenv("TOKEN")
	if TOKEN == "" {
		log.Println("-E- Provide Telegram TOKEN, not sending result.")
		return
	}
	chatID := os.Getenv("CHAT_ID")
	if chatID == "" {
		log.Println("-E- Provide Telegram CHAT_ID, not sending result.")
		return
	}
	text := fmt.Sprintf("Upload or Download speeds below 250 Mbit\n\nUpload: %f Mbit\n Download: %f Mbit", data.Upload, data.Download)

	params := url.Values{}
	params.Add("text", text)
	_, err := http.Get(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s&%s", TOKEN, chatID, params.Encode()))
	if err != nil {
		log.Println("-E- Failed to send message to Telegram")
	}

}

func verifyResult(data bbk) {
	THRESHOLD, err := strconv.ParseFloat(os.Getenv("THRESHOLD"), 64)
	if err != nil {
		log.Printf("-E- unable to parse THRESHOLD: %s", err)
		THRESHOLD = 250
	}
	if data.Download < THRESHOLD || data.Upload < THRESHOLD {
		sendMessage(data)
	}
}

func main() {
	log.Println("-I- Starting BBK Scanner")
	if DATABASE == "" {
		log.Panic("No DATABASE defined")
	}
	createDB("bbk", DATABASE)
	for {
		data := runBbk()
		result := scanner(fmt.Sprintf("%s", data))
		//result := bbk{Start: time.Now(), Download: 211.12, Upload: 211.12, Latency: 1.123, MeasurementID: "1234234", Operator: "Bahnhof AB", SupportID: "mmo2673t472634"}
		verifyResult(result)
		saveToDB(result)
		log.Println("-I- Sleeping 15 minutes")
		time.Sleep(15 * time.Minute)
	}
}

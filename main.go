package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gocarina/gocsv"
	"github.com/joho/godotenv"
)

// Name, Address, Website, Phone Reviews, Rating, Category, Verified
type Businesses struct {
	Name     string `csv:"Name"`
	Address  string `csv:"Address"`
	Website  string `csv:"Website"`
	Phone    string `csv:"Phone"`
	Reviews  string `csv:"Reviews"`
	Rating   string `csv:"Ratings"`
	Verified string `csv:"Merchant Verified"`
	Category string `csv:"Category"`
	CID      string `csv:"Listing CID"`
}

func errCheck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func csvReader(file string, business *[]Businesses) {
	bytes, err := ioutil.ReadFile(file)
	errCheck(err)
	gocsv.UnmarshalBytes(bytes, business)
}

func curl(url string) {
	cmd := exec.Command("curl", "-I", "-m", "5", url)
	stdout, err := cmd.Output()
	if err != nil {
		fmt.Printf("%s | <FLAG>\n", url)
		return
	}
	data := strings.Split(string(stdout), "\n")
	statusCode := strings.TrimSpace(string([]rune(strings.Split(data[0], " ")[1])[0]))
	switch statusCode {
	case "3", "2":
	case "4":
		fmt.Printf("%s | <NOT FOUND>\n", url)
	default:
		fmt.Printf("%s | %s\n", url, statusCode)
	}
}

func findWebsite(cid string, apiKey string) string {
	type PlacesResult struct {
		Website string `json:"website"`
	}
	type Places struct {
		HmtlAttribution []string     `json:"html_attribution"`
		Result          PlacesResult `json:"result"`
		Status          string       `json:"status"`
	}

	url := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/details/json?cid=%s&key=%s", cid, apiKey)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	errCheck(err)

	res, err := client.Do(req)
	errCheck(err)
	defer res.Body.Close()

	var places Places

	body, err := ioutil.ReadAll(res.Body)
	errCheck(err)
	json.Unmarshal(body, &places)

	return places.Result.Website
}

func main() {
	var businesses []Businesses

	godotenv.Load()

	csvReader("bad.csv", &businesses)
	for _, business := range businesses {
		website := strings.TrimSpace(business.Website)
		cid := strings.TrimSpace(business.CID)
		switch website {
		case "http://business.site", "http://godaddysites.com":
			website = findWebsite(cid, os.Getenv("API_KEY"))
			fmt.Printf("%s | <Found Mssing Website>\n", website)
			fallthrough
		default:
			curl(website)
		}
	}
}

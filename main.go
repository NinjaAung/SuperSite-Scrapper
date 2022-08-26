package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// Name, Address, Website, Phone Reviews, Rating, Category, Verified

func main() {
	f, err := os.Open("bad.csv")
	errCheck(err)
	defer f.Close()

	csvReader := csv.NewReader(f)
	data, err := csvReader.ReadAll()
	errCheck(err)

	for i, line := range data {
		if i > 0 {
			website := line[5] // Update what line is website
			website = strings.TrimSpace(website)

			switch website {
			case "http://business.site", "http://godaddysites.com ":
				fmt.Printf("%s | <Sub-Domain Missing>\n", website)
				cid := line[25]

				website = findWebsite(cid)

				fallthrough
			default:
				curl(website)
			}
		}
	}
}

func curl(url string) {
	cmd := exec.Command("curl", "-I", "-m", "5", url)
	stdout, err := cmd.Output()
	if err != nil {
		fmt.Printf("%s | <FLAG>\n", url)
		return
	}
	data := strings.Split(string(stdout), "\n")
	statusCode := strings.TrimSpace(string([]rune(data[0])[8:10]))
	switch statusCode {
	case "3", "2":
	case "4":
		fmt.Printf("%s | <NOT FOUND>\n", url)
	default:
		fmt.Printf("%s | %s\n", url, statusCode)
	}
}

func errCheck(err error) {
	if err != nil {
		log.Fatal(err)
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

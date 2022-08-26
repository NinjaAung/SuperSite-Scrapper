package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

type BusinessRecord struct {
	Name                string
	Address             string
	Street_Address      string
	City                string
	State               string
	ZipCode             string
	PlusCode            string
	Phone               string
	Category            string
	Menu                string
	Website             string
	ListingType         string
	Rating              string
	Reviews             string
	PriceRange          string
	Hours               string
	Description         string
	MerchantVerified    string
	PermanentlyClosed   string
	TemporarilyClosed   string
	DineIn              string
	Takeout             string
	Delivery            string
	ImageURL            string
	Listing_CID         string
	Lat                 string
	Long                string
	PossibleVirtualTour string
	Review_1_Text       string
	Review_1_Score      string
	Review_1_Date       string
	Review_2_Text       string
	Review_2_Score      string
	Review_2_Date       string
	Review_3_Text       string
	Review_3_Score      string
	Review_3_Date       string
	ListingURL          string
}

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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/gocarina/gocsv"
	"github.com/gocolly/colly"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

// Name, Address, Website, Phone Reviews, Rating, Category, Verified

func errCheck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	e, _ := os.Executable()
	fmt.Println("Reading and combing all CSV in:", path.Dir(e))
	godotenv.Load(path.Dir(e) + "/" + ".env")
	businesses := ReadAll(path.Dir(e))

	var reviews []Businesses
	var flaggeds []Businesses
	var emptys []Businesses

	fmt.Println("Parsing Data")
	for _, business := range businesses {
		business.Website = strings.TrimSpace(business.Website)
		business.CID = strings.TrimSpace(business.CID)
		switch business.Website {
		case "":
			emptys = append(emptys, business)
		case "http://business.site", "http://godaddysites.com":
			fmt.Println(path.Dir(e) + "/" + os.Getenv("API_KEY"))
			business.Website = findWebsite(business.CID, path.Dir(e)+"/"+os.Getenv("API_KEY"))
			// fmt.Printf("%s | <Found Missing Website>\n", business.Website)
			fallthrough
		default:
			curl(business, &flaggeds, &reviews)
		}
	}
	// Push Finalization
	pushToSheet(flaggeds, emptys, reviews, path.Dir(e))
}

func reviewPages(url string) bool {
	c := colly.NewCollector()

	counter := 0
	counter_error := 0

	c.OnHTML("body", func(e *colly.HTMLElement) { counter += len([]rune(e.Text)) })

	c.OnError(func(r *colly.Response, err error) { counter_error += 1 })

	c.Visit(url)

	if counter <= 2000 || counter_error > 0 {
		return true
	}
	return false
}

// CSV READER
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

func csvReader(file string, business *[]Businesses) {
	bytes, err := ioutil.ReadFile(file)
	errCheck(err)
	gocsv.UnmarshalBytes(bytes, business)
}

func ReadAll(filePath string) []Businesses {
	files, err := filepath.Glob(filePath + "/*.csv")
	errCheck(err)
	var business []Businesses
	for _, file := range files {
		var temp []Businesses
		csvReader(file, &temp)
		business = append(business, temp...)
	}

	return business
}

// Checkers
func curl(business Businesses, flagged *[]Businesses, review *[]Businesses) {
	url := business.Website
	cmd := exec.Command("curl", "-I", "-m", "5", url)
	stdout, err := cmd.Output()
	if err != nil {
		// fmt.Printf("%s | <FLAG>\n", url)
		*flagged = append(*flagged, business)

		return
	}
	data := strings.Split(string(stdout), "\n")
	statusCode := strings.TrimSpace(string([]rune(strings.Split(data[0], " ")[1])[0]))
	switch statusCode {
	case "3", "2":
		if reviewPages(url) {
			*flagged = append(*flagged, business)
			return
		}
		*review = append(*review, business)
	case "4":
		// fmt.Printf("%s | <NOT FOUND>\n", url)
		*flagged = append(*flagged, business)
	default:
		// fmt.Printf("%s | %s\n", url, statusCode)
		*flagged = append(*flagged, business)
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

func pushToSheet(flagged []Businesses, empty []Businesses, review []Businesses, filePath string) {
	ctx := context.Background()
	fmt.Println(filePath + "/" + os.Getenv("JWT_TOKEN"))
	b, err := ioutil.ReadFile(filePath + "/" + os.Getenv("JWT_TOKEN"))
	errCheck(err)
	conf, err := google.JWTConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	errCheck(err)

	client := conf.Client(ctx)

	srv, err := sheets.New(client)
	errCheck(err)

	spreadsheetID := os.Getenv("SPREADSHEET_ID")

	BatchUpdate("Flagged", flagged, spreadsheetID, srv, ctx)
	BatchUpdate("Empty", empty, spreadsheetID, srv, ctx)
	BatchUpdate("Review", review, spreadsheetID, srv, ctx)

}

func BatchUpdate(name string, data []Businesses, id string, srv *sheets.Service, ctx context.Context) {

	fmt.Printf("Pushing %s data to Sheets", name)
	rb := sheets.BatchUpdateValuesRequest{
		ValueInputOption: "USER_ENTERED",
	}

	rb.Data = append(rb.Data, &sheets.ValueRange{
		Range:  fmt.Sprintf("'%s'!A2:I500", name),
		Values: csvToInterface(data),
	})

	_, err := srv.Spreadsheets.Values.BatchUpdate(id, &rb).Context(ctx).Do()
	errCheck(err)
	fmt.Println("... Done!")

}

func csvToInterface(businesses []Businesses) [][]interface{} {
	var data [][]interface{}

	for _, row := range businesses {
		row_data := []interface{}{row.Name, row.Address, row.Website, row.Phone, row.Reviews, row.Rating, row.Verified, row.Category}
		data = append(data, row_data)
	}
	return data
}

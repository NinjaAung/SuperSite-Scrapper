package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/gocarina/gocsv"
	"github.com/gocolly/colly"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
	"googlemaps.github.io/maps"
)

type Business struct {
	Business string `csv:"Business"` // same as name
	Name     string `csv:"Name"`
	Email    string `csv:"Email"`
	Address  string `csv:"Address"`
	City     string `csv:"City"`
	State    string `csv:"State"`
	Zip      string `csv:"Zip"`
	Website  string `csv:"Website"`
	Phone    string `csv:"Phone"`
	Reviews  string `csv:"Reviews"`
	Rating   string `csv:"Ratings"`
	Verified string `csv:"Merchant Verified"`
	Category string `csv:"Category"`
	CID      string `csv:"Listing CID"`
	PlaceID  string `csv:"PlaceID"`
}

func main() {
	// Init Vars
	var (
		reviews  []Business
		flaggeds []Business
		emptys   []Business
	)

	// Move to working directory to exec
	e, _ := os.Executable()
	os.Chdir(path.Dir(e))

	// Program Start
	godotenv.Load()
	businesses := readAll(os.Getenv("FILE_PATH"))

	// Google API Interface
	ctx := context.Background()
	mClient, err := maps.NewClient(maps.WithAPIKey(os.Getenv("API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n\nParsing %v Business Records\n", len(businesses))
	if os.Getenv("SNAP") == "true" {
		var bCells []Business
		for _, b := range businesses {
			b.Name = b.Business
			b.PlaceID = findPlaceID(ctx, mClient, numberParse(b.Phone))
			if b.PlaceID != "" {
				b.Website = findWebsite(ctx, mClient, b.PlaceID)
			}
			bCells = append(bCells, b)
		}
		if len(bCells) != 0 {
			businesses = bCells
		}
	}
	for _, business := range businesses {
		if strings.ToLower(business.Reviews) == "no" {
			business.Reviews = "0"
		}

		switch business.Website {
		case "":
			emptys = append(emptys, business)
		case "http://godaddysites.com", "http://business.site":
			queryNumber := findPlaceID(ctx, mClient, numberParse(business.Website))

			if queryNumber == "" {
				emptys = append(emptys, business)
				break
			}

			business.Website = findWebsite(ctx, mClient, queryNumber)
			if business.Website == "" {
				emptys = append(emptys, business)
				break
			}

			fallthrough
		default:
			curl(business, &flaggeds, &reviews)
		}
	}

	// Push Finalization
	fmt.Println("\n---Batch Push---")
	pushToSheet(cleanDuplicates(flaggeds), cleanDuplicates(emptys), cleanDuplicates(reviews))
}

func cleanDuplicates(data []Business) []Business {
	check := make(map[string]Business)
	var b []Business
	for _, d := range data {
		check[d.Name] = d
	}
	for _, d := range check {
		b = append(b, d)
	}
	return b

}

func numberParse(number string) string {
	p := "+1"
	for _, char := range number {
		if unicode.IsDigit(char) {
			p += string(char)
		}
	}
	return p
}

func isPageValid(url string) bool {
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

func isPageHasFlash(url string) bool {
	c := colly.NewCollector()
	text := ""
	c.OnHTML("script", func(e *colly.HTMLElement) { text += e.Text })
	c.Visit(url)
	b, _ := regexp.MatchString(".swf", text)
	return b
}

// Checkers
func curl(business Business, flagged *[]Business, review *[]Business) {
	url := strings.TrimSpace(business.Website)
	cmd := exec.Command("curl", "-I", "-m", "5", url)
	stdout, err := cmd.Output()
	if err != nil {
		*flagged = append(*flagged, business)

		return
	}
	data := strings.Split(string(stdout), "\n")
	statusCode := strings.TrimSpace(string([]rune(strings.Split(data[0], " ")[1])[0]))
	switch statusCode {
	case "3", "2":
		if isPageValid(url) {
			*flagged = append(*flagged, business)
			return
		}

		if isPageHasFlash(url) {
			*flagged = append(*flagged, business)
			return
		}

		*review = append(*review, business)
	case "4":
		*flagged = append(*flagged, business)
	default:
		*flagged = append(*flagged, business)
	}
}

// CSV

func csvReader(filePath string, business *[]Business) {
	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.LazyQuotes = true
		r.TrimLeadingSpace = true
		return r
	})
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal("Could not read file", filePath, err)
	}
	err = gocsv.UnmarshalBytes(bytes, business)
	if err != nil {
		log.Fatal("Could not parse csv", filePath, err)
	}
}

func readAll(filePath string) []Business {
	files, err := filepath.Glob(filePath + "*.csv")
	if err != nil {
		log.Fatal("Could not read files", err)
	}
	var business []Business
	for _, file := range files {
		var temp []Business
		fmt.Println("Loading: ", file)
		csvReader(file, &temp)
		business = append(business, temp...)
	}
	return business
}

// Maps

func findWebsite(ctx context.Context, mClient *maps.Client, PlaceID string) string {
	req := &maps.PlaceDetailsRequest{
		PlaceID: PlaceID,
		Fields:  []maps.PlaceDetailsFieldMask{"website"},
	}
	resp, err := mClient.PlaceDetails(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	return resp.Website

}

func findPlaceID(ctx context.Context, mClient *maps.Client, query string) string {
	req := &maps.FindPlaceFromTextRequest{
		Input:     query,
		InputType: "phonenumber",
		Fields:    []maps.PlaceSearchFieldMask{"place_id"},
	}
	resp, err := mClient.FindPlaceFromText(ctx, req)
	if err != nil {
		log.Fatal(err)
	}
	if len(resp.Candidates) != 0 {
		return resp.Candidates[0].PlaceID
	}
	return ""
}

// Sheets

func csvToInterface(businesses []Business) [][]interface{} {
	var data [][]interface{}

	for _, row := range businesses {
		row_data := []interface{}{row.Name, strings.TrimSpace(fmt.Sprintf("%s %s, %s %s", row.Address, row.City, row.State, row.Zip)), row.Website, row.Phone, row.Reviews, row.Rating, row.Verified, row.Category}
		data = append(data, row_data)
	}
	return data
}

func BatchUpdate(name string, data []Business, id string, srv *sheets.Service, ctx context.Context) {
	fmt.Printf("Pushing %s data to Sheets", name)
	defer fmt.Println("... Done!")
	rb := sheets.BatchUpdateValuesRequest{ValueInputOption: "USER_ENTERED"}
	rb.Data = append(rb.Data, &sheets.ValueRange{Range: fmt.Sprintf("'%s'!A2:5000", name), Values: csvToInterface(data)})
	_, err := srv.Spreadsheets.Values.BatchUpdate(id, &rb).Context(ctx).Do()
	if err != nil {
		log.Fatal("Couldn't Update Spreadsheet", err)
	}
}

func pushToSheet(flagged, empty, review []Business) {

	spreadsheetID := os.Getenv("SPREADSHEET_ID")
	b, _ := os.ReadFile(os.Getenv("JWT_TOKEN"))

	conf, err := google.JWTConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatal("Could not load JWT Service Account JSON file", err)
	}

	ctx := context.Background()
	srv, _ := sheets.New(conf.Client(ctx))

	BatchUpdate("Flagged", flagged, spreadsheetID, srv, ctx)
	BatchUpdate("Empty", empty, spreadsheetID, srv, ctx)
	BatchUpdate("Review", review, spreadsheetID, srv, ctx)

}

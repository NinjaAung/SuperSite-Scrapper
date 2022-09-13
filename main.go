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
	"sync"
	"time"
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

var (
	snapChan    = make(chan Business)
	defaultChan = make(chan Business)
	wg          sync.WaitGroup
)

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
	fmt.Println("---Loading Files---")
	businesses := readAll(os.Getenv("FILE_PATH"))

	// Google API Interface
	ctx := context.Background()
	mClient, err := maps.NewClient(maps.WithAPIKey(os.Getenv("API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n---Processing %v Business Records---\n", len(businesses))
	start := time.Now()

	// SNAP Pre-Processing
	if os.Getenv("SNAP") == "true" {
		var bCells []Business
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go snapWorker(snapChan, &bCells, ctx, mClient)
		}
		for _, b := range businesses {
			snapChan <- b
		}
		close(snapChan)
		wg.Wait()

		if len(bCells) != 0 {
			businesses = bCells
		}
	}

	// Site Checker
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go defaultWorker(defaultChan, &flaggeds, &emptys, &reviews, ctx, mClient)
	}

	for _, b := range businesses {
		defaultChan <- b
	}
	close(defaultChan)
	wg.Wait()

	fmt.Printf("Processed %v in %v\n\n", len(businesses), time.Since(start))
	// Push Finalization
	fmt.Println("---Batch Push---")

	spreadsheetID := os.Getenv("SPREADSHEET_ID")
	b, _ := os.ReadFile(os.Getenv("JWT_TOKEN"))

	conf, err := google.JWTConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatal("Could not load JWT Service Account JSON file", err)
	}

	srv, _ := sheets.New(conf.Client(ctx))

	updateSheet("Flagged", flaggeds, spreadsheetID, srv, ctx)
	updateSheet("Empty", emptys, spreadsheetID, srv, ctx)
	updateSheet("Review", reviews, spreadsheetID, srv, ctx)
}

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
	numberParse := func(number string) string {
		p := "+1"
		for _, char := range number {
			if unicode.IsDigit(char) {
				p += string(char)
			}
		}
		return p
	}
	req := &maps.FindPlaceFromTextRequest{
		Input:     numberParse(query),
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

func updateSheet(name string, data []Business, id string, srv *sheets.Service, ctx context.Context) {
	csvToInterface := func(b []Business) (data [][]interface{}) {
		for _, row := range b {
			row_data := []interface{}{row.Name, strings.TrimSpace(fmt.Sprintf("%s %s %s %s", row.Address, row.City, row.State, row.Zip)), row.Website, row.Phone, row.Reviews, row.Rating, row.Verified, row.Category}
			data = append(data, row_data)
		}
		return data
	}

	b_data := csvToInterface(data)

	fmt.Printf("Pushing %v %s data to Sheets", len(data), name)
	defer fmt.Println("... Done!")
	rb := sheets.BatchUpdateValuesRequest{ValueInputOption: "USER_ENTERED"}
	rb.Data = append(rb.Data, &sheets.ValueRange{Range: fmt.Sprintf("'%s'!A2:5000", name), Values: b_data})
	_, err := srv.Spreadsheets.Values.BatchUpdate(id, &rb).Context(ctx).Do()
	if err != nil {
		log.Fatal("Couldn't Update Spreadsheet", err)
	}
}

func snapWorker(snapChan chan Business, bCells *[]Business, ctx context.Context, mClient *maps.Client) {
	for b := range snapChan {
		b.Name = b.Business
		b.PlaceID = findPlaceID(ctx, mClient, b.Phone)
		if b.PlaceID != "" {
			b.Website = findWebsite(ctx, mClient, b.PlaceID)
		}
		*bCells = append(*bCells, b)
	}
}

func defaultWorker(defaultChan chan Business, flaggeds, emptys, reviews *[]Business, ctx context.Context, mClient *maps.Client) {
	for b := range defaultChan {
		if strings.ToLower(b.Reviews) == "no" {
			b.Reviews = "0"
		}
		switch b.Website {
		case "":
			*emptys = append(*emptys, b)
		case "http://godaddysites.com", "http://business.site":
			queryNumber := findPlaceID(ctx, mClient, b.Phone)

			if queryNumber == "" {
				*emptys = append(*emptys, b)
				break
			}

			b.Website = findWebsite(ctx, mClient, queryNumber)
			if b.Website == "" {
				*emptys = append(*emptys, b)
				break
			}
			fallthrough
		default:
			curl(b, flaggeds, reviews)
		}
	}
	wg.Done()
}

// Checkers

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

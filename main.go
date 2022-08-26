package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

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

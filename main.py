import requests  # Read File or Read Google Sheets
import csv

response = requests.get('https://google.com')

with open("all-businesses.csv", "r") as f:
    business_reader: iter = csv.reader(f, delimiter=",", quotechar='"')
    next(business_reader)
    e = s = 0
    for row in business_reader:
        website = row[10].strip()
        if website == "":
            print(website, "<No Website>")
            continue

        try:
            resp = requests.get(website)
        except:
            print(website, "<No Response>")
            continue

        if resp.status_code == 403:
            print(website, "<No Authorization>")

        if resp.status_code == 404:
            print(website, "<Broken Url>")
        # TODO: Add OWNED BY DNS
        # TODO: Add minimal body text

# Package all categories and push to CSV or Google Sheet

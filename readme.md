# Broken Site Scraper

Scrap file to check if website is "broken" according to these patterns:

- Website has minimal details
  - Owned by domains
  - Not a lot of content in body
- Website 404
- Other possible page errors

## How To Use

1. Place supersite application in folder with csvs

```env
|- Folder
    |- supersite
    |- livermore.csv
    |- oakland.csv
    |- sf.csv
```

2. Create an `.env` file with these items

```env
API_KEY=        // Google Places API key from Google Console
SPREADSHEET_ID= // Spreadsheet you want to update
JWT_TOKEN=      // JSON file from google console
SNAP= // Set Snap processing to true or false
FILE_PATH= //Set folder path
```

3. run `./supersite` in terminal and it should start working

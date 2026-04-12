package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	gdeltLastUpdateURL  = "http://data.gdeltproject.org/gdeltv2/lastupdate.txt"
	maxEventsPerCountry = 10

	eventCodeIndex   = 26
	countryCodeIndex = 53
)

// FetchGDELTEvents downloads the latest GDELT update and returns a map of
// ISO country code -> list of CAMEO event codes.
func FetchGDELTEvents() (map[string][]string, error) {
	csvURL, err := resolveLatestCSVURL()
	if err != nil {
		return nil, fmt.Errorf("resolve latest url: %w", err)
	}

	records, err := downloadAndParseCSV(csvURL)
	if err != nil {
		return nil, fmt.Errorf("download csv: %w", err)
	}

	return groupEventsByCountry(records), nil
}

func resolveLatestCSVURL() (string, error) {
	resp, err := http.Get(gdeltLastUpdateURL)
	if err != nil {
		return "", fmt.Errorf("fetch lastupdate.txt: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read lastupdate body: %w", err)
	}

	// First line, third space-separated field is the CSV zip URL.
	firstLine := strings.Split(string(body), "\n")[0]
	fields := strings.Split(firstLine, " ")
	if len(fields) < 3 {
		return "", fmt.Errorf("unexpected lastupdate.txt format")
	}

	return fields[2], nil
}

func downloadAndParseCSV(zipURL string) ([][]string, error) {
	resp, err := http.Get(zipURL)
	if err != nil {
		return nil, fmt.Errorf("download zip: %w", err)
	}
	defer resp.Body.Close()

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read zip body: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	csvFile := findCSVInZip(zipReader)
	if csvFile == nil {
		return nil, fmt.Errorf("no CSV file found in zip")
	}

	f, err := csvFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open csv in zip: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = '\t'

	return reader.ReadAll()
}

func findCSVInZip(zr *zip.Reader) *zip.File {
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".CSV") {
			return f
		}
	}
	return nil
}

func groupEventsByCountry(records [][]string) map[string][]string {
	result := make(map[string][]string)

	for _, record := range records {
		if len(record) <= countryCodeIndex {
			continue
		}

		fips := record[countryCodeIndex]
		iso, ok := FipsToIso[fips]
		if !ok {
			continue
		}

		if len(result[iso]) < maxEventsPerCountry {
			result[iso] = append(result[iso], record[eventCodeIndex])
		}
	}

	return result
}

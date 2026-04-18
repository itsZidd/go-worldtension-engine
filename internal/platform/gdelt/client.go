package gdelt

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"go-worldtension-engine/internal/models"
)

const (
	gdeltLastUpdateURL = "http://data.gdeltproject.org/gdeltv2/lastupdate.txt"

	// GDELT CSV Column Indices
	eventCodeIndex   = 26
	goldsteinIndex   = 30
	numArticlesIndex = 31
	countryCodeIndex = 53
)

// FetchLatest downloads the CSV and passes it through the Quality Gate.
func FetchLatest() (map[string]map[string]*models.EventSummary, error) {
	csvURL, err := resolveLatestCSVURL()
	if err != nil {
		return nil, fmt.Errorf("resolve latest url: %w", err)
	}

	records, err := downloadAndParseCSV(csvURL)
	if err != nil {
		return nil, fmt.Errorf("download csv: %w", err)
	}

	return Process(records), nil
}

// Process is your "Quality Gate". It deduplicates and applies the Goldstein weights.
func Process(records [][]string) map[string]map[string]*models.EventSummary {
	result := make(map[string]map[string]*models.EventSummary)

	for _, record := range records {
		// 1. Bounds Check: Prevent panics if the CSV row is malformed
		if len(record) <= countryCodeIndex {
			continue
		}

		// 2. Location Check
		fips := record[countryCodeIndex]
		if fips == "" {
			continue
		}

		iso, ok := models.FipsToIso[fips]
		if !ok {
			continue
		}

		// 3. Extract the raw strings
		eventCode := record[eventCodeIndex]

		// Parse floats safely (if it fails, it defaults to 0, which is fine)
		goldstein, _ := strconv.ParseFloat(record[goldsteinIndex], 64)
		articles, _ := strconv.ParseFloat(record[numArticlesIndex], 64)

		// 4. Initialize maps if this is the first time seeing this country/event
		if result[iso] == nil {
			result[iso] = make(map[string]*models.EventSummary)
		}
		if _, exists := result[iso][eventCode]; !exists {
			result[iso][eventCode] = &models.EventSummary{Code: eventCode}
		}

		// 5. The Aggregation Logic
		summary := result[iso][eventCode]
		summary.Count++

		// Weight the intensity of the event by how many articles are talking about it
		summary.TotalGoldstein += (goldstein * articles)
	}

	return result
}

// --- Your original helper functions go below ---

func resolveLatestCSVURL() (string, error) {
	resp, err := http.Get(gdeltLastUpdateURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

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
		return nil, err
	}
	defer resp.Body.Close()

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}

	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".CSV") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			reader := csv.NewReader(rc)
			reader.Comma = '\t'
			return reader.ReadAll()
		}
	}
	return nil, fmt.Errorf("no CSV file found in zip")
}

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	GoldsteinIndex   = 30
	CountryCodeIndex = 53 // GDELT 2.0 FIPS Code
)

type CountryStat struct {
	TotalScore float64
	EventCount int
}

func main() {
	// 1. Connect to Supabase
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(ctx)

	// 2. Fetch & Parse GDELT
	resp, err := http.Get("http://data.gdeltproject.org/gdeltv2/lastupdate.txt")
	if err != nil {
		log.Fatalf("Failed to fetch master list: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	latestEventUrl := strings.Split(strings.Split(string(bodyBytes), "\n")[0], " ")[2]

	zipResp, err := http.Get(latestEventUrl)
	if err != nil {
		log.Fatalf("Failed to download zip: %v", err)
	}
	defer zipResp.Body.Close()

	zipBytes, _ := io.ReadAll(zipResp.Body)
	zipReader, _ := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))

	var csvFile *zip.File
	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".CSV") {
			csvFile = f
			break
		}
	}

	f, _ := csvFile.Open()
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	stats := make(map[string]*CountryStat)

	// Read and aggregate
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) <= CountryCodeIndex {
			continue
		}

		fipsCode := record[CountryCodeIndex]
		goldsteinStr := record[GoldsteinIndex]

		if fipsCode == "" || goldsteinStr == "" {
			continue
		}

		// Only track countries in our FIPS-to-ISO dictionary
		_, exists := FipsToIso[fipsCode]
		if !exists {
			continue
		}

		goldsteinScore, err := strconv.ParseFloat(goldsteinStr, 64)
		if err == nil {
			if _, exists := stats[fipsCode]; !exists {
				stats[fipsCode] = &CountryStat{}
			}
			stats[fipsCode].TotalScore += goldsteinScore
			stats[fipsCode].EventCount++
		}
	}

	// 3. Push to Supabase Database (Upsert)
	for fips, stat := range stats {
		isoCode := FipsToIso[fips]
		tensionScore := (stat.TotalScore / float64(stat.EventCount)) * -1.0

		// INSERT OR UPDATE logic (Upsert)
		query := `
			INSERT INTO world_tension (iso_code, fips_code, event_count, tension_score, last_updated)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (iso_code)
			DO UPDATE SET
				event_count = EXCLUDED.event_count,
				tension_score = EXCLUDED.tension_score,
				last_updated = NOW();
		`
		_, err := conn.Exec(ctx, query, isoCode, fips, stat.EventCount, tensionScore)
		if err != nil {
			log.Printf("Failed to insert %s: %v", isoCode, err)
		} else {
			fmt.Printf("Updated %s: Tension %.2f (Events: %d)\n", isoCode, tensionScore, stat.EventCount)
		}
	}

	fmt.Println("Daily Turn Complete. Data pushed to Supabase.")
}

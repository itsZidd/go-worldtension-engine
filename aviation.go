package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	openskyTokenURL  = "https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token"
	openskyStatesURL = "https://opensky-network.org/api/states/all"
)

// countryBBox defines the bounding box [minLat, maxLat, minLon, maxLon] for each ISO3.
// Used to query OpenSky for flights specifically near that country.
var countryBBox = map[string][4]float64{
	"USA": {24.0, 49.0, -125.0, -66.0},
	"RUS": {41.0, 82.0, 19.0, 180.0},
	"CHN": {18.0, 53.0, 73.0, 135.0},
	"IND": {8.0, 37.0, 68.0, 97.0},
	"PAK": {23.0, 37.0, 60.0, 77.0},
	"IRN": {25.0, 40.0, 44.0, 64.0},
	"IRQ": {29.0, 38.0, 38.0, 49.0},
	"ISR": {29.0, 34.0, 34.0, 36.0},
	"UKR": {44.0, 53.0, 22.0, 41.0},
	"KOR": {34.0, 38.0, 126.0, 130.0},
	"PRK": {37.0, 43.0, 124.0, 131.0},
	"TWN": {21.0, 26.0, 119.0, 122.0},
	"SYR": {32.0, 37.0, 35.0, 43.0},
	"YEM": {12.0, 19.0, 42.0, 55.0},
	"LBN": {33.0, 35.0, 35.0, 37.0},
	"SAU": {16.0, 32.0, 36.0, 56.0},
	"TUR": {36.0, 42.0, 26.0, 45.0},
	"GBR": {49.0, 61.0, -8.0, 2.0},
	"FRA": {42.0, 51.0, -5.0, 8.0},
	"DEU": {47.0, 55.0, 6.0, 15.0},
	"NOR": {57.0, 71.0, 4.0, 31.0},
	"FIN": {59.0, 70.0, 20.0, 32.0},
	"POL": {49.0, 55.0, 14.0, 24.0},
	"JPN": {30.0, 46.0, 129.0, 146.0},
	"AUS": {-44.0, -10.0, 113.0, 154.0},
	"EGY": {22.0, 32.0, 24.0, 37.0},
	"DJI": {11.0, 13.0, 41.0, 44.0},
	"NGA": {4.0, 14.0, 2.0, 15.0},
	"ETH": {3.0, 15.0, 33.0, 48.0},
	"SOM": {-2.0, 12.0, 40.0, 52.0},
}

var (
	// Aviation cache: ISO3 → military flight count near that country
	aviationCache   map[string]int
	aviationMu      sync.Mutex
	aviationFetched bool

	openskyToken       string
	openskyTokenExpiry time.Time
)

// AviationData holds per-country aviation signals
type AviationData struct {
	MilitaryFlights int
	AirspaceClosed  bool
}

func FetchMilitaryFlights(iso string) (int, error) {
	aviationMu.Lock()
	defer aviationMu.Unlock()

	if !aviationFetched {
		aviationCache = make(map[string]int)
		if err := refreshAviationCache(); err != nil {
			log.Printf("[aviation] fetch failed, will retry next run: %v", err)
			return 0, err
		}
		aviationFetched = true
	}

	return aviationCache[iso], nil
}

func refreshAviationCache() error {
	if err := ensureOpenskyToken(); err != nil {
		return fmt.Errorf("aviation auth: %w", err)
	}

	if err := fetchMilitaryByCountry(); err != nil {
		log.Printf("[aviation] per-country fetch failed (non-fatal): %v", err)
	}

	log.Printf("[aviation] cache refreshed: %d countries with flight data", len(aviationCache))
	return nil
}

// fetchMilitaryByCountry queries OpenSky with per-country bounding boxes.
// Only queries countries we have bbox data for — high-tension regions.
func fetchMilitaryByCountry() error {
	militaryPrefixes := []string{
		"RCH",                      // US Air Force airlift
		"NATO",                     // NATO ops
		"RRR",                      // UK RAF
		"FORTE",                    // US reconnaissance
		"JAKE",                     // US Navy
		"LAGR",                     // US Reaper drones
		"HOMER",                    // USAF
		"GHOST",                    // various military
		"REACH",                    // USAF AMC
		"IRON",                     // various
		"TOPGUN", "VIPER", "EAGLE", // training, often used in exercises
	}

	client := &http.Client{Timeout: 10 * time.Second}
	fetched := 0

	for iso, bbox := range countryBBox {
		// OpenSky bounding box: laMin, loMin, laMax, loMax
		bboxURL := fmt.Sprintf(
			"%s?lamin=%.1f&lomin=%.1f&lamax=%.1f&lomax=%.1f",
			openskyStatesURL,
			bbox[0], bbox[2], bbox[1], bbox[3],
		)

		req, err := http.NewRequest("GET", bboxURL, nil)
		if err != nil {
			log.Printf("[aviation] %s: request build failed: %v", iso, err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+openskyToken)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[aviation] %s: request failed: %v", iso, err)
			continue
		}

		// Handle token expiry mid-loop
		if resp.StatusCode == 401 {
			resp.Body.Close()
			log.Printf("[aviation] 401 mid-loop, refreshing token...")
			if err := getOpenskyToken(); err != nil {
				log.Printf("[aviation] token refresh failed: %v", err)
				continue
			}
			req2, _ := http.NewRequest("GET", bboxURL, nil)
			req2.Header.Set("Authorization", "Bearer "+openskyToken)
			resp, err = client.Do(req2)
			if err != nil {
				log.Printf("[aviation] %s: retry failed: %v", iso, err)
				continue
			}
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			log.Printf("[aviation] %s: status %d", iso, resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[aviation] %s: read failed: %v", iso, err)
			continue
		}

		var result struct {
			States [][]interface{} `json:"states"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			log.Printf("[aviation] %s: decode failed: %v", iso, err)
			continue
		}

		militaryCount := 0
		for _, flight := range result.States {
			if len(flight) < 2 {
				continue
			}
			callsign, ok := flight[1].(string)
			if !ok {
				continue
			}
			callsign = strings.TrimSpace(strings.ToUpper(callsign))
			for _, prefix := range militaryPrefixes {
				if strings.HasPrefix(callsign, prefix) {
					militaryCount++
					break
				}
			}
		}

		aviationCache[iso] = militaryCount
		fetched++

		if militaryCount > 0 {
			log.Printf("[aviation] %s: %d flights in bbox, %d military", iso, len(result.States), militaryCount)
		}

		// Rate limit: OpenSky allows ~1 req/s on free OAuth2 tier
		time.Sleep(1 * time.Second)
	}

	log.Printf("[aviation] fetched %d/%d countries", fetched, len(countryBBox))
	return nil
}

func getOpenskyToken() error {
	clientID := os.Getenv("OPENSKY_CLIENT_ID")
	clientSecret := os.Getenv("OPENSKY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("OPENSKY_CLIENT_ID or OPENSKY_CLIENT_SECRET not set")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(openskyTokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("token request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("token decode failed: %v", err)
	}
	if result.AccessToken == "" {
		return fmt.Errorf("empty access_token in response")
	}

	openskyToken = result.AccessToken
	openskyTokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-30) * time.Second)
	log.Printf("[aviation] token acquired, expires in %ds", result.ExpiresIn)
	return nil
}

func ensureOpenskyToken() error {
	if openskyToken == "" || time.Now().After(openskyTokenExpiry) {
		return getOpenskyToken()
	}
	return nil
}

package worldbank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	baseURL = "https://api.worldbank.org/v2/country/%s/indicator/NV.IND.MANF.ZS?per_page=5&format=json"
)

type WBData struct {
	Date  string   `json:"date"`
	Value *float64 `json:"value"`
}

// reusable client
var client = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		ForceAttemptHTTP2: false,

		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	},
}

func FetchIndustrialBaseline(iso string) (float64, error) {
	url := fmt.Sprintf(baseURL, iso)

	var lastErr error

	for attempt := 1; attempt <= 3; attempt++ {
		score, err := fetchOnce(url)
		if err == nil {
			return score, nil
		}

		lastErr = err
		fmt.Printf("WorldBank retry %d failed for %s: %v\n", attempt, iso, err)

		time.Sleep(time.Duration(attempt*2) * time.Second)
	}

	return 0, fmt.Errorf("worldbank fetch failed after retries: %w", lastErr)
}

func fetchOnce(url string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/123 Safari/537.36")
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	start := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request error after %s: %w", time.Since(start), err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed reading body: %w", err)
	}

	bodyBytes = bytes.TrimPrefix(bodyBytes, []byte("\xef\xbb\xbf"))
	bodyBytes = bytes.TrimSpace(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("http %d", resp.StatusCode)
	}

	var rawResponse []interface{}
	if err := json.Unmarshal(bodyBytes, &rawResponse); err != nil {
		return 0, fmt.Errorf("json decode failed: %w", err)
	}

	if len(rawResponse) < 2 || rawResponse[1] == nil {
		return 0, fmt.Errorf("no dataset returned")
	}

	innerData, _ := json.Marshal(rawResponse[1])

	var dataPoints []WBData
	if err := json.Unmarshal(innerData, &dataPoints); err != nil {
		return 0, fmt.Errorf("parse datapoints failed: %w", err)
	}

	for _, point := range dataPoints {
		if point.Value != nil {
			score := (*point.Value / 25.0) * 100.0

			if score > 100 {
				score = 100
			}
			if score < 0 {
				score = 0
			}

			return score, nil
		}
	}

	return 0, fmt.Errorf("all values null")
}

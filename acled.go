package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type ACLEDClient struct {
	Email    string
	Password string
	Token    string
	Expiry   time.Time
}

// Login fetches a fresh JWT token from ACLED
func (c *ACLEDClient) Login() error {
	loginURL := "https://acleddata.com/user/login?_format=json"
	payload, _ := json.Marshal(map[string]string{
		"name": c.Email,
		"pass": c.Password,
	})

	resp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read raw body first so we can log it
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("acled read body: %w", err)
	}

	log.Printf("[acled] login status: %d, raw response: %s", resp.StatusCode, string(body))

	var res struct {
		AccessToken string `json:"access_token"`
		Token       string `json:"token"`      // some versions use this
		CSRFToken   string `json:"csrf_token"` // Drupal login returns this
		LogoutToken string `json:"logout_token"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("acled decode: %w", err)
	}

	// Try all possible token fields
	token := res.AccessToken
	if token == "" {
		token = res.Token
	}
	if token == "" {
		token = res.CSRFToken
	}

	log.Printf("[acled] token fields — access_token: %d chars, token: %d chars, csrf_token: %d chars",
		len(res.AccessToken), len(res.Token), len(res.CSRFToken))

	if token == "" {
		return fmt.Errorf("acled login returned no usable token — check credentials or API changes")
	}

	c.Token = token
	c.Expiry = time.Now().Add(time.Hour * 24)
	return nil
}

// FetchRecentEvents gets the count of conflict events for a country ISO
func (c *ACLEDClient) FetchRecentEvents(iso string) (int, error) {
	// Re-login if token is expired or missing
	if c.Token == "" || time.Now().After(c.Expiry) {
		if err := c.Login(); err != nil {
			return 0, fmt.Errorf("auth failure: %v", err)
		}
	}

	// Filter for events in the last 48 hours to ensure we don't miss data
	dateLimit := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	url := fmt.Sprintf("https://acleddata.com/api/acled/read?iso3=%s&event_date=%s&event_date_where=>", iso, dateLimit)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []interface{} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return len(result.Data), nil
}

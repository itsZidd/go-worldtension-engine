package main

// CountryUpdate holds the AI-analyzed metrics for a single country.
type CountryUpdate struct {
	ISOCode    string  `json:"iso_code"`
	Tension    float64 `json:"tension"`
	Stability  int     `json:"stability"`
	Industrial int     `json:"industrial"`
	Report     string  `json:"report"`
}

// AIResponse is the expected top-level JSON structure from Gemini.
type AIResponse struct {
	Updates []CountryUpdate `json:"updates"`
}

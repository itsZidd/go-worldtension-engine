package models

// EventSummary holds the aggregated metrics for a specific event type.
// This is the output of the GDELT "Quality Gate".
type EventSummary struct {
	Code           string
	Count          int
	TotalGoldstein float64 // Represents the intensity * volume of the event
}

// CountrySignal represents the combined intelligence for a single country
// before it is sent to the AI for analysis.
type CountrySignal struct {
	ISOCode        string
	Events         map[string]*EventSummary // Keyed by Event Code (e.g., "14" for Protest)
	BaseStability  float64                  // World Bank / WGI Anchor
	BaseIndustrial float64                  // World Bank Anchor
}

// TensionSnapshot represents the final calculated metrics to be saved to Supabase.
type TensionSnapshot struct {
	ISOCode            string
	TensionScore       float64
	Stability          float64
	IndustrialCapacity float64
	EventCount         int
	IntelReport        string // The Gemini-generated briefing
}

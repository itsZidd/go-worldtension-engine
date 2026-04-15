package main

import "time"

type RawDataPoint struct {
	PortThroughput float64 `json:"port_throughput"` // % change
	MilFlightCount int     `json:"mil_flight_count"`
}

type CountryInput struct {
	GDELTEvents []string     `json:"gdelt_events"`
	HardData    RawDataPoint `json:"hard_data"`
}

type CountryUpdate struct {
	ISOCode       string    `json:"iso_code"`
	Tension       float64   `json:"tension"`
	Stability     int       `json:"stability"`
	Industrial    int       `json:"industrial"`
	PrimaryDriver string    `json:"primary_driver"`
	Report        string    `json:"report"`
	LastUpdate    time.Time `json:"last_update"`
}

type AIResponse struct {
	Updates []CountryUpdate `json:"updates"`
}

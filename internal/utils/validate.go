package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

type DataReadiness struct {
	ISO      string
	Maritime SourceStatus
	Aviation SourceStatus
	// ACLED SourceStatus // REMOVE
	GDELT SourceStatus
}

type SourceStatus struct {
	OK    bool
	Value string
	Err   error
}

// ValidateCountryInput checks all four sources before the data is passed to AI.
// Returns an error only if ALL sources failed (total blackout).
func ValidateCountryInput(iso string, input CountryInput) (DataReadiness, error) {
	raw := input.HardData
	r := DataReadiness{ISO: iso}

	// Maritime — zero is valid for landlocked countries, so just warn
	if raw.PortThroughput == 0 {
		r.Maritime = SourceStatus{OK: false, Err: errors.New("zero — landlocked or fetch failed")}
	} else {
		r.Maritime = SourceStatus{OK: true, Value: fmt.Sprintf("%.2f%% change", raw.PortThroughput)}
	}

	// Aviation
	if raw.MilFlightCount == 0 {
		r.Aviation = SourceStatus{OK: false, Err: errors.New("zero — no activity or fetch failed")}
	} else {
		r.Aviation = SourceStatus{OK: true, Value: fmt.Sprintf("%d military flights", raw.MilFlightCount)}
	}

	// GDELT — lives on CountryInput, not RawDataPoint
	if len(input.GDELTEvents) == 0 {
		r.GDELT = SourceStatus{OK: false, Err: errors.New("no events returned")}
	} else {
		r.GDELT = SourceStatus{OK: true, Value: fmt.Sprintf("%d event codes", len(input.GDELTEvents))}
	}

	logReadiness(r)

	if !r.Maritime.OK && !r.Aviation.OK && !r.GDELT.OK {
		return r, fmt.Errorf("[%s] all 3 sources failed — skipping AI call", iso)
	}

	return r, nil
}

func logReadiness(r DataReadiness) {
	lines := []string{fmt.Sprintf("[preflight] %s", r.ISO)}
	lines = append(lines, statusLine("maritime", r.Maritime))
	lines = append(lines, statusLine("aviation", r.Aviation))
	lines = append(lines, statusLine("gdelt   ", r.GDELT))
	log.Println(strings.Join(lines, "\n  "))
}

func statusLine(name string, s SourceStatus) string {
	if s.OK {
		return fmt.Sprintf("%s  OK   %s", name, s.Value)
	}
	return fmt.Sprintf("%s  WARN %v", name, s.Err)
}

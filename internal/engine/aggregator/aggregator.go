package aggregator

import (
	"math"

	"go-worldtension-engine/internal/models" // Adjust if your module name is different
)

// Process combines live signals and static anchors to create the final game state.
func Process(
	liveSignals map[string]map[string]*models.EventSummary,
	industrialAnchors map[string]float64, // Pass the World Bank data here
) map[string]*models.TensionSnapshot {

	snapshots := make(map[string]*models.TensionSnapshot)

	for iso, events := range liveSignals {
		var totalNegativeSignal float64
		var totalEventCount int

		// 1. Calculate the Live Signal (Sum of all negative Goldstein intensity)
		for _, summary := range events {
			totalEventCount += summary.Count
			// We only care about negative events for Tension. (Fighting, Protests, etc.)
			if summary.TotalGoldstein < 0 {
				totalNegativeSignal += summary.TotalGoldstein
			}
		}

		// 2. Fetch the "Slow Anchor" (Default to 50 if we don't have World Bank data yet)
		baseIndustrial, exists := industrialAnchors[iso]
		if !exists {
			baseIndustrial = 50.0
		}

		// --- THE SIMULATION MATH ---

		// Tension Math: We cap the raw negative signal so one crazy day doesn't break the 0-100 scale.
		// For example, an intensity of -500 maps to ~80 Tension.
		rawTension := math.Abs(totalNegativeSignal) / 40.0
		if rawTension > 100 {
			rawTension = 100
		}

		// Industrial Math:
		// Base (70%) + Live Adjustments (30%)
		// Heavy fighting (high tension) temporarily reduces industrial output!
		industrialPenalty := (rawTension / 100.0) * 30.0
		currentIndustrial := baseIndustrial - industrialPenalty
		if currentIndustrial < 0 {
			currentIndustrial = 0
		}

		// 3. Save to Snapshot
		snapshots[iso] = &models.TensionSnapshot{
			ISOCode:            iso,
			TensionScore:       rawTension,
			IndustrialCapacity: currentIndustrial,
			EventCount:         totalEventCount,
		}
	}

	return snapshots
}

package main

func CalculateWeightedTension(raw RawDataPoint, aiTension float64) float64 {
	// 1. Port Score: A 20% drop ( -20.0 ) results in a 10.0 score
	portScore := 0.0
	if raw.PortThroughput < 0 {
		portScore = (raw.PortThroughput * -1) * 0.5
	}
	if portScore > 10.0 {
		portScore = 10.0
	}

	// 2. Flight Score: 5+ military flights result in a 10.0 score
	flightScore := float64(raw.MilFlightCount) * 2.0
	if flightScore > 10.0 {
		flightScore = 10.0
	}

	// 3. Combine Hard Scores
	var combinedHardScore float64
	if raw.PortThroughput == 0 {
		// If landlocked/no port data, rely entirely on aviation for the hard anchor
		combinedHardScore = flightScore
	} else {
		combinedHardScore = (portScore + flightScore) / 2.0
	}

	// 4. Final Blend: 40% Hard Data, 60% AI Narrative Vibe
	finalTension := (combinedHardScore * 0.4) + (aiTension * 0.6)

	return clampFloat(finalTension, 0.0, 10.0)
}

func clampFloat(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

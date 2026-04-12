package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

const upsertQuery = `
	INSERT INTO world_tension (
		iso_code, fips_code, event_count, tension_score,
		stability_score, industrial_capacity, recent_activity, last_updated
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	ON CONFLICT (iso_code)
	DO UPDATE SET
		fips_code            = EXCLUDED.fips_code,
		event_count          = EXCLUDED.event_count,
		tension_score        = EXCLUDED.tension_score,
		stability_score      = EXCLUDED.stability_score,
		industrial_capacity  = EXCLUDED.industrial_capacity,
		recent_activity      = EXCLUDED.recent_activity,
		last_updated         = NOW();`

const decayQuery = `
	UPDATE world_tension
	SET
		tension_score       = GREATEST(tension_score - 0.5, 0),
		stability_score     = LEAST(stability_score + 2, 100),
		industrial_capacity = LEAST(industrial_capacity + 1, 100),
		recent_activity     = 'No significant activity. Scores normalizing.'
	WHERE last_updated < NOW() - INTERVAL '48 hours';`

// UpsertCountryUpdates writes all AI-analyzed country metrics to the database.
// Values are clamped to valid ranges before writing to prevent bad AI outputs
// from corrupting the dataset.
func UpsertCountryUpdates(ctx context.Context, db *pgx.Conn, updates []CountryUpdate, rawData map[string][]string) error {
	var errCount int

	for _, u := range updates {
		u = clampUpdate(u)

		fips := IsoToFips[u.ISOCode]
		eventCount := len(rawData[u.ISOCode])

		_, err := db.Exec(ctx, upsertQuery,
			u.ISOCode, fips, eventCount,
			u.Tension, u.Stability, u.Industrial,
			u.Report,
		)
		if err != nil {
			log.Printf("[DB] Upsert failed for %s: %v", u.ISOCode, err)
			errCount++
			continue
		}

		fmt.Printf("[DB] %s — tension: %.1f, stability: %d%%, industrial: %d%%\n",
			u.ISOCode, u.Tension, u.Stability, u.Industrial)
	}

	if errCount > 0 {
		return fmt.Errorf("%d upsert(s) failed out of %d", errCount, len(updates))
	}
	return nil
}

// ApplyDecay nudges stale countries (no update in 48h) back toward neutral.
// This prevents old crisis scores from persisting indefinitely when a country
// falls off GDELT's radar.
func ApplyDecay(ctx context.Context, db *pgx.Conn) error {
	tag, err := db.Exec(ctx, decayQuery)
	if err != nil {
		return fmt.Errorf("decay query: %w", err)
	}

	affected := tag.RowsAffected()
	if affected > 0 {
		fmt.Printf("[DB] Decay applied to %d stale country record(s).\n", affected)
	}
	return nil
}

// clampUpdate enforces valid ranges on all AI-generated numeric fields
// before they reach the database.
func clampUpdate(u CountryUpdate) CountryUpdate {
	u.Tension = clampFloat(u.Tension, 0.0, 10.0)
	u.Stability = int(clampFloat(float64(u.Stability), 0, 100))
	u.Industrial = int(clampFloat(float64(u.Industrial), 0, 100))
	return u
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

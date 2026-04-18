package gdelt

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

// Import your shared models

// Added primary_driver as $8
const upsertQuery = `
	INSERT INTO world_tension (
		iso_code, fips_code, event_count, tension_score,
		stability, industrial_capacity, recent_activity, primary_driver, last_updated
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	ON CONFLICT (iso_code)
	DO UPDATE SET
		fips_code           = EXCLUDED.fips_code,
		event_count         = EXCLUDED.event_count,
		tension_score       = EXCLUDED.tension_score,
		stability           = EXCLUDED.stability,
		industrial_capacity = EXCLUDED.industrial_capacity,
		recent_activity     = EXCLUDED.recent_activity,
		primary_driver      = EXCLUDED.primary_driver,
		last_updated        = NOW();`

// When a country decays to normal, we reset the driver so the map tooltip clears out
const decayQuery = `
	UPDATE world_tension
	SET
		tension_score       = GREATEST(tension_score - 0.5, 0),
		stability           = LEAST(stability + 2, 100),
		industrial_capacity = LEAST(industrial_capacity + 1, 100),
		recent_activity     = 'No significant activity. Scores normalizing.',
		primary_driver      = 'Normalization'
	WHERE last_updated < NOW() - INTERVAL '48 hours';`

// Updated rawData type to map[string]CountryInput
func UpsertCountryUpdates(ctx context.Context, db *pgx.Conn, updates []CountryUpdate, rawData map[string]CountryInput) error {
	for _, u := range updates {
		inputData := rawData[u.ISOCode]
		totalEvents := len(inputData.GDELTEvents) // Just GDELT now

		_, err := db.Exec(ctx, upsertQuery,
			u.ISOCode,
			IsoToFips[u.ISOCode],
			totalEvents,
			u.Tension,
			u.Stability,
			u.Industrial,
			u.Report,
			u.PrimaryDriver,
		)
		if err != nil {
			log.Printf("[DB] Fail %s: %v", u.ISOCode, err)
		}
	}
	return nil
}

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

func clampUpdate(u CountryUpdate) CountryUpdate {
	u.Tension = clampFloat(u.Tension, 0.0, 10.0)
	u.Stability = int(clampFloat(float64(u.Stability), 0, 100))
	u.Industrial = int(clampFloat(float64(u.Industrial), 0, 100))
	return u
}

func CleanupOldData(db *sql.DB) error {
	// Delete anything older than 365 days
	query := `DELETE FROM country_daily_metrics WHERE recorded_at < NOW() - INTERVAL '1 year'`
	_, err := db.Exec(query)
	return err
}

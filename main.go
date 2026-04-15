package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"github.com/jackc/pgx/v5"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()

	cfg := mustLoadConfig()
	db := mustConnectDB(ctx, cfg.DatabaseURL)
	defer db.Close(ctx)

	aiClient := mustConnectAI(ctx, cfg.GeminiKey)
	defer aiClient.Close()

	// 1. Fetch GDELT
	fmt.Println("Scraping GDELT...")
	gdeltData, err := FetchGDELTEvents()
	if err != nil {
		log.Fatalf("GDELT fetch failed: %v", err)
	}

	countryInputs := make(map[string]CountryInput)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Semaphore limits to 5 concurrent API requests
	sem := make(chan struct{}, 5)

	// Inside main() loop:
	for iso, events := range gdeltData {
		wg.Add(1)
		sem <- struct{}{}
		go func(code string, evts []string) {
			defer wg.Done()
			defer func() { <-sem }()

			portChange, _ := FetchPortThroughput(code)
			flightCount, _ := FetchMilitaryFlights(code)

			mu.Lock()
			countryInputs[code] = CountryInput{
				GDELTEvents: evts,
				HardData: RawDataPoint{
					PortThroughput: portChange,
					MilFlightCount: flightCount,
				},
			}
			mu.Unlock()
		}(iso, events)
	}

	wg.Wait()
	fmt.Printf("Data aggregated for %d countries.\n", len(countryInputs))

	// Preflight check — remove total blackouts before sending to AI
	validated := make(map[string]CountryInput, len(countryInputs))
	for iso, input := range countryInputs {
		if _, err := ValidateCountryInput(iso, input); err != nil {
			log.Printf("[preflight] skipping %s: %v", iso, err)
			continue
		}
		validated[iso] = input
	}
	fmt.Printf("[preflight] %d/%d countries passed validation.\n", len(validated), len(countryInputs))

	// 3. Analyze with AI
	fmt.Println("Sending to Gemini for analysis...")
	updates, err := AnalyzeCountries(ctx, aiClient, validated)
	if err != nil {
		log.Fatalf("AI analysis failed: %v", err)
	}

	// 4. THE AGGREGATOR
	for i := range updates {
		iso := updates[i].ISOCode
		raw := countryInputs[iso].HardData
		updates[i].Tension = CalculateWeightedTension(raw, updates[i].Tension)
	}

	// 5. Persist to database
	if err := UpsertCountryUpdates(ctx, db, updates, countryInputs); err != nil {
		log.Printf("DB upsert had errors: %v", err)
	}

	// 6. Decay stale records
	if err := ApplyDecay(ctx, db); err != nil {
		log.Printf("Decay step failed: %v", err)
	}

	fmt.Println("Daily Turn Moderation Complete.")

	if cfg.VercelHook != "" {
		fmt.Println("Triggering Frontend Rebuild...")
		if _, err := http.Post(cfg.VercelHook, "application/json", nil); err != nil {
			log.Printf("Vercel deploy hook failed: %v", err)
		}
	}
}

// --- Config & Clients ---

type config struct {
	DatabaseURL string
	GeminiKey   string
	VercelHook  string
}

func mustLoadConfig() config {
	dbURL := os.Getenv("DATABASE_URL")
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if dbURL == "" || geminiKey == "" {
		log.Fatal("Missing required env vars: DATABASE_URL and GEMINI_API_KEY must be set.")
	}
	return config{
		DatabaseURL: dbURL,
		GeminiKey:   geminiKey,
		VercelHook:  os.Getenv("VERCEL_DEPLOY_HOOK"),
	}
}

func mustConnectDB(ctx context.Context, url string) *pgx.Conn {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	return conn
}

func mustConnectAI(ctx context.Context, apiKey string) *genai.Client {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Gemini client failed: %v", err)
	}
	return client
}

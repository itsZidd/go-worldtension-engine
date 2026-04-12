package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

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

	// 1. Fetch & parse GDELT events
	fmt.Println("Scraping GDELT for latest geopolitical events...")
	rawData, err := FetchGDELTEvents()
	if err != nil {
		log.Fatalf("GDELT fetch failed: %v", err)
	}
	fmt.Printf("GDELT Scan: Found activity for %d unique countries.\n", len(rawData))

	// 2. Analyze with AI
	fmt.Printf("Sending %d countries to Gemini for analysis...\n", len(rawData))
	updates, err := AnalyzeCountries(ctx, aiClient, rawData)
	if err != nil {
		log.Fatalf("AI analysis failed: %v", err)
	}

	// 3. Persist to database
	if err := UpsertCountryUpdates(ctx, db, updates, rawData); err != nil {
		log.Printf("DB upsert had errors: %v", err)
	}

	// 4. Decay stale records toward neutral
	if err := ApplyDecay(ctx, db); err != nil {
		log.Printf("Decay step failed: %v", err)
	}

	fmt.Println("Daily Turn Moderation Complete.")

	// 5. Trigger frontend rebuild if configured
	if cfg.VercelHook != "" {
		fmt.Println("Triggering Frontend Rebuild...")
		if _, err := http.Post(cfg.VercelHook, "application/json", nil); err != nil {
			log.Printf("Vercel deploy hook failed: %v", err)
		}
	}
}

// --- Config ---

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

// --- Clients ---

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

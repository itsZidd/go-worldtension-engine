package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/jackc/pgx/v5"
	"google.golang.org/api/option"
)

const (
	EventCodeIndex   = 26 // Using EventCode for context instead of just Goldstein
	CountryCodeIndex = 53
)

func main() {
	ctx := context.Background()

	// 1. Setup Clients (Supabase & Gemini)
	dbUrl := os.Getenv("DATABASE_URL")
	geminiKey := os.Getenv("GEMINI_API_KEY")

	conn, err := pgx.Connect(ctx, dbUrl)
	if err != nil {
		log.Fatalf("DB Connection Error: %v", err)
	}
	defer conn.Close(ctx)

	aiClient, err := genai.NewClient(ctx, option.WithAPIKey(geminiKey))
	if err != nil {
		log.Fatalf("AI Client Error: %v", err)
	}
	defer aiClient.Close()
	model := aiClient.GenerativeModel("gemini-3-flash-preview")

	// 2. Fetch GDELT Data
	resp, _ := http.Get("http://data.gdeltproject.org/gdeltv2/lastupdate.txt")
	bodyBytes, _ := io.ReadAll(resp.Body)
	latestEventUrl := strings.Split(strings.Split(string(bodyBytes), "\n")[0], " ")[2]

	zipResp, _ := http.Get(latestEventUrl)
	zipBytes, _ := io.ReadAll(zipResp.Body)
	zipReader, _ := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))

	var csvFile *zip.File
	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".CSV") {
			csvFile = f
			break
		}
	}
	f, _ := csvFile.Open()
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = '\t'

	// 3. Collect Raw Data for AI context
	rawData := make(map[string][]string)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if len(record) <= CountryCodeIndex {
			continue
		}

		fips := record[CountryCodeIndex]
		iso, exists := FipsToIso[fips]
		if !exists {
			continue
		}

		if len(rawData[iso]) < 10 {
			rawData[iso] = append(rawData[iso], record[EventCodeIndex])
		}
	}

	// 4. CALL THE BATCH LOGIC (The Moderation Phase)
	processDailyBatch(ctx, model, conn, rawData)

	fmt.Println("Daily Turn Moderation Complete.")

	// --- 5. THE VERCEL TRIGGER (Put it here!) ---
	if hook := os.Getenv("VERCEL_DEPLOY_HOOK"); hook != "" {
		fmt.Println("Triggering Astro Library rebuild...")
		_, err := http.Post(hook, "application/json", nil)
		if err != nil {
			fmt.Printf("Warning: Failed to trigger Vercel hook: %v\n", err)
		}
	}
} // <--- End of main

// 5. THE HELPER FUNCTION
func processDailyBatch(ctx context.Context, model *genai.GenerativeModel, db *pgx.Conn, rawData map[string][]string) {
	// 1. PREPARE LIST: Filter countries we actually support
	var countriesToProcess []string
	for _, iso := range FipsToIso {
		if _, exists := rawData[iso]; exists {
			countriesToProcess = append(countriesToProcess, iso)
		}
	}

	// 2. FORMAT DATA FOR AI
	contextString, _ := json.Marshal(rawData)
	systemPrompt := fmt.Sprintf(`
        You are the WorldTension Engine Moderator.
        Headlines/EventCodes: %s

        Task: Calculate HoI4 metrics for these countries: %v.
        Output a STRICT JSON object:
        {"updates": [{"iso_code": "USA", "tension": 1.2, "stability": 80, "ic": 90, "report": "Intelligence report summary..."}]}`,
		contextString, countriesToProcess)

	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("Output valid JSON only.")},
	}

	// 3. CALL AI
	resp, err := model.GenerateContent(ctx, genai.Text(systemPrompt))
	if err != nil {
		log.Printf("AI Generation Error: %v", err)
		return
	}

	// 4. PARSE AI RESPONSE
	var result struct {
		Updates []struct {
			ISOCode   string  `json:"iso_code"`
			Tension   float64 `json:"tension"`
			Stability int     `json:"stability"`
			IC        int     `json:"ic"`
			Report    string  `json:"report"`
		} `json:"updates"`
	}

	jsonStr := string(resp.Candidates[0].Content.Parts[0].(genai.Text))
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimSuffix(jsonStr, "```")

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("JSON Unmarshal Error: %v | Raw: %s", err, jsonStr)
		return
	}

	// 5. UPSERT TO DATABASE (The Future-Proof way)
	for _, u := range result.Updates {
		// REVERSE LOOKUP: Find the FIPS code for this ISO code
		var fipsCode string
		for fips, iso := range FipsToIso {
			if iso == u.ISOCode {
				fipsCode = fips
				break
			}
		}

		// SAFETY CHECK: Skip if fipsCode is missing
		if fipsCode == "" {
			log.Printf("Warning: No FIPS mapping for %s. Skipping.", u.ISOCode)
			continue
		}

		// COUNT EVENTS: How many headlines did we process for this country?
		eventCount := len(rawData[u.ISOCode])

		query := `
            INSERT INTO world_tension (iso_code, fips_code, event_count, tension_score, stability, industrial_capacity, intel_report, last_updated)
            VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
            ON CONFLICT (iso_code)
            DO UPDATE SET
                fips_code = EXCLUDED.fips_code,
                event_count = EXCLUDED.event_count,
                tension_score = EXCLUDED.tension_score,
                stability = EXCLUDED.stability,
                industrial_capacity = EXCLUDED.industrial_capacity,
                intel_report = EXCLUDED.intel_report,
                last_updated = NOW();`

		// Add eventCount as the 3rd variable ($3)
		_, err := db.Exec(ctx, query, u.ISOCode, fipsCode, eventCount, u.Tension, u.Stability, u.IC, u.Report)
		if err != nil {
			log.Printf("DB Update Error for %s: %v", u.ISOCode, err)
		} else {
			fmt.Printf("MODERATOR: %s updated successfully. (Events: %d)\n", u.ISOCode, eventCount)
		}
	}
}

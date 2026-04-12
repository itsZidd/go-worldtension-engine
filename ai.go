package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/generative-ai-go/genai"
)

const (
	geminiModel = "gemini-3-flash-preview"
	chunkSize   = 50
)

// AnalyzeCountries splits rawData into chunks, calls Gemini per chunk,
// and merges all results. A failed chunk is logged and skipped so one
// bad response never aborts the full daily run.
func AnalyzeCountries(ctx context.Context, client *genai.Client, rawData map[string][]string) ([]CountryUpdate, error) {
	keys := make([]string, 0, len(rawData))
	for iso := range rawData {
		keys = append(keys, iso)
	}

	totalChunks := (len(keys) + chunkSize - 1) / chunkSize
	allUpdates := make([]CountryUpdate, 0, len(keys))

	for i := 0; i < len(keys); i += chunkSize {
		end := min(i+chunkSize, len(keys))
		chunkNum := (i / chunkSize) + 1

		chunkData := buildChunk(keys[i:end], rawData)

		fmt.Printf("[AI] Chunk %d/%d — analyzing %d countries...\n", chunkNum, totalChunks, len(chunkData))

		updates, err := analyzeChunk(ctx, client, chunkData)
		if err != nil {
			log.Printf("[AI] Chunk %d/%d failed, skipping: %v", chunkNum, totalChunks, err)
			continue
		}

		validateChunk(chunkNum, len(chunkData), len(updates))

		allUpdates = append(allUpdates, updates...)
	}

	fmt.Printf("[AI] Done — %d/%d countries analyzed successfully.\n", len(allUpdates), len(keys))

	return allUpdates, nil
}

// analyzeChunk sends a single chunk of country data to Gemini
// and returns the parsed updates.
func analyzeChunk(ctx context.Context, client *genai.Client, rawData map[string][]string) ([]CountryUpdate, error) {
	model := buildModel(client)

	prompt, err := buildPrompt(rawData)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate: %w", err)
	}

	return parseAIResponse(resp)
}

// buildChunk constructs a map subset from a slice of ISO keys.
func buildChunk(keys []string, rawData map[string][]string) map[string][]string {
	chunk := make(map[string][]string, len(keys))
	for _, iso := range keys {
		chunk[iso] = rawData[iso]
	}
	return chunk
}

// validateChunk logs a warning if Gemini returned significantly fewer
// updates than expected, which indicates silent truncation.
func validateChunk(chunkNum, sent, received int) {
	if received < sent/2 {
		log.Printf(
			"[AI] WARNING: chunk %d — sent %d countries, received %d back. Possible truncation.",
			chunkNum, sent, received,
		)
	}
}

// buildModel returns a configured Gemini model with strict JSON output instructions.
func buildModel(client *genai.Client) *genai.GenerativeModel {
	model := client.GenerativeModel(geminiModel)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("Output valid JSON only. No prose. No markdown blocks.")},
	}
	return model
}

// buildPrompt serializes the country event data and constructs
// the WorldTension analysis prompt.
func buildPrompt(rawData map[string][]string) (string, error) {
	isoCodes := make([]string, 0, len(rawData))
	for iso := range rawData {
		isoCodes = append(isoCodes, iso)
	}

	contextJSON, err := json.Marshal(rawData)
	if err != nil {
		return "", fmt.Errorf("marshal raw data: %w", err)
	}

	return fmt.Sprintf(`
You are the WorldTension Engine.
Analyze these EventCodes/Headlines: %s

Target Countries: %v.

TASK: Calculate HoI4-style metrics based on the provided event data.
1. tension:    0.0 to 10.0 (High if conflict/military events)
2. stability:  0 to 100    (Low if protests/coups)
3. industrial: 0 to 100    (Low if strikes/disasters)
4. report: A short, professional intelligence briefing sentence.

Output ONLY a strict JSON object:
{"updates": [{"iso_code": "USA", "tension": 1.2, "stability": 80, "industrial": 90, "report": "Summary"}]}`,
		contextJSON, isoCodes), nil
}

// parseAIResponse extracts and unmarshals the JSON from a Gemini response.
func parseAIResponse(resp *genai.GenerateContentResponse) ([]CountryUpdate, error) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("empty response from AI")
	}

	rawText := fmt.Sprint(resp.Candidates[0].Content.Parts[0])

	jsonStr, err := extractJSON(rawText)
	if err != nil {
		return nil, fmt.Errorf("extract json: %w (raw: %s)", err, rawText)
	}

	var result AIResponse
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Updates, nil
}

// extractJSON strips any surrounding prose or markdown fences,
// returning only the outermost JSON object.
func extractJSON(s string) (string, error) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("no JSON object found")
	}
	return s[start : end+1], nil
}

// min returns the smaller of two ints.
// Remove this if your Go version is 1.21+ (builtin min available).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

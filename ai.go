package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
)

const (
	geminiModel = "gemini-2.5-flash" // Updated to a stable version
	chunkSize   = 50
	maxRetries  = 3
)

func AnalyzeCountries(ctx context.Context, client *genai.Client, rawData map[string]CountryInput) ([]CountryUpdate, error) {
	keys := make([]string, 0, len(rawData))
	for iso := range rawData {
		keys = append(keys, iso)
	}

	totalChunks := (len(keys) + chunkSize - 1) / chunkSize
	allUpdates := make([]CountryUpdate, 0, len(keys))

	for i := 0; i < len(keys); i += chunkSize {
		end := i + chunkSize
		if end > len(keys) {
			end = len(keys)
		}
		chunkNum := (i / chunkSize) + 1
		chunkData := buildChunk(keys[i:end], rawData)

		log.Printf("[AI] chunk %d/%d — analyzing %d countries...", chunkNum, totalChunks, len(chunkData))

		updates, err := analyzeChunkWithRetry(ctx, client, chunkData, chunkNum)
		if err != nil {
			log.Printf("[AI] chunk %d/%d failed after %d retries, skipping: %v", chunkNum, totalChunks, maxRetries, err)
			continue
		}

		allUpdates = append(allUpdates, updates...)
	}

	log.Printf("[AI] done — %d/%d countries analyzed successfully.", len(allUpdates), len(keys))
	return allUpdates, nil
}

func analyzeChunkWithRetry(ctx context.Context, client *genai.Client, chunkData map[string]CountryInput, chunkNum int) ([]CountryUpdate, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		updates, err := analyzeChunk(ctx, client, chunkData)
		if err == nil {
			return updates, nil
		}
		lastErr = err
		wait := time.Duration(attempt*attempt) * time.Second
		log.Printf("[AI] chunk %d attempt %d/%d failed: %v — retrying in %s", chunkNum, attempt, maxRetries, err, wait)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func analyzeChunk(ctx context.Context, client *genai.Client, rawData map[string]CountryInput) ([]CountryUpdate, error) {
	model := client.GenerativeModel(geminiModel)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("Output valid JSON only. No prose. No markdown fences. No backticks.")},
	}

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

func buildChunk(keys []string, rawData map[string]CountryInput) map[string]CountryInput {
	chunk := make(map[string]CountryInput, len(keys))
	for _, iso := range keys {
		chunk[iso] = rawData[iso]
	}
	return chunk
}

func buildPrompt(rawData map[string]CountryInput) (string, error) {
	contextJSON, err := json.Marshal(rawData)
	if err != nil {
		return "", fmt.Errorf("marshal raw data: %w", err)
	}

	return fmt.Sprintf(`You are the WorldTension Engine.
Analyze these inputs to calculate HoI4-style metrics:

1. tension: 0.0 to 10.0 (Global threat level)
2. stability: 0 to 100 (Internal peace)
3. industrial: 0 to 100 (Economic health)
4. report: Short briefing. Mention specific GDELT event types or maritime drops.
5. primary_driver: Choose from: "Armed Conflict", "Trade Disruption", "Civil Unrest", "Diplomatic Tension".

Data: %s

Output ONLY valid JSON in this format:
{"updates":[{"iso_code":"USA","tension":1.2,"stability":80,"industrial":90,"primary_driver":"News","report":"Summary"}]}`,
		string(contextJSON)), nil
}

func parseAIResponse(resp *genai.GenerateContentResponse) ([]CountryUpdate, error) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	part := resp.Candidates[0].Content.Parts[0]
	textPart, ok := part.(genai.Text)
	if !ok {
		return nil, fmt.Errorf("unexpected response part type: %T", part)
	}
	rawText := string(textPart)

	jsonStr, err := extractJSON(rawText)
	if err != nil {
		return nil, fmt.Errorf("extract json: %w (raw: %.200s)", err, rawText)
	}

	var result AIResponse
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Updates, nil
}

func extractJSON(s string) (string, error) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("no JSON object found")
	}
	return s[start : end+1], nil
}

# Changelog: The WorldTension Engine (2026)
All notable changes to the Go-based backend engine will be documented in this file.

---

## [1.1.0] - 2026-04-12
### "The Refactor & Resilience Update"
> The engine has been restructured from a single monolithic file into a maintainable multi-file architecture, with hardened data integrity and full global country coverage.

### 🚀 Added
- **Full Global Coverage:** `countries.go` expanded from ~90 to 174 sovereign states using the authoritative FIPS 10-4 → ISO 3166-1 alpha-3 mapping.
- **O(1) Reverse FIPS Lookup:** `IsoToFips` reverse map now built once at startup via `init()`, replacing the previous O(n) linear scan on every upsert.
- **AI Chunking:** Gemini requests now batch in chunks of 50 countries to prevent silent response truncation at scale. A failed chunk is logged and skipped without aborting the full run.
- **Truncation Detection:** Warning log emitted when Gemini returns fewer than half the expected updates for a chunk, indicating possible truncation.
- **Score Clamping:** All AI-generated numeric fields (`tension`, `stability`, `industrial`) are clamped to valid ranges in Go before reaching the database, preventing corrupt values from bad model outputs.
- **Database Constraint:** `CHECK (tension_score BETWEEN 0.0 AND 10.0)` added as a final safety net at the PostgreSQL level.
- **Score Decay:** Stale records (no update in 48h) are automatically nudged back toward neutral each run — tension decays by 0.5/day, stability recovers by 2/day, industrial by 1/day.

### 🛠 Changed
- **Project Structure:** Monolithic `main.go` split into focused files: `main.go`, `gdelt.go`, `ai.go`, `db.go`, `models.go`, `countries.go`.
- **Error Handling:** All errors now wrapped with `fmt.Errorf("context: %w", err)` for meaningful stack traces throughout the pipeline.
- **Batching Strategy:** Replaced 10-country batches with 50-country chunks, reducing total API calls while staying well within free tier quota.
- **`main.go`:** Reduced to a pure orchestrator — config loading, client wiring, and sequential pipeline steps only.
- **Config:** Environment variables consolidated into a typed `config` struct via `mustLoadConfig()`.
- **Logging:** Prefixed log output with `[AI]` and `[DB]` tags for easier filtering in GitHub Actions run logs.

### 🐛 Fixed
- **Negative tension scores:** Clamping and DB constraint eliminates scores like Yemen at -8.00 or Libya at -7.40 produced by prior model runs.
- **Frozen crisis scores:** Decay logic prevents countries that fall off GDELT's radar from keeping stale high-tension values indefinitely (e.g. ZWE at 7.00 with "No recent activity").
- **Silent truncation:** Chunking and truncation detection surface cases where Gemini silently dropped countries from a large batch response.
- **Vercel hook errors:** Deploy hook failures are now logged instead of silently dropped.

---

## [1.0.0] - 2026-04-11
### "The AI Moderator Update"
> The engine has transitioned from a mathematical aggregator to a geopolitical reasoning agent.

### 🚀 Added
- **Gemini 3 Flash Integration:** Swapped the legacy Goldstein math for a LLM-based moderation layer.
- **Multivariate Scoring:** The engine now generates `stability` and `industrial_capacity` (HoI4 metrics) alongside tension.
- **Intel Reports:** Automated generation of the `intel_report` column for sidebar flavor text and Astro SEO content.
- **Vercel Deploy Hook:** Automated trigger to rebuild the Astro Intelligence Library whenever a "Daily Turn" is successful.
- **Reverse FIPS Lookup:** Strict logic to map ISO codes back to GDELT FIPS codes for data integrity.
- **Autonomous CI/CD Handshake:** Integrated the Vercel Deploy Hook directly into GitHub Actions with `if: success()` logic for a fully closed-loop update cycle.

### 🛠 Changed
- **Database Schema:** Expanded the `world_tension` table to include `stability`, `industrial_capacity`, and `intel_report`.
- **Logic Flow:** Shifted from "Parse-then-Sum" to "Parse → Batch → Moderate → Upsert."
- **Batching Strategy:** Implemented 10-country batches to optimize for the $0 Gemini Flash Free Tier.

### ⚠️ Security & Environment
- Added requirement for `GEMINI_API_KEY` environment variable.
- Added requirement for `VERCEL_DEPLOY_HOOK` environment variable.

---

## [0.1.0] - 2026-04-07
### "The Goldstein Scraper" *(Legacy Version)*
> The initial proof-of-concept for global data ingestion.

### 🏗 Features
- **GDELT 2.0 Ingestion:** Automated ZIP download and CSV parsing of GKG files.
- **Goldstein Index Calculation:** Used numerical averages of GDELT's `GoldsteinScale` to determine tension.
- **Supabase Integration:** Initial connection to PostgreSQL for `iso_code`, `fips_code`, and `tension_score`.
- **GitHub Actions:** First implementation of the `00:00` Daily Turn cron job.

### 📉 Limitations
- "Dumb" math: did not understand the content of news, only the intensity.
- Missing context: no flavor text or intelligence reports.
- Limited metrics: only tracked a single `tension_score`.

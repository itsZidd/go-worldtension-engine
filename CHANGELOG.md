# Changelog: The WorldTension Engine (2026)

All notable changes to the Go-based backend engine will be documented in this file.

---

## [1.0.0] - 2026-04-11

### "The AI Moderator Update" *(Current Version)*

> The engine has transitioned from a mathematical aggregator to a geopolitical reasoning agent.

### 🚀 Added

- **Gemini 3 Flash Integration:** Swapped the legacy Goldstein math for a LLM-based moderation layer.
- **Multivariate Scoring:** The engine now generates `stability` and `industrial_capacity` (HoI4 metrics) alongside tension.
- **Intel Reports:** Automated generation of the `intel_report` column for sidebar flavor text and Astro SEO content.
- **Vercel Deploy Hook:** Automated trigger to rebuild the Astro Intelligence Library whenever a "Daily Turn" is successful.
- **Reverse FIPS Lookup:** Strict logic to map ISO codes back to GDELT FIPS codes for data integrity.

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

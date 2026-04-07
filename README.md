# 🌍 WorldTension Data Engine

The automated backend data pipeline for **WorldTension**, a global intelligence and geopolitical monitoring dashboard.

This engine acts as the "Daily Turn" mechanic. It wakes up, aggregates global event data, calculates stability metrics, pushes the updated intelligence to a remote database, and shuts down — running entirely on zero-cost infrastructure.

---

## ⚙️ How It Works

1. **Fetch & Unpack** — Downloads the absolute latest 15-minute global event export from the **GDELT 2.0 Project** (Global Database of Events, Language, and Tone) directly into memory.
2. **Crunch the Metrics** — Parses the TSV data and aggregates the **Goldstein Scale** scores for thousands of global events. The scores are inverted to create a "Tension Score" (where highly negative events = high tension).
3. **Translation Layer** — Automatically maps legacy FIPS 10-4 country codes used by US Intelligence/GDELT into standard ISO 3166-1 alpha-3 codes for frontend web mapping.
4. **Database Upsert** — Connects to a remote **Supabase (PostgreSQL)** database via `pgx` and executes an `UPSERT` to update the global map data.
5. **Serverless Automation** — Orchestrated via a GitHub Actions Cron Job to run every 24 hours at 00:00 UTC.

---

## 🛠️ Tech Stack

| Component     | Technology                        |
|---------------|-----------------------------------|
| Language      | Go (Golang)                       |
| Database Driver | `pgx` (github.com/jackc/pgx/v5) |
| Database      | Supabase (PostgreSQL)             |
| Automation    | GitHub Actions                    |
| Data Source   | GDELT Project API                 |

---

## 🚀 Running Locally

To run the daily turn manually on your local machine:

**1. Clone the repository.**

**2. Ensure you have Go 1.22+ installed.**

**3. Install dependencies:**

```bash
go get github.com/jackc/pgx/v5
```

**4. Set your Supabase connection string as an environment variable:**

```bash
export DATABASE_URL="postgresql://postgres.[PROJECT_ID]:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:6543/postgres"
```

**5. Run the engine:**

```bash
go run .
```

---

## 📂 Project Structure

```
.
├── main.go                          # Core pipeline: fetching, parsing, calculating, and database injection
├── countries.go                     # FIPS 10-4 to ISO 3166-1 alpha-3 dictionary map
└── .github/
    └── workflows/
        └── daily-turn.yml           # CI/CD automation script
```

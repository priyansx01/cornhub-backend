# corn-hub-clone — Go Backend

Binary builds, database migrations, code generation.

## Quick Start

```bash
cp .env.example .env
make dev
```

Run the stack locally:

```bash
docker compose up -d        # Postgres, Redis, MinIO, Kafka, ClickHouse
./run_api.sh                # API on :8080
./run_worker.sh             # FFmpeg worker (needs ffmpeg + ffprobe on PATH)
```

Seed logins: `admin@ismart.com / admin123`, `learner@ismart.com / learner123`.

## Video pipeline

```
POST /courses/:id/modules/:moduleId/upload   (instructor/admin)
  → MinIO lms-raw-videos (private)
  → Kafka video.uploaded
  → worker: ffprobe + ffmpeg → HLS 1080p/720p/480p/360p + thumbnail
  → MinIO lms-hls-videos / lms-thumbnails (public-read)
  → Kafka video.processed
  → API consumer marks module + course "ready"
GET  /courses/:id/modules/:moduleId/progress      → percent + status (failed on bad input)
GET  /courses/:id/modules/:moduleId/playback-url  → master.m3u8 URL for hls.js / Safari (409 until ready)
```

`MINIO_PUBLIC_URL` sets the host players stream from (point it at a CDN in production).

## Browsing and filtering

- `GET /courses?category=Safety&search=fire` — case-insensitive category match; archived courses are hidden unless `status=archived`.
- `GET /categories` — distinct categories with course counts, for building the filter UI.
- Learners (`employee` role) only see `ready` courses; the `status` filter is for instructors/admins.

## Commands

| Command | Description |
|---|---|
| `make dev` | Run with hot-reload (requires `air`) |
| `make build` | Compile production binary |
| `make run` | Build + run |
| `make migrate-up` | Run database migrations |
| `make migrate-down` | Roll back last migration |
| `make sqlc` | Regenerate type-safe SQL |
| `make test` | Run all tests |
| `make lint` | Run golangci-lint |

## Project Structure

```
cmd/api/          → Application entry point
internal/
  config/         → Environment + config loading
  middleware/     → JWT auth, CORS, rate-limit
  domain/         → Domain models (shared across services)
  auth/           → Auth handlers + JWT logic
  user/           → User/Employee service
  course/         → Course + Module CRUD
  module/         → Module management
  assessment/     → Quiz + scoring
  analytics/      → ClickHouse analytics
  leaderboard/    → Redis sorted-set leaderboard
  content/        → Content library
  storage/        → MinIO client
  database/       → PostgreSQL connection
pkg/
  response/       → Standardized API response helpers
db/
  migrations/     → SQL migration files
  queries/        → sqlc query definitions
```

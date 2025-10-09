# Real-time Analytics Service

## Overview
This project implements a simple real-time analytics ingestion and aggregation service in Go. It exposes HTTP endpoints for event ingestion, aggregates events in-memory over time windows, and logs structured output with graceful shutdown.

- Module: `github.com/Rassimdou/Real-time-Analytics`
- Go: `1.23.0` (toolchain `go1.24.4`)
- HTTP framework: Gin
- Logging: Uber Zap
- Config: Viper (`config/config.yaml`)

## Repository Structure
- `cmd/server/`
  - `main.go`: Application entrypoint that loads config, sets up logging, starts the HTTP server, launches worker pool, and runs the aggregator.
- `internal/server/`
  - `server.go`: Gin engine, middleware, routes, request models, and HTTP handlers.
- `internal/aggregation/`
  - `metrics.go`: Metric types (counter, gauge, histogram, set), snapshots, and time windows.
  - `aggregator.go`: Aggregator that updates global metrics and time windows, periodic flush and cleanup, optional callback on window close.
- `internal/config/`
  - `config.go`: Configuration loading with Viper.
- `config/`
  - `config.yaml`: Runtime configuration file (server, logging, processing settings).

## What We Fixed During Setup
- Aligned the module and import paths to avoid "missing metadata for import" errors.
- Resolved "use of internal package not allowed" by running with package paths instead of direct file paths.

## Running the Service
From the repository root `d:\Real-time-analytics`:

- Run directly (recommended):
```bash
go run ./cmd/server -config config/config.yaml
```

- Build, then run:
```bash
go build -o server.exe ./cmd/server
./server.exe -config config/config.yaml
```

- Avoid using a single file path (this causes internal package errors):
```bash
# Do NOT do this
go run cmd/server/main.go -config config/config.yaml
```

## Configuration
`config/config.yaml` (example fields inferred from code):
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  shutdownTimeout: 10s

logging:
  level: "info"    # or "debug"
  format: "console" # or "json"

processing:
  workerCount: 4
  bufferSize:  1000
```
`cmd/server/main.go` uses these values to size the event queue and set Gin mode (debug if logging level is debug).

## HTTP API
- `GET /health`
  - Health status with current time.
- `GET /ready`
  - Readiness status with placeholder checks.
- `POST /api/v1/events`
  - Ingest a single event.
  - Body example:
```json
{
  "type": "pageview",
  "timestamp": "2025-01-01T00:00:00Z",
  "user_id": "u_123",
  "session_id": "s_abc",
  "properties": {"page": "/home"}
}
```
  - If `timestamp`/`id` are missing, they are auto-filled.
- `POST /api/v1/events/batch`
  - Ingest an array of events with size validation and non-blocking enqueue. Returns accepted/rejected counts.
- `GET /api/v1/metrics`
  - Placeholder response indicating metrics querying will be implemented.
- `GET /api/v1/metrics/:name`
  - Placeholder for querying a specific metric by name.

## Event Processing Pipeline
1. `internal/server` validates, defaults, and enqueues events into a buffered channel.
2. Worker goroutines (configured via `processing.workerCount`) read from the queue.
3. Each event is mapped to `aggregation.Event` and passed to `Aggregator.ProcessEvent`.
4. Aggregator updates:
   - Global metrics (counters, unique sets, histograms)
   - Active time window metrics (per-minute by default)
5. A ticker periodically flushes expired windows and performs cleanup. An optional callback can persist closed windows to a database/cache in the future.

## Metrics Tracked (examples)
- Global counters: `total_events`, `events_by_type:<type>`, `pageviews`, `clicks`, `purchases`
- Unique sets: `unique_users`, `unique_sessions`, `unique_pages`
- Per-dimension counters: `page_views:<page>`, `clicks:<element>`
- Histogram and totals: `revenue_histogram`, `revenue`
- Window metrics: `events`, `events:<type>`, `active_users`

## Middleware
- Recovery: panic protection.
- Structured logging: request fields, duration, errors via Zap.
- CORS: simple permissive policy for development.
- Request ID: `X-Request-ID` header propagation or auto-generation.

## Graceful Shutdown
- Listens for `SIGTERM`/interrupt, shuts down HTTP server with timeout (`server.shutdownTimeout`), cancels workers via context, and waits for completion with a bounded wait.

## Development Tips
- Keep module path in `go.mod` exactly matching the import paths used in code.
- Use package paths when running with `go run` to allow importing `internal/...` packages.
- Tune `processing.bufferSize` and `workerCount` based on expected throughput.
- Add persistence in the aggregator window-close callback to store aggregates in a DB or cache.

## Future Enhancements
- Real metrics querying endpoints backed by persisted aggregates.
- Prometheus metrics exporter and health probes for Kubernetes.
- AuthN/Z and rate limiting for ingestion endpoints.
- JSON schema validation for events and stronger type handling for `properties`.
- Replace ad-hoc IDs with UUIDs.

## License
TBD.

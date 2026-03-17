# SmartCDN — On-the-Fly Image Optimization Engine

## Project Overview
A production-grade image CDN that dynamically resizes, compresses, and transcodes images based on the requesting device. Inspired by how Meta/Instagram serves billions of images efficiently — sending a 4K JPEG to an old Android phone is a waste of bandwidth and money.

## Architecture
```
Client Request (GET /img/{id})
        │
        ▼
┌─────────────────┐
│  Go HTTP Server  │  ← Parses User-Agent, Accept headers
│  (Echo/stdlib)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌───────────┐
│  Cache Lookup    │────▶│   Redis   │  ← Key: {id}:{device}:{format}
│  (cache-aside)   │     └───────────┘
└────────┬────────┘
         │ cache miss
         ▼
┌─────────────────┐     ┌───────────┐
│ Image Processor  │────▶│   MinIO   │  ← Fetch original from S3-compatible store
│ (bimg / libvips) │     │   (S3)    │
└────────┬────────┘     └───────────┘
         │
         ▼
   Transform Image
   (resize, compress, convert format)
         │
         ▼
   Store variant in Redis (TTL cache)
         │
         ▼
   Return optimized image to client
```

## Tech Stack
- **Language:** Go 1.22+
- **HTTP Router:** net/http stdlib (keep it simple, no frameworks)
- **Image Processing:** bimg (Go binding for libvips) — used by Cloudflare/Shopify, 4-8x faster than ImageMagick
- **Cache:** Redis 7 — stores processed image variants with TTL
- **Object Storage:** MinIO — S3-compatible, self-hosted for local dev
- **Metrics:** Prometheus client + Grafana dashboards
- **Containerization:** Docker + Docker Compose
- **Load Testing:** vegeta or hey

## Project Structure
```
smartcdn/
├── CLAUDE.md
├── README.md
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
├── cmd/
│   └── server/
│       └── main.go              # Entrypoint: config loading, server startup, graceful shutdown
├── internal/
│   ├── handler/
│   │   ├── image.go             # GET /img/{id} — main serving handler
│   │   ├── upload.go            # POST /upload — stores originals in MinIO
│   │   └── health.go            # GET /health
│   ├── device/
│   │   └── classifier.go        # User-Agent parsing → device class enum
│   ├── processor/
│   │   └── transform.go         # bimg resize/compress/convert logic
│   ├── cache/
│   │   └── redis.go             # Redis get/set with TTL, cache key builder
│   ├── storage/
│   │   └── minio.go             # MinIO upload/download, presigned URLs
│   ├── middleware/
│   │   ├── metrics.go           # Prometheus request duration, cache hit/miss counters
│   │   └── logging.go           # Structured JSON request logging
│   └── config/
│       └── config.go            # Env-based config with defaults
├── configs/
│   ├── prometheus.yml
│   └── grafana/
│       └── dashboard.json
└── scripts/
    ├── seed.sh                  # Upload sample images to MinIO for testing
    └── loadtest.sh              # Run vegeta/hey against the server
```

## Device Classification Tiers
| Device Class   | Max Width | Quality | Preferred Format |
|----------------|-----------|---------|------------------|
| mobile-low     | 480px     | 60%     | WebP             |
| mobile-high    | 768px     | 75%     | WebP             |
| tablet         | 1024px    | 80%     | WebP             |
| desktop        | 1920px    | 85%     | WebP/JPEG        |
| unknown        | 1024px    | 75%     | JPEG             |

## Cache Key Format
```
smartcdn:{image_id}:{device_class}:{format}
Example: smartcdn:abc123:mobile-low:webp
```
TTL: 24 hours (configurable via CACHE_TTL_SECONDS env var)

## Key Design Decisions
1. **Cache-aside pattern** over write-through — we only cache what's actually requested, avoids pre-generating every variant
2. **bimg/libvips** over ImageMagick — constant memory usage under load, critical for not OOM-ing at scale
3. **Format negotiation via Accept header** — if browser sends `Accept: image/webp`, serve WebP; otherwise fall back to JPEG
4. **Redis for variant cache** (not filesystem) — enables horizontal scaling, multiple server instances share the cache
5. **Stdlib net/http** over Echo/Gin — fewer dependencies, demonstrates you understand the fundamentals

## Environment Variables
```
PORT=8080
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=smartcdn-originals
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
CACHE_TTL_SECONDS=86400
LOG_LEVEL=info
```

## API Endpoints
- `GET /health` — health check, returns 200 + JSON with uptime
- `POST /upload` — multipart form upload, stores original in MinIO, returns image ID
- `GET /img/{id}` — serves optimized image based on User-Agent and Accept headers
- `GET /metrics` — Prometheus metrics endpoint

## Response Headers (for debugging/demo)
- `X-SmartCDN-Device`: detected device class
- `X-SmartCDN-Cache`: HIT or MISS
- `X-SmartCDN-Original-Size`: original image bytes
- `X-SmartCDN-Optimized-Size`: served image bytes
- `X-SmartCDN-Savings`: percentage bandwidth saved

## Coding Conventions
- Use Go stdlib where possible, minimize external dependencies
- All errors must be wrapped with context: `fmt.Errorf("failed to fetch from minio: %w", err)`
- No global state — pass dependencies via struct injection
- Table-driven tests for device classification and cache key generation
- Use `context.Context` for cancellation throughout the pipeline
- Structured logging with `slog` (Go 1.21+ stdlib)

## Testing Strategy
- Unit tests: device classifier, cache key builder, config parsing
- Integration tests: Redis cache operations, MinIO upload/download (use testcontainers-go)
- End-to-end: Docker Compose up → upload image → request with different User-Agents → verify different sizes returned
- Load test: 1000 RPS with vegeta, measure p50/p95/p99 latency and cache hit rate

## Metrics to Track (Prometheus)
- `smartcdn_requests_total` (labels: device_class, cache_status, format)
- `smartcdn_request_duration_seconds` (histogram)
- `smartcdn_image_processing_duration_seconds` (histogram)
- `smartcdn_bytes_original_total` (counter)
- `smartcdn_bytes_served_total` (counter)
- `smartcdn_cache_hit_ratio` (gauge, computed)

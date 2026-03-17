# SmartCDN — On-the-Fly Image Optimization Engine

A production-grade image CDN that dynamically resizes, compresses, and transcodes images based on the requesting device. Inspired by how Meta/Instagram serves billions of images — sending a 4K JPEG to an old Android phone is a waste of bandwidth and money.

## Architecture

```
Client Request (GET /img/{id})
        │
        ▼
┌─────────────────┐
│  Go HTTP Server  │  ← Parses User-Agent, Accept headers
│  (net/http)      │
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

## Quick Start

```bash
# Start all services (app, MinIO, Redis, Prometheus, Grafana)
docker compose up --build -d

# Seed sample images
bash scripts/seed.sh

# Upload your own image
curl -X POST http://localhost:8080/upload -F "image=@photo.jpg"
# → {"id":"abc123-...", "size":2048576}

# Fetch optimized for mobile
curl -H "User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 17_0)" \
     -H "Accept: image/webp" \
     http://localhost:8080/img/abc123 -o mobile.webp

# Fetch optimized for desktop
curl -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120" \
     -H "Accept: image/webp" \
     http://localhost:8080/img/abc123 -o desktop.webp

# Health check
curl http://localhost:8080/health
```

### Services

| Service        | URL                   | Credentials        |
|----------------|-----------------------|---------------------|
| App            | http://localhost:8080  | —                   |
| MinIO Console  | http://localhost:9001  | minioadmin/minioadmin |
| Prometheus     | http://localhost:9090  | —                   |
| Grafana        | http://localhost:3000  | admin/admin         |

### Makefile

```bash
make docker-up      # Start all services
make docker-down    # Stop all services
make seed           # Upload sample images
make loadtest       # Run load tests
make test           # Run all tests (needs MinIO + Redis)
make test-unit      # Run unit tests only
make lint           # go vet + gofmt check
make build          # Build binary (needs libvips-dev)
```

## API Endpoints

| Method | Path          | Description                                      |
|--------|---------------|--------------------------------------------------|
| GET    | `/health`     | Health check — returns uptime                    |
| POST   | `/upload`     | Upload image (multipart form, field: `image`)    |
| GET    | `/img/{id}`   | Serve optimized image based on device            |
| GET    | `/metrics`    | Prometheus metrics                               |

## Device-Aware Optimization

SmartCDN classifies devices from the User-Agent header and serves appropriately sized images:

| Device Class   | Max Width | Quality | Preferred Format |
|----------------|-----------|---------|------------------|
| mobile-low     | 480px     | 60%     | WebP             |
| mobile-high    | 768px     | 75%     | WebP             |
| tablet         | 1024px    | 80%     | WebP             |
| desktop        | 1920px    | 85%     | WebP/JPEG        |
| unknown        | 1024px    | 75%     | JPEG             |

### Example: Same Image, Different Devices

Upload a 4000x3000 JPEG (3.2 MB original):

```
Device          User-Agent (shortened)              Size     Savings
─────────────────────────────────────────────────────────────────────
mobile-low      Android 4.4.2 Mobile                 48 KB    98.5%
mobile-high     iPhone OS 17 Safari                  92 KB    97.1%
tablet          iPad OS 17 Safari                   156 KB    95.2%
desktop         Chrome/120 Windows NT 10.0          380 KB    88.4%
```

Response headers for debugging:

```
X-SmartCDN-Device: mobile-high
X-SmartCDN-Cache: MISS          (HIT on subsequent requests)
X-SmartCDN-Original-Size: 3276800
X-SmartCDN-Optimized-Size: 94208
X-SmartCDN-Savings: 97.1%
```

## Metrics & Monitoring

Grafana is auto-provisioned on startup with a **SmartCDN** dashboard — no manual setup needed.

Open http://localhost:3000, login with `admin`/`admin`.

### Dashboard Panels

- **Request Rate by Device Class** — per-second throughput broken down by device tier
- **Cache Hit Ratio** — gauge showing current hit rate + time-series trend
- **Image Processing Latency** — p50, p95, p99 processing times
- **Bandwidth: Original vs Served** — stacked area chart showing savings
- **Top Device Classes** — pie chart of request distribution

### Prometheus Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `smartcdn_requests_total` | Counter | `device_class`, `cache_status`, `format` |
| `smartcdn_request_duration_seconds` | Histogram | — |
| `smartcdn_image_processing_duration_seconds` | Histogram | — |
| `smartcdn_bytes_original_total` | Counter | — |
| `smartcdn_bytes_served_total` | Counter | — |

<!-- Screenshot placeholder: add a Grafana dashboard screenshot here -->
<!-- ![Grafana Dashboard](docs/grafana-dashboard.png) -->

## Performance

Expected throughput from load testing with `scripts/loadtest.sh` (1000 requests at 50 concurrency per device type):

| Metric | Cold (MISS) | Warm (HIT) |
|--------|-------------|------------|
| Requests/sec | ~200-400 | ~2000-5000 |
| Avg latency | 20-80ms | 2-10ms |
| P99 latency | 150-300ms | 20-50ms |
| Cache hit rate | 0% (first pass) | 95%+ (steady state) |

Throughput scales linearly with Redis — multiple SmartCDN instances share the same cache. Processing latency depends on source image size and libvips is constant-memory (~50 MB RSS under load), so the server won't OOM regardless of concurrency.

Run your own benchmarks:

```bash
# Seed images first
make seed

# Run load test (uses hey if installed, falls back to curl)
make loadtest

# Custom concurrency/requests
REQUESTS=5000 CONCURRENCY=100 bash scripts/loadtest.sh <image_id>
```

## Design Decisions

**Go stdlib `net/http` over Echo/Gin** — Fewer dependencies, simpler debugging, and Go 1.22+ `ServeMux` has pattern matching (`GET /img/{id}`) which covers all routing needs. No framework overhead.

**bimg/libvips over ImageMagick** — libvips uses constant memory under load (streaming pixel pipeline), while ImageMagick loads entire images into RAM. At scale this is the difference between stable 50 MB RSS and OOM kills. Used in production at Cloudflare and Shopify.

**Redis for variant cache over filesystem** — Enables horizontal scaling. Multiple SmartCDN instances share the same cache without sticky sessions or distributed filesystem. Redis also gives TTL expiry for free.

**Cache-aside pattern over write-through** — Only cache what's actually requested. A 10K-image catalog with 5 device tiers and 2 formats = 100K variants. Pre-generating all of them wastes storage and compute for images nobody views.

**Format negotiation via Accept header** — Browsers send `Accept: image/webp` if they support it. We check this rather than relying on User-Agent, so format support is always accurate. Falls back to JPEG for clients that don't advertise WebP.

**MinIO over direct filesystem** — S3-compatible API means the storage layer works identically in local dev (MinIO) and production (AWS S3, GCS, R2). Swap the endpoint and credentials, zero code changes.

**Structured JSON logging with slog** — Go 1.21+ stdlib. Machine-parseable logs that work with any log aggregator. No external logging dependencies.

## Future Improvements

- **Edge caching with consistent hashing** — Distribute cache across edge nodes using consistent hashing so that the same image-device combination always hits the same node, maximizing hit rate without replicating the full cache everywhere.

- **AVIF support** — AVIF offers 20-30% better compression than WebP at equivalent quality. Add format negotiation for `Accept: image/avif` once libvips AVIF encoding stabilizes.

- **ML-based quality tuning** — Instead of fixed quality per device tier, use a perceptual quality model (SSIM/VMAF) to find the minimum quality that's visually indistinguishable from the original. Could save an additional 15-30% bandwidth.

- **Pre-warming popular images** — Track request frequency per image and proactively generate variants for the top-N most requested images across all device tiers, eliminating cold-start latency for popular content.

- **Content-aware cropping** — Use saliency detection to intelligently crop images for mobile viewports instead of just resizing, keeping the subject in frame.

- **Request coalescing** — When multiple concurrent requests arrive for the same uncached variant, process it once and fan out the result. Prevents thundering herd on cache miss for popular images.

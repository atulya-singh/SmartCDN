# SmartCDN

On-the-fly image optimization engine. Dynamically resizes, compresses, and transcodes images based on the requesting device.

## Quick Start

```bash
docker compose up --build
```

Services:

| Service    | URL                    |
|------------|------------------------|
| App        | http://localhost:8080  |
| MinIO Console | http://localhost:9001 |
| Prometheus | http://localhost:9090  |
| Grafana    | http://localhost:3000  |

## API

- `GET /health` — health check
- `POST /upload` — upload an image (multipart form, field: `file`)
- `GET /img/{id}` — get optimized image (adapts to User-Agent / Accept headers)
- `GET /metrics` — Prometheus metrics

## Grafana Dashboard

Grafana is auto-provisioned with a **SmartCDN** dashboard on startup. No manual setup needed.

1. Open http://localhost:3000
2. Login with `admin` / `admin`
3. The SmartCDN dashboard is set as the home dashboard

### Panels

- **Request Rate by Device Class** — per-second request rate broken down by device tier
- **Cache Hit Ratio** — gauge showing current hit rate, plus a time-series graph
- **Image Processing Latency** — p50, p95, p99 processing times
- **Bandwidth: Original vs Served** — stacked area chart comparing bytes before and after optimization
- **Top Device Classes** — pie chart of total requests by device class

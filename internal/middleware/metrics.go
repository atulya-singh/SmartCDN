package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smartcdn_requests_total",
		Help: "Total number of requests processed.",
	}, []string{"device_class", "cache_status", "format"})

	requestDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "smartcdn_request_duration_seconds",
		Help:    "Request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	processingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "smartcdn_image_processing_duration_seconds",
		Help:    "Image processing duration in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	bytesOriginalTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartcdn_bytes_original_total",
		Help: "Total bytes of original images fetched.",
	})

	bytesServedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartcdn_bytes_served_total",
		Help: "Total bytes of optimized images served.",
	})
)

// RecordProcessingDuration records the duration of an image processing operation.
// Call this from the image handler around the Transform call.
func RecordProcessingDuration(d time.Duration) {
	processingDuration.Observe(d.Seconds())
}

// RecordOriginalBytes records original image bytes fetched from storage.
func RecordOriginalBytes(n int) {
	bytesOriginalTotal.Add(float64(n))
}

// Metrics returns middleware that records Prometheus metrics for each request.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		requestDuration.Observe(duration.Seconds())

		// Extract labels from response headers set by the image handler
		deviceClass := rw.Header().Get("X-SmartCDN-Device")
		cacheStatus := rw.Header().Get("X-SmartCDN-Cache")
		contentType := rw.Header().Get("Content-Type")

		format := formatFromContentType(contentType)

		if deviceClass != "" && cacheStatus != "" {
			requestsTotal.WithLabelValues(deviceClass, cacheStatus, format).Inc()
		}

		// Track served bytes
		if rw.bytesWritten > 0 {
			bytesServedTotal.Add(float64(rw.bytesWritten))
		}

		// Track original bytes from header (only on MISS)
		if origStr := rw.Header().Get("X-SmartCDN-Original-Size"); origStr != "" {
			if orig, err := strconv.Atoi(origStr); err == nil {
				RecordOriginalBytes(orig)
			}
		}
	})
}

func formatFromContentType(ct string) string {
	switch ct {
	case "image/webp":
		return "webp"
	case "image/jpeg":
		return "jpeg"
	default:
		return "other"
	}
}

package handler

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/at/smartcdn/internal/cache"
	"github.com/at/smartcdn/internal/device"
	"github.com/at/smartcdn/internal/middleware"
)

// ImageStore retrieves original images from object storage.
type ImageStore interface {
	Download(ctx context.Context, imageID string) ([]byte, string, error)
}

// ImageCache caches and retrieves processed image variants.
type ImageCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
	DefaultTTL() time.Duration
}

// ImageTransformer resizes, compresses, and converts images.
type ImageTransformer interface {
	Transform(imageData []byte, width, quality int, format string) ([]byte, error)
}

// ImageHandler serves optimized images based on device classification.
type ImageHandler struct {
	store ImageStore
	cache ImageCache
	proc  ImageTransformer
}

// NewImageHandler creates a new ImageHandler with the given dependencies.
func NewImageHandler(store ImageStore, c ImageCache, proc ImageTransformer) *ImageHandler {
	return &ImageHandler{
		store: store,
		cache: c,
		proc:  proc,
	}
}

func (h *ImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	imageID := r.PathValue("id")
	if imageID == "" {
		http.Error(w, "missing image ID", http.StatusBadRequest)
		return
	}

	// Classify the requesting device
	profile := device.Classify(r.UserAgent())
	format := device.NegotiateFormat(r.Header.Get("Accept"), profile.PreferredFormat)

	// Build cache key
	key := cache.BuildKey(imageID, string(profile.Class), format)

	// Set common headers
	w.Header().Set("X-SmartCDN-Device", string(profile.Class))

	// ETag based on cache key hash
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(key)))
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(h.cache.DefaultTTL().Seconds())))

	// Check If-None-Match
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Try cache
	cached, err := h.cache.Get(r.Context(), key)
	if err == nil {
		// Cache HIT — we don't know original size from cache alone,
		// so we serve without original/savings headers on cache hits.
		w.Header().Set("X-SmartCDN-Cache", "HIT")
		w.Header().Set("X-SmartCDN-Optimized-Size", strconv.Itoa(len(cached)))
		w.Header().Set("Content-Type", contentTypeForFormat(format))
		w.Header().Set("Content-Length", strconv.Itoa(len(cached)))
		w.Write(cached)
		return
	}
	if !errors.Is(err, cache.ErrMiss) {
		slog.Error("cache get error", "key", key, "error", err)
	}

	// Cache MISS — fetch original from storage
	original, _, err := h.store.Download(r.Context(), imageID)
	if err != nil {
		slog.Error("storage download failed", "imageID", imageID, "error", err)
		http.Error(w, "image not found", http.StatusNotFound)
		return
	}

	// Transform
	procStart := time.Now()
	processed, err := h.proc.Transform(original, profile.MaxWidth, profile.Quality, format)
	middleware.RecordProcessingDuration(time.Since(procStart))
	if err != nil {
		slog.Error("image processing failed", "imageID", imageID, "error", err)
		http.Error(w, "failed to process image", http.StatusInternalServerError)
		return
	}

	// Cache the result asynchronously with a detached context
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.cache.Set(ctx, key, processed, h.cache.DefaultTTL()); err != nil {
			slog.Error("cache set error", "key", key, "error", err)
		}
	}()

	// Calculate savings
	originalSize := len(original)
	optimizedSize := len(processed)
	savings := 0.0
	if originalSize > 0 {
		savings = (1.0 - float64(optimizedSize)/float64(originalSize)) * 100
	}

	w.Header().Set("X-SmartCDN-Cache", "MISS")
	w.Header().Set("X-SmartCDN-Original-Size", strconv.Itoa(originalSize))
	w.Header().Set("X-SmartCDN-Optimized-Size", strconv.Itoa(optimizedSize))
	w.Header().Set("X-SmartCDN-Savings", fmt.Sprintf("%.1f%%", savings))
	w.Header().Set("Content-Type", contentTypeForFormat(format))
	w.Header().Set("Content-Length", strconv.Itoa(optimizedSize))
	w.Write(processed)
}

func contentTypeForFormat(format string) string {
	switch format {
	case "webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

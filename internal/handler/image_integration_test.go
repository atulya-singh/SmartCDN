package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/at/smartcdn/internal/cache"
	"github.com/at/smartcdn/internal/config"
	"github.com/at/smartcdn/internal/handler"
	"github.com/at/smartcdn/internal/processor"
	"github.com/at/smartcdn/internal/storage"
)

// testServer sets up a full HTTP server with all dependencies.
// Skips the test if MinIO or Redis are not available.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := &config.Config{
		Port:            0,
		MinioEndpoint:   getTestEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:  getTestEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:  getTestEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:     getTestEnv("MINIO_BUCKET", "smartcdn-test"),
		RedisAddr:       getTestEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getTestEnv("REDIS_PASSWORD", ""),
		CacheTTLSeconds: 60,
		LogLevel:        "error",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := storage.NewStorage(ctx, cfg)
	if err != nil {
		t.Skipf("skipping: MinIO not available: %v", err)
	}

	imgCache := cache.NewCache(cfg)
	proc := processor.NewProcessor()

	mux := http.NewServeMux()
	mux.Handle("POST /upload", handler.NewUploadHandler(store))
	mux.Handle("GET /img/{id}", handler.NewImageHandler(store, imgCache, proc))
	mux.Handle("GET /health", handler.NewHealthHandler(time.Now(), nil))

	return httptest.NewServer(mux)
}

// generateTestJPEG creates a simple JPEG image of the given dimensions.
func generateTestJPEG(width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 255) / width),
				G: uint8((y * 255) / height),
				B: 128,
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// uploadImage uploads a JPEG to the server and returns the image ID.
func uploadImage(t *testing.T, serverURL string, imgData []byte) string {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "test.jpg")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write(imgData)
	writer.Close()

	resp, err := http.Post(serverURL+"/upload", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload failed with status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}
	if result.ID == "" {
		t.Fatal("upload returned empty image ID")
	}
	return result.ID
}

func fetchImage(t *testing.T, serverURL, imageID, userAgent string) *http.Response {
	t.Helper()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/img/%s", serverURL, imageID), nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "image/webp,image/jpeg,*/*")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("image request failed: %v", err)
	}
	return resp
}

func TestIntegration_DifferentDevicesGetDifferentSizes(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	imgData, err := generateTestJPEG(2000, 1500)
	if err != nil {
		t.Fatalf("failed to generate test image: %v", err)
	}

	imageID := uploadImage(t, srv.URL, imgData)

	devices := []struct {
		name            string
		userAgent       string
		expectedDevice  string
		expectedSmaller bool // should be smaller than original
	}{
		{
			name:            "iPhone Safari (mobile-high)",
			userAgent:       "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			expectedDevice:  "mobile-high",
			expectedSmaller: true,
		},
		{
			name:            "Old Android (mobile-low)",
			userAgent:       "Mozilla/5.0 (Linux; Android 4.4.2; SM-G900F Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/45.0.2454.94 Mobile Safari/537.36",
			expectedDevice:  "mobile-low",
			expectedSmaller: true,
		},
		{
			name:            "iPad (tablet)",
			userAgent:       "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			expectedDevice:  "tablet",
			expectedSmaller: true,
		},
		{
			name:            "Chrome Desktop",
			userAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			expectedDevice:  "desktop",
			expectedSmaller: true,
		},
	}

	sizes := make(map[string]int)

	for _, d := range devices {
		t.Run(d.name, func(t *testing.T) {
			resp := fetchImage(t, srv.URL, imageID, d.userAgent)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response: %v", err)
			}

			// Check device classification header
			deviceHeader := resp.Header.Get("X-SmartCDN-Device")
			if deviceHeader != d.expectedDevice {
				t.Errorf("X-SmartCDN-Device = %q, want %q", deviceHeader, d.expectedDevice)
			}

			// First request should be a cache MISS
			cacheHeader := resp.Header.Get("X-SmartCDN-Cache")
			if cacheHeader != "MISS" {
				t.Errorf("X-SmartCDN-Cache = %q, want MISS (first request)", cacheHeader)
			}

			// Content-Type should be an image
			ct := resp.Header.Get("Content-Type")
			if ct != "image/webp" && ct != "image/jpeg" {
				t.Errorf("Content-Type = %q, want image/webp or image/jpeg", ct)
			}

			// Verify savings headers present on MISS
			if resp.Header.Get("X-SmartCDN-Original-Size") == "" {
				t.Error("missing X-SmartCDN-Original-Size header")
			}
			if resp.Header.Get("X-SmartCDN-Optimized-Size") == "" {
				t.Error("missing X-SmartCDN-Optimized-Size header")
			}
			if resp.Header.Get("X-SmartCDN-Savings") == "" {
				t.Error("missing X-SmartCDN-Savings header")
			}

			sizes[d.name] = len(body)
		})
	}

	// Verify mobile-low produces the smallest output
	if sizes["Old Android (mobile-low)"] > 0 && sizes["Chrome Desktop"] > 0 {
		if sizes["Old Android (mobile-low)"] >= sizes["Chrome Desktop"] {
			t.Errorf("mobile-low (%d bytes) should be smaller than desktop (%d bytes)",
				sizes["Old Android (mobile-low)"], sizes["Chrome Desktop"])
		}
	}
}

func TestIntegration_CacheHitOnSecondRequest(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	imgData, err := generateTestJPEG(800, 600)
	if err != nil {
		t.Fatalf("failed to generate test image: %v", err)
	}

	imageID := uploadImage(t, srv.URL, imgData)
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15"

	// First request — should be MISS
	resp1 := fetchImage(t, srv.URL, imageID, ua)
	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()

	if resp1.Header.Get("X-SmartCDN-Cache") != "MISS" {
		t.Errorf("first request: X-SmartCDN-Cache = %q, want MISS", resp1.Header.Get("X-SmartCDN-Cache"))
	}

	// Small delay to let async cache set complete
	time.Sleep(200 * time.Millisecond)

	// Second request — should be HIT
	resp2 := fetchImage(t, srv.URL, imageID, ua)
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()

	if resp2.Header.Get("X-SmartCDN-Cache") != "HIT" {
		t.Errorf("second request: X-SmartCDN-Cache = %q, want HIT", resp2.Header.Get("X-SmartCDN-Cache"))
	}

	// Both responses should return the same image data
	if !bytes.Equal(body1, body2) {
		t.Errorf("cached response differs from original: %d bytes vs %d bytes", len(body1), len(body2))
	}

	// Cache hit should still have device header
	if resp2.Header.Get("X-SmartCDN-Device") == "" {
		t.Error("cache HIT response missing X-SmartCDN-Device header")
	}
}

func TestIntegration_NotFoundImage(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	resp := fetchImage(t, srv.URL, "nonexistent-image-id", ua)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIntegration_HealthCheck(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %q, want ok", result["status"])
	}
}

func getTestEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

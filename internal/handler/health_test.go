package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/at/smartcdn/internal/handler"
)

type mockPinger struct{ err error }

func (m *mockPinger) Ping(_ context.Context) error { return m.err }

func TestHealth_NoPingers(t *testing.T) {
	h := handler.NewHealthHandler(time.Now(), nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestHealth_AllDepsHealthy(t *testing.T) {
	pingers := map[string]handler.Pinger{
		"redis": &mockPinger{},
		"minio": &mockPinger{},
	}
	h := handler.NewHealthHandler(time.Now(), pingers)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	deps := body["deps"].(map[string]any)
	if deps["redis"] != "ok" {
		t.Errorf("redis = %q, want ok", deps["redis"])
	}
	if deps["minio"] != "ok" {
		t.Errorf("minio = %q, want ok", deps["minio"])
	}
}

func TestHealth_OneDepDegraded(t *testing.T) {
	pingers := map[string]handler.Pinger{
		"redis": &mockPinger{err: errors.New("connection refused")},
		"minio": &mockPinger{},
	}
	h := handler.NewHealthHandler(time.Now(), pingers)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "degraded" {
		t.Errorf("status = %q, want degraded", body["status"])
	}
	deps := body["deps"].(map[string]any)
	if deps["redis"] != "unhealthy" {
		t.Errorf("redis = %q, want unhealthy", deps["redis"])
	}
	if deps["minio"] != "ok" {
		t.Errorf("minio = %q, want ok", deps["minio"])
	}
}

func TestHealth_ContainsUptime(t *testing.T) {
	h := handler.NewHealthHandler(time.Now().Add(-5*time.Second), nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))

	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	uptime, ok := body["uptime"]
	if !ok || uptime == "" {
		t.Error("missing or empty uptime field")
	}
}

func TestHealth_ContentTypeJSON(t *testing.T) {
	h := handler.NewHealthHandler(time.Now(), nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

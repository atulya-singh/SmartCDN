package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/at/smartcdn/internal/middleware"
)

func handlerWith(code int, body string, headers map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(code)
		w.Write([]byte(body))
	})
}

func TestLogging_PassesResponseThrough(t *testing.T) {
	h := middleware.Logging(handlerWith(http.StatusOK, "hello", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Errorf("body = %q, want hello", w.Body.String())
	}
}

func TestLogging_Non200StatusPassesThrough(t *testing.T) {
	h := middleware.Logging(handlerWith(http.StatusNotFound, "not found", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/missing", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestLogging_ResponseHeadersPassThrough(t *testing.T) {
	headers := map[string]string{
		"X-SmartCDN-Device": "mobile-high",
		"X-SmartCDN-Cache":  "HIT",
	}
	h := middleware.Logging(handlerWith(http.StatusOK, "", headers))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/img/x", nil))

	if got := w.Header().Get("X-SmartCDN-Device"); got != "mobile-high" {
		t.Errorf("X-SmartCDN-Device = %q, want mobile-high", got)
	}
}

func TestMetrics_PassesResponseThrough(t *testing.T) {
	h := middleware.Metrics(handlerWith(http.StatusOK, "img", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/img/abc", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "img" {
		t.Errorf("body = %q, want img", w.Body.String())
	}
}

func TestMetrics_WithImageHeaders(t *testing.T) {
	// Verify metrics middleware works correctly with the SmartCDN headers it reads.
	headers := map[string]string{
		"X-SmartCDN-Device":        "desktop",
		"X-SmartCDN-Cache":         "MISS",
		"X-SmartCDN-Original-Size": "102400",
		"Content-Type":             "image/webp",
	}
	h := middleware.Metrics(handlerWith(http.StatusOK, "data", headers))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/img/abc", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestMetrics_500StatusPassesThrough(t *testing.T) {
	h := middleware.Metrics(handlerWith(http.StatusInternalServerError, "err", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/img/bad", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

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

func TestAPIKeyAuth_NoKeyConfigured(t *testing.T) {
	// When no key is set, all requests pass through regardless of Authorization header.
	h := middleware.APIKeyAuth("", handlerWith(http.StatusOK, "ok", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/upload", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when no key configured", w.Code)
	}
}

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	h := middleware.APIKeyAuth("secret123", handlerWith(http.StatusOK, "ok", nil))
	req := httptest.NewRequest("POST", "/upload", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for valid key", w.Code)
	}
}

func TestAPIKeyAuth_InvalidKey(t *testing.T) {
	h := middleware.APIKeyAuth("secret123", handlerWith(http.StatusOK, "ok", nil))
	req := httptest.NewRequest("POST", "/upload", nil)
	req.Header.Set("Authorization", "Bearer wrongkey")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for invalid key", w.Code)
	}
}

func TestAPIKeyAuth_MissingHeader(t *testing.T) {
	h := middleware.APIKeyAuth("secret123", handlerWith(http.StatusOK, "ok", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/upload", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when Authorization header missing", w.Code)
	}
}

func TestRateLimit_AllowsNormalTraffic(t *testing.T) {
	rl := middleware.NewRateLimiter(100, 200)
	h := rl.Limit(handlerWith(http.StatusOK, "ok", nil))
	// 10 requests well within limits should all pass
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/img/x", nil))
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, w.Code)
		}
	}
}

func TestRateLimit_BlocksWhenExceeded(t *testing.T) {
	// 1 RPS, burst of 1: second request should be rejected.
	rl := middleware.NewRateLimiter(1, 1)
	h := rl.Limit(handlerWith(http.StatusOK, "ok", nil))

	// First request consumes the burst token.
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, httptest.NewRequest("GET", "/img/x", nil))
	if w1.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want 200", w1.Code)
	}

	// Second immediate request exceeds the limit.
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequest("GET", "/img/x", nil))
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want 429", w2.Code)
	}
}

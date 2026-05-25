package handler_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/at/smartcdn/internal/cache"
	"github.com/at/smartcdn/internal/handler"
)

// stubStore implements handler.ImageStore.
type stubStore struct {
	data []byte
	err  error
}

func (s *stubStore) Download(_ context.Context, _ string) ([]byte, string, error) {
	return s.data, "image/jpeg", s.err
}

// stubCache implements handler.ImageCache with a mutex to handle the async cache.Set goroutine.
type stubCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (s *stubCache) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.data[key]; ok {
		return v, nil
	}
	return nil, cache.ErrMiss
}

func (s *stubCache) Set(_ context.Context, key string, data []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = data
	return nil
}

func (s *stubCache) DefaultTTL() time.Duration { return time.Minute }

// stubTransformer implements handler.ImageTransformer.
type stubTransformer struct {
	out []byte
	err error
}

func (s *stubTransformer) Transform(data []byte, _, _ int, _ string) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.out != nil {
		return s.out, nil
	}
	return data, nil
}

func imgRequest(t *testing.T, id, userAgent, accept string) *http.Request {
	t.Helper()
	req := httptest.NewRequest("GET", "/img/"+id, nil)
	req.SetPathValue("id", id)
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return req
}

func TestImage_MissingID(t *testing.T) {
	h := handler.NewImageHandler(&stubStore{}, &stubCache{data: map[string][]byte{}}, &stubTransformer{})
	req := httptest.NewRequest("GET", "/img/", nil)
	// No SetPathValue → PathValue("id") returns ""
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestImage_CacheHit(t *testing.T) {
	imgData := []byte("cached-image-bytes")
	c := &stubCache{data: map[string][]byte{
		"smartcdn:abc:unknown:jpeg": imgData,
	}}
	h := handler.NewImageHandler(&stubStore{}, c, &stubTransformer{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, imgRequest(t, "abc", "", ""))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("X-SmartCDN-Cache") != "HIT" {
		t.Errorf("X-SmartCDN-Cache = %q, want HIT", w.Header().Get("X-SmartCDN-Cache"))
	}
	if w.Body.String() != string(imgData) {
		t.Error("body does not match cached data")
	}
}

func TestImage_CacheMiss_ServesProcessed(t *testing.T) {
	original := []byte("original-bytes")
	processed := []byte("processed-bytes")
	h := handler.NewImageHandler(
		&stubStore{data: original},
		&stubCache{data: map[string][]byte{}},
		&stubTransformer{out: processed},
	)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, imgRequest(t, "img1", "", ""))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("X-SmartCDN-Cache") != "MISS" {
		t.Errorf("X-SmartCDN-Cache = %q, want MISS", w.Header().Get("X-SmartCDN-Cache"))
	}
	if w.Header().Get("X-SmartCDN-Original-Size") == "" {
		t.Error("missing X-SmartCDN-Original-Size header")
	}
	if w.Header().Get("X-SmartCDN-Savings") == "" {
		t.Error("missing X-SmartCDN-Savings header")
	}
	if w.Body.String() != string(processed) {
		t.Error("body does not match processed data")
	}
}

func TestImage_StorageNotFound(t *testing.T) {
	h := handler.NewImageHandler(
		&stubStore{err: errors.New("not found")},
		&stubCache{data: map[string][]byte{}},
		&stubTransformer{},
	)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, imgRequest(t, "missing", "", ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestImage_TransformError(t *testing.T) {
	h := handler.NewImageHandler(
		&stubStore{data: []byte("img")},
		&stubCache{data: map[string][]byte{}},
		&stubTransformer{err: errors.New("libvips failure")},
	)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, imgRequest(t, "bad", "", ""))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestImage_ETagNotModified(t *testing.T) {
	// Pre-compute the ETag the handler will produce for this request.
	// UA="" → unknown class, Accept="" → jpeg. Key = smartcdn:x:unknown:jpeg
	key := "smartcdn:x:unknown:jpeg"
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(key)))

	c := &stubCache{data: map[string][]byte{key: []byte("data")}}
	h := handler.NewImageHandler(&stubStore{}, c, &stubTransformer{})

	req := imgRequest(t, "x", "", "")
	req.Header.Set("If-None-Match", etag)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", w.Code)
	}
}

func TestImage_DeviceClassHeader(t *testing.T) {
	tests := []struct {
		userAgent  string
		wantDevice string
	}{
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0)", "mobile-high"},
		{"Mozilla/5.0 (Linux; Android 4.4.2; SM-G900F) Mobile", "mobile-low"},
		{"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X)", "tablet"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120", "desktop"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.wantDevice, func(t *testing.T) {
			h := handler.NewImageHandler(
				&stubStore{data: []byte("img")},
				&stubCache{data: map[string][]byte{}},
				&stubTransformer{},
			)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, imgRequest(t, "id", tt.userAgent, ""))

			got := w.Header().Get("X-SmartCDN-Device")
			if got != tt.wantDevice {
				t.Errorf("X-SmartCDN-Device = %q, want %q", got, tt.wantDevice)
			}
		})
	}
}

func TestImage_WebPServedWhenAccepted(t *testing.T) {
	h := handler.NewImageHandler(
		&stubStore{data: []byte("img")},
		&stubCache{data: map[string][]byte{}},
		&stubTransformer{},
	)
	// Desktop UA + Accept: image/webp → format should be webp
	w := httptest.NewRecorder()
	h.ServeHTTP(w, imgRequest(t,
		"id",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120",
		"image/webp,image/jpeg,*/*",
	))

	if ct := w.Header().Get("Content-Type"); ct != "image/webp" {
		t.Errorf("Content-Type = %q, want image/webp", ct)
	}
}

func TestImage_ContentLengthSet(t *testing.T) {
	data := []byte("some-image-data")
	c := &stubCache{data: map[string][]byte{
		"smartcdn:z:unknown:jpeg": data,
	}}
	h := handler.NewImageHandler(&stubStore{}, c, &stubTransformer{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, imgRequest(t, "z", "", ""))

	if w.Header().Get("Content-Length") == "" {
		t.Error("missing Content-Length header")
	}
}

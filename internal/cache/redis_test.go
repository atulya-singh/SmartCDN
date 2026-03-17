package cache

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/at/smartcdn/internal/config"
)

func TestBuildKey(t *testing.T) {
	tests := []struct {
		imageID     string
		deviceClass string
		format      string
		want        string
	}{
		{"abc123", "mobile-low", "webp", "smartcdn:abc123:mobile-low:webp"},
		{"img-456", "desktop", "jpeg", "smartcdn:img-456:desktop:jpeg"},
		{"x", "unknown", "jpeg", "smartcdn:x:unknown:jpeg"},
	}

	for _, tt := range tests {
		got := BuildKey(tt.imageID, tt.deviceClass, tt.format)
		if got != tt.want {
			t.Errorf("BuildKey(%q, %q, %q) = %q, want %q",
				tt.imageID, tt.deviceClass, tt.format, got, tt.want)
		}
	}
}

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test (requires Redis)")
	}
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	cfg := &config.Config{
		RedisAddr:       addr,
		RedisPassword:   "",
		CacheTTLSeconds: 60,
	}
	c := NewCache(cfg)

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping: Redis not available: %v", err)
	}
	return c
}

func TestGet_Miss(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_, err := c.Get(ctx, "smartcdn:nonexistent:mobile-low:webp")
	if !errors.Is(err, ErrMiss) {
		t.Errorf("expected ErrMiss, got %v", err)
	}
}

func TestSetThenGet(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	key := "smartcdn:test-integration:desktop:jpeg"
	data := []byte("fake image data for testing")

	if err := c.Set(ctx, key, data, 10*time.Second); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}

	// Cleanup
	c.client.Del(ctx, key)
}

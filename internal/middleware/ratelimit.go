package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type clientBucket struct {
	mu       sync.Mutex
	tokens   float64
	lastFill time.Time
}

// allow implements a token bucket: refills at `rps` tokens/sec up to `burst`.
func (b *clientBucket) allow(rps float64, burst int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.lastFill = now

	b.tokens += elapsed * rps
	if b.tokens > float64(burst) {
		b.tokens = float64(burst)
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimiter is a per-IP token bucket rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientEntry
	rps     float64
	burst   int
}

type clientEntry struct {
	bucket   *clientBucket
	lastSeen time.Time
}

// NewRateLimiter creates a limiter allowing rps requests/sec per IP with a burst capacity.
func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*clientEntry),
		rps:     float64(rps),
		burst:   burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) bucket(ip string) *clientBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	e, ok := rl.clients[ip]
	if !ok {
		e = &clientEntry{
			bucket: &clientBucket{
				tokens:   float64(rl.burst),
				lastFill: time.Now(),
			},
		}
		rl.clients[ip] = e
	}
	e.lastSeen = time.Now()
	return e.bucket
}

// cleanup evicts entries that haven't been seen in 10 minutes.
func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		for ip, e := range rl.clients {
			if time.Since(e.lastSeen) > 10*time.Minute {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Limit returns middleware that enforces the rate limit.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.bucket(ip).allow(rl.rps, rl.burst) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the real client IP, respecting X-Forwarded-For from trusted proxies.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

package middleware

import (
	"net/http"
)

// APIKeyAuth protects a handler with a Bearer token check.
// If key is empty, all requests are allowed (useful for local dev).
func APIKeyAuth(key string, next http.Handler) http.Handler {
	if key == "" {
		return next
	}
	expected := "Bearer " + key
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expected {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

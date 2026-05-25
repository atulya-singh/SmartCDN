package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Pinger is implemented by any dependency that can report its health.
type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	startTime time.Time
	pingers   map[string]Pinger
}

func NewHealthHandler(startTime time.Time, pingers map[string]Pinger) *HealthHandler {
	return &HealthHandler{startTime: startTime, pingers: pingers}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	deps := make(map[string]string, len(h.pingers))
	allOK := true
	for name, p := range h.pingers {
		if err := p.Ping(ctx); err != nil {
			deps[name] = "unhealthy"
			allOK = false
		} else {
			deps[name] = "ok"
		}
	}

	status := "ok"
	code := http.StatusOK
	if !allOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"uptime": time.Since(h.startTime).Round(time.Second).String(),
		"deps":   deps,
	})
}

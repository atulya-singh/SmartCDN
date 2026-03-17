package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthHandler struct {
	startTime time.Time
}

func NewHealthHandler(startTime time.Time) *HealthHandler {
	return &HealthHandler{startTime: startTime}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"uptime": time.Since(h.startTime).Round(time.Second).String(),
	})
}

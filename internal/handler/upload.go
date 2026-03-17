package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/at/smartcdn/internal/storage"
)

const maxUploadSize = 10 << 20 // 10MB

var allowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

type UploadHandler struct {
	store *storage.Storage
}

func NewUploadHandler(store *storage.Storage) *UploadHandler {
	return &UploadHandler{store: store}
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "file too large (max 10MB)", http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing 'image' field in form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !allowedContentTypes[contentType] {
		http.Error(w, "unsupported content type: must be image/jpeg, image/png, or image/webp", http.StatusUnsupportedMediaType)
		return
	}

	id, err := h.store.Upload(r.Context(), header.Filename, file, contentType)
	if err != nil {
		slog.Error("upload failed", "error", err)
		http.Error(w, "failed to store image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":   id,
		"size": header.Size,
	})
}

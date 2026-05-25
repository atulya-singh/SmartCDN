package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

const maxUploadSize = 10 << 20 // 10MB

// Uploader stores original image files.
type Uploader interface {
	Upload(ctx context.Context, filename string, data io.Reader, contentType string) (string, error)
}

// image format magic bytes
var imageMagic = []struct {
	mime   string
	prefix []byte
}{
	{"image/jpeg", []byte{0xFF, 0xD8, 0xFF}},
	{"image/png", []byte{0x89, 0x50, 0x4E, 0x47}},
	{"image/webp", []byte{0x52, 0x49, 0x46, 0x46}}, // "RIFF" — WebP container
}

type UploadHandler struct {
	store Uploader
}

func NewUploadHandler(store Uploader) *UploadHandler {
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

	// Read the full body so we can sniff magic bytes and re-present to storage.
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusBadRequest)
		return
	}

	contentType := DetectImageType(data)
	if contentType == "" {
		http.Error(w, "unsupported file type: must be JPEG, PNG, or WebP", http.StatusUnsupportedMediaType)
		return
	}

	id, err := h.store.Upload(r.Context(), header.Filename, bytes.NewReader(data), contentType)
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

// DetectImageType returns the MIME type by inspecting magic bytes.
// Returns empty string if the format is not JPEG, PNG, or WebP.
func DetectImageType(data []byte) string {
	for _, m := range imageMagic {
		if bytes.HasPrefix(data, m.prefix) {
			return m.mime
		}
	}
	return ""
}

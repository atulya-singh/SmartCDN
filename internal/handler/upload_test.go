package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/at/smartcdn/internal/handler"
)

type stubUploader struct {
	id  string
	err error
}

func (s *stubUploader) Upload(_ context.Context, _ string, _ io.Reader, _ string) (string, error) {
	return s.id, s.err
}

func makeUploadRequest(data []byte, fieldName string) *http.Request {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if fieldName != "" {
		part, _ := writer.CreateFormFile(fieldName, "test.jpg")
		part.Write(data)
	}
	writer.Close()
	req := httptest.NewRequest("POST", "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestDetectImageType(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, "image/jpeg"},
		{"png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, "image/png"},
		{"webp", []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}, "image/webp"},
		{"riff non-webp rejected", []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x41, 0x56, 0x49, 0x20}, ""},
		{"unknown", []byte{0x00, 0x01, 0x02, 0x03}, ""},
		{"empty", []byte{}, ""},
		{"too short for jpeg", []byte{0xFF, 0xD8}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.DetectImageType(tt.data)
			if got != tt.want {
				t.Errorf("DetectImageType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUploadHandler_Success(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}
	h := handler.NewUploadHandler(&stubUploader{id: "abc-123"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, makeUploadRequest(jpegData, "image"))

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] != "abc-123" {
		t.Errorf("id = %v, want abc-123", resp["id"])
	}
}

func TestUploadHandler_BadMagicBytes(t *testing.T) {
	h := handler.NewUploadHandler(&stubUploader{id: "x"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, makeUploadRequest([]byte{0x00, 0x01, 0x02, 0x03, 0x04}, "image"))

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", w.Code)
	}
}

func TestUploadHandler_MissingField(t *testing.T) {
	// Send a multipart body with a different field name, not "image"
	h := handler.NewUploadHandler(&stubUploader{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, makeUploadRequest([]byte{0xFF, 0xD8, 0xFF}, "other"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUploadHandler_StorageError(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	h := handler.NewUploadHandler(&stubUploader{err: errors.New("bucket full")})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, makeUploadRequest(jpegData, "image"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestUploadHandler_PNGAccepted(t *testing.T) {
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	h := handler.NewUploadHandler(&stubUploader{id: "png-id"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, makeUploadRequest(pngData, "image"))

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201 for PNG", w.Code)
	}
}

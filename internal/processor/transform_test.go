package processor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata", name)
}

func TestTransform_ResizeToWebP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires libvips")
	}
	input, err := os.ReadFile(testdataPath("test_100x100.jpg"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	proc := NewProcessor()
	output, err := proc.Transform(input, TransformOptions{
		Width:   50,
		Quality: 75,
		Format:  "webp",
	})
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(output) == 0 {
		t.Fatal("output is empty")
	}

	if len(output) >= len(input) {
		t.Errorf("expected output (%d bytes) to be smaller than input (%d bytes)", len(output), len(input))
	}

	// WebP files start with "RIFF" magic bytes
	if len(output) < 4 || string(output[:4]) != "RIFF" {
		t.Error("output does not have WebP RIFF header")
	}
}

func TestTransform_NoUpscale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires libvips")
	}
	input, err := os.ReadFile(testdataPath("test_100x100.jpg"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	proc := NewProcessor()
	// Request width larger than original — should not upscale
	output, err := proc.Transform(input, TransformOptions{
		Width:   500,
		Quality: 90,
		Format:  "jpeg",
	})
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(output) == 0 {
		t.Fatal("output is empty")
	}
}

func TestTransform_ZeroInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires libvips")
	}
	proc := NewProcessor()
	_, err := proc.Transform([]byte{}, TransformOptions{
		Width:   100,
		Quality: 80,
		Format:  "jpeg",
	})
	if err == nil {
		t.Error("expected error for zero-byte input")
	}
}

func TestTransform_UnsupportedFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires libvips")
	}
	proc := NewProcessor()
	_, err := proc.Transform([]byte("not an image"), TransformOptions{
		Width:   100,
		Quality: 80,
		Format:  "jpeg",
	})
	if err == nil {
		t.Error("expected error for unsupported input format")
	}
}

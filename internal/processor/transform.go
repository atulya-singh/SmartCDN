package processor

import (
	"fmt"

	"github.com/h2non/bimg"
)

// Processor handles image transformation via libvips/bimg.
type Processor struct{}

// NewProcessor creates a new Processor.
func NewProcessor() *Processor {
	return &Processor{}
}

// Transform resizes, compresses, and converts an image.
// width=0 means no resize. format must be "webp" or "jpeg".
func (p *Processor) Transform(imageData []byte, width, quality int, format string) ([]byte, error) {
	if len(imageData) == 0 {
		return nil, fmt.Errorf("transform: zero-byte input")
	}

	imgType := bimg.DetermineImageType(imageData)
	if imgType == bimg.UNKNOWN {
		return nil, fmt.Errorf("transform: unsupported input format")
	}

	img := bimg.NewImage(imageData)
	size, err := img.Size()
	if err != nil {
		return nil, fmt.Errorf("transform: failed to read image size: %w", err)
	}

	bimgOpts := bimg.Options{
		Quality: quality,
		Type:    toBimgType(format),
	}

	// Only resize if the image is wider than the target and target > 0
	if width > 0 && size.Width > width {
		bimgOpts.Width = width
	}

	out, err := img.Process(bimgOpts)
	if err != nil {
		return nil, fmt.Errorf("transform: processing failed: %w", err)
	}

	return out, nil
}

func toBimgType(format string) bimg.ImageType {
	switch format {
	case "webp":
		return bimg.WEBP
	case "jpeg", "jpg":
		return bimg.JPEG
	default:
		return bimg.JPEG
	}
}

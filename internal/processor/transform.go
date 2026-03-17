package processor

import (
	"fmt"

	"github.com/h2non/bimg"
)

// TransformOptions specifies how an image should be processed.
type TransformOptions struct {
	Width   int
	Quality int
	Format  string // "webp" or "jpeg"
}

// Processor handles image transformation via libvips/bimg.
type Processor struct{}

// NewProcessor creates a new Processor.
func NewProcessor() *Processor {
	return &Processor{}
}

// Transform resizes, compresses, and converts an image according to the given options.
func (p *Processor) Transform(imageData []byte, opts TransformOptions) ([]byte, error) {
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
		Quality: opts.Quality,
		Type:    toBimgType(opts.Format),
	}

	// Only resize if the image is wider than the target and target > 0
	if opts.Width > 0 && size.Width > opts.Width {
		bimgOpts.Width = opts.Width
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

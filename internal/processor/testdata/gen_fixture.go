//go:build ignore

package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
)

func main() {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 2),
				G: uint8(y * 2),
				B: 128,
				A: 255,
			})
		}
	}

	f, err := os.Create("internal/processor/testdata/test_100x100.jpg")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		panic(err)
	}
}

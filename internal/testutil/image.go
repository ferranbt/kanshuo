package testutil

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
)

func CropBottomQuarter(imagePath string) (string, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	height := bounds.Dy()
	startY := bounds.Min.Y + (height * 3 / 4)

	cropped := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(bounds.Min.X, startY, bounds.Max.X, bounds.Max.Y))

	tmpFile, err := os.CreateTemp("", "cropped-*."+format)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		err = jpeg.Encode(tmpFile, cropped, &jpeg.Options{Quality: 90})
	case "png":
		err = png.Encode(tmpFile, cropped)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	return tmpFile.Name(), nil
}

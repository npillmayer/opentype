package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

func writeProofPNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return fmt.Errorf("cannot encode PNG: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("cannot close PNG: %w", err)
	}
	return nil
}

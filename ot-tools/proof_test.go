package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// File output stays at the CLI edge after renderer extraction.
func TestWriteProofPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "space.png")
	img := image.NewRGBA(image.Rect(0, 0, 60, 90))
	if err := writeProofPNG(path, img); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := png.Decode(f)
	if err != nil || got.Bounds() != img.Bounds() {
		t.Fatalf("written PNG: %v %v", got, err)
	}
	if err := writeProofPNG(filepath.Join(path, "blocked.png"), img); err == nil {
		t.Fatal("ignored output-directory failure")
	}
}

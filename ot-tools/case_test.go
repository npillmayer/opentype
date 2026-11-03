package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSavedCaseSnapshotAndVariants(t *testing.T) {
	path := "../testdata/proof-cases/latin-features.json"
	result, img, err := prepareProofCase(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 4 || len(result.Case.FontSHA256) != 64 || !filepath.IsAbs(result.Case.Font) || result.Case.Script != "Latn" || result.Case.Preset != "" || result.Case.Render.PPEM != 72 {
		t.Fatalf("unresolved report: %+v", result.Case)
	}
	var counts [4]int
	for i, row := range result.Rows {
		for _, g := range row.Glyphs {
			if g.GID == 741 {
				counts[i]++
				if i == 3 && g.Cluster != 8 {
					t.Fatalf("range not anchored to second office: %+v", g)
				}
			}
		}
	}
	if counts != [4]int{2, 0, 2, 1} {
		t.Fatalf("ligature feature variants: %v", counts)
	}
	if reflect.DeepEqual(result.Rows[0].Glyphs, result.Rows[2].Glyphs) {
		t.Fatal("kerning variant made no difference")
	}
	data, err := json.Marshal(result.Case)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(t.TempDir(), "saved.case.json")
	if err := os.WriteFile(snapshot, data, 0o600); err != nil {
		t.Fatal(err)
	}
	replay, replayed, err := prepareProofCase(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Rows, replay.Rows) || img.Rect != replayed.Rect || !bytes.Equal(img.Pix, replayed.Pix) {
		t.Fatal("resolved case did not reproduce data/image")
	}
	// Prove that the stored fingerprint is enforced before shaping.
	result.Case.FontSHA256 = strings.Repeat("0", 64)
	data, _ = json.Marshal(result.Case)
	if err := os.WriteFile(snapshot, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareProofCase(snapshot); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("font mismatch: %v", err)
	}
	hebrew, _, err := prepareProofCase("../testdata/proof-cases/hebrew-marks.json")
	if err != nil {
		t.Fatal(err)
	}
	if hebrew.Case.Direction != "rtl" || hebrew.Rows[0].Glyphs[0].VisualIndex != len(hebrew.Rows[0].Glyphs)-1 {
		t.Fatal("Hebrew report lost logical/visual distinction")
	}
}

func TestCaseInputValidation(t *testing.T) {
	dir := t.TempDir()
	textPath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(textPath, []byte("cafe\u0301\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "case.json")
	for _, c := range []struct{ body, contains string }{
		{`{"version":2,"font":"font.ttf","text":"x","variants":[{"label":"a"}]}`, "version"},
		{`{"version":1,"font":"font.ttf","text":"x","text_file":"sample.txt","variants":[{"label":"a"}]}`, "exactly one"},
		{`{"version":1,"font":"font.ttf","text":"x","variants":[{"label":"a"},{"label":"a"}]}`, "unique"},
		{`{"version":1,"font":"font.ttf","text":"x","varaints":[]}`, "unknown field"},
		{`{"version":1,"font":"font.ttf","text":"x","variants":[]}`, "1 to 64"},
	} {
		if err := os.WriteFile(path, []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readProofCase(path); err == nil || !strings.Contains(err.Error(), c.contains) {
			t.Errorf("validation got %v want %s", err, c.contains)
		}
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"font":"font.ttf","text_file":"sample.txt","variants":[{"label":"a"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := readProofCase(path)
	if err != nil || *c.Text != "cafe\u0301\n" || c.TextFile != "" || c.Font != filepath.Join(dir, "font.ttf") {
		t.Fatalf("relative file input: %+v %v", c, err)
	}
}

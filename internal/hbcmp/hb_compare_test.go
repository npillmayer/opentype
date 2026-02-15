package hbcmp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestHarfBuzzParityFixtures(t *testing.T) {
	fixtures, err := loadFixtures("testdata")
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatalf("no hbcmp fixtures found in testdata")
	}
	for i, fx := range fixtures {
		name := fixtureName(i, fx)
		t.Run(name, func(t *testing.T) {
			got, err := shapeFixture(fx)
			if err != nil {
				t.Fatalf("shape fixture: %v", err)
			}
			if err := comparePositionedGlyphs(got, fx.Output); err != nil {
				t.Fatalf("%v\ngot=%#v\nwant=%#v", err, got, fx.Output)
			}
		})
	}
}

func fixtureName(i int, fx fixture) string {
	font := strings.TrimSuffix(filepath.Base(fx.Context.Font), filepath.Ext(fx.Context.Font))
	return fmt.Sprintf("%02d_%s_%s_%s", i, font,
		strings.ToLower(fx.Context.Script), strings.ToLower(fx.Context.Dir))
}

package otcomplex

import (
	"fmt"

	"github.com/npillmayer/opentype/harfbuzz"
	"github.com/npillmayer/opentype/harfbuzz/otarabic"
	"github.com/npillmayer/opentype/harfbuzz/othebrew"
)

// HebrewShaper aliases the dedicated Hebrew shaper implementation.
type HebrewShaper = othebrew.Shaper

// ArabicShaper aliases the dedicated Arabic shaper implementation.
type ArabicShaper = otarabic.Shaper

// NewHebrew returns the Hebrew shaping engine.
func NewHebrew() harfbuzz.ShapingEngine { return othebrew.New() }

// NewArabic returns the Arabic shaping engine.
func NewArabic() harfbuzz.ShapingEngine { return otarabic.New() }

// Register registers all complex shapers in the global registry.
func Register() error {
	if err := othebrew.Register(); err != nil {
		return fmt.Errorf("register otcomplex hebrew shaper: %w", err)
	}
	if err := otarabic.Register(); err != nil {
		return fmt.Errorf("register otcomplex arabic shaper: %w", err)
	}
	return nil
}

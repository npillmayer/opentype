package otcore

import (
	"errors"
	"fmt"

	"github.com/go-text/typesetting/harfbuzz"
)

// New returns the core/default shaping engine.
func New() harfbuzz.ShapingEngine {
	return harfbuzz.NewDefaultShapingEngine()
}

// Register registers the core/default shaping engine in the global registry.
func Register() error {
	if err := harfbuzz.RegisterShaper(New()); err != nil {
		if errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
			return nil
		}
		return fmt.Errorf("register otcore shaper: %w", err)
	}
	return nil
}

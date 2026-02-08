package othebrew

import (
	"errors"
	"fmt"

	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
	"github.com/npillmayer/opentype/harfbuzz"
)

type Shaper struct{}

var _ harfbuzz.ShapingEngine = Shaper{}
var _ harfbuzz.ShapingEnginePolicy = Shaper{}
var _ harfbuzz.ShapingEngineComposeHook = Shaper{}
var _ harfbuzz.ShapingEngineReorderHook = Shaper{}

func (Shaper) Name() string { return "hebrew" }

func (Shaper) Match(ctx harfbuzz.SelectionContext) int {
	if ctx.Script == language.Hebrew {
		return 100
	}
	return -1
}

func (Shaper) New() harfbuzz.ShapingEngine { return Shaper{} }

func (Shaper) MarksBehavior() (harfbuzz.ZeroWidthMarksMode, bool) {
	return harfbuzz.ZeroWidthMarksByGDEFLate, true
}

func (Shaper) NormalizationPreference() harfbuzz.NormalizationMode {
	return harfbuzz.NormalizationDefault
}

func (Shaper) GposTag() tables.Tag {
	return tables.Tag(hebrewGposTag())
}

func (Shaper) Compose(c harfbuzz.NormalizeContext, a, b rune) (rune, bool) {
	return hebrewCompose(c, a, b)
}

func (Shaper) ReorderMarks(buffer *harfbuzz.Buffer, start, end int) {
	hebrewReorderMarks(buffer, start, end)
}

// New returns the Hebrew shaping engine.
func New() harfbuzz.ShapingEngine { return Shaper{} }

// Register registers the Hebrew shaping engine in the global registry.
func Register() error {
	if err := harfbuzz.RegisterShaper(New()); err != nil {
		if errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
			return nil
		}
		return fmt.Errorf("register othebrew shaper: %w", err)
	}
	return nil
}

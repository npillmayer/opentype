package otcomplex

import (
	"testing"

	"github.com/go-text/typesetting/harfbuzz"
)

type runtimeBufferSurface interface {
	MergeClusters(start, end int)
	UnsafeToBreak(start, end int)
	UnsafeToConcat(start, end int)
	UnsafeToConcatFromOutbuffer(start, end int)
	SafeToInsertTatweel(start, end int)
	PreContext() []rune
	PostContext() []rune
}

type runtimeGlyphSurface interface {
	Codepoint() rune
	SetCodepoint(rune)
	ComplexAux() uint8
	SetComplexAux(uint8)
	ModifiedCombiningClass() uint8
	SetModifiedCombiningClass(uint8)
	GeneralCategory() uint8
	IsDefaultIgnorable() bool
	Multiplied() bool
	LigComp() uint8
}

var (
	_ runtimeBufferSurface = (*harfbuzz.Buffer)(nil)
	_ runtimeGlyphSurface  = (*harfbuzz.GlyphInfo)(nil)
)

func TestPhaseC_RuntimeSurfaceCompiles(t *testing.T) {}

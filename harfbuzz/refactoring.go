package harfbuzz

import (
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
)

// FeaturePlanner is the temporary plan-time view exposed to shaper-specific
// feature collection hooks. It intentionally hides most planner internals.
type FeaturePlanner interface {
	EnableFeature(tag tables.Tag)
	AddFeatureExt(tag tables.Tag, flags FeatureFlags, value uint32)
	EnableFeatureExt(tag tables.Tag, flags FeatureFlags, value uint32)
	AddGSUBPause(fn GSUBPauseFunc)
	HasFeature(tag tables.Tag) bool
}

// NormalizeContext is the temporary runtime view exposed to shaper-specific
// normalization hooks.
type NormalizeContext interface {
	DecomposeUnicode(ab rune) (a, b rune, ok bool)
	ComposeUnicode(a, b rune) (ab rune, ok bool)
	HasGposMark() bool
}

// SelectionContext contains the minimum planner state needed to select a
// shaping engine instance.
type SelectionContext struct {
	Script       language.Script
	Direction    Direction
	ChosenScript [2]tables.Tag
	FoundScript  [2]bool
}

// PlanContext is the narrow per-plan view exposed to shaping engines during
// plan initialization.
type PlanContext interface {
	Script() language.Script
	Direction() Direction
	FeatureMask1(tag tables.Tag) GlyphMask
	FeatureNeedsFallback(tag tables.Tag) bool
}

// PauseContext is the narrow runtime view exposed to GSUB pause hooks.
type PauseContext interface {
	Font() *Font
	Buffer() *Buffer
}

// GSUBPauseFunc can mutate the shaping buffer between lookup stages.
type GSUBPauseFunc func(ctx PauseContext) bool

// ShapingEngine is a temporary, all-encompassing interface used to drive the
// shaper-engine refactor. It mirrors the current hook surface and will be
// decomposed into smaller interfaces in later phases.
type ShapingEngine interface {
	// Selection hooks.
	Name() string
	// Match returns a non-negative score when the engine matches the context.
	// Negative scores mean "no match". Higher score wins.
	Match(ctx SelectionContext) int
	// New returns a fresh engine instance for the selected plan.
	New() ShapingEngine

	// Policy hooks.
	MarksBehavior() (zwm ZeroWidthMarksMode, fallbackPosition bool)
	NormalizationPreference() NormalizationMode
	GposTag() tables.Tag

	// Plan-time hooks.
	CollectFeatures(plan FeaturePlanner, script language.Script)
	OverrideFeatures(plan FeaturePlanner)
	InitPlan(plan PlanContext)

	// Runtime hooks.
	PreprocessText(buffer *Buffer, font *Font)
	Decompose(c NormalizeContext, ab rune) (a, b rune, ok bool)
	Compose(c NormalizeContext, a, b rune) (ab rune, ok bool)
	SetupMasks(buffer *Buffer, font *Font, script language.Script)
	ReorderMarks(buffer *Buffer, start, end int)
	PostprocessGlyphs(buffer *Buffer, font *Font)
}

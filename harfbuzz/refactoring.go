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

// ShapingEngine is the minimal selection/instantiation contract.
//
// Additional shaping behavior is provided through optional sub-interfaces
// (policy, feature, normalization, runtime hooks). This keeps the registry
// surface small while remaining backward-compatible with legacy engines that
// implement the larger hook set.
type ShapingEngine interface {
	// Selection hooks.
	Name() string
	// Match returns a non-negative score when the engine matches the context.
	// Negative scores mean "no match". Higher score wins.
	Match(ctx SelectionContext) int
	// New returns a fresh engine instance for the selected plan.
	New() ShapingEngine
}

// ShapingEnginePolicy exposes policy hooks for marks/normalization/GPOS script.
type ShapingEnginePolicy interface {
	MarksBehavior() (zwm ZeroWidthMarksMode, fallbackPosition bool)
	NormalizationPreference() NormalizationMode
	GposTag() tables.Tag
}

// ShapingEnginePlanHooks exposes plan-time feature and initialization hooks.
type ShapingEnginePlanHooks interface {
	CollectFeatures(plan FeaturePlanner, script language.Script)
	OverrideFeatures(plan FeaturePlanner)
	InitPlan(plan PlanContext)
}

// ShapingEnginePreprocessHook allows pre-GSUB text preprocessing.
type ShapingEnginePreprocessHook interface {
	PreprocessText(buffer *Buffer, font *Font)
}

// ShapingEngineDecomposeHook allows custom normalization decomposition.
type ShapingEngineDecomposeHook interface {
	Decompose(c NormalizeContext, ab rune) (a, b rune, ok bool)
}

// ShapingEngineComposeHook allows custom normalization composition.
type ShapingEngineComposeHook interface {
	Compose(c NormalizeContext, a, b rune) (ab rune, ok bool)
}

// ShapingEngineMaskHook allows script-specific mask setup.
type ShapingEngineMaskHook interface {
	SetupMasks(buffer *Buffer, font *Font, script language.Script)
}

// ShapingEngineReorderHook allows script-specific mark reordering.
type ShapingEngineReorderHook interface {
	ReorderMarks(buffer *Buffer, start, end int)
}

// ShapingEnginePostprocessHook allows post-position glyph processing.
type ShapingEnginePostprocessHook interface {
	PostprocessGlyphs(buffer *Buffer, font *Font)
}

func shaperMarksBehavior(engine ShapingEngine) (zwm ZeroWidthMarksMode, fallbackPosition bool) {
	if hooks, ok := engine.(ShapingEnginePolicy); ok {
		return hooks.MarksBehavior()
	}
	return zeroWidthMarksByGdefLate, true
}

func shaperNormalizationPreference(engine ShapingEngine) NormalizationMode {
	if hooks, ok := engine.(ShapingEnginePolicy); ok {
		return hooks.NormalizationPreference()
	}
	return nmDefault
}

func shaperGposTag(engine ShapingEngine) tables.Tag {
	if hooks, ok := engine.(ShapingEnginePolicy); ok {
		return hooks.GposTag()
	}
	return 0
}

func shaperCollectFeatures(engine ShapingEngine, plan FeaturePlanner, script language.Script) {
	if hooks, ok := engine.(ShapingEnginePlanHooks); ok {
		hooks.CollectFeatures(plan, script)
	}
}

func shaperOverrideFeatures(engine ShapingEngine, plan FeaturePlanner) {
	if hooks, ok := engine.(ShapingEnginePlanHooks); ok {
		hooks.OverrideFeatures(plan)
	}
}

func shaperInitPlan(engine ShapingEngine, plan PlanContext) {
	if hooks, ok := engine.(ShapingEnginePlanHooks); ok {
		hooks.InitPlan(plan)
	}
}

func shaperPreprocessText(engine ShapingEngine, buffer *Buffer, font *Font) {
	if hooks, ok := engine.(ShapingEnginePreprocessHook); ok {
		hooks.PreprocessText(buffer, font)
	}
}

func shaperDecompose(engine ShapingEngine, c NormalizeContext, ab rune) (a, b rune, ok bool) {
	if hooks, ok := engine.(ShapingEngineDecomposeHook); ok {
		return hooks.Decompose(c, ab)
	}
	return c.DecomposeUnicode(ab)
}

func shaperCompose(engine ShapingEngine, c NormalizeContext, a, b rune) (ab rune, ok bool) {
	if hooks, ok := engine.(ShapingEngineComposeHook); ok {
		return hooks.Compose(c, a, b)
	}
	return c.ComposeUnicode(a, b)
}

func shaperSetupMasks(engine ShapingEngine, buffer *Buffer, font *Font, script language.Script) {
	if hooks, ok := engine.(ShapingEngineMaskHook); ok {
		hooks.SetupMasks(buffer, font, script)
	}
}

func shaperReorderMarks(engine ShapingEngine, buffer *Buffer, start, end int) {
	if hooks, ok := engine.(ShapingEngineReorderHook); ok {
		hooks.ReorderMarks(buffer, start, end)
	}
}

func shaperPostprocessGlyphs(engine ShapingEngine, buffer *Buffer, font *Font) {
	if hooks, ok := engine.(ShapingEnginePostprocessHook); ok {
		hooks.PostprocessGlyphs(buffer, font)
	}
}

package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// SelectionContext carries the minimum segment metadata for shaper selection.
type SelectionContext struct {
	Direction bidi.Direction
	Script    language.Script // unicode.org/iso15924/iso15924-codes.html
	Language  language.Tag
	ScriptTag ot.Tag
	LangTag   ot.Tag
}

// LayoutTable identifies one OpenType layout table.
type LayoutTable uint8

const (
	LayoutGSUB LayoutTable = iota
	LayoutGPOS
)

// NormalizationMode controls Unicode normalization behavior before GSUB.
type NormalizationMode uint8

const (
	NormalizationAuto NormalizationMode = iota
	NormalizationNone
	NormalizationComposed
	NormalizationDecomposed
)

// FeatureFlags guide feature resolution and lookup behavior in plan compilation.
type FeatureFlags uint16

const FeatureNone FeatureFlags = 0

const (
	FeatureGlobal FeatureFlags = 1 << iota
	FeatureHasFallback
	FeatureManualZWNJ
	FeatureManualZWJ
	FeatureGlobalSearch
	FeatureRandom
	FeaturePerSyllable
)

// FeatureManualJoiners disables automatic skipping for both ZWJ and ZWNJ.
const FeatureManualJoiners = FeatureManualZWNJ | FeatureManualZWJ

// ResolvedFeature describes one feature selected during plan compilation.
type ResolvedFeature struct {
	Tag            ot.Tag
	Stage          int
	NeedsFallback  bool
	AutoZWNJ       bool
	AutoZWJ        bool
	PerSyllable    bool
	SupportsRandom bool
}

// FeaturePlanner is the plan-time interface for collecting feature intents.
type FeaturePlanner interface {
	EnableFeature(tag ot.Tag)
	AddFeature(tag ot.Tag, flags FeatureFlags, value uint32)
	DisableFeature(tag ot.Tag)
	AddGSUBPause(fn PauseHook)
	HasFeature(tag ot.Tag) bool
}

// ResolvedFeaturePlanner exposes post-resolution stage anchoring hooks.
type ResolvedFeaturePlanner interface {
	AddGSUBPauseBefore(tag ot.Tag, fn PauseHook) bool
	AddGSUBPauseAfter(tag ot.Tag, fn PauseHook) bool
}

// ResolvedFeatureView is a read-only snapshot of selected features.
type ResolvedFeatureView interface {
	SelectedFeatures(table LayoutTable) []ResolvedFeature
	HasSelectedFeature(table LayoutTable, tag ot.Tag) bool
}

// PlanContext is the narrow plan view available to shaper hooks.
type PlanContext interface {
	Font() *ot.Font
	Selection() SelectionContext
	FeatureMask1(tag ot.Tag) uint32
	FeatureNeedsFallback(tag ot.Tag) bool
}

// RunContext is the narrow runtime view available to shaper hooks.
type RunContext interface {
	Len() int
	Glyph(i int) ot.GlyphIndex
	SetGlyph(i int, gid ot.GlyphIndex)
	Codepoint(i int) rune
	SetCodepoint(i int, cp rune)
	Cluster(i int) uint32
	SetCluster(i int, cluster uint32)
	MergeClusters(start, end int)
	Pos(i int) otlayout.PosItem
	SetPos(i int, pos otlayout.PosItem)
	Mask(i int) uint32
	SetMask(i int, mask uint32)
	InsertGlyphs(index int, glyphs []ot.GlyphIndex)
	InsertGlyphCopies(index int, source int, count int)
	Swap(i, j int)
}

// NormalizeContext is the callback context for custom normalization hooks.
type NormalizeContext interface {
	Font() *ot.Font
	Selection() SelectionContext
	ComposeUnicode(a, b rune) (rune, bool)
	HasGposMark() bool
}

// PauseContext is the callback context for GSUB stage pauses.
type PauseContext interface {
	Font() *ot.Font
	Run() RunContext
}

// PauseHook may mutate run data between GSUB stages.
type PauseHook func(ctx PauseContext) error

type ShaperConfidence int

const (
	ShaperConfidenceNone ShaperConfidence = iota
	ShaperConfidenceLow
	ShaperConfidenceMedium
	ShaperConfidenceHigh
	ShaperConfidenceCertain
)

// ShapingEngine is the mandatory minimal interface for shaper selection.
type ShapingEngine interface {
	Name() string
	Match(ctx SelectionContext) ShaperConfidence
	New() ShapingEngine
}

// ShapingEnginePolicy exposes policy decisions used by the base pipeline.
type ShapingEnginePolicy interface {
	NormalizationPreference() NormalizationMode
	ApplyGPOS() bool
}

// ShapingEnginePlanHooks exposes plan-time hooks.
type ShapingEnginePlanHooks interface {
	CollectFeatures(plan FeaturePlanner, ctx SelectionContext)
	OverrideFeatures(plan FeaturePlanner)
	InitPlan(plan PlanContext)
}

// ShapingEnginePostResolveHook exposes post-resolution stage anchoring.
type ShapingEnginePostResolveHook interface {
	PostResolveFeatures(plan ResolvedFeaturePlanner, view ResolvedFeatureView, ctx SelectionContext)
}

// ShapingEnginePreprocessHook exposes a pre-normalization run hook.
type ShapingEnginePreprocessHook interface {
	PreprocessRun(run RunContext)
}

// ShapingEnginePreGSUBHook exposes a hook after normalization and before GSUB.
type ShapingEnginePreGSUBHook interface {
	PrepareGSUB(run RunContext)
}

// ShapingEngineComposeHook exposes custom pair-composition during normalization.
type ShapingEngineComposeHook interface {
	Compose(ctx NormalizeContext, a, b rune) (rune, bool)
}

// ShapingEngineReorderHook exposes mark-reordering before GSUB.
type ShapingEngineReorderHook interface {
	ReorderMarks(run RunContext, start, end int)
}

// ShapingEngineMaskHook exposes a hook to customize runtime mask values.
type ShapingEngineMaskHook interface {
	SetupMasks(run RunContext)
}

// ShapingEnginePostprocessHook exposes a hook after shaping stages complete.
type ShapingEnginePostprocessHook interface {
	PostprocessRun(run RunContext)
}

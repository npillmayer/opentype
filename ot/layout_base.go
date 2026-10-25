package ot

import "iter"

// --- BASE table ------------------------------------------------------------

// BaseTable, the Baseline table (BASE), provides information used to align glyphs
// of different scripts and sizes in a line of text, whether the glyphs are in the
// same font or in different fonts.
// BaseTable, the Glyph Definition (BSE) table, provides various glyph properties
// used in OpenType Layout processing.
//
// See also
// https://docs.microsoft.com/en-us/typography/opentype/spec/base
type BaseTable struct {
	tableBase
	versionHeader
	horizontal         *BaseAxis
	vertical           *BaseAxis
	itemVarStoreOffset uint32
	err                error
}

func newBaseTable(tag Tag, b binarySegm, offset, size uint32) *BaseTable {
	t := &BaseTable{}
	base := tableBase{
		data:   b,
		name:   tag,
		offset: offset,
		length: size,
	}
	t.tableBase = base
	t.self = t
	return t
}

// BaseAxis is a projected view over one BASE axis table (horizontal or vertical).
// It keeps raw table bytes and record-list projections without copying records.
type BaseAxis struct {
	raw          binarySegm
	baselineTags baseTagListView
	scripts      baseTagOffset16View
	err          error
}

// BaseScript is a projected view over one BaseScript table.
type BaseScript struct {
	raw              binarySegm
	baseValuesOffset uint16
	defaultMinMax    uint16
	langSysMinMax    baseTagOffset16View
	err              error
}

// BaseValues is a projected view over a BaseValues table.
type BaseValues struct {
	raw                  binarySegm
	defaultBaselineIndex uint16
	coords               offset16View
	err                  error
}

// MinMax is a projected view over a MinMax table.
type MinMax struct {
	raw             binarySegm
	minCoordOffset  uint16
	maxCoordOffset  uint16
	featureMinMaxes baseTagOffset16View
	err             error
}

// FeatureMinMax is a projected view over a FeatureTableMinMaxValue table.
type FeatureMinMax struct {
	raw            binarySegm
	minCoordOffset uint16
	maxCoordOffset uint16
	err            error
}

// BaseCoord is a projected view over a BaseCoord table.
type BaseCoord struct {
	raw    binarySegm
	format uint16
	err    error
}

// Version returns major and minor BASE table version numbers.
func (b *BaseTable) Version() (uint16, uint16) {
	if b == nil {
		return 0, 0
	}
	return b.Major, b.Minor
}

// Horizontal returns the horizontal BASE axis table, if present.
func (b *BaseTable) Horizontal() *BaseAxis {
	if b == nil {
		return nil
	}
	return b.horizontal
}

// Vertical returns the vertical BASE axis table, if present.
func (b *BaseTable) Vertical() *BaseAxis {
	if b == nil {
		return nil
	}
	return b.vertical
}

// ItemVarStoreOffset returns the optional offset to ItemVariationStore (BASE v1.1+).
func (b *BaseTable) ItemVarStoreOffset() uint32 {
	if b == nil {
		return 0
	}
	return b.itemVarStoreOffset
}

// Error returns parser/validation errors attached to this BASE table view.
func (b *BaseTable) Error() error {
	if b == nil {
		return nil
	}
	return b.err
}

// BaselineTags returns the baseline tags in declaration order.
func (a *BaseAxis) BaselineTags() []Tag {
	if a == nil {
		return nil
	}
	return a.baselineTags.All()
}

// ScriptCount returns the number of script records in this axis.
func (a *BaseAxis) ScriptCount() int {
	if a == nil {
		return 0
	}
	return a.scripts.count
}

// Script returns a BaseScript by tag.
func (a *BaseAxis) Script(tag Tag) (*BaseScript, bool) {
	if a == nil {
		return nil, false
	}
	off, ok := a.scripts.LookupOffset(tag)
	if !ok || off == 0 {
		return nil, false
	}
	return viewBaseScript(a.raw, off)
}

// ScriptAt returns script record i as (tag, script, ok).
func (a *BaseAxis) ScriptAt(i int) (Tag, *BaseScript, bool) {
	if a == nil {
		return 0, nil, false
	}
	tag, off, ok := a.scripts.Record(i)
	if !ok || off == 0 {
		return 0, nil, false
	}
	s, ok := viewBaseScript(a.raw, off)
	return tag, s, ok
}

// RangeScripts iterates all script records in declaration order.
func (a *BaseAxis) RangeScripts() iter.Seq2[Tag, *BaseScript] {
	return func(yield func(Tag, *BaseScript) bool) {
		if a == nil {
			return
		}
		for i := 0; i < a.scripts.count; i++ {
			tag, script, ok := a.ScriptAt(i)
			if !ok {
				continue
			}
			if !yield(tag, script) {
				return
			}
		}
	}
}

// Error returns parser/validation errors attached to this axis view.
func (a *BaseAxis) Error() error {
	if a == nil {
		return nil
	}
	return a.err
}

// BaseValues returns the script-default BaseValues, if present.
func (s *BaseScript) BaseValues() (*BaseValues, bool) {
	if s == nil || s.baseValuesOffset == 0 {
		return nil, false
	}
	if int(s.baseValuesOffset) >= len(s.raw) {
		return nil, false
	}
	return viewBaseValues(s.raw[s.baseValuesOffset:])
}

// DefaultMinMax returns the default MinMax for this script, if present.
func (s *BaseScript) DefaultMinMax() (*MinMax, bool) {
	if s == nil || s.defaultMinMax == 0 {
		return nil, false
	}
	if int(s.defaultMinMax) >= len(s.raw) {
		return nil, false
	}
	return viewMinMax(s.raw[s.defaultMinMax:])
}

// LangSysMinMax returns language-specific MinMax by language-system tag.
func (s *BaseScript) LangSysMinMax(tag Tag) (*MinMax, bool) {
	if s == nil {
		return nil, false
	}
	off, ok := s.langSysMinMax.LookupOffset(tag)
	if !ok || off == 0 || int(off) >= len(s.raw) {
		return nil, false
	}
	return viewMinMax(s.raw[off:])
}

// LangSysCount returns the number of language-system MinMax records.
func (s *BaseScript) LangSysCount() int {
	if s == nil {
		return 0
	}
	return s.langSysMinMax.count
}

// LangSysMinMaxAt returns record i as (langTag, minmax, ok).
func (s *BaseScript) LangSysMinMaxAt(i int) (Tag, *MinMax, bool) {
	if s == nil {
		return 0, nil, false
	}
	tag, off, ok := s.langSysMinMax.Record(i)
	if !ok || off == 0 || int(off) >= len(s.raw) {
		return 0, nil, false
	}
	mm, ok := viewMinMax(s.raw[off:])
	return tag, mm, ok
}

// RangeLangSysMinMax iterates language-system specific MinMax records.
func (s *BaseScript) RangeLangSysMinMax() iter.Seq2[Tag, *MinMax] {
	return func(yield func(Tag, *MinMax) bool) {
		if s == nil {
			return
		}
		for i := 0; i < s.langSysMinMax.count; i++ {
			tag, mm, ok := s.LangSysMinMaxAt(i)
			if !ok {
				continue
			}
			if !yield(tag, mm) {
				return
			}
		}
	}
}

// Error returns parser/validation errors attached to this script view.
func (s *BaseScript) Error() error {
	if s == nil {
		return nil
	}
	return s.err
}

// DefaultBaselineIndex returns the default baseline index.
func (v *BaseValues) DefaultBaselineIndex() uint16 {
	if v == nil {
		return 0
	}
	return v.defaultBaselineIndex
}

// Len returns the number of BaseCoord offsets in this BaseValues table.
func (v *BaseValues) Len() int {
	if v == nil {
		return 0
	}
	return v.coords.count
}

// CoordAt returns BaseCoord #i from this BaseValues table.
func (v *BaseValues) CoordAt(i int) (*BaseCoord, bool) {
	if v == nil {
		return nil, false
	}
	off, ok := v.coords.At(i)
	if !ok || off == 0 || int(off) >= len(v.raw) {
		return nil, false
	}
	return viewBaseCoord(v.raw[off:])
}

// Error returns parser/validation errors attached to this BaseValues view.
func (v *BaseValues) Error() error {
	if v == nil {
		return nil
	}
	return v.err
}

// Min returns the minimum coordinate, if present.
func (m *MinMax) Min() (*BaseCoord, bool) {
	if m == nil || m.minCoordOffset == 0 || int(m.minCoordOffset) >= len(m.raw) {
		return nil, false
	}
	return viewBaseCoord(m.raw[m.minCoordOffset:])
}

// Max returns the maximum coordinate, if present.
func (m *MinMax) Max() (*BaseCoord, bool) {
	if m == nil || m.maxCoordOffset == 0 || int(m.maxCoordOffset) >= len(m.raw) {
		return nil, false
	}
	return viewBaseCoord(m.raw[m.maxCoordOffset:])
}

// Feature returns a feature-specific min/max override by feature tag.
func (m *MinMax) Feature(tag Tag) (*FeatureMinMax, bool) {
	if m == nil {
		return nil, false
	}
	off, ok := m.featureMinMaxes.LookupOffset(tag)
	if !ok || off == 0 || int(off) >= len(m.raw) {
		return nil, false
	}
	return viewFeatureMinMax(m.raw[off:])
}

// FeatureCount returns the number of feature-specific min/max records.
func (m *MinMax) FeatureCount() int {
	if m == nil {
		return 0
	}
	return m.featureMinMaxes.count
}

// FeatureAt returns feature record i as (featureTag, minmax, ok).
func (m *MinMax) FeatureAt(i int) (Tag, *FeatureMinMax, bool) {
	if m == nil {
		return 0, nil, false
	}
	tag, off, ok := m.featureMinMaxes.Record(i)
	if !ok || off == 0 || int(off) >= len(m.raw) {
		return 0, nil, false
	}
	fmm, ok := viewFeatureMinMax(m.raw[off:])
	return tag, fmm, ok
}

// RangeFeatures iterates feature-specific MinMax records.
func (m *MinMax) RangeFeatures() iter.Seq2[Tag, *FeatureMinMax] {
	return func(yield func(Tag, *FeatureMinMax) bool) {
		if m == nil {
			return
		}
		for i := 0; i < m.featureMinMaxes.count; i++ {
			tag, fmm, ok := m.FeatureAt(i)
			if !ok {
				continue
			}
			if !yield(tag, fmm) {
				return
			}
		}
	}
}

// Error returns parser/validation errors attached to this MinMax view.
func (m *MinMax) Error() error {
	if m == nil {
		return nil
	}
	return m.err
}

// Min returns the minimum coordinate override, if present.
func (f *FeatureMinMax) Min() (*BaseCoord, bool) {
	if f == nil || f.minCoordOffset == 0 || int(f.minCoordOffset) >= len(f.raw) {
		return nil, false
	}
	return viewBaseCoord(f.raw[f.minCoordOffset:])
}

// Max returns the maximum coordinate override, if present.
func (f *FeatureMinMax) Max() (*BaseCoord, bool) {
	if f == nil || f.maxCoordOffset == 0 || int(f.maxCoordOffset) >= len(f.raw) {
		return nil, false
	}
	return viewBaseCoord(f.raw[f.maxCoordOffset:])
}

// Error returns parser/validation errors attached to this feature override.
func (f *FeatureMinMax) Error() error {
	if f == nil {
		return nil
	}
	return f.err
}

// Format returns BaseCoord format number (1, 2, or 3).
func (c *BaseCoord) Format() uint16 {
	if c == nil {
		return 0
	}
	return c.format
}

// Coordinate returns the design-units baseline coordinate.
func (c *BaseCoord) Coordinate() int16 {
	if c == nil || len(c.raw) < 4 {
		return 0
	}
	return int16(c.raw.U16(2))
}

// ReferenceGlyph returns the reference glyph for BaseCoord format 2.
func (c *BaseCoord) ReferenceGlyph() (GlyphIndex, bool) {
	if c == nil || c.format != 2 || len(c.raw) < 8 {
		return 0, false
	}
	return GlyphIndex(c.raw.U16(4)), true
}

// BaseCoordPoint returns the contour point index for BaseCoord format 2.
func (c *BaseCoord) BaseCoordPoint() (uint16, bool) {
	if c == nil || c.format != 2 || len(c.raw) < 8 {
		return 0, false
	}
	return c.raw.U16(6), true
}

// DeviceOrVarIdxOffset returns device/variation offset for BaseCoord format 3.
func (c *BaseCoord) DeviceOrVarIdxOffset() (uint16, bool) {
	if c == nil || c.format != 3 || len(c.raw) < 6 {
		return 0, false
	}
	return c.raw.U16(4), true
}

// Error returns parser/validation errors attached to this BaseCoord view.
func (c *BaseCoord) Error() error {
	if c == nil {
		return nil
	}
	return c.err
}

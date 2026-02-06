package ttxtest

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
)

// ParseTTXGPOS parses a GPOS-only TTX XML dump into an ExpectedGPOS model.
// Currently supports GPOS-1 format 1, GPOS-2 format 1, and GPOS-8 format 3.
func ParseTTXGPOS(path string) (*ExpectedGPOS, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var font ttxFontGPOS
	if err := xml.Unmarshal(data, &font); err != nil {
		return nil, err
	}
	if font.GPOS.LookupList == nil {
		return nil, fmt.Errorf("ttx: missing GPOS/LookupList")
	}

	exp := &ExpectedGPOS{}
	for _, lk := range font.GPOS.LookupList.Lookups {
		ltype, err := lk.LookupType.Int()
		if err != nil {
			return nil, fmt.Errorf("ttx: invalid LookupType: %w", err)
		}
		flag, err := lk.LookupFlag.Int()
		if err != nil {
			return nil, fmt.Errorf("ttx: invalid LookupFlag: %w", err)
		}
		el := ExpectedGPosLookup{
			Index: lk.Index,
			Type:  ltype,
			Flag:  uint16(flag),
		}

		for _, st := range lk.SinglePos {
			sub, err := normalizeSinglePos(ltype, st)
			if err != nil {
				return nil, err
			}
			el.Subtables = append(el.Subtables, sub)
		}
		for _, st := range lk.PairPos {
			sub, err := normalizePairPos(ltype, st)
			if err != nil {
				return nil, err
			}
			el.Subtables = append(el.Subtables, sub)
		}
		for _, st := range lk.MarkBasePos {
			sub, err := normalizeMarkBasePos(ltype, st)
			if err != nil {
				return nil, err
			}
			el.Subtables = append(el.Subtables, sub)
		}
		for _, st := range lk.MarkLigPos {
			sub, err := normalizeMarkLigPos(ltype, st)
			if err != nil {
				return nil, err
			}
			el.Subtables = append(el.Subtables, sub)
		}
		for _, st := range lk.ChainContextPos {
			sub, err := normalizeChainContextPos(ltype, st)
			if err != nil {
				return nil, err
			}
			el.Subtables = append(el.Subtables, sub)
		}
		exp.Lookups = append(exp.Lookups, el)
	}
	return exp, nil
}

func normalizeSinglePos(lookupType int, st ttxSinglePos) (ExpectedGPosSubtable, error) {
	if lookupType != 1 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with SinglePos", lookupType)
	}
	format, err := strconv.Atoi(st.FormatAttr)
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid SinglePos format %q", st.FormatAttr)
	}
	if format != 1 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported SinglePos format %d", format)
	}
	vf, err := st.ValueFormat.Int()
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid ValueFormat: %w", err)
	}
	val, err := st.Value.toExpected()
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid Value record: %w", err)
	}
	return ExpectedGPosSubtable{
		Type:        lookupType,
		Format:      format,
		Coverage:    st.Coverage.Glyphs(),
		ValueFormat: uint16(vf),
		Value:       val,
	}, nil
}

func normalizePairPos(lookupType int, st ttxPairPos) (ExpectedGPosSubtable, error) {
	if lookupType != 2 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with PairPos", lookupType)
	}
	format, err := strconv.Atoi(st.FormatAttr)
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid PairPos format %q", st.FormatAttr)
	}
	if format != 1 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported PairPos format %d", format)
	}
	vf1, err := st.ValueFormat1.Int()
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid ValueFormat1: %w", err)
	}
	vf2, err := st.ValueFormat2.Int()
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid ValueFormat2: %w", err)
	}
	coverage := st.Coverage.Glyphs()
	if len(coverage) != len(st.PairSet) {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: PairSet count %d != Coverage count %d", len(st.PairSet), len(coverage))
	}
	pairs := make(map[string][]ExpectedPairValueRecord, len(coverage))
	for i, ps := range st.PairSet {
		first := coverage[i]
		out := make([]ExpectedPairValueRecord, 0, len(ps.PairValueRecord))
		for _, pr := range ps.PairValueRecord {
			v1, err := pr.Value1.toExpected()
			if err != nil {
				return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid Value1: %w", err)
			}
			v2, err := pr.Value2.toExpected()
			if err != nil {
				return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid Value2: %w", err)
			}
			out = append(out, ExpectedPairValueRecord{
				SecondGlyph: pr.SecondGlyph.Value,
				Value1:      v1,
				Value2:      v2,
			})
		}
		pairs[first] = out
	}
	return ExpectedGPosSubtable{
		Type:         lookupType,
		Format:       format,
		Coverage:     coverage,
		ValueFormat1: uint16(vf1),
		ValueFormat2: uint16(vf2),
		PairValues:   pairs,
	}, nil
}

func normalizeChainContextPos(lookupType int, st ttxChainContextPos) (ExpectedGPosSubtable, error) {
	if lookupType != 8 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with ChainContextPos", lookupType)
	}
	format, err := strconv.Atoi(st.FormatAttr)
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid ChainContextPos format %q", st.FormatAttr)
	}
	if format != 3 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported ChainContextPos format %d", format)
	}
	bt := make([][]string, 0, len(st.BacktrackCoverage))
	for _, c := range st.BacktrackCoverage {
		bt = append(bt, c.Glyphs())
	}
	in := make([][]string, 0, len(st.InputCoverage))
	for _, c := range st.InputCoverage {
		in = append(in, c.Glyphs())
	}
	la := make([][]string, 0, len(st.LookAheadCoverage))
	for _, c := range st.LookAheadCoverage {
		la = append(la, c.Glyphs())
	}
	records := make([]ExpectedSequenceLookupRecord, 0, len(st.PosLookupRecord))
	for _, r := range st.PosLookupRecord {
		seq, err := r.SequenceIndex.Int()
		if err != nil {
			return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid SequenceIndex: %w", err)
		}
		lk, err := r.LookupListIndex.Int()
		if err != nil {
			return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid LookupListIndex: %w", err)
		}
		records = append(records, ExpectedSequenceLookupRecord{
			SequenceIndex:   seq,
			LookupListIndex: lk,
		})
	}
	return ExpectedGPosSubtable{
		Type:              lookupType,
		Format:            format,
		BacktrackCoverage: bt,
		InputCoverage:     in,
		LookAheadCoverage: la,
		PosLookupRecords:  records,
	}, nil
}

func normalizeMarkBasePos(lookupType int, st ttxMarkBasePos) (ExpectedGPosSubtable, error) {
	if lookupType != 4 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with MarkBasePos", lookupType)
	}
	format, err := strconv.Atoi(st.FormatAttr)
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid MarkBasePos format %q", st.FormatAttr)
	}
	if format != 1 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported MarkBasePos format %d", format)
	}
	classCount := 0
	if st.ClassCount.Value != "" {
		cc, err := st.ClassCount.Int()
		if err != nil {
			return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid ClassCount: %w", err)
		}
		classCount = cc
	} else {
		maxClass := -1
		for _, rec := range st.MarkArray.MarkRecord {
			class, err := rec.Class.Int()
			if err != nil {
				return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid MarkRecord class: %w", err)
			}
			if class > maxClass {
				maxClass = class
			}
		}
		for _, rec := range st.BaseArray.BaseRecord {
			for _, ba := range rec.BaseAnchor {
				if ba.Index > maxClass {
					maxClass = ba.Index
				}
			}
		}
		if maxClass >= 0 {
			classCount = maxClass + 1
		}
	}
	if classCount == 0 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: missing ClassCount")
	}
	marks := make([]ExpectedMarkRecord, 0, len(st.MarkArray.MarkRecord))
	for _, rec := range st.MarkArray.MarkRecord {
		class, err := rec.Class.Int()
		if err != nil {
			return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid MarkRecord class: %w", err)
		}
		anchor, err := rec.MarkAnchor.toExpected()
		if err != nil {
			return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid MarkAnchor: %w", err)
		}
		marks = append(marks, ExpectedMarkRecord{
			Class:  class,
			Anchor: anchor,
		})
	}
	baseAnchors := make([][]ExpectedAnchor, 0, len(st.BaseArray.BaseRecord))
	for _, rec := range st.BaseArray.BaseRecord {
		anchors := make([]ExpectedAnchor, classCount)
		for _, ba := range rec.BaseAnchor {
			if ba.Index < 0 || ba.Index >= classCount {
				return ExpectedGPosSubtable{}, fmt.Errorf("ttx: base anchor index %d out of range", ba.Index)
			}
			anchor, err := ba.toExpected()
			if err != nil {
				return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid BaseAnchor: %w", err)
			}
			anchors[ba.Index] = anchor
		}
		baseAnchors = append(baseAnchors, anchors)
	}
	return ExpectedGPosSubtable{
		Type:           lookupType,
		Format:         format,
		MarkCoverage:   st.MarkCoverage.Glyphs(),
		BaseCoverage:   st.BaseCoverage.Glyphs(),
		MarkClassCount: classCount,
		MarkAnchors:    marks,
		BaseAnchors:    baseAnchors,
	}, nil
}

func normalizeMarkLigPos(lookupType int, st ttxMarkLigPos) (ExpectedGPosSubtable, error) {
	if lookupType != 5 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with MarkLigPos", lookupType)
	}
	format, err := strconv.Atoi(st.FormatAttr)
	if err != nil {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid MarkLigPos format %q", st.FormatAttr)
	}
	if format != 1 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: unsupported MarkLigPos format %d", format)
	}
	classCount := 0
	if st.ClassCount.Value != "" {
		cc, err := st.ClassCount.Int()
		if err != nil {
			return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid ClassCount: %w", err)
		}
		classCount = cc
	} else {
		maxClass := -1
		for _, rec := range st.MarkArray.MarkRecord {
			class, err := rec.Class.Int()
			if err != nil {
				return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid MarkRecord class: %w", err)
			}
			if class > maxClass {
				maxClass = class
			}
		}
		for _, lig := range st.LigatureArray.LigatureAttach {
			for _, comp := range lig.ComponentRecord {
				for _, la := range comp.LigatureAnchor {
					if la.Index > maxClass {
						maxClass = la.Index
					}
				}
			}
		}
		if maxClass >= 0 {
			classCount = maxClass + 1
		}
	}
	if classCount == 0 {
		return ExpectedGPosSubtable{}, fmt.Errorf("ttx: missing ClassCount")
	}
	marks := make([]ExpectedMarkRecord, 0, len(st.MarkArray.MarkRecord))
	for _, rec := range st.MarkArray.MarkRecord {
		class, err := rec.Class.Int()
		if err != nil {
			return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid MarkRecord class: %w", err)
		}
		anchor, err := rec.MarkAnchor.toExpected()
		if err != nil {
			return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid MarkAnchor: %w", err)
		}
		marks = append(marks, ExpectedMarkRecord{
			Class:  class,
			Anchor: anchor,
		})
	}
	ligAnchors := make([][][]ExpectedAnchor, 0, len(st.LigatureArray.LigatureAttach))
	for _, lig := range st.LigatureArray.LigatureAttach {
		components := make([][]ExpectedAnchor, 0, len(lig.ComponentRecord))
		for _, comp := range lig.ComponentRecord {
			anchors := make([]ExpectedAnchor, classCount)
			for _, la := range comp.LigatureAnchor {
				if la.Index < 0 || la.Index >= classCount {
					return ExpectedGPosSubtable{}, fmt.Errorf("ttx: ligature anchor index %d out of range", la.Index)
				}
				anchor, err := la.toExpected()
				if err != nil {
					return ExpectedGPosSubtable{}, fmt.Errorf("ttx: invalid LigatureAnchor: %w", err)
				}
				anchors[la.Index] = anchor
			}
			components = append(components, anchors)
		}
		ligAnchors = append(ligAnchors, components)
	}
	return ExpectedGPosSubtable{
		Type:             lookupType,
		Format:           format,
		MarkCoverage:     st.MarkCoverage.Glyphs(),
		LigatureCoverage: st.LigatureCoverage.Glyphs(),
		MarkClassCount:   classCount,
		MarkAnchors:      marks,
		LigatureAnchors:  ligAnchors,
	}, nil
}

type ttxFontGPOS struct {
	GPOS ttxGPOS `xml:"GPOS"`
}

type ttxGPOS struct {
	LookupList *ttxLookupListGPOS `xml:"LookupList"`
}

type ttxLookupListGPOS struct {
	Lookups []ttxLookupGPOS `xml:"Lookup"`
}

type ttxLookupGPOS struct {
	Index           int                  `xml:"index,attr"`
	LookupType      ttxValue             `xml:"LookupType"`
	LookupFlag      ttxValue             `xml:"LookupFlag"`
	SinglePos       []ttxSinglePos       `xml:"SinglePos"`
	PairPos         []ttxPairPos         `xml:"PairPos"`
	ChainContextPos []ttxChainContextPos `xml:"ChainContextPos"`
	ExtensionPos    []ttxExtensionPos    `xml:"ExtensionPos"`
	ContextPos      []ttxContextPos      `xml:"ContextPos"`
	MarkBasePos     []ttxMarkBasePos     `xml:"MarkBasePos"`
	MarkLigPos      []ttxMarkLigPos      `xml:"MarkLigPos"`
	MarkMarkPos     []ttxMarkMarkPos     `xml:"MarkMarkPos"`
	CursivePos      []ttxCursivePos      `xml:"CursivePos"`
}

type ttxSinglePos struct {
	FormatAttr  string      `xml:"Format,attr"`
	Coverage    ttxCoverage `xml:"Coverage"`
	ValueFormat ttxValue    `xml:"ValueFormat"`
	Value       ttxValueRec `xml:"Value"`
}

type ttxPairPos struct {
	FormatAttr   string       `xml:"Format,attr"`
	Coverage     ttxCoverage  `xml:"Coverage"`
	ValueFormat1 ttxValue     `xml:"ValueFormat1"`
	ValueFormat2 ttxValue     `xml:"ValueFormat2"`
	PairSet      []ttxPairSet `xml:"PairSet"`
}

type ttxPairSet struct {
	PairValueRecord []ttxPairValueRecord `xml:"PairValueRecord"`
}

type ttxPairValueRecord struct {
	SecondGlyph ttxGlyph    `xml:"SecondGlyph"`
	Value1      ttxValueRec `xml:"Value1"`
	Value2      ttxValueRec `xml:"Value2"`
}

type ttxChainContextPos struct {
	FormatAttr        string               `xml:"Format,attr"`
	BacktrackCoverage []ttxCoverage        `xml:"BacktrackCoverage"`
	InputCoverage     []ttxCoverage        `xml:"InputCoverage"`
	LookAheadCoverage []ttxCoverage        `xml:"LookAheadCoverage"`
	PosLookupRecord   []ttxPosLookupRecord `xml:"PosLookupRecord"`
}

type ttxPosLookupRecord struct {
	SequenceIndex   ttxValue `xml:"SequenceIndex"`
	LookupListIndex ttxValue `xml:"LookupListIndex"`
}

type ttxMarkBasePos struct {
	FormatAttr   string       `xml:"Format,attr"`
	MarkCoverage ttxCoverage  `xml:"MarkCoverage"`
	BaseCoverage ttxCoverage  `xml:"BaseCoverage"`
	ClassCount   ttxValue     `xml:"ClassCount"`
	MarkArray    ttxMarkArray `xml:"MarkArray"`
	BaseArray    ttxBaseArray `xml:"BaseArray"`
}

type ttxMarkArray struct {
	MarkRecord []ttxMarkRecord `xml:"MarkRecord"`
}

type ttxMarkRecord struct {
	Class      ttxValue  `xml:"Class"`
	MarkAnchor ttxAnchor `xml:"MarkAnchor"`
}

type ttxBaseArray struct {
	BaseRecord []ttxBaseRecord `xml:"BaseRecord"`
}

type ttxBaseRecord struct {
	BaseAnchor []ttxBaseAnchor `xml:"BaseAnchor"`
}

type ttxMarkLigPos struct {
	FormatAttr       string           `xml:"Format,attr"`
	MarkCoverage     ttxCoverage      `xml:"MarkCoverage"`
	LigatureCoverage ttxCoverage      `xml:"LigatureCoverage"`
	ClassCount       ttxValue         `xml:"ClassCount"`
	MarkArray        ttxMarkArray     `xml:"MarkArray"`
	LigatureArray    ttxLigatureArray `xml:"LigatureArray"`
}

type ttxLigatureArray struct {
	LigatureAttach []ttxLigatureAttach `xml:"LigatureAttach"`
}

type ttxLigatureAttach struct {
	ComponentRecord []ttxComponentRecord `xml:"ComponentRecord"`
}

type ttxComponentRecord struct {
	LigatureAnchor []ttxBaseAnchor `xml:"LigatureAnchor"`
}

type ttxValueRec struct {
	XPlacement string `xml:"XPlacement,attr"`
	YPlacement string `xml:"YPlacement,attr"`
	XAdvance   string `xml:"XAdvance,attr"`
	YAdvance   string `xml:"YAdvance,attr"`
	XPlaDevice string `xml:"XPlaDevice,attr"`
	YPlaDevice string `xml:"YPlaDevice,attr"`
	XAdvDevice string `xml:"XAdvDevice,attr"`
	YAdvDevice string `xml:"YAdvDevice,attr"`
}

func (v ttxValueRec) toExpected() (ExpectedValueRecord, error) {
	var out ExpectedValueRecord
	var err error
	if v.XPlacement != "" {
		out.XPlacement, err = strconv.Atoi(v.XPlacement)
		if err != nil {
			return ExpectedValueRecord{}, err
		}
		out.HasXPlacement = true
	}
	if v.YPlacement != "" {
		out.YPlacement, err = strconv.Atoi(v.YPlacement)
		if err != nil {
			return ExpectedValueRecord{}, err
		}
		out.HasYPlacement = true
	}
	if v.XAdvance != "" {
		out.XAdvance, err = strconv.Atoi(v.XAdvance)
		if err != nil {
			return ExpectedValueRecord{}, err
		}
		out.HasXAdvance = true
	}
	if v.YAdvance != "" {
		out.YAdvance, err = strconv.Atoi(v.YAdvance)
		if err != nil {
			return ExpectedValueRecord{}, err
		}
		out.HasYAdvance = true
	}
	if v.XPlaDevice != "" {
		out.XPlaDevice, err = strconv.Atoi(v.XPlaDevice)
		if err != nil {
			return ExpectedValueRecord{}, err
		}
		out.HasXPlaDevice = true
	}
	if v.YPlaDevice != "" {
		out.YPlaDevice, err = strconv.Atoi(v.YPlaDevice)
		if err != nil {
			return ExpectedValueRecord{}, err
		}
		out.HasYPlaDevice = true
	}
	if v.XAdvDevice != "" {
		out.XAdvDevice, err = strconv.Atoi(v.XAdvDevice)
		if err != nil {
			return ExpectedValueRecord{}, err
		}
		out.HasXAdvDevice = true
	}
	if v.YAdvDevice != "" {
		out.YAdvDevice, err = strconv.Atoi(v.YAdvDevice)
		if err != nil {
			return ExpectedValueRecord{}, err
		}
		out.HasYAdvDevice = true
	}
	return out, nil
}

type ttxAnchor struct {
	FormatAttr    string   `xml:"Format,attr"`
	XCoordinate   ttxValue `xml:"XCoordinate"`
	YCoordinate   ttxValue `xml:"YCoordinate"`
	AnchorPoint   ttxValue `xml:"AnchorPoint"`
	XDeviceOffset ttxValue `xml:"XDeviceOffset"`
	YDeviceOffset ttxValue `xml:"YDeviceOffset"`
}

func (a ttxAnchor) toExpected() (ExpectedAnchor, error) {
	var out ExpectedAnchor
	if a.FormatAttr != "" {
		f, err := strconv.Atoi(a.FormatAttr)
		if err != nil {
			return ExpectedAnchor{}, err
		}
		out.Format = f
	}
	if a.XCoordinate.Value != "" {
		x, err := a.XCoordinate.Int()
		if err != nil {
			return ExpectedAnchor{}, err
		}
		out.XCoordinate = x
	}
	if a.YCoordinate.Value != "" {
		y, err := a.YCoordinate.Int()
		if err != nil {
			return ExpectedAnchor{}, err
		}
		out.YCoordinate = y
	}
	if a.AnchorPoint.Value != "" {
		p, err := a.AnchorPoint.Int()
		if err != nil {
			return ExpectedAnchor{}, err
		}
		out.AnchorPoint = p
		out.HasPoint = true
	}
	if a.XDeviceOffset.Value != "" {
		x, err := a.XDeviceOffset.Int()
		if err != nil {
			return ExpectedAnchor{}, err
		}
		out.XDevice = x
		out.HasXDevice = true
	}
	if a.YDeviceOffset.Value != "" {
		y, err := a.YDeviceOffset.Int()
		if err != nil {
			return ExpectedAnchor{}, err
		}
		out.YDevice = y
		out.HasYDevice = true
	}
	return out, nil
}

type ttxBaseAnchor struct {
	Index         int      `xml:"index,attr"`
	FormatAttr    string   `xml:"Format,attr"`
	XCoordinate   ttxValue `xml:"XCoordinate"`
	YCoordinate   ttxValue `xml:"YCoordinate"`
	AnchorPoint   ttxValue `xml:"AnchorPoint"`
	XDeviceOffset ttxValue `xml:"XDeviceOffset"`
	YDeviceOffset ttxValue `xml:"YDeviceOffset"`
}

func (a ttxBaseAnchor) toExpected() (ExpectedAnchor, error) {
	return ttxAnchor{
		FormatAttr:    a.FormatAttr,
		XCoordinate:   a.XCoordinate,
		YCoordinate:   a.YCoordinate,
		AnchorPoint:   a.AnchorPoint,
		XDeviceOffset: a.XDeviceOffset,
		YDeviceOffset: a.YDeviceOffset,
	}.toExpected()
}

// The following TTX structs are placeholders to keep XML decoding tolerant to
// unrelated tables in the same lookup. They are intentionally empty.
type ttxExtensionPos struct{}
type ttxContextPos struct{}
type ttxMarkMarkPos struct{}
type ttxCursivePos struct{}

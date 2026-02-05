package ttxtest

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ParseTTXGSUB parses a GSUB-only TTX XML dump into an ExpectedGSUB model.
// Currently supports GSUB-3 (Alternate Substitution) format 1.
func ParseTTXGSUB(path string) (*ExpectedGSUB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var font ttxFont
	if err := xml.Unmarshal(data, &font); err != nil {
		return nil, err
	}
	if font.GSUB.LookupList == nil {
		return nil, fmt.Errorf("ttx: missing GSUB/LookupList")
	}

	exp := &ExpectedGSUB{}
	for _, lk := range font.GSUB.LookupList.Lookups {
		lt, err := lk.LookupType.Int()
		if err != nil {
			return nil, fmt.Errorf("ttx: invalid LookupType: %w", err)
		}
		var lf uint16
		if lk.LookupFlag.Value != "" {
			n, err := lk.LookupFlag.Int()
			if err != nil {
				return nil, fmt.Errorf("ttx: invalid LookupFlag: %w", err)
			}
			lf = uint16(n)
		}
		expLookup := ExpectedLookup{
			Index: lk.Index,
			Type:  lt,
			Flag:  lf,
		}
		for _, st := range lk.SingleSubst {
			sub, err := normalizeSingleSubst(lt, st)
			if err != nil {
				return nil, err
			}
			expLookup.Subtables = append(expLookup.Subtables, sub)
		}
		for _, st := range lk.AlternateSubst {
			sub, err := normalizeAlternateSubst(lt, st)
			if err != nil {
				return nil, err
			}
			expLookup.Subtables = append(expLookup.Subtables, sub)
		}
		for _, st := range lk.LigatureSubst {
			sub, err := normalizeLigatureSubst(lt, st)
			if err != nil {
				return nil, err
			}
			expLookup.Subtables = append(expLookup.Subtables, sub)
		}
		for _, st := range lk.ContextSubst {
			sub, err := normalizeContextSubst(lt, st)
			if err != nil {
				return nil, err
			}
			expLookup.Subtables = append(expLookup.Subtables, sub)
		}
		exp.Lookups = append(exp.Lookups, expLookup)
	}
	return exp, nil
}

func normalizeSingleSubst(lookupType int, st ttxSingleSubst) (ExpectedSubtable, error) {
	if lookupType != 1 {
		return ExpectedSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with SingleSubst", lookupType)
	}
	subst := make(map[string]string)
	coverage := make([]string, 0, len(st.Substitutions))
	seen := make(map[string]bool)
	for _, s := range st.Substitutions {
		if s.In != "" && s.Out != "" {
			subst[s.In] = s.Out
			if !seen[s.In] {
				coverage = append(coverage, s.In)
				seen[s.In] = true
			}
		}
	}
	format := 2
	if len(subst) > 0 {
		deltaSet := false
		var delta int
		for _, in := range coverage {
			out, ok := subst[in]
			if !ok {
				deltaSet = false
				break
			}
			inID, err := glyphNameToIDTTX(in)
			if err != nil {
				deltaSet = false
				break
			}
			outID, err := glyphNameToIDTTX(out)
			if err != nil {
				deltaSet = false
				break
			}
			d := int(outID) - int(inID)
			if !deltaSet {
				delta = d
				deltaSet = true
				continue
			}
			if d != delta {
				deltaSet = false
				break
			}
		}
		if deltaSet {
			format = 1
		}
	}
	return ExpectedSubtable{
		Type:        1,
		Format:      format,
		Coverage:    coverage,
		SingleSubst: subst,
	}, nil
}

func normalizeAlternateSubst(lookupType int, st ttxAlternateSubst) (ExpectedSubtable, error) {
	if lookupType != 3 {
		return ExpectedSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with AlternateSubst", lookupType)
	}
	format := 1
	if st.FormatAttr != "" {
		n, err := strconv.Atoi(st.FormatAttr)
		if err != nil {
			return ExpectedSubtable{}, fmt.Errorf("ttx: invalid AlternateSubst format %q", st.FormatAttr)
		}
		format = n
	}
	if format != 1 {
		return ExpectedSubtable{}, fmt.Errorf("ttx: unsupported AlternateSubst format %d", format)
	}

	coverage := st.Coverage.Glyphs()
	if len(coverage) == 0 {
		coverage = st.AlternateSetGlyphs()
	}

	alts := make(map[string][]string)
	for _, set := range st.AlternateSet {
		g := strings.TrimSpace(set.Glyph)
		if g == "" {
			continue
		}
		var list []string
		for _, a := range set.Alternates {
			if a.Glyph != "" {
				list = append(list, a.Glyph)
			}
		}
		alts[g] = list
	}

	return ExpectedSubtable{
		Type:       3,
		Format:     1,
		Coverage:   coverage,
		Alternates: alts,
	}, nil
}

func normalizeLigatureSubst(lookupType int, st ttxLigatureSubst) (ExpectedSubtable, error) {
	if lookupType != 4 {
		return ExpectedSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with LigatureSubst", lookupType)
	}
	ligs := make(map[string][]ExpectedLigature)
	coverage := make([]string, 0, len(st.LigatureSet))
	for _, set := range st.LigatureSet {
		first := strings.TrimSpace(set.Glyph)
		if first == "" {
			continue
		}
		var list []ExpectedLigature
		for _, lig := range set.Ligatures {
			if lig.Glyph == "" {
				continue
			}
			components := splitGlyphList(lig.Components)
			list = append(list, ExpectedLigature{
				Components: components,
				Glyph:      lig.Glyph,
			})
		}
		ligs[first] = list
		coverage = append(coverage, first)
	}
	return ExpectedSubtable{
		Type:      4,
		Format:    1,
		Coverage:  coverage,
		Ligatures: ligs,
	}, nil
}

func normalizeContextSubst(lookupType int, st ttxContextSubst) (ExpectedSubtable, error) {
	if lookupType != 5 {
		return ExpectedSubtable{}, fmt.Errorf("ttx: unsupported lookup type %d with ContextSubst", lookupType)
	}
	format := 1
	if st.FormatAttr != "" {
		n, err := strconv.Atoi(st.FormatAttr)
		if err != nil {
			return ExpectedSubtable{}, fmt.Errorf("ttx: invalid ContextSubst format %q", st.FormatAttr)
		}
		format = n
	}
	coverage := st.Coverage.Glyphs()
	switch format {
	case 1:
		maxSet := -1
		for _, rs := range st.SubRuleSet {
			if rs.Index > maxSet {
				maxSet = rs.Index
			}
		}
		ruleSets := make([]ExpectedSubRuleSet, maxSet+1)
		for _, rs := range st.SubRuleSet {
			rules := make([]ExpectedSequenceRule, 0, len(rs.SubRule))
			for _, r := range rs.SubRule {
				rule := ExpectedSequenceRule{
					Input:         inputsByIndex(r.Input),
					LookupRecords: lookupRecordsByIndex(r.SubstLookupRecord),
				}
				rules = append(rules, rule)
			}
			if rs.Index >= 0 && rs.Index < len(ruleSets) {
				ruleSets[rs.Index] = ExpectedSubRuleSet{Rules: rules}
			}
		}

		return ExpectedSubtable{
			Type:     5,
			Format:   1,
			Coverage: coverage,
			ContextSubst: &ExpectedContextSubst{
				RuleSets: ruleSets,
			},
		}, nil
	case 2:
		maxSet := -1
		for _, rs := range st.SubClassSet {
			if rs.Index > maxSet {
				maxSet = rs.Index
			}
		}
		classRuleSets := make([]ExpectedClassRuleSet, maxSet+1)
		for _, rs := range st.SubClassSet {
			if rs.EmptyAttr == "1" {
				continue
			}
			rules := make([]ExpectedClassSequenceRule, 0, len(rs.SubClassRule))
			for _, r := range rs.SubClassRule {
				rule := ExpectedClassSequenceRule{
					Classes:       classValuesByIndex(r.Class),
					LookupRecords: lookupRecordsByIndex(r.SubstLookupRecord),
				}
				rules = append(rules, rule)
			}
			if rs.Index >= 0 && rs.Index < len(classRuleSets) {
				classRuleSets[rs.Index] = ExpectedClassRuleSet{Rules: rules}
			}
		}
		return ExpectedSubtable{
			Type:     5,
			Format:   2,
			Coverage: coverage,
			ContextSubst: &ExpectedContextSubst{
				ClassRuleSets: classRuleSets,
				ClassDefs:     classDefsByGlyph(st.ClassDef),
			},
		}, nil
	default:
		return ExpectedSubtable{}, fmt.Errorf("ttx: unsupported ContextSubst format %d", format)
	}
}

func splitGlyphList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func glyphNameToIDTTX(name string) (int, error) {
	if name == ".notdef" {
		return 0, nil
	}
	if strings.HasPrefix(name, "g") && len(name) > 1 {
		n, err := strconv.Atoi(name[1:])
		if err != nil {
			return 0, err
		}
		return n, nil
	}
	if strings.HasPrefix(name, "glyph") && len(name) > 5 {
		n, err := strconv.Atoi(name[5:])
		if err != nil {
			return 0, err
		}
		return n, nil
	}
	return 0, fmt.Errorf("unsupported glyph name %q (expected gNN)", name)
}

type ttxFont struct {
	GSUB ttxGSUB `xml:"GSUB"`
}

type ttxGSUB struct {
	LookupList *ttxLookupList `xml:"LookupList"`
}

type ttxLookupList struct {
	Lookups []ttxLookup `xml:"Lookup"`
}

type ttxLookup struct {
	Index          int                 `xml:"index,attr"`
	LookupType     ttxValue            `xml:"LookupType"`
	LookupFlag     ttxValue            `xml:"LookupFlag"`
	SingleSubst    []ttxSingleSubst    `xml:"SingleSubst"`
	AlternateSubst []ttxAlternateSubst `xml:"AlternateSubst"`
	LigatureSubst  []ttxLigatureSubst  `xml:"LigatureSubst"`
	ContextSubst   []ttxContextSubst   `xml:"ContextSubst"`
}

type ttxSingleSubst struct {
	Substitutions []ttxSingleSubstitution `xml:"Substitution"`
}

type ttxSingleSubstitution struct {
	In  string `xml:"in,attr"`
	Out string `xml:"out,attr"`
}

type ttxAlternateSubst struct {
	FormatAttr   string            `xml:"Format,attr"`
	Coverage     ttxCoverage       `xml:"Coverage"`
	AlternateSet []ttxAlternateSet `xml:"AlternateSet"`
}

func (st ttxAlternateSubst) AlternateSetGlyphs() []string {
	var out []string
	for _, set := range st.AlternateSet {
		if set.Glyph != "" {
			out = append(out, set.Glyph)
		}
	}
	return out
}

type ttxCoverage struct {
	GlyphList []ttxGlyph `xml:"Glyph"`
}

func (c ttxCoverage) Glyphs() []string {
	if len(c.GlyphList) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.GlyphList))
	for _, g := range c.GlyphList {
		if g.Value != "" {
			out = append(out, g.Value)
		}
	}
	return out
}

type ttxAlternateSet struct {
	Glyph      string         `xml:"glyph,attr"`
	Alternates []ttxAlternate `xml:"Alternate"`
}

type ttxAlternate struct {
	Glyph string `xml:"glyph,attr"`
}

type ttxLigatureSubst struct {
	LigatureSet []ttxLigatureSet `xml:"LigatureSet"`
}

type ttxLigatureSet struct {
	Glyph     string        `xml:"glyph,attr"`
	Ligatures []ttxLigature `xml:"Ligature"`
}

type ttxLigature struct {
	Components string `xml:"components,attr"`
	Glyph      string `xml:"glyph,attr"`
}

type ttxContextSubst struct {
	FormatAttr  string           `xml:"Format,attr"`
	Coverage    ttxCoverage      `xml:"Coverage"`
	ClassDef    ttxClassDef      `xml:"ClassDef"`
	SubRuleSet  []ttxSubRuleSet  `xml:"SubRuleSet"`
	SubClassSet []ttxSubClassSet `xml:"SubClassSet"`
}

type ttxSubRuleSet struct {
	Index   int          `xml:"index,attr"`
	SubRule []ttxSubRule `xml:"SubRule"`
}

type ttxSubRule struct {
	Index             int                    `xml:"index,attr"`
	Input             []ttxInput             `xml:"Input"`
	SubstLookupRecord []ttxSubstLookupRecord `xml:"SubstLookupRecord"`
}

type ttxSubClassSet struct {
	Index        int               `xml:"index,attr"`
	EmptyAttr    string            `xml:"empty,attr"`
	SubClassRule []ttxSubClassRule `xml:"SubClassRule"`
}

type ttxSubClassRule struct {
	Index             int                    `xml:"index,attr"`
	Class             []ttxClassValue        `xml:"Class"`
	SubstLookupRecord []ttxSubstLookupRecord `xml:"SubstLookupRecord"`
}

type ttxClassValue struct {
	Index int `xml:"index,attr"`
	Value int `xml:"value,attr"`
}

type ttxInput struct {
	Index int    `xml:"index,attr"`
	Value string `xml:"value,attr"`
}

type ttxSubstLookupRecord struct {
	Index           int      `xml:"index,attr"`
	SequenceIndex   ttxValue `xml:"SequenceIndex"`
	LookupListIndex ttxValue `xml:"LookupListIndex"`
}

type ttxGlyph struct {
	Value string `xml:"value,attr"`
}

type ttxValue struct {
	Value string `xml:"value,attr"`
}

func (v ttxValue) Int() (int, error) {
	if v.Value == "" {
		return 0, fmt.Errorf("missing value")
	}
	if strings.HasPrefix(v.Value, "0x") || strings.HasPrefix(v.Value, "0X") {
		n, err := strconv.ParseInt(v.Value[2:], 16, 32)
		return int(n), err
	}
	n, err := strconv.Atoi(v.Value)
	return n, err
}

type ttxClassDef struct {
	Entries []ttxClassDefEntry `xml:"ClassDef"`
}

type ttxClassDefEntry struct {
	Glyph string `xml:"glyph,attr"`
	Class int    `xml:"class,attr"`
}

func classDefsByGlyph(cd ttxClassDef) map[string]int {
	if len(cd.Entries) == 0 {
		return nil
	}
	out := make(map[string]int, len(cd.Entries))
	for _, e := range cd.Entries {
		if e.Glyph == "" {
			continue
		}
		out[e.Glyph] = e.Class
	}
	return out
}

func inputsByIndex(in []ttxInput) []string {
	if len(in) == 0 {
		return nil
	}
	max := -1
	for _, item := range in {
		if item.Index > max {
			max = item.Index
		}
	}
	out := make([]string, max+1)
	for _, item := range in {
		if item.Index >= 0 && item.Index < len(out) {
			out[item.Index] = item.Value
		}
	}
	return out
}

func classValuesByIndex(in []ttxClassValue) []int {
	if len(in) == 0 {
		return nil
	}
	max := -1
	for _, item := range in {
		if item.Index > max {
			max = item.Index
		}
	}
	out := make([]int, max+1)
	for _, item := range in {
		if item.Index >= 0 && item.Index < len(out) {
			out[item.Index] = item.Value
		}
	}
	return out
}

func lookupRecordsByIndex(in []ttxSubstLookupRecord) []ExpectedSequenceLookupRecord {
	if len(in) == 0 {
		return nil
	}
	max := -1
	for _, item := range in {
		if item.Index > max {
			max = item.Index
		}
	}
	out := make([]ExpectedSequenceLookupRecord, max+1)
	for _, item := range in {
		if item.Index < 0 || item.Index >= len(out) {
			continue
		}
		si, err := item.SequenceIndex.Int()
		if err != nil {
			continue
		}
		li, err := item.LookupListIndex.Int()
		if err != nil {
			continue
		}
		out[item.Index] = ExpectedSequenceLookupRecord{
			SequenceIndex:   si,
			LookupListIndex: li,
		}
	}
	return out
}

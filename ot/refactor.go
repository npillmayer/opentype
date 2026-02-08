package ot

import "iter"

// This file contains Phase-1 refactor types for shared GSUB/GPOS layout-graph
// structures. The types are intentionally semantic API containers, while record-
// level link representations remain internal parser details.

// ScriptList is a semantic container for scripts in a GSUB/GPOS ScriptList.
// It does not expose record-layout details from the OpenType byte format.
type ScriptList struct {
	scriptOrder []Tag
	offsetByTag map[Tag]uint16
	scriptByTag map[Tag]*Script

	raw binarySegm
	err error
}

// Script is a semantic container for one OpenType Script table.
type Script struct {
	defaultLangSysOffset uint16
	langOrder            []Tag
	langOffsetsByTag     map[Tag]uint16
	langByTag            map[Tag]*LangSys
	defaultLangSys       *LangSys

	raw binarySegm
	err error
}

// LangSys is a semantic list-like view for one OpenType LangSys table.
// It keeps linkage to FeatureList internal and exposes feature semantics.
type LangSys struct {
	lookupOrderOffset    uint16
	requiredFeatureIndex uint16 // 0xFFFF means no required feature

	// Internal linkage and lazy-resolved semantic list.
	featureIndices []uint16
	features       []*Feature

	err error
}

// FeatureList is a semantic container for features in a GSUB/GPOS FeatureList.
// Duplicate feature tags are preserved via indicesByTag.
type FeatureList struct {
	featureOrder    []Tag
	featuresByIndex []*Feature
	indicesByTag    map[Tag][]int

	raw binarySegm
	err error
}

// Feature is a semantic view of one OpenType Feature table.
type Feature struct {
	featureParamsOffset uint16
	lookupListIndices   []uint16

	raw binarySegm
	err error
}

// Len returns the number of scripts in the list.
func (sl *ScriptList) Len() int {
	if sl == nil {
		return 0
	}
	return len(sl.scriptOrder)
}

// Script returns a script by tag.
func (sl *ScriptList) Script(tag Tag) *Script {
	if sl == nil || sl.scriptByTag == nil {
		return nil
	}
	return sl.scriptByTag[tag]
}

// Range iterates scripts in declaration order.
func (sl *ScriptList) Range() iter.Seq2[Tag, *Script] {
	return func(yield func(Tag, *Script) bool) {
		if sl == nil {
			return
		}
		for _, tag := range sl.scriptOrder {
			if !yield(tag, sl.scriptByTag[tag]) {
				return
			}
		}
	}
}

// Error returns an accumulated error for the list.
func (sl *ScriptList) Error() error {
	if sl == nil {
		return nil
	}
	return sl.err
}

// LangSys returns a language system by tag.
func (s *Script) LangSys(tag Tag) *LangSys {
	if s == nil {
		return nil
	}
	if tag == DFLT {
		return s.defaultLangSys
	}
	if s.langByTag == nil {
		return nil
	}
	return s.langByTag[tag]
}

// Range iterates language-systems in declaration order.
func (s *Script) Range() iter.Seq2[Tag, *LangSys] {
	return func(yield func(Tag, *LangSys) bool) {
		if s == nil {
			return
		}
		for _, tag := range s.langOrder {
			if !yield(tag, s.langByTag[tag]) {
				return
			}
		}
	}
}

// Error returns an accumulated error for the script.
func (s *Script) Error() error {
	if s == nil {
		return nil
	}
	return s.err
}

// RequiredFeatureIndex returns the required-feature index and whether it is set.
func (ls *LangSys) RequiredFeatureIndex() (uint16, bool) {
	if ls == nil || ls.requiredFeatureIndex == 0xffff {
		return 0, false
	}
	return ls.requiredFeatureIndex, true
}

// FeatureAt returns a resolved feature by feature-link position.
func (ls *LangSys) FeatureAt(i int) *Feature {
	if ls == nil || i < 0 || i >= len(ls.features) {
		return nil
	}
	return ls.features[i]
}

// Features returns resolved features in language-system link order.
func (ls *LangSys) Features() []*Feature {
	if ls == nil || len(ls.features) == 0 {
		return nil
	}
	features := make([]*Feature, len(ls.features))
	copy(features, ls.features)
	return features
}

// Error returns an accumulated error for the language system.
func (ls *LangSys) Error() error {
	if ls == nil {
		return nil
	}
	return ls.err
}

// Len returns the number of features in the feature list.
func (fl *FeatureList) Len() int {
	if fl == nil {
		return 0
	}
	return len(fl.featuresByIndex)
}

// Range iterates features in declaration order and preserves duplicate tags.
func (fl *FeatureList) Range() iter.Seq2[Tag, *Feature] {
	return func(yield func(Tag, *Feature) bool) {
		if fl == nil {
			return
		}
		for i, tag := range fl.featureOrder {
			var feature *Feature
			if i >= 0 && i < len(fl.featuresByIndex) {
				feature = fl.featuresByIndex[i]
			}
			if !yield(tag, feature) {
				return
			}
		}
	}
}

// Indices returns all indices matching a feature tag.
func (fl *FeatureList) Indices(tag Tag) []int {
	if fl == nil || fl.indicesByTag == nil {
		return nil
	}
	indices := fl.indicesByTag[tag]
	if len(indices) == 0 {
		return nil
	}
	out := make([]int, len(indices))
	copy(out, indices)
	return out
}

// First returns the first feature matching a feature tag.
func (fl *FeatureList) First(tag Tag) *Feature {
	if fl == nil {
		return nil
	}
	indices := fl.indicesByTag[tag]
	if len(indices) == 0 {
		return nil
	}
	i := indices[0]
	if i < 0 || i >= len(fl.featuresByIndex) {
		return nil
	}
	return fl.featuresByIndex[i]
}

// All returns all features matching a feature tag.
func (fl *FeatureList) All(tag Tag) []*Feature {
	if fl == nil {
		return nil
	}
	indices := fl.indicesByTag[tag]
	if len(indices) == 0 {
		return nil
	}
	out := make([]*Feature, 0, len(indices))
	for _, i := range indices {
		if i < 0 || i >= len(fl.featuresByIndex) {
			continue
		}
		if f := fl.featuresByIndex[i]; f != nil {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Error returns an accumulated error for the feature list.
func (fl *FeatureList) Error() error {
	if fl == nil {
		return nil
	}
	return fl.err
}

// LookupCount returns the number of linked lookups.
func (f *Feature) LookupCount() int {
	if f == nil {
		return 0
	}
	return len(f.lookupListIndices)
}

// Error returns an accumulated error for the feature.
func (f *Feature) Error() error {
	if f == nil {
		return nil
	}
	return f.err
}

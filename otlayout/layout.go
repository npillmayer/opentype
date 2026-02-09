package otlayout

import (
	"errors"
	"fmt"

	"github.com/npillmayer/opentype/ot"
)

var (
	ErrVoid             = errors.New("void")
	ErrNoScriptGraph    = errors.New("no concrete script graph")
	ErrNoFeatureGraph   = errors.New("no concrete feature graph")
	ErrNoLookupGraph    = errors.New("no concrete lookup graph")
	ErrFeatureHasNoRefs = errors.New("feature has no lookup references")
)

// GetLayoutTable returns the layout table component for a given OpenType GSUB or GPOS table.
func GetLayoutTable(table ot.Table) (*ot.LayoutTable, error) {
	return peekLayoutProperty(table,
		func(gsub *ot.GSubTable) (*ot.LayoutTable, error) {
			return &gsub.LayoutTable, nil
		}, func(gpos *ot.GPosTable) (*ot.LayoutTable, error) {
			return &gpos.LayoutTable, nil
		})
}

// GetScriptGraph returns the concrete ScriptList graph for a GSUB/GPOS table.
func GetScriptGraph(table ot.Table) (*ot.ScriptList, error) {
	lyt, err := GetLayoutTable(table)
	if err != nil {
		return nil, err
	}
	if lyt == nil || lyt.ScriptGraph() == nil {
		return nil, ErrNoScriptGraph
	}
	return lyt.ScriptGraph(), nil
}

// GetFeatureGraph returns the concrete FeatureList graph for a GSUB/GPOS table.
func GetFeatureGraph(table ot.Table) (*ot.FeatureList, error) {
	lyt, err := GetLayoutTable(table)
	if err != nil {
		return nil, err
	}
	if lyt == nil || lyt.FeatureGraph() == nil {
		return nil, ErrNoFeatureGraph
	}
	return lyt.FeatureGraph(), nil
}

// GetLookupGraph returns the concrete LookupList graph for a GSUB/GPOS table.
func GetLookupGraph(table ot.Table) (*ot.LookupListGraph, error) {
	lyt, err := GetLayoutTable(table)
	if err != nil {
		return nil, err
	}
	if lyt == nil || lyt.LookupGraph() == nil {
		return nil, ErrNoLookupGraph
	}
	return lyt.LookupGraph(), nil
}

// ScriptTags returns script tags in declaration order.
func ScriptTags(scriptGraph *ot.ScriptList) []ot.Tag {
	if scriptGraph == nil || scriptGraph.Len() == 0 {
		return nil
	}
	tags := make([]ot.Tag, 0, scriptGraph.Len())
	for tag := range scriptGraph.Range() {
		tags = append(tags, tag)
	}
	return tags
}

// FeatureTags returns feature tags in declaration order (including duplicates).
func FeatureTags(featureGraph *ot.FeatureList) []ot.Tag {
	if featureGraph == nil || featureGraph.Len() == 0 {
		return nil
	}
	tags := make([]ot.Tag, 0, featureGraph.Len())
	for tag := range featureGraph.Range() {
		tags = append(tags, tag)
	}
	return tags
}

// FeaturesForLangSys resolves all feature links for a language system.
func FeaturesForLangSys(langSys *ot.LangSys) ([]*ot.Feature, error) {
	if langSys == nil {
		return nil, ErrVoid
	}
	features := langSys.Features()
	if len(features) == 0 {
		return nil, ErrVoid
	}
	return features, nil
}

// LookupsForFeature resolves lookup references for a feature against a lookup graph.
func LookupsForFeature(feature *ot.Feature, lookupGraph *ot.LookupListGraph) ([]*ot.LookupTable, error) {
	if feature == nil {
		return nil, ErrVoid
	}
	if lookupGraph == nil || lookupGraph.Len() == 0 {
		return nil, ErrNoLookupGraph
	}
	if feature.LookupCount() == 0 {
		return nil, ErrFeatureHasNoRefs
	}
	out := make([]*ot.LookupTable, 0, feature.LookupCount())
	for i := 0; i < feature.LookupCount(); i++ {
		inx := feature.LookupIndex(i)
		if inx < 0 {
			continue
		}
		if lookup := lookupGraph.Lookup(inx); lookup != nil {
			out = append(out, lookup)
		}
	}
	if len(out) == 0 {
		return nil, ErrFeatureHasNoRefs
	}
	return out, nil
}

// Reach inside a GSUB or GPOS table and extract a property safely.
func peekLayoutProperty[T any](table ot.Table,
	sub func(*ot.GSubTable) (*T, error), pos func(*ot.GPosTable) (*T, error)) (*T, error) {
	switch table.Self().NameTag() {
	case ot.T("GSUB"):
		if gsub := table.Self().AsGSub(); gsub != nil {
			return sub(gsub)
		}
		return nil, errors.New("invalid GSUB table")
	case ot.T("GPOS"):
		if gpos := table.Self().AsGPos(); gpos != nil {
			return pos(gpos)
		}
		return nil, errors.New("invalid GPOS table")
	}
	return nil, errors.New("not a layout table: " + table.Self().NameTag().String())
}

// get GSUB and GPOS from a font safely
func getLayoutTables(otf *ot.Font) ([]*ot.LayoutTable, error) {
	var table ot.Table
	var lytt = make([]*ot.LayoutTable, 2)
	if table = otf.Table(ot.T("GSUB")); table == nil {
		return nil, errFontFormat(fmt.Sprintf("font %s has no GSUB table", otf.F.Fontname))
	}
	lytt[0] = &table.Self().AsGSub().LayoutTable
	if table = otf.Table(ot.T("GPOS")); table == nil {
		return nil, errFontFormat(fmt.Sprintf("font %s has no GPOS table", otf.F.Fontname))
	}
	lytt[1] = &table.Self().AsGPos().LayoutTable
	return lytt, nil
}

package otlayout

import (
	"errors"
	"fmt"

	"github.com/npillmayer/opentype/ot"
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

// GetFeatureList returns the featuress for a given OpenType GSUB or GPOS table.
func GetFeatureList(table ot.Table) (ot.TagRecordMap, error) {
	f, err := peekLayoutProperty(table,
		func(gsub *ot.GSubTable) (*ot.TagRecordMap, error) {
			return &gsub.FeatureList, nil
		}, func(gpos *ot.GPosTable) (*ot.TagRecordMap, error) {
			return &gpos.FeatureList, nil
		})
	return *f, err
}

// GetScriptList returns the scripts for a given OpenType GSUB or GPOS table.
func GetScriptList(table ot.Table) (ot.Navigator, error) {
	scr, err := peekLayoutProperty(table,
		func(gsub *ot.GSubTable) (*ot.Navigator, error) {
			return &gsub.ScriptList, nil
		}, func(gpos *ot.GPosTable) (*ot.Navigator, error) {
			return &gpos.ScriptList, nil
		})
	return *scr, err
}

// Reach inside a GSUB or GPOS table and extract a property safely.
func peekLayoutProperty[T any](table ot.Table,
	sub func(*ot.GSubTable) (*T, error), pos func(*ot.GPosTable) (*T, error)) (*T, error) {
	//
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

var ERROR_VOID error = errors.New("void")
var ERROR_NO_TAG_RECORD_MAP error = errors.New("not a tag record map")
var ERROR_NO_LIST error = errors.New("not a list")

func NavAsTagRecordMap(nav ot.Navigator) (ot.TagRecordMap, error) {
	if nav == nil || nav.IsVoid() {
		return nil, ERROR_VOID
	} else if nav.Map().IsTagRecordMap() {
		m := nav.Map().AsTagRecordMap()
		if m.Len() > 0 {
			return nav.Map().AsTagRecordMap(), nil
		}
	}
	return nil, ERROR_NO_TAG_RECORD_MAP
}

func NavAsList(nav ot.Navigator) (ot.NavList, error) {
	if nav == nil || nav.IsVoid() {
		return nil, ERROR_VOID
	} else if nav.List().Len() > 0 {
		return nav.List(), nil
	}
	return nil, ERROR_NO_LIST
}

func FeatureSubsetForLangSys(langSys ot.NavList, featureList ot.TagRecordMap) (ot.TagRecordMap, error) {
	if langSys == nil || langSys.Len() == 0 {
		return nil, ERROR_VOID
	} else if featureList == nil || featureList.Len() == 0 {
		return nil, ERROR_NO_TAG_RECORD_MAP
	}
	root, ok := featureList.(ot.RootTagMap)
	if !ok {
		return nil, ERROR_NO_TAG_RECORD_MAP
	}
	indices := make([]int, 0, langSys.Len())
	for _, loc := range langSys.Range() {
		indices = append(indices, int(loc.U16(0)))
	}
	subset := root.Subset(indices)
	return subset, nil
}

func LookupSubsetForFeature(featureLookups ot.NavList, lookupList ot.NavList) (ot.RootList, error) {
	if featureLookups == nil || featureLookups.Len() == 0 {
		return nil, ERROR_VOID
	} else if lookupList == nil || lookupList.Len() == 0 {
		return nil, ERROR_NO_LIST
	}
	root, ok := lookupList.(ot.RootList)
	if !ok {
		return nil, ERROR_NO_LIST
	}
	indices := make([]int, 0, featureLookups.Len())
	for _, loc := range featureLookups.Range() {
		indices = append(indices, int(loc.U16(0)))
	}
	subset := root.Subset(indices)
	return subset, nil
}

func KeyTags(m ot.TagRecordMap) []ot.Tag {
	if m == nil || m.Len() == 0 {
		return nil
	}
	keyTags := make([]ot.Tag, 0, m.Len())
	for tag, _ := range m.Range() {
		keyTags = append(keyTags, tag)
	}
	return keyTags
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

package otlayout

import (
	"errors"

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
		return nav.Map().AsTagRecordMap(), nil
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

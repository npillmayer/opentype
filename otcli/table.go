package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/pterm/pterm"
)

func tableOp(intp *Intp, op *Op) (error, bool) {
	tag := op.arg
	if intp.table = intp.font.Table(ot.T(tag)); intp.table == nil {
		return errors.New("table not found in font"), false
	}
	intp.stack = intp.stack[:0]
	tracer().Infof("setting table: %v", tag)
	return nil, false
}

func listOp(intp *Intp, op *Op) (err error, stop bool) {
	var n pathNode
	if n, err = intp.checkNode(); err != nil {
		return
	}
	var l ot.NavList
	switch n.kind {
	case nodeNav:
		l = n.nav.List()
	case nodeList:
		l = n.list
	default:
		err = errors.New("list is empty / not a list")
		return
	}
	var inx int
	if l.Len() == 0 {
		err = errors.New("list is empty / not a list")
	} else if op.noArg() {
		pterm.Printf("List has %d entries\n", l.Len())
	} else if inx, err = strconv.Atoi(op.arg); err != nil {
		err = fmt.Errorf("List index not numeric: %v\n", op.arg)
	} else if inx >= l.Len() {
		err = fmt.Errorf("List index out of range: %d", inx)
	} else {
		loc := l.Get(inx)
		size, value := loc.Size(), decodeLocation(loc, l.Name())
		switch value.(type) {
		case int:
			pterm.Printf("%s list index %d holds number = %d\n", l.Name(), inx, value)
		default:
			pterm.Printf("%s list index %d holds data of %d bytes\n", l.Name(), inx, size)
		}
	}
	return
}

func mapOp(intp *Intp, op *Op) (err error, stop bool) {
	var n pathNode
	if n, err = intp.checkNode(); err != nil {
		return
	}
	var target ot.NavLink
	var m ot.NavMap
	var trm ot.TagRecordMap
	switch n.kind {
	case nodeNav:
		m = n.nav.Map()
		if m.IsTagRecordMap() {
			trm = m.AsTagRecordMap()
		}
	case nodeMap:
		m = n.m
		if m.IsTagRecordMap() {
			trm = m.AsTagRecordMap()
		}
	case nodeTagMap:
		trm = n.tm
	default:
		err = errors.New("not a map")
		return
	}
	if m == nil && trm == nil {
		err = errors.New("not a map")
		return
	}
	if tag, ok := op.hasArg(); ok {
		if trm == nil {
			err = errors.New("map is not a tag record map")
			return
		}
		target = trm.LookupTag(ot.T(tag))
		tracer().Infof("%s map keys = %v", trm.Name(), otlayout.KeyTags(trm))
		pterm.Printf("%s table maps [tag %v] => %v\n", trm.Name(), ot.T(tag), target.Name())
		n := intp.lastPathNode()
		n.key, n.link = tag, target
		intp.setLastPathNode(n)
	} else if trm != nil {
		pterm.Printf("%s map keys = %v\n", trm.Name(), otlayout.KeyTags(trm))
	}
	return
}

func scriptsOp(intp *Intp, op *Op) (err error, stop bool) {
	var nav ot.Navigator
	var scr ot.TagRecordMap
	if table := intp.clearPath(); table == nil {
		return errors.New("no table set"), false
	} else if nav, err = otlayout.GetScriptList(intp.table); err != nil {
		return
	} else if scr = nav.Map().AsTagRecordMap(); scr == nil {
		return errors.New("table has no script list"), false
	}
	pterm.Printf("ScriptList keys: %v\n", otlayout.KeyTags(scr))
	n := pathNode{kind: nodeNav, nav: nav, inx: -1}
	var ok bool
	if n.key, ok = op.hasArg(); ok {
		l := scr.LookupTag(ot.T(op.arg))
		if l.IsNull() {
			err = fmt.Errorf("script lookup [%s] returns null", ot.T(op.arg).String())
			return
		}
		n.link = l
	}
	intp.stack = append(intp.stack, n)
	return
}

func subsetOp(intp *Intp, op *Op) (err error, stop bool) {
	var n pathNode
	if n, err = intp.checkNode(); err != nil {
		return
	}
	var l ot.NavList
	switch n.kind {
	case nodeNav:
		l = n.nav.List()
	case nodeList:
		l = n.list
	default:
		err = errors.New("subset needs list to subset anything to")
		return
	}
	if l.Len() == 0 {
		err = errors.New("subset needs list to subset anything to")
		return
	}
	if tableName, ok := op.hasArg(); ok {
		switch tableName {
		case "FeatureList":
			var features ot.TagRecordMap
			if features, err = otlayout.GetFeatureList(intp.table); err != nil {
				return
			}
			root, ok := features.(ot.RootTagMap)
			if !ok {
				err = errors.New("feature list is not a root tag map")
				return
			}
			indices := make([]int, 0, l.Len())
			for _, loc := range l.Range() {
				indices = append(indices, int(loc.U16(0)))
			}
			subset := root.Subset(indices)
			pterm.Printf("Subset of %d features\n", subset.Len())
			n := pathNode{kind: nodeTagMap, tm: subset, inx: -1}
			intp.stack = append(intp.stack, n)
		case "LookupList":
			panic("not implemented")
		}
	} else {
		err = errors.New("need to know which sub-table to subset")
	}
	return
}

func featureListOp(intp *Intp, op *Op) (err error, stop bool) {
	if err = intp.checkTable(); err != nil {
		return
	}
	var features ot.TagRecordMap
	if features, err = otlayout.GetFeatureList(intp.table); err != nil {
		return
	}
	n := intp.lastPathNode()
	if n.kind != nodeTagMap || n.tm == nil || n.tm.Name() != features.Name() {
		nn := pathNode{kind: nodeTagMap, tm: features, inx: -1}
		intp.stack = append(intp.stack, nn)
		n = intp.lastPathNode()
	}
	var i int
	if op.noArg() {
		pterm.Printf("%s table has %d entries\n", features.Name(), features.Len())
	} else if i, err = strconv.Atoi(op.arg); err == nil {
		tag, _ := features.Get(i) //tag, lnk := f.Get(i)
		pterm.Printf("%s list index %d holds feature record = %v\n", features.Name(), i, tag)
	} else { // treat FeatureList as a tag-map with arg = key
		err = nil
		l := features.LookupTag(ot.T(op.arg))
		if l.IsNull() {
			err = fmt.Errorf("FeatureList lookup [%s] returns null", ot.T(op.arg).String())
			return
		}
		pterm.Printf("FeatureList[%s] = %v\n", op.arg, l.Name())
		n.key = op.arg
		n.link = l
		intp.setLastPathNode(n)
	}
	return
}

func lookupListOp(intp *Intp, op *Op) (err error, stop bool) {
	if err = intp.checkTable(); err != nil {
		return
	}
	var lyt *ot.LayoutTable
	if lyt, err = otlayout.GetLayoutTable(intp.table); err != nil {
		return
	}
	n := intp.lastPathNode()
	if n.kind != nodeList || n.list == nil || n.list.Name() != lyt.LookupList.Name() {
		nn := pathNode{kind: nodeList, list: lyt.LookupList, inx: -1}
		intp.stack = append(intp.stack, nn)
		n = intp.lastPathNode()
	}
	if op.noArg() {
		printLookupList(lyt)
	} else if i, err := strconv.Atoi(op.arg); err == nil {
		printLookup(lyt, i)
		n.inx = i
		intp.setLastPathNode(n)
	} else {
		tracer().Errorf("Lookup index not numeric: %v\n", op.arg)
		err = errors.New("invalid lookup index")
	}
	return
}

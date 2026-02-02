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
		if trm != nil {
			target = trm.LookupTag(ot.T(tag))
			tracer().Infof("%s map keys = %v", trm.Name(), otlayout.KeyTags(trm))
			pterm.Printf("%s table maps [tag %v] => %v\n", trm.Name(), ot.T(tag), target.Name())
		} else {
			target = m.LookupTag(ot.T(tag))
			pterm.Printf("%s table maps [%v] => %v\n", m.Name(), ot.T(tag), target.Name())
		}
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
			subset := features.Subset(l)
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

func featuresOp(intp *Intp, op *Op) (err error, stop bool) {
	if err = intp.checkTable(); err != nil {
		return
	}
	var features ot.TagRecordMap
	if features, err = otlayout.GetFeatureList(intp.table); err != nil {
		return
	}
	if op.noArg() {
		pterm.Printf("%s table has %d entries\n", features.Name(), features.Len())
	} else if i, err := strconv.Atoi(op.arg); err == nil {
		tag, _ := features.Get(i) //tag, lnk := f.Get(i)
		pterm.Printf("%s list index %d holds feature record = %v\n", features.Name(), i, tag)
	} else {
		err = fmt.Errorf("List index not numeric: %v\n", op.arg)
	}
	return
}

func lookupsOp(intp *Intp, op *Op) (err error, stop bool) {
	if err = intp.checkTable(); err != nil {
		return
	}
	var lyt *ot.LayoutTable
	if lyt, err = otlayout.GetLayoutTable(intp.table); err != nil {
		return
	}
	if op.noArg() {
		printLookupList(lyt)
	} else if i, err := strconv.Atoi(op.arg); err == nil {
		printLookup(lyt, i)
	} else {
		tracer().Errorf("Lookup index not numeric: %v\n", op.arg)
		err = errors.New("invalid lookup index")
	}
	return
}

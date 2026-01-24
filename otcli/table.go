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
	var nav ot.Navigator
	if nav, err = intp.checkLocation(); err == nil {
		return
	}
	l := nav.List()
	if l.Len() == 0 {
		err = errors.New("list is empty / not a list")
	} else if op.noArg() {
		pterm.Printf("List has %d entries\n", l.Len())
	} else if i, err := strconv.Atoi(op.arg); err == nil {
		loc := l.Get(i)
		size, value := loc.Size(), decodeLocation(loc, l.Name())
		switch value.(type) {
		case int:
			pterm.Printf("%s list index %d holds number = %d\n", l.Name(), i, value)
		default:
			pterm.Printf("%s list index %d holds data of %d bytes\n", l.Name(), i, size)
		}
	} else {
		err = fmt.Errorf("List index not numeric: %v\n", op.arg)
	}
	return
}

func mapOp(intp *Intp, op *Op) (err error, stop bool) {
	var nav ot.Navigator
	if nav, err = intp.checkLocation(); err == nil {
		return
	}
	var target ot.NavLink
	m := nav.Map()
	if tag, ok := op.hasArg(); ok {
		if m.IsTagRecordMap() {
			trm := m.AsTagRecordMap()
			target = trm.LookupTag(ot.T(tag))
			tracer().Infof("%s map keys = %v", trm.Name(), trm.Tags())
			pterm.Printf("%s table maps [tag %v] => %v\n", trm.Name(), ot.T(tag), target.Name())
		} else {
			target = m.LookupTag(ot.T(tag))
			pterm.Printf("%s table maps [%v] => %v\n", m.Name(), ot.T(tag), target.Name())
		}
		n := intp.lastPathNode()
		n.key, n.link = tag, target
		intp.setLastPathNode(n)
	} else if m.IsTagRecordMap() {
		trm := m.AsTagRecordMap()
		pterm.Printf("%s map keys = %v\n", trm.Name(), trm.Tags())
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
	pterm.Printf("ScriptList keys: %v\n", scr.Tags())
	n := pathNode{location: nav}
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

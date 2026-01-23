package main

import (
	"errors"
	"strconv"

	"github.com/npillmayer/opentype/ot"
	"github.com/pterm/pterm"
)

func tableOp(intp *Intp, op *Op) (error, bool) {
	tag := op.arg
	if intp.table = intp.font.Table(ot.T(tag)); intp.table == nil {
		return errors.New("table not found"), false
	}
	intp.stack = intp.stack[:0]
	//intp.stack = append(intp.stack, pathNode{table: intp.table})
	tracer().Infof("setting table: %v", tag)
	return nil, false
}

func listOp(intp *Intp, op *Op) (error, bool) {
	if intp.table == nil {
		pterm.Error.Println("cannot list without table being set")
		return errors.New("table not set"), false
	} else if len(intp.stack) == 0 || intp.lastPathNode().location == nil {
		pterm.Info.Println("no starting point set")
		return errors.New("no starting point set"), false
	}
	l := intp.lastPathNode().location.List()
	if op.arg == "" {
		pterm.Printf("List has %d entries\n", l.Len())
	} else if i, err := strconv.Atoi(op.arg); err == nil {
		loc := l.Get(i)
		size := loc.Size()
		value := decodeLocation(loc, l.Name())
		switch value.(type) {
		case int:
			pterm.Printf("%s list index %d holds number = %d\n", l.Name(), i, value)
		default:
			pterm.Printf("%s list index %d holds data of %d bytes\n", l.Name(), i, size)
		}
	} else {
		pterm.Error.Printf("List index not numeric: %v\n", op.arg)
		return errors.New("invalid list index"), false
	}
	return nil, false
}

func mapOp(intp *Intp, op *Op) (error, bool) {
	if intp.table == nil {
		pterm.Error.Println("cannot map without table being set")
		return errors.New("table not set"), false
	} else if len(intp.stack) == 0 || intp.lastPathNode().location == nil {
		pterm.Info.Println("no starting point set")
		return errors.New("no starting point set"), false
	}
	var target ot.NavLink
	m := intp.lastPathNode().location.Map()
	if op.arg != "" {
		tag := op.arg
		if m.IsTagRecordMap() {
			trm := m.AsTagRecordMap()
			target = trm.LookupTag(ot.T(tag))
			tracer().Infof("%s map keys = %v", trm.Name(), trm.Tags())
			pterm.Printf("%s table maps [tag %v] = %v\n", trm.Name(), ot.T(tag), target.Name())
		} else {
			target = m.LookupTag(ot.T(tag))
			pterm.Printf("%s table maps [%v] = %v\n", m.Name(), ot.T(tag), target.Name())
		}
		n := intp.lastPathNode()
		n.key, n.link = tag, target
		intp.setLastPathNode(n)
	} else if m.IsTagRecordMap() {
		trm := m.AsTagRecordMap()
		pterm.Printf("%s map keys = %v\n", trm.Name(), trm.Tags())
	}
	return nil, false
}

func scriptsOp(intp *Intp, op *Op) (error, bool) {
	if err := intp.checkTable(); err != nil {
		return err, false
	}
	var s ot.Navigator
	switch intp.table.Self().NameTag().String() {
	case "GSUB":
		s = intp.table.Self().AsGSub().ScriptList
	case "GPOS":
		s = intp.table.Self().AsGPos().ScriptList
	default:
		return errors.New("unsupported table type for ScriptList"), false
	}
	if s == nil {
		return errors.New("table has no script list"), false
	}
	m := s.Map().AsTagRecordMap()
	pterm.Printf("ScriptList keys: %v\n", m.Tags())
	n := pathNode{location: s, inx: -1}
	if op.arg != "" {
		n.key = op.arg
		l := m.LookupTag(ot.T(op.arg))
		if l.IsNull() {
			tracer().Infof("script lookup [%s] returns null", ot.T(op.arg).String())
			return errors.New("script lookup returns null"), false
		}
		n.link = l
	}
	intp.stack = append(intp.stack, n)
	return nil, false
}

func featuresOp(intp *Intp, op *Op) (error, bool) {
	var f ot.TagRecordMap
	switch intp.table.Self().NameTag().String() {
	case "GSUB":
		f = intp.table.Self().AsGSub().FeatureList
	case "GPOS":
		f = intp.table.Self().AsGPos().FeatureList
	default:
		return errors.New("unsupported table type for FeatureList"), false
	}
	if f == nil {
		return errors.New("table has no feature list"), false
	}
	if op.arg == "" {
		tracer().Infof("%s table has %d entries", f.Name(), f.Len())
	} else if i, err := strconv.Atoi(op.arg); err == nil {
		tag, _ := f.Get(i)
		//tag, lnk := f.Get(i)
		pterm.Printf("%s list index %d holds feature record = %v\n", f.Name(), i, tag)
	} else {
		pterm.Error.Printf("List index not numeric: %v\n", op.arg)
	}
	return nil, false
}

func lookupsOp(intp *Intp, op *Op) (error, bool) {
	if err := intp.checkTable(); err != nil {
		return err, false
	}
	switch intp.table.Self().NameTag().String() {
	case "GSUB":
		gsub := intp.table.Self().AsGSub()
		if gsub == nil {
			return errors.New("GSUB table is nil"), false
		}
		if op.arg == "" {
			printLookupList(gsub)
			break
		}
		if i, err := strconv.Atoi(op.arg); err == nil {
			printLookup(gsub, i)
		} else {
			pterm.Error.Printf("Lookup index not numeric: %v\n", op.arg)
			return errors.New("invalid lookup index"), false
		}
	default:
		return errors.New("unsupported table type for lookups"), false
	}
	return nil, false
}

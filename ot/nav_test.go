package ot

import (
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestNavLink(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	otf := loadCalibri(t)
	table := otf.Table(T("GSUB"))
	if table == nil {
		t.Fatal("cannot locate table GSUB in font")
	}
	gsub := table.Self().AsGSub()
	if gsub == nil {
		t.Fatal("cannot convert GSUB table")
	}
	sg := gsub.ScriptGraph()
	if sg == nil {
		t.Fatalf("expected concrete ScriptGraph for GSUB")
	}
	script := sg.Script(T("latn"))
	if script == nil {
		t.Fatalf("expected concrete script for tag 'latn'")
	}
	lang := script.LangSys(T("TRK"))
	if lang == nil {
		t.Fatalf("expected concrete LangSys for tag 'TRK'")
	}
	features := lang.Features()
	t.Logf("LangSys[TRK] has %d feature links", len(features))
	if len(features) == 0 {
		t.Errorf("expected LangSys[TRK] to contain feature links")
	}
}

func TestTableNav(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	otf := loadCalibri(t)
	table := otf.Table(T("name"))
	if table == nil {
		t.Fatal("cannot locate table name in font")
	}
	name := table.Fields().Name()
	if name != "name" {
		t.Errorf("expected table to have name 'name', have %s", name)
	}
	nameRecs, ok := AsNameRecords(table.Fields())
	if !ok {
		t.Fatal("name table does not provide NameRecords view")
	}
	x := ""
	for k, link := range nameRecs.Range() {
		if k.PlatformID == 3 && k.EncodingID == 1 && k.NameID == 1 {
			x = link.Navigate().Name()
			if x != "" {
				break
			}
		}
	}
	if x != "Calibri" {
		t.Errorf("expected Windows/1 encoded field 1 to be 'Calibri', is %s", x)
	}
}

func TestTableNavOS2(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	otf := loadCalibri(t)
	table := otf.Table(T("OS/2"))
	if table == nil {
		t.Fatal("cannot locate table OS/2 in font")
	}
	name := table.Fields().Name()
	if name != "OS/2" {
		t.Errorf("expected table name to be 'OS/2', is %s", name)
	}
	loc := table.Fields().List().Get(1)
	if loc.U16(0) != 400 {
		t.Errorf("expected xAvgCharWidth (size %d) of Calibri to be 400, is %d", loc.Size(), loc.U16(0))
	}
}

// ---------------------------------------------------------------------------

func loadCalibri(t *testing.T) *Font {
	//f := loadTestFont(t, "calibri")
	f := loadTestdataFont(t, "Calibri")
	otf, err := Parse(f.F.Binary)
	if err != nil {
		t.Fatal(err)
	}
	tracer().Infof("========= loading done =================")
	return otf
}

func setTagRecord(m tagRecordMap16, i int, tag Tag, offset uint16) {
	b := m.records.Get(i).Bytes()
	copy(b[:4], []byte(tag.String()))
	b[4] = byte(offset >> 8)
	b[5] = byte(offset)
}

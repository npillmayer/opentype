package ot

import (
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestNavigation1(t *testing.T) {
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

// ---------------------------------------------------------------------------

func loadCalibri(t *testing.T) *Font {
	//f := loadTestFont(t, "calibri")
	f := loadTestdataFont(t, "Calibri")
	otf, err := Parse(f.Binary())
	if err != nil {
		t.Fatal(err)
	}
	tracer().Infof("========= loading done =================")
	return otf
}

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
	m := gsub.ScriptList.Map()
	if !m.IsTagRecordMap() {
		t.Fatalf("script list is not a tag record map")
	}
	recname := m.AsTagRecordMap().LookupTag(T("latn")).Navigate().Name()
	t.Logf("walked to %s", recname)
	lang := m.AsTagRecordMap().LookupTag(T("latn")).Navigate().Map().AsTagRecordMap().LookupTag(T("TRK"))
	langlist := lang.Navigate().List()
	t.Logf("list is %s of length %v", lang.Name(), langlist.Len())
	if lang.Name() != "LangSys" || langlist.Len() != 24 {
		t.Errorf("expected LangSys[IPPH] to contain 24 feature entries, has %d", langlist.Len())
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
	key := MakeTag([]byte{3, 1, 0, 1}) // Windows 1-encoded field 1 = Font Family Name
	x := table.Fields().Map().AsTagRecordMap().LookupTag(key).Navigate().Name()
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

func TestTagRecordMapSubset(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	base := binarySegm(make([]byte, 64))
	tags := []Tag{T("f001"), T("f002"), T("f003"), T("f004")}
	offsets := []uint16{10, 20, 30, 40}
	values := []uint16{0x1111, 0x2222, 0x3333, 0x4444}

	for i := range offsets {
		writeU16(base, int(offsets[i]), values[i])
	}

	m := makeTagRecordMap16("FeatureList", "Feature", nil, base, 0, len(tags))
	for i := range tags {
		setTagRecord(m, i, tags[i], offsets[i])
	}

	indices := []int{2, 0}
	subset := m.Subset(indices)

	if subset.Len() != 2 {
		t.Fatalf("expected subset length 2, got %d", subset.Len())
	}

	tag0, link0 := subset.Get(0)
	if tag0 != tags[2] {
		t.Fatalf("expected tag %s at subset[0], got %s", tags[2], tag0)
	}
	if link0.Name() != "Feature" {
		t.Fatalf("expected link target Feature, got %s", link0.Name())
	}
	if link0.Jump().U16(0) != values[2] {
		t.Fatalf("expected link0 to jump to value %x, got %x", values[2], link0.Jump().U16(0))
	}

	tag1, link1 := subset.Get(1)
	if tag1 != tags[0] {
		t.Fatalf("expected tag %s at subset[1], got %s", tags[0], tag1)
	}
	if link1.Name() != "Feature" {
		t.Fatalf("expected link target Feature, got %s", link1.Name())
	}
	if link1.Jump().U16(0) != values[0] {
		t.Fatalf("expected link1 to jump to value %x, got %x", values[0], link1.Jump().U16(0))
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

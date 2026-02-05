package ot

import "testing"

func TestVarArrayGetDeepFalse(t *testing.T) {
	// Layout:
	// [0..1] count=1
	// [2..3] offset=4 (to level-1 array)
	// [4..5] count=1
	// [6..7] offset=8 (to entry)
	// [8..9] entry data (0x002A)
	b := binarySegm{
		0x00, 0x01,
		0x00, 0x04,
		0x00, 0x01,
		0x00, 0x08,
		0x00, 0x2A,
	}
	va := parseVarArray16(b, 0, 2, 2, "test")
	loc, err := va.Get(0, false)
	if err != nil {
		t.Fatalf("Get(deep=false): %v", err)
	}
	if loc.U16(0) != 1 || loc.U16(2) != 8 {
		t.Fatalf("unexpected level-1 array header: %v", loc.Bytes()[:4])
	}
}

func TestVarArrayGetDeepTrue(t *testing.T) {
	b := binarySegm{
		0x00, 0x01,
		0x00, 0x04,
		0x00, 0x01,
		0x00, 0x08,
		0x00, 0x2A,
	}
	va := parseVarArray16(b, 0, 2, 2, "test")
	loc, err := va.Get(0, true)
	if err != nil {
		t.Fatalf("Get(deep=true): %v", err)
	}
	if loc.U16(0) != 0x002A {
		t.Fatalf("unexpected entry value: %d", loc.U16(0))
	}
}

func TestVarArrayGetDeepOneLevel(t *testing.T) {
	b := binarySegm{
		0x00, 0x01,
		0x00, 0x04,
		0x00, 0x2A,
	}
	va := parseVarArray16(b, 0, 2, 1, "test")
	locFalse, err := va.Get(0, false)
	if err != nil {
		t.Fatalf("Get(deep=false): %v", err)
	}
	locTrue, err := va.Get(0, true)
	if err != nil {
		t.Fatalf("Get(deep=true): %v", err)
	}
	if locFalse.U16(0) != locTrue.U16(0) || locTrue.U16(0) != 0x002A {
		t.Fatalf("unexpected entry values: false=%d true=%d", locFalse.U16(0), locTrue.U16(0))
	}
}

func TestVarArrayGetDeepTwoLevelsIndex(t *testing.T) {
	b := binarySegm{
		0x00, 0x02, // count=2
		0x00, 0x08, // off0 -> level1 A
		0x00, 0x12, // off1 -> level1 B
		0x00, 0x00, // padding
		0x00, 0x02, // level1 A count=2
		0x00, 0x1A, // A0 -> entry
		0x00, 0x1C, // A1 -> entry
		0x00, 0x00, // padding
		0x00, 0x00, // padding
		0x00, 0x02, // level1 B count=2
		0x00, 0x1E, // B0 -> entry
		0x00, 0x20, // B1 -> entry
		0x00, 0x00, // padding
		0x00, 0xAA, // entry A0
		0x00, 0xBB, // entry A1
		0x00, 0xCC, // entry B0
		0x00, 0xDD, // entry B1
	}
	va := parseVarArray16(b, 0, 2, 2, "test")
	loc, err := va.Get(1, true)
	if err != nil {
		t.Fatalf("Get(deep=true): %v", err)
	}
	if loc.U16(0) != 0x00DD {
		t.Fatalf("unexpected entry value: %d", loc.U16(0))
	}
}

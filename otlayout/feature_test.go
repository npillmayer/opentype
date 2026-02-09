package otlayout

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
)

type testGlyphRange struct {
	glyph ot.GlyphIndex
}

func (t testGlyphRange) Match(g ot.GlyphIndex) (int, bool) {
	if g == t.glyph {
		return 0, true
	}
	return 0, false
}

func (t testGlyphRange) ByteSize() int {
	return 0
}

func TestDispatchGSubLookupSingleFmt1Routing(t *testing.T) {
	sub := ot.LookupSubtable{
		LookupType: ot.GSubLookupTypeSingle,
		Format:     1,
		Coverage: ot.Coverage{
			GlyphRange: testGlyphRange{glyph: 10},
		},
		Support: ot.GlyphIndex(2),
	}
	ctx := applyCtx{
		lookup: &ot.Lookup{},
		subnode: &ot.LookupNode{
			GSub: &ot.GSubLookupPayload{
				SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 2},
			},
		},
		buf: &BufferState{Glyphs: GlyphBuffer{10}},
		pos: 0,
	}

	pos, ok, buf, _, edit := dispatchGSubLookup(&ctx, &sub)
	if !ok {
		t.Fatalf("expected lookup to apply")
	}
	if pos != 1 {
		t.Fatalf("expected pos to advance to 1, got %d", pos)
	}
	if edit == nil || edit.From != 0 || edit.To != 1 || edit.Len != 1 {
		t.Fatalf("unexpected edit span: %+v", edit)
	}
	if buf[0] != 12 {
		t.Fatalf("expected glyph 12, got %d", buf[0])
	}
}

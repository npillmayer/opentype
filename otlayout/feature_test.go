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
		buf:    GlyphSlice{10},
		pos:    0,
	}

	pos, ok, buf, edit := dispatchGSubLookup(&ctx, &sub)
	if !ok {
		t.Fatalf("expected lookup to apply")
	}
	if pos != 1 {
		t.Fatalf("expected pos to advance to 1, got %d", pos)
	}
	if edit == nil || edit.From != 0 || edit.To != 1 || edit.Len != 1 {
		t.Fatalf("unexpected edit span: %+v", edit)
	}
	out, ok := buf.(GlyphSlice)
	if !ok {
		t.Fatalf("expected GlyphSlice buffer, got %T", buf)
	}
	if out[0] != 12 {
		t.Fatalf("expected glyph 12, got %d", out[0])
	}
}

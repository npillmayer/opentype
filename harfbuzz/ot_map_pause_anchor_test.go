package harfbuzz

import (
	"testing"

	ot "github.com/go-text/typesetting/font/opentype"
)

func TestAddAnchoredGSUBPausesMapsTags(t *testing.T) {
	stages := []stageInfo{
		{index: 1},
		{index: 3},
	}
	features := []featureMap{
		{tag: ot.NewTag('s', 't', 'c', 'h'), stage: [2]int{2, 0}},
		{tag: ot.NewTag('r', 'l', 'i', 'g'), stage: [2]int{5, 0}},
	}
	anchors := []gsubPauseAnchor{
		{tag: ot.NewTag('s', 't', 'c', 'h'), kind: gsubPauseAfterTag},
		{tag: ot.NewTag('r', 'l', 'i', 'g'), kind: gsubPauseBeforeTag},
		{tag: ot.NewTag('x', 'x', 'x', 'x'), kind: gsubPauseAfterTag}, // ignored
	}

	got := addAnchoredGSUBPauses(stages, anchors, features)
	if len(got) != 4 {
		t.Fatalf("len(stages) = %d, want 4", len(got))
	}

	indices := []int{got[0].index, got[1].index, got[2].index, got[3].index}
	want := []int{1, 2, 3, 4}
	for i := range want {
		if indices[i] != want[i] {
			t.Fatalf("stage indices[%d] = %d, want %d (%v)", i, indices[i], want[i], indices)
		}
	}
}

func TestAddAnchoredGSUBPausesBeforeStageZero(t *testing.T) {
	stages := []stageInfo{{index: 0}}
	features := []featureMap{
		{tag: ot.NewTag('s', 't', 'c', 'h'), stage: [2]int{0, 0}},
	}
	anchors := []gsubPauseAnchor{
		{tag: ot.NewTag('s', 't', 'c', 'h'), kind: gsubPauseBeforeTag},
	}

	got := addAnchoredGSUBPauses(stages, anchors, features)
	if len(got) != 2 {
		t.Fatalf("len(stages) = %d, want 2", len(got))
	}
	if got[0].index != -1 {
		t.Fatalf("first stage index = %d, want -1", got[0].index)
	}
}

package otshape

import (
	"errors"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestFeatureSetApplyPushAndExport(t *testing.T) {
	base := newFeatureSet([]FeatureRange{
		{Feature: ot.T("liga"), On: true, Arg: 1},
		{Feature: ot.T("kern"), On: false},
		{Feature: ot.T("smcp"), On: true, Start: 2, End: 5}, // ignored (not global)
	})
	next := base.applyPush([]FeatureSetting{
		{Tag: ot.T("smcp"), Enabled: true},
		{Tag: ot.T("kern"), Enabled: true, Value: 2},
	})
	ranges := next.asGlobalFeatureRanges()
	if len(ranges) != 3 {
		t.Fatalf("global feature range count=%d, want 3", len(ranges))
	}
	found := map[ot.Tag]FeatureRange{}
	for _, r := range ranges {
		found[r.Feature] = r
	}
	if r, ok := found[ot.T("liga")]; !ok || !r.On || r.Arg != 1 {
		t.Fatalf("unexpected liga range: %+v", r)
	}
	if r, ok := found[ot.T("kern")]; !ok || !r.On || r.Arg != 2 {
		t.Fatalf("unexpected kern range: %+v", r)
	}
	if r, ok := found[ot.T("smcp")]; !ok || !r.On || r.Arg != 1 {
		t.Fatalf("unexpected smcp range: %+v", r)
	}
}

func TestPlanStackPushPopAndClose(t *testing.T) {
	stack := newPlanStack([]FeatureRange{
		{Feature: ot.T("liga"), On: true},
	})
	if stack.depth() != 1 {
		t.Fatalf("stack depth=%d, want 1", stack.depth())
	}
	if stack.currentPlanID() != 1 {
		t.Fatalf("root plan id=%d, want 1", stack.currentPlanID())
	}
	id, err := stack.push([]FeatureSetting{{Tag: ot.T("smcp"), Enabled: true}})
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}
	if id != 2 {
		t.Fatalf("pushed plan id=%d, want 2", id)
	}
	if stack.depth() != 2 {
		t.Fatalf("stack depth=%d, want 2", stack.depth())
	}
	if !stack.currentFeatures().byTag[ot.T("smcp")].enabled || !stack.currentFeatures().byTag[ot.T("liga")].enabled {
		t.Fatal("push did not inherit liga and enable smcp")
	}
	if err := stack.ensureClosed(); !errors.Is(err, errPlanStackUnclosed) {
		t.Fatalf("ensureClosed error=%v, want errPlanStackUnclosed", err)
	}
	if err := stack.pop(); err != nil {
		t.Fatalf("pop failed: %v", err)
	}
	if stack.depth() != 1 {
		t.Fatalf("stack depth=%d, want 1", stack.depth())
	}
	if _, exists := stack.currentFeatures().byTag[ot.T("smcp")]; exists {
		t.Fatal("pop retained pushed feature")
	}
	if err := stack.ensureClosed(); err != nil {
		t.Fatalf("ensureClosed failed: %v", err)
	}
}

func TestPlanStackPopUnderflow(t *testing.T) {
	stack := newPlanStack(nil)
	err := stack.pop()
	if !errors.Is(err, errPlanStackUnderflow) {
		t.Fatalf("pop error=%v, want errPlanStackUnderflow", err)
	}
}

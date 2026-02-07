package harfbuzz

import "testing"

var (
	_ ShapingEngine = complexShaperDefault{}
)

func TestPhaseA_BuiltinsExposeExportedHookSurface(t *testing.T) {
	engines := []ShapingEngine{
		NewDefaultShapingEngine(),
	}

	for i, eng := range engines {
		if eng == nil {
			t.Fatalf("engine %d is nil", i)
		}

		clone := eng.New()
		if clone == nil {
			t.Fatalf("engine %q returned nil from New", eng.Name())
		}

		_ = clone.Name()
		_ = clone.Match(SelectionContext{})
		_ = clone.GposTag()
		_, _ = clone.MarksBehavior()
		_ = clone.NormalizationPreference()
	}
}

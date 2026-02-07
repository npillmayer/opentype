package harfbuzz

// NewDefaultShapingEngine returns a fresh default OpenType shaping engine.
func NewDefaultShapingEngine() ShapingEngine {
	return complexShaperDefault{}.New()
}

package harfbuzz

import (
	otfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// SyntheticGSUBLookup describes a synthetic GSUB lookup to be applied with a
// specific glyph mask.
type SyntheticGSUBLookup struct {
	Mask        GlyphMask
	LookupFlags uint16
	Subtables   []tables.GSUBLookup
}

// SyntheticGSUBProgram is an executable sequence of synthetic GSUB lookups.
// It is intentionally opaque to callers.
type SyntheticGSUBProgram struct {
	lookups []otLayoutLookupAccelerator
	masks   []GlyphMask
}

// CompileSyntheticGSUBProgram compiles lookup specs into an executable program.
func CompileSyntheticGSUBProgram(specs []SyntheticGSUBLookup) *SyntheticGSUBProgram {
	program := &SyntheticGSUBProgram{}

	for _, spec := range specs {
		if spec.Mask == 0 || len(spec.Subtables) == 0 {
			continue
		}

		lookup := lookupGSUB{
			LookupOptions: otfont.LookupOptions{Flag: spec.LookupFlags},
			Subtables:     append([]tables.GSUBLookup(nil), spec.Subtables...),
		}

		var accel otLayoutLookupAccelerator
		accel.init(lookup)
		program.lookups = append(program.lookups, accel)
		program.masks = append(program.masks, spec.Mask)
	}

	return program
}

// Empty reports whether a program has no executable lookups.
func (p *SyntheticGSUBProgram) Empty() bool {
	return p == nil || len(p.lookups) == 0
}

// NumLookups reports the number of executable lookups in a program.
func (p *SyntheticGSUBProgram) NumLookups() int {
	if p == nil {
		return 0
	}
	return len(p.lookups)
}

// Apply executes all synthetic lookups on a shaping buffer.
// It returns true if at least one lookup was executed.
func (p *SyntheticGSUBProgram) Apply(font *Font, buffer *Buffer) bool {
	if p.Empty() {
		return false
	}

	var c otApplyContext
	c.reset(0, font, buffer)
	for i := range p.lookups {
		if p.lookups[i].lookup == nil || p.masks[i] == 0 {
			continue
		}
		c.setLookupMask(p.masks[i])
		c.substituteLookup(&p.lookups[i])
	}

	return true
}

package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
)

func mapRunesToRunBuffer(runes []rune, clusters []uint32, font *ot.Font) *runBuffer {
	return mapRunesToRunBufferWithPlanIDs(runes, clusters, nil, font)
}

func mapRunesToRunBufferWithPlanIDs(runes []rune, clusters []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	return mapRunesToRunBufferInto(newRunBuffer(len(runes)), runes, clusters, planIDs, font)
}

func mapRunesToRunBufferInto(run *runBuffer, runes []rune, clusters []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	if run == nil {
		run = newRunBuffer(len(runes))
	}
	withPlanIDs := len(planIDs) == len(runes)
	run.PrepareForMappedRun(withPlanIDs, len(runes))
	for i, r := range runes {
		gid, lookupErr := otquery.GlyphIndex(font, r)
		if lookupErr != nil {
			tracer().Errorf("cmap lookup: %v", lookupErr)
		}
		cluster := uint32(i)
		if len(clusters) == len(runes) {
			cluster = clusters[i]
		}
		planID := uint16(0)
		if withPlanIDs {
			planID = planIDs[i]
		}
		run.AppendMappedGlyph(gid, r, cluster, planID, withPlanIDs)
	}
	return run
}

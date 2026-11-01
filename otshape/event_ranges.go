package otshape

import (
	"sort"

	"github.com/npillmayer/opentype/ot"
)

// eventFeatureRanges expresses immutable scope assignments in the same original
// scalar coordinates as Shape. Group-start selection happens later, after
// forming initial clusters across the whole carry (including scope boundaries).
func eventFeatureRanges(st *streamingState, root []FeatureRange, rootPlan *plan, scopes map[uint16]featureSet) ([]FeatureRange, error) {
	if len(st.rawPlanIDs) != len(st.rawRunes) {
		return nil, errShaper("event carry scope-id alignment mismatch")
	}
	base := newFeatureSet(root)
	tags := map[ot.Tag]bool{}
	for _, id := range st.rawPlanIDs {
		scope, ok := scopes[id]
		if !ok {
			return nil, errShaper("missing feature scope for event carry")
		}
		for tag := range scope.byTag {
			tags[tag] = true
		}
	}
	// Ranged off/on settings must not change the default outside their ranges.
	// Resolve implicit defaults from the root plan, including engine defaults.
	for tag := range tags {
		if _, explicit := base.byTag[tag]; !explicit {
			base.byTag[tag] = planDefaultAssignment(rootPlan, tag)
		}
	}
	features := base.asGlobalFeatureRanges()
	ordered := make([]ot.Tag, 0, len(tags))
	for tag := range tags {
		ordered = append(ordered, tag)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for start := 0; start < len(st.rawRunes); {
		id := st.rawPlanIDs[start]
		end := start + 1
		for end < len(st.rawRunes) && st.rawPlanIDs[end] == id {
			end++
		}
		for _, tag := range ordered {
			assignment, present := scopes[id].byTag[tag]
			if !present || assignment == base.byTag[tag] {
				continue
			}
			features = append(features, FeatureRange{Feature: tag, On: assignment.enabled, Arg: assignment.value,
				Start: int(st.rawClusters[start]), End: int(uint64(st.rawClusters[end-1]) + 1)})
		}
		start = end
	}
	return features, nil
}

func planDefaultAssignment(pl *plan, tag ot.Tag) featureAssignment {
	if spec, ok := pl.maskForFeature(tag); ok && spec.Mask != 0 {
		return featureAssignment{enabled: pl.Masks.GlobalMask&spec.Mask != 0, value: spec.decode(pl.Masks.GlobalMask)}
	}
	for _, program := range []tableProgram{pl.GSUB, pl.GPOS} {
		for _, lookup := range program.Lookups {
			for _, feature := range lookup.Features {
				if feature.Tag == tag {
					return featureAssignment{enabled: true, value: 1}
				}
			}
		}
	}
	return featureAssignment{}
}

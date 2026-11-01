package otshape

import "unicode"

// formInputClusters implements the shared base/mark grouping needed by Latin
// and Hebrew at cluster level 0. A leading mark starts its own group; following
// marks (including variation selectors) and ZWJ continue the preceding group.
// ZWNJ remains a boundary. Script-specific and emoji sequence rules are separate.
func formInputClusters(dst []uint32, runes []rune, sources []uint32) []uint32 {
	for i, r := range runes {
		cluster := sources[i]
		if i > 0 && (unicode.Is(unicode.M, r) || r == '\u200D') {
			cluster = dst[i-1]
		}
		dst = append(dst, cluster)
	}
	return dst
}

// mergeClusterRange includes adjacent members of either boundary group. A
// substitution or reorder must not split an existing base/mark association.
// SourceAnchors and already-assigned feature masks are intentionally unchanged.
func mergeClusterRange(clusters []uint32, start, end int) {
	if start >= end {
		return
	}
	for start > 0 && clusters[start-1] == clusters[start] {
		start--
	}
	for end < len(clusters) && clusters[end] == clusters[end-1] {
		end++
	}
	minimum := clusters[start]
	for _, c := range clusters[start:end] {
		if c < minimum {
			minimum = c
		}
	}
	for i := start; i < end; i++ {
		clusters[i] = minimum
	}
}

// mergeDeletionClusters preserves level-0 cluster coverage when the last glyph
// of a cluster disappears. Prefer the preceding surviving group; at the start
// of a run merge into the following group. An already surviving cluster needs
// no change. Source anchors and feature masks are deliberately untouched.
// This follows HarfBuzz 15 hb_buffer_t::delete_glyph's cluster merge rules.
func mergeDeletionClusters(clusters []uint32, start, end int) {
	cluster := clusters[start] // the consumed range has already been merged
	if start > 0 && clusters[start-1] == cluster || end < len(clusters) && clusters[end] == cluster {
		return
	}
	if start > 0 {
		if cluster < clusters[start-1] {
			mergeClusterRange(clusters, start-1, end)
		}
	} else if end < len(clusters) {
		mergeClusterRange(clusters, start, end+1)
	}
}

type runSpan struct {
	start int
	end   int
}

func clusterSpans(run *runBuffer) []runSpan {
	n := run.Len()
	if n == 0 {
		return nil
	}
	if len(run.Clusters) != n {
		return []runSpan{{start: 0, end: n}}
	}
	spans := make([]runSpan, 0, n)
	start := 0
	current := run.Clusters[0]
	for i := 1; i < n; i++ {
		if run.Clusters[i] == current {
			continue
		}
		spans = append(spans, runSpan{start: start, end: i})
		start = i
		current = run.Clusters[i]
	}
	spans = append(spans, runSpan{start: start, end: n})
	return spans
}

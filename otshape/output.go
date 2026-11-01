package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
)

func writeRunBufferPrefixToSink(run *runBuffer, sink GlyphSink, boundary FlushBoundary, end int) error {
	return writeRunBufferPrefixToSinkWithFont(run, sink, nil, boundary, end)
}

func writeRunBufferPrefixToSinkWithFont(run *runBuffer, sink GlyphSink, font *ot.Font, boundary FlushBoundary, end int) error {
	if run == nil {
		return nil
	}
	n := run.Len()
	if end < 0 {
		end = 0
	}
	if end > n {
		end = n
	}
	if end == 0 {
		return nil
	}
	switch boundary {
	case FlushOnRunBoundary:
		return writeRunBufferRangeWithFont(run, sink, font, 0, end)
	case FlushOnClusterBoundary:
		for _, span := range clusterSpans(run) {
			if span.start >= end {
				break
			}
			if span.end > end {
				return errShaper("streaming prefix cut is not at a cluster boundary")
			}
			if err := writeRunBufferRangeWithFont(run, sink, font, span.start, span.end); err != nil {
				return err
			}
		}
		return nil
	case FlushExplicit:
		return ErrFlushExplicitUnsupported
	default:
		return writeRunBufferRangeWithFont(run, sink, font, 0, end)
	}
}

func writeRunBufferToSink(run *runBuffer, sink GlyphSink, boundary FlushBoundary) error {
	return writeRunBufferToSinkWithFont(run, sink, nil, boundary)
}

func writeRunBufferToSinkWithFont(run *runBuffer, sink GlyphSink, font *ot.Font, boundary FlushBoundary) error {
	switch boundary {
	case FlushOnRunBoundary:
		return writeRunBufferRangeWithFont(run, sink, font, 0, run.Len())
	case FlushOnClusterBoundary:
		for _, span := range clusterSpans(run) {
			if err := writeRunBufferRangeWithFont(run, sink, font, span.start, span.end); err != nil {
				return err
			}
		}
		return nil
	case FlushExplicit:
		return ErrFlushExplicitUnsupported
	default:
		return writeRunBufferRangeWithFont(run, sink, font, 0, run.Len())
	}
}

func writeRunBufferRange(run *runBuffer, sink GlyphSink, start int, end int) error {
	return writeRunBufferRangeWithFont(run, sink, nil, start, end)
}

func writeRunBufferRangeWithFont(run *runBuffer, sink GlyphSink, font *ot.Font, start int, end int) error {
	n := run.Len()
	if n == 0 || start >= end {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	hasPos := len(run.Pos) == n
	hasClusters := len(run.Clusters) == n
	hasMasks := len(run.Masks) == n
	hasUnsafe := len(run.UnsafeFlags) == n
	for i := start; i < end; i++ {
		record := materializeGlyphRecord(run, i, font, hasPos, hasClusters, hasMasks, hasUnsafe)
		if err := sink.WriteGlyph(record); err != nil {
			return err
		}
	}
	return nil
}

func materializeGlyphRecord(
	run *runBuffer,
	inx int,
	font *ot.Font,
	hasPos bool,
	hasClusters bool,
	hasMasks bool,
	hasUnsafe bool,
) GlyphRecord {
	record := GlyphRecord{GID: run.Glyphs[inx]}
	if hasPos {
		record.Pos = run.Pos[inx].OutputPosition()
	}
	if font != nil {
		record.Pos.XAdvance += int32(otquery.GlyphMetrics(font, record.GID).Advance)
	}
	if hasClusters {
		record.Cluster = run.Clusters[inx]
	}
	if hasMasks {
		record.Mask = run.Masks[inx]
	}
	if hasUnsafe {
		record.UnsafeFlags = run.UnsafeFlags[inx]
	}
	return record
}

package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

// RuneSource is the input side of a streaming shaper pipeline.
// The most common implementation will be `bufio.Reader`, but clients may provide
// other implementations.
type RuneSource interface {
	ReadRune() (r rune, size int, err error)
}

// GlyphRecord is one shaped glyph output record (AoS boundary type).
type GlyphRecord struct {
	GID         ot.GlyphIndex
	Pos         otlayout.PosItem
	Cluster     uint32
	Mask        uint32
	UnsafeFlags uint16
}

// GlyphSink receives shaped glyph records.
type GlyphSink interface {
	WriteGlyph(g GlyphRecord) error
}

// GlyphsSink is a compatibility sink shape matching earlier API notes.
type GlyphsSink interface {
	WriteGlyphPos(gid ot.GlyphIndex, pos otlayout.PosItem, cluster int) error
}

// FlushBoundary controls when shaped glyphs are emitted to the sink.
type FlushBoundary uint8

const (
	// FlushOnRunBoundary emits only after the complete shaped run is ready.
	FlushOnRunBoundary FlushBoundary = iota
	// FlushOnClusterBoundary emits run output cluster by cluster in glyph order.
	FlushOnClusterBoundary
	// FlushExplicit is reserved for future explicit flush signaling from RuneSource.
	// It is currently unsupported.
	FlushExplicit
)

// ShapeOptions configures the streaming pipeline.
type ShapeOptions struct {
	Params
	FlushBoundary FlushBoundary
}

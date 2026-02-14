package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

// RuneSource is the input side of the shaping pipeline.
//
// ReadRune returns the next input rune, the rune's byte size in the original
// encoded stream, and an error. A source must return io.EOF to terminate input.
type RuneSource interface {
	ReadRune() (r rune, size int, err error)
}

// GlyphRecord is one shaped output glyph in array-of-struct form.
type GlyphRecord struct {
	GID         ot.GlyphIndex    // GID is the shaped glyph ID in the selected font.
	Pos         otlayout.PosItem // Pos holds positioning and attachment data.
	Cluster     uint32           // Cluster is the input cluster ID associated with this glyph.
	Mask        uint32           // Mask is the final feature mask used during lookup filtering.
	UnsafeFlags uint16           // UnsafeFlags carries break/concat safety hints for boundaries.
}

// GlyphSink is the output side of the shaping pipeline.
//
// WriteGlyph is called once per emitted glyph record; returning a non-nil error
// aborts shaping and returns that error to the caller.
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

// ShapeOptions configures a shaping request.
//
// Embedded [Params] describes what to shape; watermark values tune when buffered
// input is shaped and flushed. Zero watermark values use internal defaults.
type ShapeOptions struct {
	Params
	FlushBoundary FlushBoundary
	// HighWatermark is the preferred max fill level before shaping starts.
	// If zero, an internal default is used.
	HighWatermark int
	// LowWatermark is the minimum cursor position before considering a flush cut.
	// If zero, an internal default is used.
	LowWatermark int
	// MaxBuffer is a hard cap used for forced progress when no safe cut appears.
	// If zero, an internal default is used.
	MaxBuffer int
}

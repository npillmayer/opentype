package otshape

import (
	"context"
	"errors"
	"math"
	"sync"
)

var (
	// ErrChannelCapacity indicates a nonpositive output channel capacity.
	ErrChannelCapacity = errors.New("otshape: channel capacity must be positive")
	// ErrStreamBusy indicates an operation attempted during an active Shape call.
	ErrStreamBusy = errors.New("otshape: streaming shaper is busy")
	// ErrStreamClosed indicates an operation attempted after stream completion.
	ErrStreamClosed = errors.New("otshape: streaming shaper is closed")
	// ErrClusterOverflow indicates that source or output cluster numbering would overflow.
	ErrClusterOverflow = errors.New("otshape: cluster index overflow")
	// ErrNilContext indicates a nil context supplied to StreamingShaper.Shape.
	ErrNilContext = errors.New("otshape: nil shaping context")
)

// StreamingShaper shapes successive uniform runs into one buffered channel.
// Each Shape call is synchronous; typically a producer goroutine calls Shape
// sequentially while a consumer drains Glyphs. Successful calls leave the
// channel open. Call Finish after the last run, including an empty sequence.
// A shaping error or cancellation terminates the stream automatically.
//
// Lifecycle methods are safe to call concurrently. While Shape is active,
// another Shape, SetInitialCluster, or Finish returns ErrStreamBusy. This does
// not make shared fonts or engines safe for other concurrent shaping use.
// Use NewStreamingShaper; the zero value is not usable. Do not copy this value.
type StreamingShaper struct {
	shaper *Shaper
	sink   channelGlyphSink

	mu          sync.Mutex
	running     bool
	closed      bool
	err         error
	nextCluster uint64 // Can hold MaxUint32+1 after the last representable scalar.
}

// NewStreamingShaper creates a shaper, a buffered output channel, and its sink.
// Capacity counts glyph records, independently of BufferOptions. It must be
// positive. Engines have the same selection semantics as in NewShaper.
// The constructor starts no goroutines.
func NewStreamingShaper(capacity int, engines ...ShapingEngine) (*StreamingShaper, error) {
	if capacity <= 0 {
		return nil, ErrChannelCapacity
	}
	return &StreamingShaper{
		shaper: NewShaper(engines...),
		sink:   channelGlyphSink{glyphs: make(chan GlyphRecord, capacity)},
	}, nil
}

// SetInitialCluster sets the original-scalar offset for the next run. The
// default is zero. Each successful Shape advances it by the number of input
// scalars, including those merged into clusters or producing no glyphs.
// Calling this between runs overrides that automatic continuation. It does
// not change run-relative FeatureRange indexes or shaping error indexes.
func (s *StreamingShaper) SetInitialCluster(id uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrStreamClosed
	}
	if s.running {
		return ErrStreamBusy
	}
	s.nextCluster = uint64(id)
	return nil
}

// Glyphs returns the same receive-only channel throughout the stream's life.
// Records are in logical shaping order, including RTL. There are no run-boundary
// markers. After the channel is drained and closed, Err reports its final result.
func (s *StreamingShaper) Glyphs() <-chan GlyphRecord { return s.sink.glyphs }

// Shape shapes one run using the ordinary Shaper pipeline. Params, feature
// ranges, and BufferOptions apply only to this call. Only emitted cluster IDs
// are rebased; all other glyph fields retain their ordinary Shape meanings.
//
// EOF finishes this run without closing Glyphs. Any accepted call that fails
// stores its error and closes the channel, preserving already queued records.
// ErrStreamBusy and ErrStreamClosed reject a call without altering the stream.
//
// Cancellation interrupts blocked channel sends and is checked between source
// reads. It cannot interrupt an already executing ReadRune or shaping lookup;
// a blocking source must support cancellation itself. The context applies only
// during this call. Between calls, the producer must call Finish or make another
// Shape call. A consumer stopping early must cancel the active producer context.
func (s *StreamingShaper) Shape(ctx context.Context, params Params, src RuneSource, opts BufferOptions) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrStreamClosed
	}
	if s.running {
		s.mu.Unlock()
		return ErrStreamBusy
	}
	s.running = true
	base := s.nextCluster
	s.mu.Unlock()

	var err error
	var count uint64
	switch {
	case ctx == nil:
		err = ErrNilContext
	case ctx.Err() != nil:
		err = ctx.Err()
	case src == nil:
		err = ErrNilRuneSource
	default:
		input := &channelRuneSource{ctx: ctx, src: src, base: base}
		sink := s.sink
		sink.ctx, sink.base = ctx, base
		err = s.shaper.Shape(params, input, sink, opts)
		count = input.count
		if err == nil {
			err = ctx.Err()
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	if err != nil {
		s.err = err
		s.closed = true
		close(s.sink.glyphs)
	} else {
		s.nextCluster = base + count
	}
	return err
}

// Finish closes the stream after the final run. It does not wait for consumers
// to drain queued glyphs. Repeated calls return the same terminal result. During
// Shape it returns ErrStreamBusy without closing; cancel that call's context
// to stop it instead. No more runs may be submitted after Finish succeeds.
func (s *StreamingShaper) Finish() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return ErrStreamBusy
	}
	if !s.closed {
		s.closed = true
		close(s.sink.glyphs)
	}
	return s.err
}

// Err returns the terminal shaping or cancellation error, or nil. A nil result
// while the channel is still open does not imply completion. After draining
// Glyphs, the result is final. Lifecycle misuse errors are not stored here.
func (s *StreamingShaper) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

type channelGlyphSink struct {
	glyphs chan GlyphRecord
	ctx    context.Context
	base   uint64
}

func (s channelGlyphSink) WriteGlyph(g GlyphRecord) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	cluster := s.base + uint64(g.Cluster)
	if cluster > math.MaxUint32 {
		return ErrClusterOverflow
	}
	g.Cluster = uint32(cluster)
	select {
	case s.glyphs <- g:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

type channelRuneSource struct {
	ctx   context.Context
	src   RuneSource
	base  uint64
	count uint64
}

func (s *channelRuneSource) ReadRune() (rune, int, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, 0, err
	}
	r, size, err := s.src.ReadRune()
	if err != nil {
		return r, size, err
	}
	if err := s.ctx.Err(); err != nil {
		return 0, 0, err
	}
	// Validate every source scalar, even one later absorbed into a cluster.
	// The ordinary pipeline also needs a representable run-local nextCluster,
	// limiting each call to MaxUint32 scalars. Read first so EOF at the limit
	// still succeeds, including an empty run after global numbering is exhausted.
	if s.base+s.count > math.MaxUint32 || s.count == math.MaxUint32 {
		return 0, 0, ErrClusterOverflow
	}
	s.count++
	return r, size, nil
}

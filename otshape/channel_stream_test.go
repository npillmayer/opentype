package otshape

import (
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

func newTestChannelStream(t *testing.T, capacity int) *StreamingShaper {
	t.Helper()
	s, err := NewStreamingShaper(capacity, &hookProbeShaper{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func awaitChannelShape(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("shaping did not terminate")
		return nil
	}
}

func drainChannelStream(t *testing.T, s *StreamingShaper) []GlyphRecord {
	t.Helper()
	var glyphs []GlyphRecord
	timeout := time.After(5 * time.Second)
	for {
		select {
		case g, ok := <-s.Glyphs():
			if !ok {
				return glyphs
			}
			glyphs = append(glyphs, g)
		case <-timeout:
			t.Fatal("stream did not close")
			return nil
		}
	}
}

func TestStreamingShaperLifecycle(t *testing.T) {
	for _, capacity := range []int{-1, 0} {
		if s, err := NewStreamingShaper(capacity); s != nil || !errors.Is(err, ErrChannelCapacity) {
			t.Fatalf("capacity %d: got %v, %v", capacity, s, err)
		}
	}
	params := standardParams(loadMiniOTFont(t, "gpos3_font1.otf"))
	s := newTestChannelStream(t, 2)
	channel := s.Glyphs()
	if cap(channel) != 2 {
		t.Fatalf("capacity = %d", cap(channel))
	}
	for i := 0; i < 2; i++ {
		if err := s.Shape(context.Background(), params, strings.NewReader(""), BufferOptions{}); err != nil {
			t.Fatal(err)
		}
		select {
		case _, ok := <-channel:
			t.Fatalf("empty run delivered a record or closed the stream: open=%v", ok)
		default:
		}
	}
	if err := s.SetInitialCluster(8); err != nil {
		t.Fatal(err)
	}
	if err := s.Shape(context.Background(), params, strings.NewReader("\x12"), BufferOptions{}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Finish(); err != nil {
			t.Fatal(err)
		}
	}
	if channel != s.Glyphs() {
		t.Fatal("channel changed")
	}
	if err := s.Shape(context.Background(), params, strings.NewReader(""), BufferOptions{}); !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("shape after finish: %v", err)
	}
	if err := s.SetInitialCluster(0); !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("configure after finish: %v", err)
	}
	got := drainChannelStream(t, s)
	if len(got) != 1 || got[0].Cluster != 8 || s.Err() != nil {
		t.Fatalf("result = %v, %v", got, s.Err())
	}

	empty := newTestChannelStream(t, 1)
	if err := empty.Finish(); err != nil {
		t.Fatal(err)
	}
	if got := drainChannelStream(t, empty); len(got) != 0 || empty.Err() != nil {
		t.Fatalf("empty sequence = %v, %v", got, empty.Err())
	}
}

type channelReadFunc func() (rune, int, error)

func (f channelReadFunc) ReadRune() (rune, int, error) { return f() }

func TestStreamingShaperBusy(t *testing.T) {
	params := standardParams(loadMiniOTFont(t, "gpos3_font1.otf"))
	s := newTestChannelStream(t, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	src := channelReadFunc(func() (rune, int, error) {
		close(entered)
		<-release
		return 0, 0, io.EOF
	})
	done := make(chan error, 1)
	go func() { done <- s.Shape(context.Background(), params, src, BufferOptions{}) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("source not read")
	}
	for _, err := range []error{
		s.Shape(context.Background(), params, strings.NewReader(""), BufferOptions{}),
		s.SetInitialCluster(20),
		s.Finish(),
	} {
		if !errors.Is(err, ErrStreamBusy) {
			t.Fatalf("concurrent operation: %v", err)
		}
	}
	if s.Err() != nil {
		t.Fatalf("misuse changed terminal result: %v", s.Err())
	}
	// Permit the blocked read to finish, then verify rejected Finish did not close.
	release <- struct{}{}
	if err := awaitChannelShape(t, done); err != nil {
		t.Fatal(err)
	}
	if err := s.SetInitialCluster(21); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(); err != nil {
		t.Fatal(err)
	}
	drainChannelStream(t, s)
}

func TestStreamingShaperCancelBlockedSend(t *testing.T) {
	params := standardParams(loadMiniOTFont(t, "gpos3_font1.otf"))
	s := newTestChannelStream(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Shape(ctx, params, strings.NewReader("\x12\x13\x12"), BufferOptions{}) }()
	// With three outputs and only one slot, shaping cannot finish without a
	// consumer. Wait until output starts, then cancel without draining anything.
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(5 * time.Second)
	for len(s.Glyphs()) == 0 {
		select {
		case err := <-done:
			t.Fatalf("shaping finished before backpressure: %v", err)
		case <-ticker.C:
		case <-timeout:
			t.Fatal("no output")
		}
	}
	cancel()
	if err := awaitChannelShape(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("Shape: %v", err)
	}
	got := drainChannelStream(t, s)
	if len(got) != 1 || got[0].Cluster != 0 {
		t.Fatalf("queued prefix lost: %v", got)
	}
	if !errors.Is(s.Err(), context.Canceled) || !errors.Is(s.Finish(), context.Canceled) {
		t.Fatalf("terminal error: %v", s.Err())
	}
}

func TestStreamingShaperCancellationDuringRead(t *testing.T) {
	params := standardParams(loadMiniOTFont(t, "gpos3_font1.otf"))
	s := newTestChannelStream(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reads := 0
	src := channelReadFunc(func() (rune, int, error) {
		reads++
		cancel()
		return '\x12', 1, nil
	})
	if err := s.Shape(ctx, params, src, BufferOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Shape: %v", err)
	}
	if got := drainChannelStream(t, s); len(got) != 0 || reads != 1 {
		t.Fatalf("cancellation ignored: glyphs=%v reads=%d", got, reads)
	}
}

func TestStreamingShaperTerminalFailures(t *testing.T) {
	params := standardParams(loadMiniOTFont(t, "gpos3_font1.otf"))
	sourceErr := errors.New("source failed")
	planErr := errors.New("plan failed")
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	deadline, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		params Params
		src    RuneSource
		opts   BufferOptions
		engine ShapingEngine
		want   error
	}{
		{"source", context.Background(), params, channelReadFunc(func() (rune, int, error) { return 0, 0, sourceErr }), BufferOptions{}, nil, sourceErr},
		{"font", context.Background(), Params{}, strings.NewReader(""), BufferOptions{}, nil, ErrNilFont},
		{"source nil", context.Background(), params, nil, BufferOptions{}, nil, ErrNilRuneSource},
		{"context nil", nil, params, strings.NewReader(""), BufferOptions{}, nil, ErrNilContext},
		{"canceled", canceled, params, strings.NewReader(""), BufferOptions{}, nil, context.Canceled},
		{"deadline", deadline, params, strings.NewReader(""), BufferOptions{}, nil, context.DeadlineExceeded},
		{"buffer", context.Background(), params, strings.NewReader(""), BufferOptions{MaxBuffer: -1}, nil, ErrBufferConfig},
		{"plan", context.Background(), params, strings.NewReader("\x12"), BufferOptions{}, &planValidateProbeShaper{validateErr: planErr}, planErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestChannelStream(t, 2)
			// Keep earlier output queued when the next run fails.
			if err := s.Shape(context.Background(), params, strings.NewReader("\x12"), BufferOptions{}); err != nil {
				t.Fatal(err)
			}
			if tc.engine != nil {
				s.shaper = NewShaper(tc.engine)
			}
			if err := s.Shape(tc.ctx, tc.params, tc.src, tc.opts); !errors.Is(err, tc.want) {
				t.Fatalf("Shape: %v, want %v", err, tc.want)
			}
			got := drainChannelStream(t, s)
			if len(got) != 1 || got[0].Cluster != 0 {
				t.Fatalf("previous run lost: %v", got)
			}
			if !errors.Is(s.Err(), tc.want) || !errors.Is(s.Finish(), tc.want) {
				t.Fatalf("terminal result: %v", s.Err())
			}
			if err := s.Shape(context.Background(), params, strings.NewReader(""), BufferOptions{}); !errors.Is(err, ErrStreamClosed) {
				t.Fatalf("retry: %v", err)
			}
		})
	}
}

func TestStreamingShaperClusterOverflow(t *testing.T) {
	params := standardParams(loadMiniOTFont(t, "gpos3_font1.otf"))
	for _, reset := range []bool{false, true} {
		s := newTestChannelStream(t, 3)
		if err := s.SetInitialCluster(math.MaxUint32 - 1); err != nil {
			t.Fatal(err)
		}
		for _, input := range []string{"\x12", "\x13", ""} {
			if err := s.Shape(context.Background(), params, strings.NewReader(input), BufferOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		if reset {
			if err := s.SetInitialCluster(0); err != nil {
				t.Fatal(err)
			}
			if err := s.Shape(context.Background(), params, strings.NewReader("\x12"), BufferOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := s.Finish(); err != nil {
				t.Fatal(err)
			}
		} else if err := s.Shape(context.Background(), params, strings.NewReader("\x12"), BufferOptions{}); !errors.Is(err, ErrClusterOverflow) {
			t.Fatalf("overflow: %v", err)
		}
		got := drainChannelStream(t, s)
		if len(got) < 2 || got[0].Cluster != math.MaxUint32-1 || got[1].Cluster != math.MaxUint32 {
			t.Fatalf("boundary output: %v", got)
		}
		if reset && (len(got) != 3 || got[2].Cluster != 0 || s.Err() != nil) {
			t.Fatalf("reset output: %v, %v", got, s.Err())
		}
		if !reset && !errors.Is(s.Err(), ErrClusterOverflow) {
			t.Fatalf("overflow not published: %v", s.Err())
		}
	}

	// Even if a combining scalar would share its predecessor's output cluster,
	// it cannot exceed the representable original-scalar range.
	s := newTestChannelStream(t, 2)
	if err := s.SetInitialCluster(math.MaxUint32); err != nil {
		t.Fatal(err)
	}
	if err := s.Shape(context.Background(), params, strings.NewReader("a\u0301"), BufferOptions{}); !errors.Is(err, ErrClusterOverflow) {
		t.Fatalf("merged source overflow: %v", err)
	}
	if got := drainChannelStream(t, s); len(got) != 0 {
		t.Fatalf("failing carry emitted: %v", got)
	}

	// Exercise the existing pipeline's run-local next-index limit without
	// having to read four billion scalars in a regression test.
	src := &channelRuneSource{ctx: context.Background(), src: strings.NewReader("a"), count: math.MaxUint32}
	if _, _, err := src.ReadRune(); !errors.Is(err, ErrClusterOverflow) {
		t.Fatalf("run-local counter overflow: %v", err)
	}
	src.src = strings.NewReader("")
	if _, _, err := src.ReadRune(); err != io.EOF {
		t.Fatalf("EOF at run-local limit: %v", err)
	}
}

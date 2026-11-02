package otshape

import (
	"errors"
	"fmt"
	"unicode"
)

const (
	// MinBuffer is the minimum raw-input capacity. Smaller positive requests are raised silently.
	MinBuffer = 4096
	// MaxInputGroup bounds an initial base/mark group, before normalization.
	MaxInputGroup = 256
	// MaxIntermediateEntries bounds each normalized-rune or glyph buffer. Storage grows on demand.
	MaxIntermediateEntries = 65536
)

var (
	ErrBufferConfig   = errors.New("otshape: invalid buffer configuration")
	ErrBufferFull     = errors.New("otshape: no safe streaming boundary before raw buffer capacity")
	ErrSupportedLimit = errors.New("otshape: supported shaping limit exceeded")
)

// LimitError describes a supported-size limit. Index is an original scalar
// index (the group start for input groups, the carry start for intermediates).
// It wraps ErrSupportedLimit and can be inspected with errors.As.
type LimitError struct {
	Resource string
	Limit    int
	Index    uint32
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%v: %s maximum %d at input index %d", ErrSupportedLimit, e.Resource, e.Limit, e.Index)
}
func (e *LimitError) Unwrap() error { return ErrSupportedLimit }

func (st *streamingState) checkInputRune(r rune) error {
	if st.groupCount == 0 || !(unicode.Is(unicode.M, r) || r == '\u200D') {
		st.groupStart, st.groupCount = st.nextCluster, 0
	}
	if st.groupCount == MaxInputGroup {
		return &LimitError{"input group", MaxInputGroup, st.groupStart}
	}
	st.groupCount++
	return nil
}
func (st *streamingState) fullError() error {
	return fmt.Errorf("%w: capacity %d, input index %d", ErrBufferFull, st.cfg.maxBuffer, st.rawClusters[0])
}

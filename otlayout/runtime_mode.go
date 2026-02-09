package otlayout

import "sync/atomic"

// LookupExecutionMode controls how lookup application resolves runtime payloads.
//
// ConcreteFirst tries concrete payloads first and falls back to legacy decoded
// support/index structures when needed.
//
// ConcreteOnly requires concrete payloads and disables legacy fallback.
type LookupExecutionMode uint32

const (
	ConcreteFirst LookupExecutionMode = iota
	ConcreteOnly
)

var lookupExecutionMode atomic.Uint32

func init() {
	lookupExecutionMode.Store(uint32(ConcreteFirst))
}

// SetLookupExecutionMode sets global lookup execution mode and returns previous mode.
func SetLookupExecutionMode(mode LookupExecutionMode) LookupExecutionMode {
	prev := LookupExecutionMode(lookupExecutionMode.Swap(uint32(mode)))
	return prev
}

// LookupMode returns the current lookup execution mode.
func LookupMode() LookupExecutionMode {
	return LookupExecutionMode(lookupExecutionMode.Load())
}

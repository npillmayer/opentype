package harfbuzz

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var ErrShaperAlreadyRegistered = errors.New("shaping engine already registered")

type shaperRegistration struct {
	prototype ShapingEngine
	order     int
}

type shaperRegistry struct {
	mu        sync.RWMutex
	entries   []shaperRegistration
	nextOrder int
}

func newShaperRegistry() *shaperRegistry {
	return &shaperRegistry{}
}

func (r *shaperRegistry) registerShaper(shaper ShapingEngine) error {
	if shaper == nil {
		return fmt.Errorf("cannot register nil shaping engine")
	}

	name := strings.TrimSpace(shaper.Name())
	if name == "" {
		return fmt.Errorf("cannot register shaping engine with empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.entries {
		if entry.prototype.Name() == name {
			return fmt.Errorf("%w: %q", ErrShaperAlreadyRegistered, name)
		}
	}

	r.entries = append(r.entries, shaperRegistration{
		prototype: shaper,
		order:     r.nextOrder,
	})
	r.nextOrder++
	return nil
}

func (r *shaperRegistry) clear() {
	r.mu.Lock()
	r.entries = nil
	r.nextOrder = 0
	r.mu.Unlock()
}

func (r *shaperRegistry) resolve(ctx SelectionContext) ShapingEngine {
	r.mu.RLock()
	var (
		best      ShapingEngine
		bestScore = -1
		bestName  string
		bestOrder = -1
	)
	for _, entry := range r.entries {
		score := entry.prototype.Match(ctx)
		if score < 0 {
			continue
		}

		name := entry.prototype.Name()
		if best == nil ||
			score > bestScore ||
			(score == bestScore && (name < bestName || (name == bestName && entry.order < bestOrder))) {
			best = entry.prototype
			bestScore = score
			bestName = name
			bestOrder = entry.order
		}
	}
	r.mu.RUnlock()

	if best == nil {
		return complexShaperDefault{}.New()
	}

	instance := best.New()
	if instance == nil {
		return complexShaperDefault{}.New()
	}
	return instance
}

var defaultShaperRegistry = newShaperRegistry()

func init() {
	for _, shaper := range builtInShapers() {
		if err := defaultShaperRegistry.registerShaper(shaper); err != nil {
			panic(err)
		}
	}
}

func builtInShapers() []ShapingEngine {
	return []ShapingEngine{
		NewDefaultShapingEngine(),
	}
}

func resolveShaperForContext(ctx SelectionContext) ShapingEngine {
	return defaultShaperRegistry.resolve(ctx)
}

func RegisterShaper(shaper ShapingEngine) error {
	return defaultShaperRegistry.registerShaper(shaper)
}

func ClearRegistry() {
	defaultShaperRegistry.clear()
}

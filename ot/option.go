package ot

// Option represents an optional value.
type Option[T any] struct {
	value T
	ok    bool
}

// Some constructs an Option with a value.
func Some[T any](v T) Option[T] {
	return Option[T]{value: v, ok: true}
}

// None constructs an empty Option.
func None[T any]() Option[T] {
	var zero T
	return Option[T]{value: zero, ok: false}
}

// IsSome reports whether the option contains a value.
func (o Option[T]) IsSome() bool {
	return o.ok
}

// IsNone reports whether the option is empty.
func (o Option[T]) IsNone() bool {
	return !o.ok
}

// Unwrap returns the value and a boolean indicating presence.
// This mirrors the common Go "(value, ok)" pattern.
func (o Option[T]) Unwrap() (T, bool) {
	return o.value, o.ok
}

// MustUnwrap returns the value or panics if None.
// Useful in tests or when invariants are guaranteed.
func (o Option[T]) MustUnwrap() T {
	if !o.ok {
		panic("option: unwrap of None")
	}
	return o.value
}

// Or returns the contained value or a default.
func (o Option[T]) Or(def T) T {
	if o.ok {
		return o.value
	}
	return def
}

// Map transforms the value if present.
func Map[T any, U any](o Option[T], f func(T) U) Option[U] {
	if o.ok {
		return Some(f(o.value))
	}
	return None[U]()
}

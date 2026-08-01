package htmlbind

import "context"

// Pending is one value the caller started before rendering and a template waits
// for in an await clause. It is the Go form of the template's `async T`.
//
// It settles once and stays readable, which is the whole reason it is a handle
// and not a channel: a layout and the page inside it may hold the same value,
// and a channel would deliver it to whichever of them received first while the
// other blocked until the boundary deadline. Every await of one handle sees the
// same settled result, and the work behind it runs once.
//
// The zero value is unset. Awaiting an unset handle is legal exactly where the
// template declared the awaited type optional, and yields an absent value
// rather than a failure; generation rejects an unset handle anywhere else
// before the response commits. Nothing about an unset handle panics or blocks.
//
// This is a render parameter type, not a general future. It has no combinators
// and no caller-facing wait: code that wants to compose work does so with
// ordinary goroutines before handing the result over.
type Pending[T any] struct {
	state *pendingState[T]
}

// pendingState is settled exactly once by whoever created it. Closing done is
// what publishes value and err to every reader, so readers need no lock.
type pendingState[T any] struct {
	done  chan struct{}
	value T
	err   error
}

// Go starts work in its own goroutine and returns the handle to pass as a
// template parameter. The work receives the caller's context, so bounding or
// cancelling it stays the caller's business; a render only bounds how long it
// waits.
//
// A panic in the work becomes the handle's error, the way an async external's
// panic does, so a boundary reports it through its recover clause instead of
// taking the process down.
func Go[T any](ctx context.Context, work func(context.Context) (T, error)) Pending[T] {
	state := &pendingState[T]{done: make(chan struct{})}
	go func() {
		// Registered first, so it runs last: the value and the error are both
		// final before any reader can observe them.
		defer close(state.done)
		defer func() {
			if recovered := recover(); recovered != nil {
				state.err = &panicError{value: recovered}
			}
		}()
		state.value, state.err = work(ctx)
	}()
	return Pending[T]{state: state}
}

// Resolved returns a handle that has already settled to value. It is what a
// caller passes when it computed the value itself, and what a test passes
// instead of starting a goroutine.
func Resolved[T any](value T) Pending[T] {
	state := &pendingState[T]{done: make(chan struct{}), value: value}
	close(state.done)
	return Pending[T]{state: state}
}

// Failed returns a handle that has already settled to err. The boundary that
// awaits it renders its recover subtree.
func Failed[T any](err error) Pending[T] {
	state := &pendingState[T]{done: make(chan struct{}), err: err}
	close(state.done)
	return Pending[T]{state: state}
}

// IsSet reports whether this handle carries work. Generated code calls it
// before the response commits, so a caller that forgot to supply a required
// value gets an error response rather than a boundary that waits for nothing.
func (p Pending[T]) IsSet() bool { return p.state != nil }

// Wait returns the settled value. ctx bounds the wait, never the work.
//
// An unset handle returns the zero value and no error, because absence is data
// rather than failure wherever the template allowed it. It is deliberately not
// a panic and deliberately not an infinite wait: the zero value of a struct
// field is exactly what a caller who supplied nothing left behind.
func (p Pending[T]) Wait(ctx context.Context) (T, error) {
	var zero T
	if p.state == nil {
		return zero, nil
	}
	// Checked first so an already settled handle reports its result even when
	// the context is cancelled in the same instant. Otherwise select would pick
	// between two ready cases at random and one render in two would discard a
	// value it already had.
	select {
	case <-p.state.done:
		return p.state.value, p.state.err
	default:
	}
	select {
	case <-p.state.done:
		return p.state.value, p.state.err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// UnsetPendingError reports a required async parameter or record field the
// caller left unset. It is raised during the initial pass, before any byte
// commits, so a handler can still turn it into an error response.
type UnsetPendingError struct {
	// Path names the parameter or field as the template declared it.
	Path string
}

func (e *UnsetPendingError) Error() string {
	return "htmlbind: async value " + e.Path + " was not set by the caller"
}

// ErrUnsetPending builds the error generated code raises for an unset required
// async value.
func ErrUnsetPending(path string) error { return &UnsetPendingError{Path: path} }

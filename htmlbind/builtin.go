package htmlbind

import (
	"context"
	"errors"
	"fmt"
)

// A builtin element is markup a framework registers with the generator, written
// by name in a template and rewritten at generation time. Most of one is fixed
// bytes, and generation folds those into the surrounding static run; what
// remains is the per-request part, and that is what this file carries.
//
// The reason it is a seam of its own rather than sugar over an external call is
// what does not happen: the value never enters template scope. An author writing
// <csrf-token/> cannot interpolate the token somewhere else, log it, or put it
// in a query string, because no name is ever bound to it. An external returning
// the same value hands it to the template, and everything a template may do with
// a value becomes possible.

// ErrNoRenderContext reports a render that reached a per-request builtin element
// with no context to read from.
var ErrNoRenderContext = errors.New("htmlbind: render needs a context")

// Segment is one piece of a builtin element's lowered markup: either fixed bytes
// or a hole filled from the component's parameters and the provider's result.
//
// A hole is escaped, always. Both of the positions a hole may occupy — element
// text and an attribute value — take the same escaping, so a provider returning
// a value with a quote in it cannot close the attribute it sits in. Generation
// refuses a hole in any other position rather than widening this rule.
type Segment[P, V any] struct {
	// Static is written as-is. It is the markup the definition declared, which
	// is generation-time input and never a request's.
	Static string
	// Hole produces one value. Nil means this segment is Static.
	Hole func(P, V) string
}

// Provide runs a per-request provider and writes one builtin element's markup,
// filling each hole from the result.
//
// The provider is called at most once per render, whatever the element's
// occurrence count, and the result is shared by every occurrence. That is a
// contract rather than an optimization: a token that reaches the browser twice —
// once in a response header for script to read, once in a hidden input for a form
// that must work without script — has to be the same token, and a header carries
// one value. Two forms on a page holding two different tokens is a bug nobody
// sees until one of them is submitted.
//
// The memo is keyed by the provider, so two elements backed by one function share
// one value; and it is scoped to one render, because that is the unit whose
// output has to agree with itself. A redraw and a live delivery are separate
// renders and call again, which is correct: each is a separate response with its
// own header.
//
// What this asks of a provider is that it be a read rather than a mint. See
// requirement:render-value-provider: a provider returns the value the session
// already has, so calling it once, twice, or never yields the same answer.
//
// An error from the provider ends the render. During the initial pass that is
// still before the response commits, so a caller can turn it into an error
// status rather than a half-written document; from a settled await boundary or a
// delta rerender it travels as any other member failure does.
func Provide[P, V any](element, provider string, fn func(context.Context) (V, error), segments []Segment[P, V]) Op[P] {
	return provideOp[P, V]{element: element, provider: provider, fn: fn, segments: segments}
}

type provideOp[P, V any] struct {
	element  string
	provider string
	fn       func(context.Context) (V, error)
	segments []Segment[P, V]
}

func (o provideOp[P, V]) Exec(r *Renderer, params P) error {
	ctx, ok := r.suppliedContext()
	if !ok {
		// Rendering to a buffer with no context is the ordinary way this
		// happens: a test, a mail body, a static export. Naming the element is
		// what makes it fixable, because the template that wrote it is the thing
		// to look at.
		return fmt.Errorf("%w: <%s> reads a per-request value; pass one through WithContext or an async entry", ErrNoRenderContext, o.element)
	}
	value, err := o.resolve(ctx, r)
	if err != nil {
		return fmt.Errorf("htmlbind: <%s> provider failed: %w", o.element, err)
	}
	for _, segment := range o.segments {
		if segment.Hole == nil {
			if err := r.Write(segment.Static); err != nil {
				return err
			}
			continue
		}
		if err := r.Write(Escape(segment.Hole(params, value))); err != nil {
			return err
		}
	}
	return nil
}

// resolve returns this provider's value for the render, calling it the first
// time and reusing it afterwards.
//
// A failure is not memoized. The render ends on it, so there is nothing left to
// share it with, and storing one would mean deciding whether a later occurrence
// should see a stale error.
func (o provideOp[P, V]) resolve(ctx context.Context, r *Renderer) (V, error) {
	if r.opts == nil {
		return o.fn(ctx)
	}
	r.opts.providedMu.Lock()
	defer r.opts.providedMu.Unlock()
	if stored, ok := r.opts.provided[o.provider]; ok {
		if value, ok := stored.(V); ok {
			return value, nil
		}
	}
	value, err := o.fn(ctx)
	if err != nil {
		return value, err
	}
	if r.opts.provided == nil {
		r.opts.provided = map[string]any{}
	}
	r.opts.provided[o.provider] = value
	return value, nil
}

// suppliedContext returns the context this render was given, and whether it was
// given one at all.
//
// It is deliberately not context(), which substitutes Background so a cache
// store and a blocking await have something to pass along. A per-request value
// read from Background is not a value; it is the absence of one, silently
// rendered, which is the failure this reports instead.
func (r *Renderer) suppliedContext() (context.Context, bool) {
	// The order is the one boundaryContext already sets: a boundary's own
	// context is the more specific one, so a provider inside a live delivery is
	// bounded by that delivery rather than by the whole response.
	if r.boundaryCtx != nil {
		return r.boundaryCtx, true
	}
	if r.async != nil && r.async.ctx != nil {
		return r.async.ctx, true
	}
	if r.opts != nil && r.opts.ctx != nil {
		return r.opts.ctx, true
	}
	return nil, false
}

// Vary reports the request properties this fragment's output depends on, such as
// a cookie or a header a builtin element's provider reads.
//
// It exists because nothing else says so. A component reading a cookie through a
// registered element makes the whole response vary on that cookie, and the
// template says nothing a caller could read: a caller cannot build a Vary header
// for a dependency it cannot see, and an output cache cannot refuse to store what
// it cannot key.
//
// The axes are declared by whoever registered the element, not derived, because
// only the implementation knows what its provider reads.
func (f Fragment) Vary() []string { return f.vary }

// Vary is the Wrapper form of the accessor documented on Fragment.
func (w Wrapper) Vary() []string { return w.vary }

// MergeVary is the union of a whole chain's vary axes, deduplicated and in
// composition order. It takes the same argument pair as MergeHead.
func MergeVary(wrappers []Wrapper, leaf Fragment) []string {
	var merged []string
	seen := map[string]bool{}
	add := func(axes []string) {
		for _, axis := range axes {
			if axis == "" || seen[axis] {
				continue
			}
			seen[axis] = true
			merged = append(merged, axis)
		}
	}
	for _, wrapper := range wrappers {
		add(wrapper.vary)
	}
	add(leaf.vary)
	return merged
}

// appendVary adds axes to a set, dropping ones already in it. The result is a
// fresh slice whenever anything is added, because the set it starts from is the
// plan's own and a plan is shared by every render.
func appendVary(vary, add []string) []string {
	if len(add) == 0 {
		return vary
	}
	seen := make(map[string]bool, len(vary))
	for _, axis := range vary {
		seen[axis] = true
	}
	grown := append([]string(nil), vary...)
	for _, axis := range add {
		if axis == "" || seen[axis] {
			continue
		}
		seen[axis] = true
		grown = append(grown, axis)
	}
	if len(grown) == len(vary) {
		return vary
	}
	return grown
}

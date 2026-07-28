package htmlbind

import (
	"context"
	"errors"
	"io"
	"iter"
	"time"
)

// Wrapper is a component that renders another one into its unnamed slot.
// Generated code returns one from Bind<Name> for a component with a children
// parameter.
type Wrapper struct {
	head     []string
	hasAwait bool
	validate func() error
	render   func(*Renderer, Fragment) error
}

// Validate runs the wrapper's parameter check without rendering.
func (w Wrapper) Validate() error {
	if w.validate == nil {
		return nil
	}
	return w.validate()
}

// BindWrapper pairs a plan with parameters and the setter that installs the
// child fragment. Generated code supplies the setter because only it knows
// which field the unnamed slot binds to.
func BindWrapper[P any](plan *Plan[P], params P, setChildren func(*P, Fragment)) Wrapper {
	wrapper := Wrapper{
		head:     plan.Head,
		hasAwait: plan.HasAwaitBlock,
		render: func(r *Renderer, children Fragment) error {
			local := params
			setChildren(&local, children)
			return plan.Exec(r, local)
		},
	}
	if plan.Check != nil {
		// The children field is still unset here, and a check never reads it:
		// a slot is a continuation the wrapper renders, not a value it waits
		// for.
		wrapper.validate = func() error { return plan.Check(params) }
	}
	return wrapper
}

// Head returns the wrapper's own head contributions.
func (w Wrapper) Head() []string { return w.head }

// HasAwaitBlock reports whether rendering this wrapper can open an await
// boundary. The child it wraps is counted separately, because a wrapper is
// bound before it is told what it wraps.
func (w Wrapper) HasAwaitBlock() bool { return w.hasAwait }

// ErrNoLeaf reports a chain assembled without an innermost component.
var ErrNoLeaf = errors.New("htmlbind: chain needs a leaf component")

// ErrNilWrapper reports a wrapper that was left unset.
var ErrNilWrapper = errors.New("htmlbind: chain contains an unset wrapper")

// Option configures one render. Options are variadic so a call that needs
// nothing beyond a writer and a component stays two arguments long.
type Option func(*renderOptions)

type renderOptions struct {
	ctx         context.Context
	cache       CacheStore
	report      func(error)
	timeout     time.Duration
	concurrency int
}

// WithCache supplies the store used by components declared with the cache
// annotation. Without it those components render normally, so caching is a
// deployment choice rather than a template rewrite.
//
// The store belongs to the caller: passing it per render keeps two servers in
// one process from sharing entries through package state.
func WithCache(store CacheStore) Option {
	return func(o *renderOptions) { o.cache = store }
}

// WithContext supplies the context for the synchronous entries, which take no
// ctx parameter of their own. A cache store and a blocking await boundary both
// use it, so a cancelled request stops doing work. The async entries take their
// context directly and ignore this option.
func WithContext(ctx context.Context) Option {
	return func(o *renderOptions) { o.ctx = ctx }
}

// WithErrorReporter receives the original Go error behind every await boundary
// failure, including failures a recover clause handled and therefore never
// surfaced to the caller. Recover subtrees see only the safe AsyncError, so this
// is where logging and metrics attach.
//
// It is called from each boundary's own goroutine, so a reporter that
// accumulates rather than logs has to guard its own state.
func WithErrorReporter(report func(error)) Option {
	return func(o *renderOptions) { o.report = report }
}

// WithAsyncTimeout bounds how long one await boundary's bindings may run. An
// expired boundary fails with ErrorCodeTimeout. Zero means no deadline beyond
// the request context.
func WithAsyncTimeout(timeout time.Duration) Option {
	return func(o *renderOptions) { o.timeout = timeout }
}

// WithConcurrencyLimit bounds how many await boundaries may have work running
// at once across the whole render. Zero or less means unbounded.
func WithConcurrencyLimit(limit int) Option {
	return func(o *renderOptions) { o.concurrency = limit }
}

func newRenderOptions(options []Option) *renderOptions {
	resolved := &renderOptions{}
	for _, option := range options {
		if option != nil {
			option(resolved)
		}
	}
	return resolved
}

// Render writes one component to w.
func Render(w io.Writer, leaf Fragment, options ...Option) error {
	return RenderChain(w, nil, leaf, options...)
}

// RenderChain writes a composed document to w. Wrappers apply outermost first,
// so RenderChain(w, []Wrapper{document, layout}, page) renders page inside
// layout inside document. An empty wrapper list renders the leaf alone.
//
// Head contributions are merged before the first byte is written, so the shell
// can emit them without buffering the body. Assembly is validated up front, so
// a malformed chain cannot leave a partial response behind.
//
// An await boundary reached on this path blocks and emits its settled subtree
// in place, so one template renders correctly with or without progressive
// delivery. Use RenderChainAsync to send fallbacks first instead.
//
// Bindings that fail in a clause with no recover subtree return an
// UnrecoveredError rather than writing the fallback, because a finished document
// holding a loading state is a document that lies. This path writes as it goes,
// so render into a buffer when you want that failure to become an error status.
func RenderChain(w io.Writer, wrappers []Wrapper, leaf Fragment, options ...Option) error {
	composed, head, err := assemble(wrappers, leaf)
	if err != nil {
		return err
	}
	return composed(&Renderer{w: w, head: head, opts: newRenderOptions(options)})
}

// RenderAsync renders one component and yields each await boundary as it
// settles.
func RenderAsync(ctx context.Context, w io.Writer, leaf Fragment, options ...Option) iter.Seq2[Content, error] {
	return RenderChainAsync(ctx, w, nil, leaf, options...)
}

// RenderChainAsync renders a composed document to w and yields one Content per
// settled await boundary, in completion order.
//
// Rendering starts on the first pull. The initial pass writes the document with
// every boundary's fallback inside its placeholder and flushes, so a slow
// dependency does not delay the first bytes. Each later item is the replacement
// for one placeholder, and the ranging caller writes it, because only the
// caller may touch the response:
//
//	for content, err := range htmlbind.RenderChainAsync(ctx, w, wrappers, page) {
//		if err != nil {
//			log.Printf("render failed: %v", err)
//			break
//		}
//		if err := writeCompletion(w, content); err != nil {
//			break
//		}
//		htmlbind.Flush(w)
//	}
//
// A yielded item is the bare fragment plus the id of the placeholder it belongs
// to. How that pair travels — an inert template and a marker element, a JSON
// record, anything else — is the caller's choice, because it has to match the
// client runtime the caller ships. Nothing on this path writes script, and the
// merged head carries component contributions only.
//
// There is no variant that hides this loop. How many boundaries a render
// produces is not knowable up front, least of all for a chain assembled at
// request time, so a handler that streams has to be written against the
// sequence anyway.
//
// Once the initial pass commits, the response status can no longer change, so a
// later error is for logging rather than for rewriting the response.
//
// One of those errors is UnrecoveredError: a boundary's bindings failed in a
// clause with no recover subtree. Ending the sequence there is the point, since
// the alternative is a page left showing a fallback that will never be replaced.
// The caller still owns the response, and what it writes next — an error screen
// replacing the document, whatever its runtime applies — is its own to frame.
//
// The sequence is single-use and single-consumer. Stopping the range early ends
// the render without waiting for the outstanding boundaries.
func RenderChainAsync(ctx context.Context, w io.Writer, wrappers []Wrapper, leaf Fragment, options ...Option) iter.Seq2[Content, error] {
	return func(yield func(Content, error) bool) {
		composed, head, err := assemble(wrappers, leaf)
		if err != nil {
			yield(Content{}, err)
			return
		}
		coordinator := newAsyncCoordinator(ctx, newRenderOptions(options))
		defer coordinator.stop()
		// The head carries component contributions only. Nothing is injected on
		// this path: the script that applies a completion belongs with the framing
		// the caller writes around it, so both are the framework's to ship.
		renderer := &Renderer{w: w, head: head, opts: coordinator.opts, async: coordinator}
		if err := composed(renderer); err != nil {
			yield(Content{}, err)
			return
		}
		flush(w)
		coordinator.wait()
		for {
			select {
			case result, open := <-coordinator.results:
				if !open {
					return
				}
				if result.err != nil {
					yield(Content{}, result.err)
					return
				}
				if !yield(result.content, nil) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

// assemble validates the chain and returns the composed render entry together
// with the merged head, before anything is written.
func assemble(wrappers []Wrapper, leaf Fragment) (func(*Renderer) error, []string, error) {
	if !leaf.Present() {
		return nil, nil, ErrNoLeaf
	}
	for _, wrapper := range wrappers {
		if wrapper.render == nil {
			return nil, nil, ErrNilWrapper
		}
	}
	// Parameter checks run here, with nothing written yet, so a caller that
	// left a required async value unset can still send an error response.
	if err := leaf.Validate(); err != nil {
		return nil, nil, err
	}
	for _, wrapper := range wrappers {
		if err := wrapper.Validate(); err != nil {
			return nil, nil, err
		}
	}
	head := MergeHead(wrappers, leaf)
	next := leaf
	for i := len(wrappers) - 1; i >= 0; i-- {
		wrapper, inner := wrappers[i], next
		next = Fragment{
			hasAwait: wrapper.hasAwait || inner.hasAwait,
			render:   func(r *Renderer) error { return wrapper.render(r, inner) },
		}
	}
	return next.render, head, nil
}

// Flush pushes buffered bytes toward the client when the writer can. io.Writer
// has no flush, so the capability is discovered by interface assertion rather
// than reflection, and a writer without it still produces correct output, only
// without progressive delivery.
//
// The render entries flush after the initial pass. Call this after writing each
// Content, because a completion sitting in a buffer defeats the point of having
// sent it early.
func Flush(w io.Writer) { flush(w) }

func flush(w io.Writer) {
	if flusher, ok := w.(interface{ Flush() }); ok {
		flusher.Flush()
		return
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	}
}

// MergeHead collects head contributions in composition order, outermost first,
// dropping later duplicates so two components declaring the same stylesheet
// emit one tag.
func MergeHead(wrappers []Wrapper, leaf Fragment) []string {
	var merged []string
	seen := map[string]bool{}
	add := func(tags []string) {
		for _, tag := range tags {
			if tag == "" || seen[tag] {
				continue
			}
			seen[tag] = true
			merged = append(merged, tag)
		}
	}
	for _, wrapper := range wrappers {
		add(wrapper.head)
	}
	add(leaf.head)
	return merged
}

// HasAwaitBlock reports whether any member of a chain can open an await
// boundary. It answers the one question a caller has to settle before
// rendering: whether this response needs the client runtime that applies
// settled boundaries.
//
// Ask once for the whole chain rather than per member, so a chain whose layout
// and page both await still contributes one runtime script.
func HasAwaitBlock(wrappers []Wrapper, leaf Fragment) bool {
	for _, wrapper := range wrappers {
		if wrapper.hasAwait {
			return true
		}
	}
	return leaf.hasAwait
}

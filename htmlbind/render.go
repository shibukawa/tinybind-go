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
	head   []string
	render func(*Renderer, Fragment) error
}

// BindWrapper pairs a plan with parameters and the setter that installs the
// child fragment. Generated code supplies the setter because only it knows
// which field the unnamed slot binds to.
func BindWrapper[P any](plan *Plan[P], params P, setChildren func(*P, Fragment)) Wrapper {
	return Wrapper{
		head: plan.Head,
		render: func(r *Renderer, children Fragment) error {
			local := params
			setChildren(&local, children)
			return plan.Exec(r, local)
		},
	}
}

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
// failure, including failures a recover clause handled and failures a clause
// without recover left as a committed fallback. Recover subtrees see only the
// safe AsyncError, so this is where logging and metrics attach.
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
//		if _, err := content.WriteTo(w); err != nil {
//			break
//		}
//		htmlbind.Flush(w)
//	}
//
// There is no variant that hides this loop. How many boundaries a render
// produces is not knowable up front, least of all for a chain assembled at
// request time, so a handler that streams has to be written against the
// sequence anyway.
//
// Once the initial pass commits, the response status can no longer change, so a
// later error is for logging rather than for rewriting the response.
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
		// The update runtime is fixed trusted code belonging to the render mode
		// rather than to any component, so it joins the merged head here. A
		// document with no shell head simply keeps its fallbacks.
		merged := append([]string{boundaryRuntime}, head...)
		renderer := &Renderer{w: w, head: merged, opts: coordinator.opts, async: coordinator}
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
	head := MergeHead(wrappers, leaf)
	next := leaf
	for i := len(wrappers) - 1; i >= 0; i-- {
		wrapper, inner := wrappers[i], next
		next = Fragment{render: func(r *Renderer) error { return wrapper.render(r, inner) }}
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

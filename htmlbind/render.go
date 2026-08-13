package htmlbind

import (
	"context"
	"errors"
	"io"
	"iter"
	"sync"
	"time"
)

// Wrapper is a component that renders another one into its unnamed slot.
// Generated code returns one from Bind<Name> for a component with a children
// parameter.
type Wrapper struct {
	head            []string
	headSources     []string
	assets          []Asset
	vary            []string
	boundary        *boundary
	hasAwait        bool
	hasLive         bool
	declaresPrivate bool
	declaresPublic  bool
	privateSource   string
	validate        func() error
	render          func(*Renderer, Fragment) error
	// sequence derives this component's static half, as it does on a Fragment.
	sequence func() *Sequence
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
//
// Deprecated: use the BindWrapper method on Plan. It carries no type parameter
// beyond the receiver's own, so the method form was always available; this
// function remains so no generated or hand-written caller is forced to move.
func BindWrapper[P any](plan *Plan[P], params P, setChildren func(*P, Fragment)) Wrapper {
	return plan.BindWrapper(params, setChildren)
}

// BindWrapper pairs a plan with parameters and the setter that installs the
// child fragment. Generated code supplies the setter because only it knows
// which field the unnamed slot binds to.
func (plan *Plan[P]) BindWrapper(params P, setChildren func(*P, Fragment)) Wrapper {
	wrapper := Wrapper{
		head:            plan.Head,
		headSources:     plan.HeadSources,
		assets:          plan.Assets,
		vary:            plan.Vary,
		boundary:        bindBoundary(plan.Boundary, params),
		hasAwait:        plan.HasAwaitBlock,
		hasLive:         plan.HasLiveBlock,
		declaresPrivate: plan.DeclaresPrivate,
		declaresPublic:  plan.DeclaresPublic,
		privateSource:   plan.PrivateSource,
		render: func(r *Renderer, children Fragment) error {
			local := params
			setChildren(&local, children)
			return plan.Exec(r, local)
		},
		sequence: plan.Sequence,
	}
	if plan.Check != nil {
		// The children field is still unset here, and a check never reads it:
		// a slot is a continuation the wrapper renders, not a value it waits
		// for.
		wrapper.validate = func() error { return plan.Check(params) }
	}
	// A wrapper's own named slots are already filled; its unnamed one is not,
	// and it contributes nothing until it is, which is correct because the chain
	// merges the child separately.
	folded := foldSlots(Fragment{
		head: wrapper.head, headSources: wrapper.headSources, assets: wrapper.assets,
		vary: wrapper.vary, hasAwait: wrapper.hasAwait, hasLive: wrapper.hasLive,
		declaresPrivate: wrapper.declaresPrivate, privateSource: wrapper.privateSource,
	}, plan.Slots, params)
	wrapper.head, wrapper.headSources, wrapper.assets = folded.head, folded.headSources, folded.assets
	wrapper.vary = folded.vary
	wrapper.hasAwait, wrapper.hasLive = folded.hasAwait, folded.hasLive
	wrapper.declaresPrivate, wrapper.privateSource = folded.declaresPrivate, folded.privateSource
	return wrapper
}

// Head returns the wrapper's own head contributions, one entry per tag.
func (w Wrapper) Head() []string { return w.head }

// HeadSources names the component that declared each Head entry, in the same
// order and with the same length. It is the Wrapper form of the accessor
// documented on Fragment.
func (w Wrapper) HeadSources() []string { return w.headSources }

// HasAwaitBlock reports whether rendering this wrapper can open an await
// boundary. The child it wraps is counted separately, because a wrapper is
// bound before it is told what it wraps.
func (w Wrapper) HasAwaitBlock() bool { return w.hasAwait }

// HasLiveBlock reports whether rendering this wrapper can open a live boundary.
// It is the Wrapper form of the accessor documented on Fragment.
func (w Wrapper) HasLiveBlock() bool { return w.hasLive }

// ErrNoLeaf reports a chain assembled without an innermost component.
var ErrNoLeaf = errors.New("htmlbind: chain needs a leaf component")

// ErrNilWrapper reports a wrapper that was left unset.
var ErrNilWrapper = errors.New("htmlbind: chain contains an unset wrapper")

// Option configures one render. Options are variadic so a call that needs
// nothing beyond a writer and a component stays two arguments long.
type Option func(*renderOptions)

type renderOptions struct {
	ctx   context.Context
	cache CacheStore
	// cacheScope prefixes the key of every component declared private, so one
	// key yields a separate entry per scope. It is opaque here: the caller
	// supplies whatever identifies the reader, and this package never learns
	// what it means.
	cacheScope  string
	report      func(error)
	timeout     time.Duration
	concurrency int
	// head holds the caller's own contributions, merged after the components'.
	head []HeadNode
	// live keeps live subscriptions open instead of taking one delivery and
	// unsubscribing. Only the live render entries set it.
	live bool
	// boundaryPrefix names the placeholder element and the boundary
	// identifiers. Empty means DefaultBoundaryPrefix.
	boundaryPrefix string
	// validatorTag seeds every digest this render produces. Empty is allowed and
	// means the digests are seeded by the key alone.
	validatorTag string
	// provided memoizes each builtin element provider's result for the whole of
	// this render, keyed by the provider rather than by the element, so two
	// elements backed by one function share one value.
	//
	// It lives here because this value is already the one thing every renderer in
	// a render shares — a buffered subtree, a boundary subtree, and a cached
	// component all carry the same pointer — so memoizing costs no allocation
	// beyond the map itself, and none at all for a render that reaches no
	// provider.
	//
	// The lock is not decorative: await boundaries render in their own
	// goroutines, and two of them may each hold the same element.
	providedMu sync.Mutex
	provided   map[string]any
	// csrf is the session token every unsafe form in this render writes.
	// csrfSupplied separates an absent option from a supplied empty string, and
	// csrfOmitted is the explicit statement that this render has no session.
	csrf         string
	csrfSupplied bool
	csrfOmitted  bool
	// urlSchemes and dataURLMediaTypes hold the scheme policy a URL-bearing
	// attribute renders under. The paired booleans separate an unset option
	// from one deliberately set to nothing, because permitting no scheme at all
	// is a legitimate policy and must not read as "use the defaults".
	urlSchemes           []string
	urlSchemesSet        bool
	dataURLMediaTypes    []string
	dataURLMediaTypesSet bool
}

// DefaultBoundaryPrefix names the placeholder element a progressive render
// writes and the identifiers it allocates: <tb-boundary id="tb-1">.
//
// It matches the generator's default data-attribute prefix, because a document
// carrying data-tb-id on its boundaries and <tb-boundary> placeholders is one
// naming system rather than two.
const DefaultBoundaryPrefix = "tb"

// WithBoundaryPrefix renames the placeholder element and the boundary
// identifiers, so a framework's markup carries the framework's own prefix.
//
// It must be the prefix the generator wrote the instance attributes with. A
// document naming its attributes data-pw-id and its placeholders <tb-boundary>
// has two naming systems in it, only one of which anything can configure.
//
// The value must be a valid custom element name prefix: lowercase letters and
// digits, starting with a letter, with no leading or trailing hyphen. Anything
// else produces markup a browser will not parse as an element.
func WithBoundaryPrefix(prefix string) Option {
	return func(o *renderOptions) { o.boundaryPrefix = prefix }
}

// WithValidatorTag seeds every validator this render produces, so two renders
// that must never be compared cannot produce equal digests.
//
// The transport half passes its build identity. That is the axis that actually
// moves: it covers a changed template, a changed Go function a template calls,
// and a changed browser client, none of which a component's own identity sees.
//
// It replaced a protocol version this module owned. Once the browser client
// belongs to the caller, a version constant here versions a contract this module
// only half implements, and the caller is the party that can say what a mismatch
// means. Leaving it empty is allowed and keeps the digests keyed by Key alone,
// which is what a caller comparing renders within one build wants.
func WithValidatorTag(tag string) Option {
	return func(o *renderOptions) { o.validatorTag = tag }
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

// WithCacheScope supplies the value prefixed to the key of every component
// declared private, so the same parameters yield a separate entry per scope.
//
// The value is opaque: pass whatever identifies the reader a private entry
// belongs to, and this package never interprets it. It is framed into the key
// like any other value, so a scope value cannot spell out another key.
//
// Without it a private component stores nothing. That is deliberate: an entry
// written under an empty scope is a shared entry wearing a private label, and a
// miss is preferable to serving one reader's output to the next.
func WithCacheScope(scope string) Option {
	return func(o *renderOptions) { o.cacheScope = scope }
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
	opts := newRenderOptions(options)
	composed, head, err := assemble(nil, wrappers, leaf)
	if err != nil {
		return err
	}
	head, err = mergeCallerHead(head, opts.head)
	if err != nil {
		return err
	}
	return composed(&Renderer{w: w, sw: stringWriterOf(w), head: head, opts: opts})
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
	return renderStreaming(ctx, w, wrappers, leaf, options)
}

// renderStreaming is the body shared by the async and live entries. The only
// difference between them is whether the options keep live boundaries
// subscribed, which the coordinator reads.
func renderStreaming(ctx context.Context, w io.Writer, wrappers []Wrapper, leaf Fragment, options []Option) iter.Seq2[Content, error] {
	return streamChain(ctx, w, nil, nil, wrappers, leaf, options)
}

// streamChain is the body behind every streaming entry, with or without a
// collector attached. rendered runs once the initial pass has committed, so a
// caller comparing renders can speak before the first completion; returning
// false ends the sequence.
func streamChain(ctx context.Context, w io.Writer, collect Collector, rendered func() bool, wrappers []Wrapper, leaf Fragment, options []Option) iter.Seq2[Content, error] {
	return func(yield func(Content, error) bool) {
		coordinator := newAsyncCoordinator(ctx, newRenderOptions(options))
		defer coordinator.stop()
		if collect != nil {
			collect.Begin(coordinator.opts.validatorTag)
		}
		composed, head, err := assemble(collect, wrappers, leaf)
		if err != nil {
			yield(Content{}, err)
			return
		}
		// The caller's own contributions join the merge here, before the head is
		// written, so a malformed one still fails while the status can change.
		head, err = mergeCallerHead(head, coordinator.opts.head)
		if err != nil {
			yield(Content{}, err)
			return
		}
		// The head carries component contributions only. Nothing is injected on
		// this path: the script that applies a completion belongs with the framing
		// the caller writes around it, so both are the framework's to ship.
		renderer := &Renderer{w: w, sw: stringWriterOf(w), head: head, opts: coordinator.opts, async: coordinator, collect: collect}
		if err := composed(renderer); err != nil {
			yield(Content{}, err)
			return
		}
		flush(w)
		if rendered != nil && !rendered() {
			return
		}
		coordinator.wait()
		for {
			select {
			case result, open := <-coordinator.results:
				if !open {
					return
				}
				if result.signal != nil {
					// A signal shares the error position with a failure and is
					// the one value there the sequence does not end on. The
					// caller separates them with AsSignal.
					if !yield(Content{}, *result.signal) {
						return
					}
					continue
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
//
// A collecting render composes through memberFragment instead, so each member
// declaring a boundary opens one around its own output. The chain index is the
// instance identity, assigned before rendering, which is why an unchanged
// chain shape yields comparable manifests even when parameters change.
func assemble(collect Collector, wrappers []Wrapper, leaf Fragment) (func(*Renderer) error, []string, error) {
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
	if collect == nil {
		next := leaf
		for i := len(wrappers) - 1; i >= 0; i-- {
			wrapper, inner := wrappers[i], next
			next = Fragment{
				hasAwait: wrapper.hasAwait || inner.hasAwait,
				hasLive:  wrapper.hasLive || inner.hasLive,
				render:   func(r *Renderer) error { return wrapper.render(r, inner) },
			}
		}
		return next.render, head, nil
	}
	next := memberFragment(leaf, leaf.boundary, len(wrappers))
	for i := len(wrappers) - 1; i >= 0; i-- {
		wrapper, inner, index := wrappers[i], next, i
		child := Fragment{
			render:   func(r *Renderer) error { return wrapper.render(r, inner) },
			sequence: wrapper.sequence,
		}
		next = memberFragment(child, wrapper.boundary, index)
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
	// One contributor has nothing to deduplicate against, so its slice is the
	// answer; merged results are treated as immutable everywhere they travel.
	single, contributors := leaf.head, 0
	if len(leaf.head) > 0 {
		contributors = 1
	}
	for _, wrapper := range wrappers {
		if len(wrapper.head) > 0 {
			single = wrapper.head
			contributors++
		}
	}
	if contributors == 0 {
		return nil
	}
	if contributors == 1 {
		return single
	}
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

// ChainHead returns the merged head a render of this chain would write: every
// member's contributions plus the caller's own, deduplicated in composition
// order. It renders nothing, so a caller that sends the head ahead of any
// markup — a streamed delta does — can know it first.
func ChainHead(wrappers []Wrapper, leaf Fragment, options ...Option) ([]string, error) {
	return mergeCallerHead(MergeHead(wrappers, leaf), newRenderOptions(options).head)
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

// HasLiveBlock reports whether any member of a chain can open a live boundary.
//
// It answers a different question from HasAwaitBlock. That one decides whether
// this response needs the client runtime that applies boundaries at all; this
// one decides whether the screen will keep changing once the document has
// finished, and so whether a live request is worth issuing. A document with
// await boundaries and no live ones is complete when its sequence ends.
func HasLiveBlock(wrappers []Wrapper, leaf Fragment) bool {
	for _, wrapper := range wrappers {
		if wrapper.hasLive {
			return true
		}
	}
	return leaf.hasLive
}

// WithLiveSubscriptions keeps live boundaries subscribed instead of taking one
// delivery and unsubscribing. RenderChainLive sets it; it is exported so a
// caller assembling its own options can be explicit about which behaviour it is
// asking for.
func WithLiveSubscriptions() Option {
	return func(o *renderOptions) { o.live = true }
}

// RenderLive renders one component and keeps yielding its live boundaries'
// deliveries.
func RenderLive(ctx context.Context, w io.Writer, leaf Fragment, options ...Option) iter.Seq2[Content, error] {
	return RenderChainLive(ctx, w, nil, leaf, options...)
}

// RenderChainLive renders a composed document to w and yields one Content per
// delivery, for as long as the live boundaries keep producing.
//
// It is RenderChainAsync with one difference: a live boundary stays subscribed.
// The returned sequence therefore does not end when the await boundaries have
// settled. It ends when every live source has ended, when the consumer stops
// ranging, or when ctx is cancelled — which is the shape a screen that updates
// on the server's clock needs, and the reason the entry is named for the
// subscription rather than for the transport carrying it.
//
// Pass io.Discard as w to run the render for its deliveries alone. The document
// bytes are still produced, because evaluating a live clause's source arguments
// means executing the component that holds them, but nothing is transferred.
// Boundary ids are allocated in render order, so the same chain rendered again
// for the same request produces the same ids as the document render did, and a
// client can address the placeholders already on its screen without being told
// what they are.
//
// Everything else matches RenderChainAsync: rendering starts on the first pull,
// only the ranging caller writes the response, and the framing around each
// Content is the caller's to choose. Deliveries from two boundaries interleave
// in completion order and carry no ordering guarantee between them.
//
// A source that keeps producing while nobody reads is not a problem here: the
// sequence pulls, so the source blocks in its own yield until this boundary is
// ready for the next value. A fast source misses ticks rather than filling a
// queue.
//
// The sequence is single-use and single-consumer.
func RenderChainLive(ctx context.Context, w io.Writer, wrappers []Wrapper, leaf Fragment, options ...Option) iter.Seq2[Content, error] {
	return renderStreaming(ctx, w, wrappers, leaf, append([]Option{WithLiveSubscriptions()}, options...))
}

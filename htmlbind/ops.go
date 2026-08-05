package htmlbind

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"
)

// Builder constructs instructions for one component. Generated code declares
// one per component so the parameter type is written once instead of on every
// instruction.
type Builder[P any] struct{}

// Static writes literal markup. Adjacent literal output is coalesced at
// generation time, so one instruction covers a whole run.
func (Builder[P]) Static(text string) Op[P] { return staticOp[P](text) }

type staticOp[P any] string

func (o staticOp[P]) Exec(r *Renderer, _ P) error { return r.Write(string(o)) }

// Text writes a value into child-node or attribute position with HTML escaping.
func (Builder[P]) Text(value func(P) string) Op[P] { return textOp[P]{value: value} }

type textOp[P any] struct{ value func(P) string }

func (o textOp[P]) Exec(r *Renderer, params P) error { return r.WriteEscaped(o.value(params)) }

// Raw writes a value that the template already marked trusted for its context.
func (Builder[P]) Raw(value func(P) string) Op[P] { return rawOp[P]{value: value} }

type rawOp[P any] struct{ value func(P) string }

func (o rawOp[P]) Exec(r *Renderer, params P) error { return r.Write(o.value(params)) }

// The Ctx forms below are the same instructions with one more argument: the
// context this render runs under. Generation emits one only where a template
// expression calls an external whose Go implementation declared a leading
// context.Context, so a project using none produces exactly the instructions it
// produced before these existed.
//
// The context is the render's own — the ctx argument of an async entry, or the
// value WithContext supplied to a synchronous one — read at the position the
// instruction occupies. Inside a boundary subtree that is the boundary's
// context, so a live delivery's work is bounded by that delivery.

// TextCtx is Text for a value that needs the render context.
func (Builder[P]) TextCtx(value func(context.Context, P) string) Op[P] {
	return textCtxOp[P]{value: value}
}

type textCtxOp[P any] struct {
	value func(context.Context, P) string
}

func (o textCtxOp[P]) Exec(r *Renderer, params P) error {
	return r.WriteEscaped(o.value(r.boundaryContext(), params))
}

// RawCtx is Raw for a value that needs the render context.
func (Builder[P]) RawCtx(value func(context.Context, P) string) Op[P] {
	return rawCtxOp[P]{value: value}
}

type rawCtxOp[P any] struct {
	value func(context.Context, P) string
}

func (o rawCtxOp[P]) Exec(r *Renderer, params P) error {
	return r.Write(o.value(r.boundaryContext(), params))
}

// Attr writes one attribute. The value arrives already escaped, because a
// mixed value concatenates author literals with escaped expressions and only
// the expressions may be escaped. present reports whether an optional value
// exists; an absent value omits the whole attribute.
//
// The attribute instructions all precompute their ` name="` prefix here, where
// the plan is built once, so a render concatenates nothing.
func (Builder[P]) Attr(name string, value func(P) (string, bool)) Op[P] {
	return attrOp[P]{prefix: attrPrefix(name), value: value}
}

func attrPrefix(name string) string { return " " + name + `="` }

type attrOp[P any] struct {
	prefix string
	value  func(P) (string, bool)
}

func (o attrOp[P]) Exec(r *Renderer, params P) error {
	value, present := o.value(params)
	if !present {
		return nil
	}
	return r.writeAttr(o.prefix, value)
}

// URLAttr writes an attribute a browser resolves as a URL, applying this
// render's scheme policy before escaping the value.
//
// It exists rather than a wider Escape because the policy is a render option
// and an Attr value function receives only the parameters, never the renderer.
// Exec does receive it, so the check lives here — the same reason AttrCtx
// reaches the boundary context from Exec instead of from its closure.
//
// value returns the assembled attribute text unescaped, because the scheme has
// to be read before the ampersands and quotes are encoded.
func (Builder[P]) URLAttr(name string, value func(P) (string, bool)) Op[P] {
	return urlAttrOp[P]{prefix: attrPrefix(name), value: value}
}

type urlAttrOp[P any] struct {
	prefix string
	value  func(P) (string, bool)
}

func (o urlAttrOp[P]) Exec(r *Renderer, params P) error {
	value, present := o.value(params)
	if !present {
		return nil
	}
	return r.writeAttrEscaped(o.prefix, r.opts.safeURL(value))
}

// URLAttrCtx is URLAttr for a value that needs the render context.
func (Builder[P]) URLAttrCtx(name string, value func(context.Context, P) (string, bool)) Op[P] {
	return urlAttrCtxOp[P]{prefix: attrPrefix(name), value: value}
}

type urlAttrCtxOp[P any] struct {
	prefix string
	value  func(context.Context, P) (string, bool)
}

func (o urlAttrCtxOp[P]) Exec(r *Renderer, params P) error {
	value, present := o.value(r.boundaryContext(), params)
	if !present {
		return nil
	}
	return r.writeAttrEscaped(o.prefix, r.opts.safeURL(value))
}

// URLListAttr writes an attribute holding several URLs, applying the scheme
// policy to each one and keeping the rest when one is refused.
//
// srcset names the comma-separated form whose entries carry a descriptor, and
// ping the whitespace-separated form. Dropping only the refused entry matters
// because these are lists of alternatives: refusing the whole attribute would
// turn one hostile candidate into a missing image.
func (Builder[P]) URLListAttr(name, shape string, value func(P) (string, bool)) Op[P] {
	return urlListAttrOp[P]{prefix: attrPrefix(name), shape: shape, value: value}
}

type urlListAttrOp[P any] struct {
	prefix string
	shape  string
	value  func(P) (string, bool)
}

func (o urlListAttrOp[P]) Exec(r *Renderer, params P) error {
	value, present := o.value(params)
	if !present {
		return nil
	}
	return r.writeAttrEscaped(o.prefix, safeURLList(r.opts, o.shape, value))
}

// URLListAttrCtx is URLListAttr for a value that needs the render context.
func (Builder[P]) URLListAttrCtx(name, shape string, value func(context.Context, P) (string, bool)) Op[P] {
	return urlListAttrCtxOp[P]{prefix: attrPrefix(name), shape: shape, value: value}
}

type urlListAttrCtxOp[P any] struct {
	prefix string
	shape  string
	value  func(context.Context, P) (string, bool)
}

func (o urlListAttrCtxOp[P]) Exec(r *Renderer, params P) error {
	value, present := o.value(r.boundaryContext(), params)
	if !present {
		return nil
	}
	return r.writeAttrEscaped(o.prefix, safeURLList(r.opts, o.shape, value))
}

// URLListSrcset and URLListSpace name the two list grammars URLListAttr reads.
// They travel as strings because they appear in generated source, where a
// named constant reads better than a bare true or false.
const (
	URLListSrcset = "srcset"
	URLListSpace  = "space"
)

func safeURLList(opts *renderOptions, shape, value string) string {
	if shape == URLListSrcset {
		return opts.safeSrcsetURLs(value)
	}
	return opts.safeSpaceURLs(value)
}

// AttrCtx is Attr for a value that needs the render context.
func (Builder[P]) AttrCtx(name string, value func(context.Context, P) (string, bool)) Op[P] {
	return attrCtxOp[P]{prefix: attrPrefix(name), value: value}
}

type attrCtxOp[P any] struct {
	prefix string
	value  func(context.Context, P) (string, bool)
}

func (o attrCtxOp[P]) Exec(r *Renderer, params P) error {
	value, present := o.value(r.boundaryContext(), params)
	if !present {
		return nil
	}
	return r.writeAttr(o.prefix, value)
}

// BoolAttr writes a bare attribute name when the value is true and omits it
// otherwise.
func (Builder[P]) BoolAttr(name string, value func(P) bool) Op[P] {
	return boolAttrOp[P]{text: " " + name, value: value}
}

type boolAttrOp[P any] struct {
	text  string
	value func(P) bool
}

func (o boolAttrOp[P]) Exec(r *Renderer, params P) error {
	if !o.value(params) {
		return nil
	}
	return r.Write(o.text)
}

// BoolAttrCtx is BoolAttr for a value that needs the render context.
func (Builder[P]) BoolAttrCtx(name string, value func(context.Context, P) bool) Op[P] {
	return boolAttrCtxOp[P]{text: " " + name, value: value}
}

type boolAttrCtxOp[P any] struct {
	text  string
	value func(context.Context, P) bool
}

func (o boolAttrCtxOp[P]) Exec(r *Renderer, params P) error {
	if !o.value(r.boundaryContext(), params) {
		return nil
	}
	return r.Write(o.text)
}

// BoundaryAttr writes the instance attribute of the boundary this component
// opened. It sits in the attribute position of the component's single root
// element, which is the reason an update boundary must have exactly one.
//
// The instruction writes nothing during an ordinary render, and nothing when
// the component was rendered as an ordinary call rather than as a chain
// member, so a nested component can never claim its parent's instance ID.
func (Builder[P]) BoundaryAttr() Op[P] { return boundaryAttrOp[P]{} }

type boundaryAttrOp[P any] struct{}

func (boundaryAttrOp[P]) Exec(r *Renderer, _ P) error {
	if r.collect == nil || r.collect.pending == nil {
		return nil
	}
	state := r.collect.pending
	r.collect.pending = nil
	return r.Write(" " + state.attr + `="` + state.id + `"`)
}

// If selects one of two instruction lists.
func (Builder[P]) If(condition func(P) bool, then, otherwise []Op[P]) Op[P] {
	return ifOp[P]{condition: condition, then: then, otherwise: otherwise}
}

type ifOp[P any] struct {
	condition       func(P) bool
	then, otherwise []Op[P]
}

func (o ifOp[P]) Exec(r *Renderer, params P) error {
	if o.condition(params) {
		return execOps(r, o.then, params)
	}
	return execOps(r, o.otherwise, params)
}

// IfCtx is If for a condition that needs the render context.
func (Builder[P]) IfCtx(condition func(context.Context, P) bool, then, otherwise []Op[P]) Op[P] {
	return ifCtxOp[P]{condition: condition, then: then, otherwise: otherwise}
}

type ifCtxOp[P any] struct {
	condition       func(context.Context, P) bool
	then, otherwise []Op[P]
}

func (o ifCtxOp[P]) Exec(r *Renderer, params P) error {
	if o.condition(r.boundaryContext(), params) {
		return execOps(r, o.then, params)
	}
	return execOps(r, o.otherwise, params)
}

// For repeats body once per item. scope builds the body's parameter value from
// the enclosing parameters, the item, and its index, so the loop variable stays
// statically typed instead of becoming an untyped lookup.
func For[P, E, S any](items func(P) []E, scope func(P, E, int) S, body []Op[S]) Op[P] {
	return forOp[P, E, S]{items: items, scope: scope, body: body}
}

// ForCtx is For for an item list that needs the render context.
func ForCtx[P, E, S any](items func(context.Context, P) []E, scope func(P, E, int) S, body []Op[S]) Op[P] {
	return forCtxOp[P, E, S]{items: items, scope: scope, body: body}
}

type forCtxOp[P, E, S any] struct {
	items func(context.Context, P) []E
	scope func(P, E, int) S
	body  []Op[S]
}

func (o forCtxOp[P, E, S]) Exec(r *Renderer, params P) error {
	for index, item := range o.items(r.boundaryContext(), params) {
		if err := execOps(r, o.body, o.scope(params, item, index)); err != nil {
			return err
		}
	}
	return nil
}

type forOp[P, E, S any] struct {
	items func(P) []E
	scope func(P, E, int) S
	body  []Op[S]
}

func (o forOp[P, E, S]) Exec(r *Renderer, params P) error {
	for index, item := range o.items(params) {
		if err := execOps(r, o.body, o.scope(params, item, index)); err != nil {
			return err
		}
	}
	return nil
}

// Require fails the render when check rejects the parameters. Generation emits
// it ahead of an await boundary that binds a required async parameter, so a
// caller who left one unset gets an error before the boundary commits its
// fallback and fixes the response status.
//
// It writes nothing, which is the point: the check has to run on the initial
// pass, where a failure can still become an error response, rather than in the
// boundary goroutine that runs after the response is already committed.
func Require[P any](check func(P) error) Op[P] { return requireOp[P]{check: check} }

type requireOp[P any] struct {
	check func(P) error
}

func (o requireOp[P]) Exec(_ *Renderer, params P) error { return o.check(params) }

// Await opens an await boundary. resolve runs the clause's bindings and builds
// the primary subtree's scope; recovery builds the recover subtree's scope from
// the outer parameters and the safe error. handler is nil when the clause
// declared no recover subtree, and then a failure becomes an UnrecoveredError
// for the caller instead of anything on the page.
//
// It is a free function rather than a Builder method because the primary and
// recover subtrees each read their own generated scope type.
func Await[P, S, R any](
	resolve func(context.Context, P) (S, error),
	recovery func(P, AsyncError) R,
	primary []Op[S],
	fallback []Op[P],
	handler []Op[R],
) Op[P] {
	return awaitOp[P, S, R]{resolve: resolve, recovery: recovery, primary: primary, fallback: fallback, handler: handler}
}

type awaitOp[P, S, R any] struct {
	resolve  func(context.Context, P) (S, error)
	recovery func(P, AsyncError) R
	primary  []Op[S]
	fallback []Op[P]
	handler  []Op[R]
}

func (o awaitOp[P, S, R]) Exec(r *Renderer, params P) error {
	if r.async == nil {
		return o.execBlocking(r, params)
	}
	coordinator := r.async
	id := r.nextBoundaryID()
	// display:contents keeps the placeholder out of layout, so a boundary
	// cannot change how its fallback or its replacement is positioned.
	if err := r.Write(`<` + r.boundaryElement() + ` id="` + id + `" style="display:contents">`); err != nil {
		return err
	}
	if err := execOps(r, o.fallback, params); err != nil {
		return err
	}
	if err := r.Write(`</` + r.boundaryElement() + `>`); err != nil {
		return err
	}
	boundaryCtx := r.boundaryContext()
	coordinator.start(boundaryCtx, func(ctx context.Context) (Content, bool, error) {
		var buffer bytes.Buffer
		// The subtree renders into its own buffer, so boundary work never
		// touches the response writer. A boundary nested in this subtree
		// registers with the same coordinator, under this boundary's own
		// identifier namespace, and streams like any other.
		sub := r.subtree(&buffer, id, ctx)
		value, err := o.resolve(ctx, params)
		if err != nil {
			if coordinator.ctx.Err() != nil {
				// Expected request cancellation, including an early consumer
				// stop. The committed fallback is the final content.
				return Content{}, false, nil
			}
			r.reportError(err)
			if o.handler == nil {
				// Nothing in this template can render the failure, so it ends
				// the sequence instead of being dropped. Leaving it out would
				// leave the committed fallback as the final content, and that
				// fallback says the value is still coming.
				return Content{}, false, &UnrecoveredError{BoundaryID: id, Err: err}
			}
			if err := execOps(sub, o.handler, o.recovery(params, normalizeAsyncError(err))); err != nil {
				return Content{}, false, err
			}
			return Content{BoundaryID: id, HTML: buffer.Bytes()}, true, nil
		}
		if err := execOps(sub, o.primary, value); err != nil {
			return Content{}, false, err
		}
		return Content{BoundaryID: id, HTML: buffer.Bytes()}, true, nil
	})
	return nil
}

// execBlocking settles the boundary in place, which is what the synchronous
// render entries do. The fallback subtree is never written, because the final
// content is already known by the time anything is emitted.
func (o awaitOp[P, S, R]) execBlocking(r *Renderer, params P) error {
	value, err := o.resolve(r.context(), params)
	if err != nil {
		r.reportError(err)
		if o.handler == nil {
			// Writing the fallback here would finish a document that promises
			// content nothing will ever deliver. This path commits no status of
			// its own, so returning the failure is what lets a caller rendering
			// into a buffer drop it and answer with an error instead.
			return &UnrecoveredError{Err: err}
		}
		return execOps(r, o.handler, o.recovery(params, normalizeAsyncError(err)))
	}
	return execOps(r, o.primary, value)
}

// Component renders another component. bind pairs the callee's plan with
// arguments derived from the caller's parameters.
func (Builder[P]) Component(bind func(P) Fragment) Op[P] { return componentOp[P]{bind: bind} }

type componentOp[P any] struct{ bind func(P) Fragment }

func (o componentOp[P]) Exec(r *Renderer, params P) error {
	fragment := o.bind(params)
	if !fragment.Present() {
		return nil
	}
	return fragment.render(r)
}

// ComponentCtx is Component for a binding that needs the render context.
func (Builder[P]) ComponentCtx(bind func(context.Context, P) Fragment) Op[P] {
	return componentCtxOp[P]{bind: bind}
}

type componentCtxOp[P any] struct {
	bind func(context.Context, P) Fragment
}

func (o componentCtxOp[P]) Exec(r *Renderer, params P) error {
	fragment := o.bind(r.boundaryContext(), params)
	if !fragment.Present() {
		return nil
	}
	return fragment.render(r)
}

// Slot inserts a bound slot argument. When the argument is absent the fallback
// instructions run, which is how default slot content is expressed. An absent
// slot with no fallback emits nothing at all.
func (Builder[P]) Slot(value func(P) Fragment, fallback []Op[P]) Op[P] {
	return slotOp[P]{value: value, fallback: fallback}
}

type slotOp[P any] struct {
	value    func(P) Fragment
	fallback []Op[P]
}

func (o slotOp[P]) Exec(r *Renderer, params P) error {
	fragment := o.value(params)
	if !fragment.Present() {
		return execOps(r, o.fallback, params)
	}
	return fragment.render(r)
}

// SlotCtx is Slot for a value that needs the render context. It is also what an
// html-returning external lowers to when its implementation takes one, so a
// framework fragment such as a hidden CSRF field renders as a subtree under the
// ordinary context checks rather than as escaped text.
func (Builder[P]) SlotCtx(value func(context.Context, P) Fragment, fallback []Op[P]) Op[P] {
	return slotCtxOp[P]{value: value, fallback: fallback}
}

type slotCtxOp[P any] struct {
	value    func(context.Context, P) Fragment
	fallback []Op[P]
}

func (o slotCtxOp[P]) Exec(r *Renderer, params P) error {
	fragment := o.value(r.boundaryContext(), params)
	if !fragment.Present() {
		return execOps(r, o.fallback, params)
	}
	return fragment.render(r)
}

// MergedHead writes every chain member's head contributions. The document
// shell places it inside its own head element.
func (Builder[P]) MergedHead() Op[P] { return mergedHeadOp[P]{} }

type mergedHeadOp[P any] struct{}

func (mergedHeadOp[P]) Exec(r *Renderer, _ P) error {
	for _, tag := range r.head {
		if err := r.Write(tag); err != nil {
			return err
		}
	}
	return nil
}

// Escape applies HTML text and attribute escaping. It is exported so generated
// helpers can reuse exactly the runtime's rules.
func Escape(value string) string {
	if !strings.ContainsAny(value, `&<>"'`) {
		return value
	}
	var out strings.Builder
	out.Grow(len(value) + 8)
	for _, r := range value {
		switch r {
		case '&':
			out.WriteString("&amp;")
		case '<':
			out.WriteString("&lt;")
		case '>':
			out.WriteString("&gt;")
		case '"':
			out.WriteString("&#34;")
		case '\'':
			out.WriteString("&#39;")
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// LiveBinding pumps one binding of a live clause. It ranges its own source and
// calls deliver once per value, passing a function that writes that value into
// the boundary scope. A non-nil err is a failure delivery rather than the end of
// the source, mirroring the (value, error) pair the source itself yields.
//
// deliver reports false when the boundary is gone, which is the signal to stop
// ranging. Returning ends this binding; the boundary lives until every binding
// has returned.
type LiveBinding[S any] func(deliver func(assign func(*S), err error) bool) error

// Live opens a live boundary. bindings subscribes each of the clause's sources,
// scope builds the boundary scope the primary subtree reads, and recovery builds
// the recover subtree's scope from the outer parameters and the safe error.
// handler is nil when the clause declared no recover subtree.
//
// Where Await settles once, this renders the primary subtree again for every
// delivery, for as long as the subscription lives. The sequence ending is the
// only terminal signal: a yielded error is a delivery of a failure, so a
// transient fault shows the recover subtree and the next value replaces it.
//
// A clause with several bindings holds the latest value of each and re-renders
// whenever any of them moves. Nothing has to say which source fired, because the
// subtree reads them all: putting every current value on every render is what
// removes the need for a selector. The first render waits until every binding
// has produced a value, since the subtree would otherwise read a zero one.
//
// Delivery is pull-based on purpose. A source blocks in its own yield until the
// boundary is ready for its next value, so a source producing faster than the
// screen can use it simply misses ticks. That is the coalescing rule with no
// queue to size and nothing to discard, and it is why a source is a sequence
// rather than a channel.
//
// On the document entries the boundary takes its first delivery and
// unsubscribes, so the first paint shows real content rather than a loading
// state and the response still finishes. Only the live entries keep the
// subscription open.
func Live[P, S, R any](
	bindings func(context.Context, P) []LiveBinding[S],
	scope func(P) S,
	recovery func(P, AsyncError) R,
	primary []Op[S],
	fallback []Op[P],
	handler []Op[R],
) Op[P] {
	return liveOp[P, S, R]{
		bindings: bindings,
		scope:    scope,
		recovery: recovery,
		primary:  primary,
		fallback: fallback,
		handler:  handler,
	}
}

type liveOp[P, S, R any] struct {
	bindings func(context.Context, P) []LiveBinding[S]
	scope    func(P) S
	recovery func(P, AsyncError) R
	primary  []Op[S]
	fallback []Op[P]
	handler  []Op[R]
}

func (o liveOp[P, S, R]) Exec(r *Renderer, params P) error {
	if r.async == nil {
		return o.execBlocking(r, params)
	}
	coordinator := r.async
	id := r.nextBoundaryID()
	// display:contents keeps the placeholder out of layout, so a boundary
	// cannot change how its fallback or its replacement is positioned.
	if err := r.Write(`<` + r.boundaryElement() + ` id="` + id + `" style="display:contents">`); err != nil {
		return err
	}
	if err := execOps(r, o.fallback, params); err != nil {
		return err
	}
	if err := r.Write(`</` + r.boundaryElement() + `>`); err != nil {
		return err
	}
	keepOpen := coordinator.liveMode()
	// The document response has to finish, so a boundary that has shown nothing
	// stops waiting at the boundary deadline. The live entry passes none: a
	// source there is allowed to be quiet for as long as its data is quiet.
	var firstDelivery time.Duration
	if !keepOpen {
		firstDelivery = coordinator.opts.timeout
	}
	coordinator.startStream(func(ctx context.Context, emit func(Content) bool) error {
		delivery := &deliveryScope{}
		defer delivery.stop()
		return o.pump(ctx, params, firstDelivery, r.reportError, func(scope S, deliveryErr error) deliveryResult {
			var buffer bytes.Buffer
			// The subtree renders into its own buffer, so a subscription never
			// touches the response writer. A boundary nested in this subtree
			// registers with the same coordinator, under this boundary's own
			// identifier namespace, and is cancelled when the next delivery
			// reuses those identifiers.
			sub := r.subtree(&buffer, id, delivery.next(ctx))
			var report error
			if deliveryErr != nil {
				if ctx.Err() != nil {
					// Expected cancellation. The boundary's current content is
					// its final content and nothing is left to read a delivery.
					return deliveryResult{}
				}
				report = deliveryErr
				if o.handler == nil {
					// Same rule as an await clause with no recover subtree:
					// nothing here can render the failure, so it leaves the
					// boundary rather than being dropped.
					return deliveryResult{report: report, failure: &UnrecoveredError{BoundaryID: id, Err: deliveryErr}}
				}
				if err := execOps(sub, o.handler, o.recovery(params, normalizeAsyncError(deliveryErr))); err != nil {
					return deliveryResult{report: report, failure: err}
				}
			} else if err := execOps(sub, o.primary, scope); err != nil {
				return deliveryResult{failure: err}
			}
			if !emit(Content{BoundaryID: id, HTML: buffer.Bytes()}) {
				return deliveryResult{report: report}
			}
			return deliveryResult{keep: keepOpen, report: report}
		})
	})
	return nil
}

// deliveryScope hands each delivery its own renderer context and cancels the
// previous delivery's. A live boundary's subtree hands out the same placeholder
// identifiers every time it renders, so a nested boundary left over from the
// superseded delivery would otherwise settle into the replacement's placeholder
// and put stale content on screen.
type deliveryScope struct {
	cancel context.CancelFunc
}

func (d *deliveryScope) next(parent context.Context) context.Context {
	d.stop()
	ctx, cancel := context.WithCancel(parent)
	d.cancel = cancel
	return ctx
}

func (d *deliveryScope) stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

// execBlocking renders the first complete delivery in place and stops watching,
// which is what the synchronous entries do. One template therefore serves a live
// client and a client that will never ask for updates, including one with no
// JavaScript.
//
// A source that ends without delivering leaves the fallback: a document holding
// a loading state is worse than a settled one only while something is still
// coming, and here nothing is.
func (o liveOp[P, S, R]) execBlocking(r *Renderer, params P) error {
	ctx := r.context()
	delivered := false
	failure := o.pump(ctx, params, r.boundaryTimeout(), r.reportError, func(scope S, deliveryErr error) deliveryResult {
		if deliveryErr != nil {
			if ctx.Err() != nil {
				// The wait ran out, or the request went away. Neither is a
				// failure of the source, so nothing is rendered here and the
				// caller falls back below.
				return deliveryResult{}
			}
			delivered = true
			if o.handler == nil {
				return deliveryResult{report: deliveryErr, failure: &UnrecoveredError{Err: deliveryErr}}
			}
			return deliveryResult{
				report:  deliveryErr,
				failure: execOps(r, o.handler, o.recovery(params, normalizeAsyncError(deliveryErr))),
			}
		}
		delivered = true
		return deliveryResult{failure: execOps(r, o.primary, scope)}
	})
	if failure != nil {
		return failure
	}
	if !delivered {
		return execOps(r, o.fallback, params)
	}
	return nil
}

// pump subscribes every binding and calls render for each delivery the boundary
// should show. It returns when every binding has ended, when render reports that
// the boundary is gone, or with the first failure render reported.
//
// render is called with the whole boundary scope rather than with the value that
// moved, which is what lets a clause hold several sources without anything
// having to select between them.
// firstDelivery bounds how long the boundary may show nothing. Zero means no
// bound beyond the request context, which is what the live entry passes.
// report hands a failure to the caller's reporter. It is called here rather than
// inside render because render runs under the boundary lock, and a reporter that
// blocks would otherwise stall the subscription it is reporting on.
func (o liveOp[P, S, R]) pump(ctx context.Context, params P, firstDelivery time.Duration, report func(error), render func(S, error) deliveryResult) error {
	ctx, cancel := context.WithCancel(ctx)
	// Cancelling on the way out is what stops the other bindings once one has
	// ended the boundary, and what makes an abandoned source observe the stop
	// through the context it was handed.
	defer cancel()
	if firstDelivery > 0 {
		// The entries that must answer give a boundary this long to produce
		// something. Running out is not a failure of the source, so it leaves
		// the fallback rather than rendering recover: a fallback is honest
		// about a value that has not arrived, where a recover subtree would
		// claim one went wrong.
		//
		// It bounds the wait alone. The entry stops watching after its first
		// render anyway, so the deadline is only ever live while the boundary
		// still has nothing to show.
		timed, stop := context.WithTimeout(ctx, firstDelivery)
		defer stop()
		ctx = timed
	}
	bindings := o.bindings(ctx, params)
	if len(bindings) == 0 {
		return nil
	}
	state := &liveState[S]{scope: o.scope(params), ready: make([]bool, len(bindings)), pending: len(bindings)}
	var wg sync.WaitGroup
	for index, binding := range bindings {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deliver := func(assign func(*S), err error) bool {
				keep, reported := state.deliver(index, assign, err, render)
				if reported != nil {
					// Outside the lock, so a reporter that blocks costs this
					// binding's next pull rather than the whole boundary.
					report(reported)
				}
				if !keep {
					cancel()
				}
				return keep
			}
			if err := runBinding(binding, deliver); err != nil {
				// A binding only returns when its source panicked: generated
				// pumps report a source's own failures through deliver and
				// return nil. Delivering it here is what makes a panic travel
				// the same path as a returned error, so the clause shows its
				// recover subtree instead of the whole render ending.
				deliver(nil, err)
			}
		}()
	}
	wg.Wait()
	return state.failure()
}

// runBinding executes one binding, turning a panic in the source into an
// ordinary error so it travels the same path as a returned one.
func runBinding[S any](binding LiveBinding[S], deliver func(func(*S), error) bool) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &panicError{value: recovered}
		}
	}()
	return binding(deliver)
}

// deliveryResult is what one render of a live boundary produced besides its
// output.
//
// report travels back out instead of being handed to the caller's reporter in
// place, because a render runs under the boundary lock. Reporting there would
// let a reporter that blocks — a full pipe, a synchronous exporter — hold the
// lock, and the failure that most wants logging is exactly the one whose
// boundary would then stop delivering.
type deliveryResult struct {
	// keep reports whether the subscription continues past this delivery.
	keep bool
	// report is the failure to hand to the caller's error reporter, or nil when
	// this delivery is not one the caller is told about.
	report error
	// failure ends the subscription and reaches the caller as the error of the
	// render sequence.
	failure error
}

// liveState is one live boundary's shared scope. Bindings run in their own
// goroutines and write their own fields, but unlike an await clause's tasks they
// keep writing while the subtree is being rendered, so the scope needs a lock
// rather than a join.
type liveState[S any] struct {
	mu      sync.Mutex
	scope   S
	ready   []bool
	pending int
	stopped bool
	err     error
}

// deliver applies one binding's value and renders once every binding has one.
// It returns whether the subscription continues, and any failure the caller is
// to report once this lock is released.
//
// The lock is held across the render and the emit. That serializes deliveries,
// so two bindings moving at once cannot put an older render on screen after a
// newer one, and a consumer that is not reading blocks the sources instead of
// queueing behind them — which is the same coalescing the pull sequence gives
// one source, extended to several.
//
// It is deliberately not held across the report. Serialization is for the render
// and the emit, which touch shared state; the reporter touches none of it and is
// the caller's code, so its speed is not something this boundary can assume.
func (s *liveState[S]) deliver(index int, assign func(*S), err error, render func(S, error) deliveryResult) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false, nil
	}
	if err == nil {
		assign(&s.scope)
		if !s.ready[index] {
			s.ready[index] = true
			s.pending--
		}
		if s.pending > 0 {
			// The primary subtree reads every binding, so rendering now would
			// show a zero value for one that has not arrived.
			return true, nil
		}
	}
	// A failure decides the boundary whether or not the others have arrived:
	// there is nothing to wait for once the clause is going to show recover.
	result := render(s.scope, err)
	if result.failure != nil {
		s.stopped, s.err = true, result.failure
		return false, result.report
	}
	if !result.keep {
		s.stopped = true
		return false, result.report
	}
	return true, result.report
}

func (s *liveState[S]) failure() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

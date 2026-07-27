package htmlbind

import (
	"bytes"
	"context"
	"strings"
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

func (o textOp[P]) Exec(r *Renderer, params P) error { return r.Write(Escape(o.value(params))) }

// Raw writes a value that the template already marked trusted for its context.
func (Builder[P]) Raw(value func(P) string) Op[P] { return rawOp[P]{value: value} }

type rawOp[P any] struct{ value func(P) string }

func (o rawOp[P]) Exec(r *Renderer, params P) error { return r.Write(o.value(params)) }

// Attr writes one attribute. The value arrives already escaped, because a
// mixed value concatenates author literals with escaped expressions and only
// the expressions may be escaped. present reports whether an optional value
// exists; an absent value omits the whole attribute.
func (Builder[P]) Attr(name string, value func(P) (string, bool)) Op[P] {
	return attrOp[P]{name: name, value: value}
}

type attrOp[P any] struct {
	name  string
	value func(P) (string, bool)
}

func (o attrOp[P]) Exec(r *Renderer, params P) error {
	value, present := o.value(params)
	if !present {
		return nil
	}
	return r.Write(" " + o.name + `="` + value + `"`)
}

// BoolAttr writes a bare attribute name when the value is true and omits it
// otherwise.
func (Builder[P]) BoolAttr(name string, value func(P) bool) Op[P] {
	return boolAttrOp[P]{name: name, value: value}
}

type boolAttrOp[P any] struct {
	name  string
	value func(P) bool
}

func (o boolAttrOp[P]) Exec(r *Renderer, params P) error {
	if !o.value(params) {
		return nil
	}
	return r.Write(" " + o.name)
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

// For repeats body once per item. scope builds the body's parameter value from
// the enclosing parameters, the item, and its index, so the loop variable stays
// statically typed instead of becoming an untyped lookup.
func For[P, E, S any](items func(P) []E, scope func(P, E, int) S, body []Op[S]) Op[P] {
	return forOp[P, E, S]{items: items, scope: scope, body: body}
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
// declared no recover subtree.
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
	id := coordinator.nextID()
	// display:contents keeps the placeholder out of layout, so a boundary
	// cannot change how its fallback or its replacement is positioned.
	if err := r.Write(`<tb-boundary id="` + id + `" style="display:contents">`); err != nil {
		return err
	}
	if err := execOps(r, o.fallback, params); err != nil {
		return err
	}
	if err := r.Write(`</tb-boundary>`); err != nil {
		return err
	}
	coordinator.start(func(ctx context.Context) (Content, bool, error) {
		var buffer bytes.Buffer
		// The subtree renders into its own buffer, so boundary work never
		// touches the response writer. A boundary nested in this subtree
		// registers with the same coordinator and streams like any other.
		sub := r.buffered(&buffer)
		value, err := o.resolve(ctx, params)
		if err != nil {
			if coordinator.ctx.Err() != nil {
				// Expected request cancellation, including an early consumer
				// stop. The committed fallback is the final content.
				return Content{}, false, nil
			}
			r.reportError(err)
			if o.handler == nil {
				return Content{}, false, nil
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
			return execOps(r, o.fallback, params)
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

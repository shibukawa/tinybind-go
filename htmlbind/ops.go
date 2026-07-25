package htmlbind

import "strings"

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

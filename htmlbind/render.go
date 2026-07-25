package htmlbind

import (
	"errors"
	"io"
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

// Render writes one component to w.
func Render(w io.Writer, leaf Fragment) error {
	return RenderChain(w, nil, leaf)
}

// RenderChain writes a composed document to w. Wrappers apply outermost first,
// so RenderChain(w, []Wrapper{document, layout}, page) renders page inside
// layout inside document. An empty wrapper list renders the leaf alone.
//
// Head contributions are merged before the first byte is written, so the shell
// can emit them without buffering the body. Assembly is validated up front, so
// a malformed chain cannot leave a partial response behind.
func RenderChain(w io.Writer, wrappers []Wrapper, leaf Fragment) error {
	if !leaf.Present() {
		return ErrNoLeaf
	}
	for _, wrapper := range wrappers {
		if wrapper.render == nil {
			return ErrNilWrapper
		}
	}
	renderer := &Renderer{w: w, head: MergeHead(wrappers, leaf)}
	next := leaf
	for i := len(wrappers) - 1; i >= 0; i-- {
		wrapper, inner := wrappers[i], next
		next = Fragment{render: func(r *Renderer) error { return wrapper.render(r, inner) }}
	}
	return next.render(renderer)
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

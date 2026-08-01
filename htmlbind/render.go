package htmlbind

import (
	"errors"
	"io"
	"strconv"
)

// Wrapper is a component that renders another one into its unnamed slot.
// Generated code returns one from Bind<Name> for a component with a children
// parameter.
type Wrapper struct {
	head     []string
	boundary *boundary
	render   func(*Renderer, Fragment) error
}

// BindWrapper pairs a plan with parameters and the setter that installs the
// child fragment. Generated code supplies the setter because only it knows
// which field the unnamed slot binds to.
func BindWrapper[P any](plan *Plan[P], params P, setChildren func(*P, Fragment)) Wrapper {
	return Wrapper{
		head:     plan.Head,
		boundary: bindBoundary(plan.Boundary, params),
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
	_, err := renderChain(w, nil, wrappers, leaf)
	return err
}

// CollectChain renders like RenderChain and additionally returns the update
// manifest of the boundaries it wrote. Every chain member declaring a boundary
// becomes one instance, so the manifest describes the layout chain rather than
// every component call.
//
// key authenticates the returned validators. A digest published to a browser
// must be keyed, because an unkeyed hash of low entropy content lets anyone
// confirm a guess by comparing digests. The same key must be used for two
// renders that are to be compared; changing it forces a full render, which is
// the intended effect of a key rotation.
//
// Collecting emits the instance attribute on each boundary's root element, so
// its output differs from RenderChain by exactly those attributes.
func CollectChain(w io.Writer, key []byte, wrappers []Wrapper, leaf Fragment) (Manifest, error) {
	return renderChain(w, &collector{key: key}, wrappers, leaf)
}

// renderChainHead is renderChain plus the merged head, which a delta needs so
// the browser can install contributions a newly reachable component brought.
func renderChainHead(w io.Writer, collect *collector, wrappers []Wrapper, leaf Fragment) (Manifest, []string, error) {
	head := MergeHead(wrappers, leaf)
	manifest, err := renderChain(w, collect, wrappers, leaf)
	return manifest, head, err
}

func renderChain(w io.Writer, collect *collector, wrappers []Wrapper, leaf Fragment) (Manifest, error) {
	if !leaf.Present() {
		return Manifest{}, ErrNoLeaf
	}
	for _, wrapper := range wrappers {
		if wrapper.render == nil {
			return Manifest{}, ErrNilWrapper
		}
	}
	renderer := &Renderer{w: w, head: MergeHead(wrappers, leaf), collect: collect}
	// The chain index is the instance identity, so a member keeps its ID when
	// only its parameters change. Position is assigned before rendering, which
	// is why an unchanged chain shape yields comparable manifests.
	next := memberFragment(leaf, leaf.boundary, len(wrappers))
	for i := len(wrappers) - 1; i >= 0; i-- {
		wrapper, inner, index := wrappers[i], next, i
		child := Fragment{render: func(r *Renderer) error { return wrapper.render(r, inner) }}
		next = memberFragment(child, wrapper.boundary, index)
	}
	if err := next.render(renderer); err != nil {
		return Manifest{}, err
	}
	if collect == nil {
		return Manifest{}, nil
	}
	return collect.manifest, nil
}

// memberFragment wraps one chain member so its boundary opens before its first
// byte and closes after its last, which is what makes a frame validator cover
// the member's own markup only.
func memberFragment(member Fragment, decl *boundary, index int) Fragment {
	if decl == nil {
		return member
	}
	id := "c" + strconv.Itoa(index)
	return Fragment{
		head: member.head,
		render: func(r *Renderer) error {
			if r.collect == nil {
				return member.render(r)
			}
			r.collect.open(id, decl.componentID, decl.attr, decl.input())
			if err := member.render(r); err != nil {
				return err
			}
			r.collect.close()
			return nil
		},
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

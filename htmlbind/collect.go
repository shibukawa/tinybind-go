package htmlbind

import (
	"io"
	"strconv"
)

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
func CollectChain(w io.Writer, key []byte, wrappers []Wrapper, leaf Fragment, options ...Option) (Manifest, error) {
	manifest, _, err := collectChain(w, &collector{key: key}, wrappers, leaf, options)
	return manifest, err
}

// collectChain composes the chain like RenderChain does and additionally opens
// a boundary around each member that declares one.
//
// It returns the merged head as well, because a delta reuses the browser's
// existing document shell and has to say which contributions a newly reachable
// component brought with it.
func collectChain(w io.Writer, collect *collector, wrappers []Wrapper, leaf Fragment, options []Option) (Manifest, []string, error) {
	opts := newRenderOptions(options)
	if err := validateChain(wrappers, leaf); err != nil {
		return Manifest{}, nil, err
	}
	head, err := mergeCallerHead(MergeHead(wrappers, leaf), opts.head)
	if err != nil {
		return Manifest{}, nil, err
	}
	// The chain index is the instance identity, so a member keeps its ID when
	// only its parameters change. Position is assigned before rendering, which
	// is why an unchanged chain shape yields comparable manifests.
	next := memberFragment(leaf, leaf.boundary, len(wrappers))
	for i := len(wrappers) - 1; i >= 0; i-- {
		wrapper, inner, index := wrappers[i], next, i
		child := Fragment{render: func(r *Renderer) error { return wrapper.render(r, inner) }}
		next = memberFragment(child, wrapper.boundary, index)
	}
	if err := next.render(&Renderer{w: w, head: head, opts: opts, collect: collect}); err != nil {
		return Manifest{}, nil, err
	}
	if collect == nil {
		return Manifest{}, head, nil
	}
	return collect.manifest, head, nil
}

// validateChain repeats the assembly checks, so a malformed chain fails with
// nothing written here exactly as it does on the ordinary path.
func validateChain(wrappers []Wrapper, leaf Fragment) error {
	if !leaf.Present() {
		return ErrNoLeaf
	}
	for _, wrapper := range wrappers {
		if wrapper.render == nil {
			return ErrNilWrapper
		}
	}
	if err := leaf.Validate(); err != nil {
		return err
	}
	for _, wrapper := range wrappers {
		if err := wrapper.Validate(); err != nil {
			return err
		}
	}
	return nil
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
		head:     member.head,
		hasAwait: member.hasAwait,
		hasLive:  member.hasLive,
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

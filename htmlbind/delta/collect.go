package delta

import (
	"io"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

var _ htmlbind.Collector = (*collector)(nil)

// CollectChain renders like htmlbind.RenderChain and additionally returns the
// update manifest of the boundaries it wrote. Every chain member declaring a
// boundary becomes one instance, so the manifest describes the layout chain
// rather than every component call.
//
// key authenticates the returned validators. A digest published to a browser
// must be keyed, because an unkeyed hash of low entropy content lets anyone
// confirm a guess by comparing digests. The same key must be used for two
// renders that are to be compared; changing it forces a full render, which is
// the intended effect of a key rotation.
//
// Collecting emits the instance attribute on each boundary's root element, so
// its output differs from RenderChain by exactly those attributes.
func CollectChain(w io.Writer, key []byte, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) (Manifest, error) {
	collect := &collector{key: key}
	if _, err := htmlbind.CollectChain(w, collect, wrappers, leaf, options...); err != nil {
		return Manifest{}, err
	}
	return collect.manifest, nil
}

package htmlbind

import (
	"bytes"
	"context"
	"sync"
	"time"
)

// cacheGroup collects the settled content of every boundary one cached
// component opened, so its stored form is the settled markup rather than the
// placeholder its miss delivered.
//
// It exists because the coordinator tracks a whole render's boundaries in one
// wait group and one result channel, with nothing tying a boundary to the
// component that owns it. Knowing when one component's set is complete is what
// a store has to wait for, and this is the only thing that knows it.
//
// A miss delivers exactly what the same component delivers uncached — the
// placeholder, the streamed fallback, the completion frame — because
// decision:cached-boundary-delivery makes delivery a property of the template
// and storage a deployment question. Only the storing happens here.
type cacheGroup struct {
	mu sync.Mutex
	// shell is what the initial pass wrote for this component: static markup
	// with a fence around each fallback. It is copied rather than aliased,
	// because the buffer it came from is reused once the miss has been written.
	shell []byte
	// settled maps a boundary id to the markup that replaced its placeholder.
	settled map[string][]byte
	// pending counts boundaries that have not reported yet. The component is
	// storable when it reaches zero with the shell captured and nothing failed.
	pending int
	// captured marks that the initial pass finished, so a boundary settling
	// before the shell is recorded cannot store a half-built entry.
	captured bool
	// failed marks a boundary that did not settle. A failure stores nothing,
	// which is the existing rule that a failed render publishes nothing.
	failed bool
	// stored guards against a second write, since the last boundary and the
	// capture can race to be the one that completes the set.
	stored bool

	store  CacheStore
	key    string
	ttl    time.Duration
	ctx    context.Context
	prefix string
	// parent is the group of an enclosing cached component, if any. A cached
	// component may contain another, and the outer one's stored form contains
	// the inner one's output — so the outer cannot publish until the inner's
	// boundaries have settled, or it stores a placeholder nothing will ever
	// replace and a hit serves a permanent loading state.
	//
	// Every registration and every outcome therefore travels up as well. The
	// fence ids are unique across the render and the outer's own shell holds
	// the inner's fences, so one settled subtree splices into both.
	parent *cacheGroup
}

// open registers one boundary with the group. A boundary opened inside a
// settled subtree registers too, because the stored form has to hold it.
func (g *cacheGroup) open() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.pending++
	g.mu.Unlock()
	g.parent.open()
}

// settle records one boundary's content. present is false for a boundary that
// produced nothing, which a cancelled request leaves behind.
func (g *cacheGroup) settle(id string, html []byte, present bool, err error) {
	if g == nil {
		return
	}
	defer g.parent.settle(id, html, present, err)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pending--
	if err != nil || !present {
		g.failed = true
		return
	}
	if g.settled == nil {
		g.settled = map[string][]byte{}
	}
	held := make([]byte, len(html))
	copy(held, html)
	g.settled[id] = held
	g.publishLocked()
}

// capture records the bytes the initial pass wrote for this component.
func (g *cacheGroup) capture(shell []byte) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.shell = make([]byte, len(shell))
	copy(g.shell, shell)
	g.captured = true
	g.publishLocked()
}

// publishLocked stores the settled form once the shell is captured and every
// boundary has reported. It runs under the group's lock.
func (g *cacheGroup) publishLocked() {
	if g.stored || g.failed || !g.captured || g.pending > 0 {
		return
	}
	g.stored = true
	g.store.Set(g.ctx, g.key, spliceSettled(g.prefix, g.shell, g.settled), g.ttl)
}

// spliceSettled replaces each fence span with the markup that settled for it.
//
// The fences are comment markers this package wrote around each fallback, and
// the ids are ones it issued, so this is a rewrite of its own bytes rather than
// a second implementation of the client's apply logic. It repeats until nothing
// is left to replace, because a boundary nested inside a settled subtree arrives
// as its own entry and its fence is inside another one's content.
func spliceSettled(prefix string, shell []byte, settled map[string][]byte) []byte {
	out := shell
	for range len(settled) + 1 {
		replaced := false
		for id, html := range settled {
			open := []byte(awaitFenceOpen(prefix, id))
			close := []byte(awaitFenceClose(prefix, id))
			start := bytes.Index(out, open)
			if start < 0 {
				continue
			}
			end := bytes.Index(out[start:], close)
			if end < 0 {
				continue
			}
			end += start + len(close)
			next := make([]byte, 0, len(out)-(end-start)+len(html))
			next = append(next, out[:start]...)
			next = append(next, html...)
			next = append(next, out[end:]...)
			out = next
			replaced = true
		}
		if !replaced {
			break
		}
	}
	return out
}

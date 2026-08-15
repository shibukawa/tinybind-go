package htmlbind

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type cgParams struct{ ID string }
type cgScope struct {
	Outer cgParams
	Value string
}

type recordingStore struct {
	mu      sync.Mutex
	entries map[string][]byte
}

func newRecordingStore() *recordingStore {
	return &recordingStore{entries: map[string][]byte{}}
}

func (s *recordingStore) Get(_ context.Context, key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.entries[key]
	return value, ok
}

func (s *recordingStore) Set(_ context.Context, key string, value []byte, _ time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held := make([]byte, len(value))
	copy(held, value)
	s.entries[key] = held
}

func (s *recordingStore) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// cachedAwaitPlan is a cached component whose body is one await boundary, which
// is the shape the whole change exists for: a component that loads its own data
// and caches the load and the render together.
func cachedAwaitPlan(resolve func(context.Context, cgParams) (cgScope, error)) *Plan[cgParams] {
	body := Builder[cgScope]{}
	outer := Builder[cgParams]{}
	return &Plan[cgParams]{
		HasAwaitBlock: true,
		Cache: &CachePolicy[cgParams]{
			ID:  "Card",
			TTL: time.Minute,
			Key: func(p cgParams) string { return KeyString(p.ID) },
		},
		Ops: []Op[cgParams]{
			outer.Static("<article>"),
			Await(resolve,
				func(p cgParams, _ AsyncError) cgParams { return p },
				[]Op[cgScope]{body.Static("<h1>"), body.Text(func(p cgScope) string { return p.Value }), body.Static("</h1>")},
				[]Op[cgParams]{outer.Static("<p>loading</p>")},
				nil),
			outer.Static("</article>"),
		},
	}
}

func settledPlan(value string) *Plan[cgParams] {
	return cachedAwaitPlan(func(_ context.Context, p cgParams) (cgScope, error) {
		return cgScope{Outer: p, Value: value}, nil
	})
}

// drain runs a streaming render to completion, returning the document and every
// settled boundary in order.
func drain(t *testing.T, plan *Plan[cgParams], params cgParams, options ...Option) (string, []Content) {
	t.Helper()
	var out strings.Builder
	var settled []Content
	for content, err := range RenderAsync(context.Background(), &out, Bind(plan, params), options...) {
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		settled = append(settled, content)
	}
	return out.String(), settled
}

// A miss delivers exactly what the same component delivers uncached: the
// placeholder, the streamed fallback, and the completion frame. Configuring a
// store changes what is stored and never what is sent.
func TestCachedBoundaryMissStreamsLikeAnUncachedOne(t *testing.T) {
	store := newRecordingStore()
	withStore, missBoundaries := drain(t, settledPlan("seven"), cgParams{ID: "7"}, WithCache(store))
	without, plainBoundaries := drain(t, settledPlan("seven"), cgParams{ID: "7"})
	if withStore != without {
		t.Fatalf("a store changed the delivered document:\n with: %q\nwithout: %q", withStore, without)
	}
	if len(missBoundaries) != len(plainBoundaries) || len(missBoundaries) != 1 {
		t.Fatalf("boundary count = %d, want the %d of the uncached render", len(missBoundaries), len(plainBoundaries))
	}
	if !strings.Contains(withStore, "loading") {
		t.Fatalf("the miss did not stream its fallback: %q", withStore)
	}
}

// The settled markup is what gets stored, not the placeholder the miss wrote.
func TestAMissStoresTheSettledForm(t *testing.T) {
	store := newRecordingStore()
	drain(t, settledPlan("seven"), cgParams{ID: "7"}, WithCache(store))
	if store.len() != 1 {
		t.Fatalf("stored %d entries, want one", store.len())
	}
	var stored string
	for _, value := range store.entries {
		stored = string(value)
	}
	if !strings.Contains(stored, "<h1>seven</h1>") {
		t.Fatalf("the stored form is not the settled markup: %q", stored)
	}
	if strings.Contains(stored, "loading") || strings.Contains(stored, "<!--") {
		t.Fatalf("the stored form still holds the fallback or a fence: %q", stored)
	}
}

// On a hit the await becomes synchronous: the settled markup is written in
// place, with no placeholder, no fallback, and no completion frame.
func TestAHitWritesTheSettledFormWithNoBoundary(t *testing.T) {
	store := newRecordingStore()
	drain(t, settledPlan("seven"), cgParams{ID: "7"}, WithCache(store))

	calls := 0
	counted := cachedAwaitPlan(func(_ context.Context, p cgParams) (cgScope, error) {
		calls++
		return cgScope{Outer: p, Value: "seven"}, nil
	})
	document, boundaries := drain(t, counted, cgParams{ID: "7"}, WithCache(store))
	if calls != 0 {
		t.Fatalf("a hit ran the loader %d times, want none", calls)
	}
	if len(boundaries) != 0 {
		t.Fatalf("a hit opened %d boundaries, want none", len(boundaries))
	}
	if !strings.Contains(document, "<h1>seven</h1>") {
		t.Fatalf("the hit did not write the settled markup: %q", document)
	}
	if strings.Contains(document, "loading") {
		t.Fatalf("the hit wrote a fallback: %q", document)
	}
}

// A boundary that fails stores nothing, which extends the existing rule that a
// failed render publishes nothing.
func TestAFailedBoundaryStoresNothing(t *testing.T) {
	store := newRecordingStore()
	failing := cachedAwaitPlan(func(_ context.Context, _ cgParams) (cgScope, error) {
		return cgScope{}, errors.New("upstream is down")
	})
	for range RenderAsync(context.Background(), io.Discard, Bind(failing, cgParams{ID: "7"}), WithCache(store)) {
	}
	if store.len() != 0 {
		t.Fatalf("stored %d entries after a failure, want none", store.len())
	}
}

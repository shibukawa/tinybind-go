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

type cgInner struct {
	Outer cgScope
	Extra string
}

// A boundary inside a settled subtree settles separately, so its fence sits
// inside another one's content and the splice has to run again to reach it.
// The commit that added this claimed the replacement iterates; this is the case
// that proves it rather than the claim.
func TestNestedBoundariesAreSplicedIntoTheStoredForm(t *testing.T) {
	outerBody := Builder[cgScope]{}
	innerBody := Builder[cgInner]{}
	outer := Builder[cgParams]{}
	plan := &Plan[cgParams]{
		HasAwaitBlock: true,
		Cache: &CachePolicy[cgParams]{
			ID:  "Nested",
			TTL: time.Minute,
			Key: func(p cgParams) string { return KeyString(p.ID) },
		},
		Ops: []Op[cgParams]{
			outer.Static("<article>"),
			Await(
				func(_ context.Context, p cgParams) (cgScope, error) {
					return cgScope{Outer: p, Value: "outer"}, nil
				},
				func(p cgParams, _ AsyncError) cgParams { return p },
				[]Op[cgScope]{
					outerBody.Static("<h1>"),
					outerBody.Text(func(p cgScope) string { return p.Value }),
					outerBody.Static("</h1>"),
					Await(
						func(_ context.Context, p cgScope) (cgInner, error) {
							return cgInner{Outer: p, Extra: "inner"}, nil
						},
						func(p cgScope, _ AsyncError) cgScope { return p },
						[]Op[cgInner]{
							innerBody.Static("<em>"),
							innerBody.Text(func(p cgInner) string { return p.Extra }),
							innerBody.Static("</em>"),
						},
						[]Op[cgScope]{outerBody.Static("<span>inner loading</span>")},
						nil),
				},
				[]Op[cgParams]{outer.Static("<p>outer loading</p>")},
				nil),
			outer.Static("</article>"),
		},
	}

	store := newRecordingStore()
	drain(t, plan, cgParams{ID: "7"}, WithCache(store))
	var stored string
	for _, value := range store.entries {
		stored = string(value)
	}
	for _, want := range []string{"<h1>outer</h1>", "<em>inner</em>"} {
		if !strings.Contains(stored, want) {
			t.Fatalf("the stored form is missing %q:\n%s", want, stored)
		}
	}
	// Neither fallback nor any fence may survive, or a hit would replay a
	// placeholder nothing is coming for.
	for _, unwanted := range []string{"loading", "<!--"} {
		if strings.Contains(stored, unwanted) {
			t.Fatalf("the stored form still holds %q:\n%s", unwanted, stored)
		}
	}

	// And the hit replays the whole nest with no boundary at all.
	document, boundaries := drain(t, plan, cgParams{ID: "7"}, WithCache(store))
	if len(boundaries) != 0 {
		t.Fatalf("a hit opened %d boundaries, want none", len(boundaries))
	}
	if !strings.Contains(document, "<em>inner</em>") {
		t.Fatalf("the hit lost the nested subtree: %q", document)
	}
}

// A recover subtree settles, and the render succeeds: the page shows its error
// UI and returns no error at all. Storing that would cache an error page and
// serve it for the whole TTL, which is the worst shape this cache could take —
// so a settled recover counts as a failure for storage and the next request is
// a miss.
func TestASettledRecoverIsNotStored(t *testing.T) {
	body := Builder[cgScope]{}
	outer := Builder[cgParams]{}
	attempts := 0
	plan := &Plan[cgParams]{
		HasAwaitBlock: true,
		Cache: &CachePolicy[cgParams]{
			ID:  "Recovering",
			TTL: time.Minute,
			Key: func(p cgParams) string { return KeyString(p.ID) },
		},
		Ops: []Op[cgParams]{
			Await(
				func(_ context.Context, p cgParams) (cgScope, error) {
					attempts++
					return cgScope{}, errors.New("upstream is down")
				},
				func(p cgParams, _ AsyncError) cgParams { return p },
				[]Op[cgScope]{body.Text(func(p cgScope) string { return p.Value })},
				[]Op[cgParams]{outer.Static("<p>loading</p>")},
				[]Op[cgParams]{outer.Static("<p>unavailable</p>")}),
		},
	}

	store := newRecordingStore()
	// On the streaming path the recover markup arrives as the boundary's
	// completion frame rather than in the initial document.
	_, settled := drain(t, plan, cgParams{ID: "7"}, WithCache(store))
	if len(settled) != 1 || !strings.Contains(string(settled[0].HTML), "unavailable") {
		t.Fatalf("the recover subtree did not settle: %+v", settled)
	}
	if store.len() != 0 {
		t.Fatalf("a rendered failure was stored: %v", store.entries)
	}

	// The next request tries again rather than replaying the error UI.
	before := attempts
	drain(t, plan, cgParams{ID: "7"}, WithCache(store))
	if attempts == before {
		t.Fatal("the second request did not retry, so the failure was cached after all")
	}
}

package htmlbind

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type requireParams struct{ ID string }

type requireScope struct {
	Outer requireParams
	Value string
}

// A check writes nothing and runs once, so it contributes no sequence node at
// all. Left as an opaque node instead, the sequence would carry one hole more
// than the render produced values for, and reassembly of a page holding one
// would fail against a render that was correct.
func TestRequireContributesNoSequenceNode(t *testing.T) {
	body := Builder[requireParams]{}
	checked := &Plan[requireParams]{Ops: []Op[requireParams]{
		body.Static("<h1>"),
		body.Require(func(requireParams) error { return nil }),
		body.Text(func(p requireParams) string { return p.ID }),
		body.Static("</h1>"),
	}}
	plain := &Plan[requireParams]{Ops: []Op[requireParams]{
		body.Static("<h1>"),
		body.Text(func(p requireParams) string { return p.ID }),
		body.Static("</h1>"),
	}}
	if got, want := len(checked.Sequence().Nodes), len(plain.Sequence().Nodes); got != want {
		t.Fatalf("checked sequence has %d nodes, want the %d of the same markup written without a check", got, want)
	}
	for i, node := range checked.Sequence().Nodes {
		if other := plain.Sequence().Nodes[i]; node.Kind != other.Kind || node.Text != other.Text {
			t.Fatalf("node %d = %+v, want %+v", i, node, other)
		}
	}
	// The property everything rests on: the sequence and the values one render
	// produced reproduce that render's bytes.
	var out strings.Builder
	if err := Render(&out, Bind(checked, requireParams{ID: "7"})); err != nil {
		t.Fatalf("render: %v", err)
	}
	rebuilt, err := checked.Sequence().Reassemble([]string{"7"})
	if err != nil {
		t.Fatalf("reassemble: %v", err)
	}
	if rebuilt != out.String() {
		t.Fatalf("round trip:\n got %q\nwant %q", rebuilt, out.String())
	}
}

// The same hole would open under an await boundary that checks a required
// async parameter, which is the one position generation emitted a check from
// before the template gained a check directive.
func TestRequireCtxContributesNoSequenceNode(t *testing.T) {
	body := Builder[requireParams]{}
	plan := &Plan[requireParams]{Ops: []Op[requireParams]{
		body.RequireCtx(func(context.Context, requireParams) error { return nil }),
		body.Static("<p>ok</p>"),
	}}
	nodes := plan.Sequence().Nodes
	if len(nodes) != 1 || nodes[0].Kind != SeqStatic {
		t.Fatalf("sequence = %+v, want one static node", nodes)
	}
}

// A failing check ends the render before anything is written, which is what
// leaves the response status free to become the error's own.
func TestRequireEndsTheRenderBeforeWriting(t *testing.T) {
	want := errors.New("forbidden")
	body := Builder[requireParams]{}
	plan := &Plan[requireParams]{Ops: []Op[requireParams]{
		body.Require(func(requireParams) error { return want }),
		body.Static("<h1>secret</h1>"),
	}}
	var out strings.Builder
	if err := Render(&out, Bind(plan, requireParams{ID: "7"})); !errors.Is(err, want) {
		t.Fatalf("render error = %v, want %v", err, want)
	}
	if out.String() != "" {
		t.Fatalf("the failed check wrote %q", out.String())
	}
}

// A check runs during assembly, before any byte reaches the writer, which is
// what lets its error choose the response status the way a leading binding's
// does.
func TestRequireRunsBeforeAnyByte(t *testing.T) {
	want := errors.New("forbidden")
	body := Builder[requireParams]{}
	leaf := &Plan[requireParams]{Ops: []Op[requireParams]{
		body.Static(" "),
		body.Require(func(requireParams) error { return want }),
		body.Static("<h1>secret</h1>"),
	}}
	var out strings.Builder
	if err := Render(&out, Bind(leaf, requireParams{ID: "7"})); !errors.Is(err, want) {
		t.Fatalf("render error = %v, want %v", err, want)
	}
	if out.Len() != 0 {
		t.Fatalf("wrote %q before the check refused, want nothing", out.String())
	}
}

// A check encloses nothing, so the binding written after it is still a leading
// instruction. Missing this leaves a guarded loader unprepared: it fails after
// the shell is written, and its 404 becomes a 200.
func TestACheckDoesNotStopTheBindingAfterItBeingPrepared(t *testing.T) {
	want := errors.New("no such record")
	body := Builder[requireScope]{}
	leaf := &Plan[requireParams]{Ops: []Op[requireParams]{
		Builder[requireParams]{}.Require(func(requireParams) error { return nil }),
		ValErr(
			func(p requireParams) (string, error) { return "", want },
			func(p requireParams, value string) requireScope { return requireScope{Outer: p, Value: value} },
			[]Op[requireScope]{body.Static("<h1>"), body.Text(func(p requireScope) string { return p.Value })}),
	}}
	var out strings.Builder
	if err := Render(&out, Bind(leaf, requireParams{ID: "7"})); !errors.Is(err, want) {
		t.Fatalf("render error = %v, want %v", err, want)
	}
	if out.Len() != 0 {
		t.Fatalf("wrote %q before the guarded loader failed, want nothing", out.String())
	}
}

// The check answers once. Running during assembly and again at render would
// call it twice, which is the difference between preparing an instruction and
// merely checking one.
func TestAPreparedCheckIsNotRerun(t *testing.T) {
	calls := 0
	body := Builder[requireParams]{}
	leaf := &Plan[requireParams]{Ops: []Op[requireParams]{
		body.Require(func(requireParams) error { calls++; return nil }),
		body.Static("<h1>ok</h1>"),
	}}
	var out strings.Builder
	if err := Render(&out, Bind(leaf, requireParams{ID: "7"})); err != nil {
		t.Fatalf("render: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the check ran %d times, want once", calls)
	}
	if out.String() != "<h1>ok</h1>" {
		t.Fatalf("output = %q", out.String())
	}
}

// A check inside a cached component is part of what the cache stores, so a hit
// skips it exactly as it skips the loader beside it. That is right for a check
// reading only its declared parameters and wrong for one reading the request,
// which is the existing rule about what a cached component may depend on rather
// than a new one.
func TestACachedComponentRunsItsCheckOnAMissAndNotOnAHit(t *testing.T) {
	calls := 0
	plan := func() *Plan[requireParams] {
		body := Builder[requireParams]{}
		return &Plan[requireParams]{
			Cache: &CachePolicy[requireParams]{
				ID:  "Guarded",
				TTL: time.Minute,
				Key: func(p requireParams) string { return KeyString(p.ID) },
			},
			Ops: []Op[requireParams]{
				body.Require(func(requireParams) error { calls++; return nil }),
				body.Static("<h1>ok</h1>"),
			},
		}
	}
	store := newRecordingStore()
	var miss strings.Builder
	if err := Render(&miss, Bind(plan(), requireParams{ID: "7"}), WithCache(store)); err != nil {
		t.Fatalf("miss: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the miss ran the check %d times, want once", calls)
	}
	var hit strings.Builder
	if err := Render(&hit, Bind(plan(), requireParams{ID: "7"}), WithCache(store)); err != nil {
		t.Fatalf("hit: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the hit ran the check again; calls = %d", calls)
	}
	if hit.String() != miss.String() {
		t.Fatalf("hit = %q, miss = %q", hit.String(), miss.String())
	}
}

// The context-carrying form exists because a check's Go implementation may
// declare a leading context.Context, exactly as a bound external's may.
func TestRequireCtxReceivesTheRenderContext(t *testing.T) {
	type key struct{}
	denied := errors.New("denied")
	body := Builder[requireParams]{}
	plan := &Plan[requireParams]{Ops: []Op[requireParams]{
		body.RequireCtx(func(ctx context.Context, p requireParams) error {
			if value, _ := ctx.Value(key{}).(string); value != "allowed" {
				return denied
			}
			return nil
		}),
		body.Static("<h1>ok</h1>"),
	}}
	var out strings.Builder
	ctx := context.WithValue(context.Background(), key{}, "allowed")
	if err := Render(&out, Bind(plan, requireParams{}), WithContext(ctx)); err != nil {
		t.Fatalf("render: %v", err)
	}
	if out.String() != "<h1>ok</h1>" {
		t.Fatalf("output = %q", out.String())
	}
	// A render that supplied no context still has one, so the check runs and
	// answers rather than failing for want of a context.
	var refused strings.Builder
	if err := Render(&refused, Bind(plan, requireParams{})); !errors.Is(err, denied) {
		t.Fatalf("render error = %v, want %v", err, denied)
	}
}

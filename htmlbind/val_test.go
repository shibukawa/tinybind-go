package htmlbind

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type valParams struct{ ID string }

type valScope struct {
	Outer valParams
	Value string
}

// A binding evaluates its value once and hands the body a scope holding it, so
// a subtree that reads the name four times still costs one call. That is the
// whole reason the construct exists: without it a component that loads its own
// data calls its loader once per field it renders.
func TestValEvaluatesItsValueOncePerRender(t *testing.T) {
	calls := 0
	body := Builder[valScope]{}
	plan := &Plan[valParams]{Ops: []Op[valParams]{
		Val(
			func(p valParams) string { calls++; return "loaded-" + p.ID },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{
				body.Static("<h1>"),
				body.Text(func(p valScope) string { return p.Value }),
				body.Static("</h1><p>"),
				body.Text(func(p valScope) string { return p.Value }),
				body.Static("</p>"),
			}),
	}}
	var out strings.Builder
	if err := Render(&out, Bind(plan, valParams{ID: "7"})); err != nil {
		t.Fatalf("render: %v", err)
	}
	if calls != 1 {
		t.Fatalf("value closure ran %d times, want once", calls)
	}
	if want := "<h1>loaded-7</h1><p>loaded-7</p>"; out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

// The context-carrying form exists because the bound expression is exactly
// where an external that declared a leading context.Context is expected to
// appear.
func TestValCtxReceivesTheRenderContext(t *testing.T) {
	type key struct{}
	body := Builder[valScope]{}
	plan := &Plan[valParams]{Ops: []Op[valParams]{
		ValCtx(
			func(ctx context.Context, p valParams) string {
				value, _ := ctx.Value(key{}).(string)
				return value
			},
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{body.Text(func(p valScope) string { return p.Value })}),
	}}
	var out strings.Builder
	ctx := context.WithValue(context.Background(), key{}, "from-context")
	if err := Render(&out, Bind(plan, valParams{}), WithContext(ctx)); err != nil {
		t.Fatalf("render: %v", err)
	}
	if out.String() != "from-context" {
		t.Fatalf("output = %q, want the context value", out.String())
	}
}

// A binding runs its body exactly once, so it contributes no value-stream
// marker and its nodes belong where it stands. Left as an opaque node instead,
// every bound subtree would stop decomposing and the delta path would quietly
// degrade while the render still looked right.
func TestValSplicesItsBodyIntoTheSequence(t *testing.T) {
	body := Builder[valScope]{}
	bound := &Plan[valParams]{Ops: []Op[valParams]{
		Val(
			func(p valParams) string { return p.ID },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{
				body.Static("<h1>"),
				body.Text(func(p valScope) string { return p.Value }),
				body.Static("</h1>"),
			}),
	}}
	plain := Builder[valParams]{}
	unbound := &Plan[valParams]{Ops: []Op[valParams]{
		plain.Static("<h1>"),
		plain.Text(func(p valParams) string { return p.ID }),
		plain.Static("</h1>"),
	}}
	if got, want := len(bound.Sequence().Nodes), len(unbound.Sequence().Nodes); got != want {
		t.Fatalf("bound sequence has %d nodes, want the %d of the same markup written without a binding", got, want)
	}
	for i, node := range bound.Sequence().Nodes {
		if other := unbound.Sequence().Nodes[i]; node.Kind != other.Kind || node.Text != other.Text {
			t.Fatalf("node %d = %+v, want %+v", i, node, other)
		}
	}
}

// A failing binding ends the render and the error reaches the caller. Unlike an
// async external, whose failure an await clause can route to recover, a
// synchronous one has no boundary to recover at: it has nowhere to go but out.
func TestValErrEndsTheRender(t *testing.T) {
	want := errors.New("load failed")
	body := Builder[valScope]{}
	plan := &Plan[valParams]{Ops: []Op[valParams]{
		Builder[valParams]{}.Static("<main>"),
		ValErr(
			func(p valParams) (string, error) { return "", want },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{body.Static("<h1>"), body.Text(func(p valScope) string { return p.Value })}),
	}}
	var out strings.Builder
	err := Render(&out, Bind(plan, valParams{ID: "7"}))
	if !errors.Is(err, want) {
		t.Fatalf("render error = %v, want %v", err, want)
	}
	// Nothing is written before the value is computed, so the binding's subtree
	// is absent rather than half-rendered.
	if strings.Contains(out.String(), "<h1>") {
		t.Fatalf("the failed binding rendered part of its body: %q", out.String())
	}
}

// A binding that succeeds is the ordinary path, and the error result costs it
// nothing.
func TestValErrRendersWhenTheCallSucceeds(t *testing.T) {
	body := Builder[valScope]{}
	plan := &Plan[valParams]{Ops: []Op[valParams]{
		ValErr(
			func(p valParams) (string, error) { return "loaded-" + p.ID, nil },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{body.Text(func(p valScope) string { return p.Value })}),
	}}
	var out strings.Builder
	if err := Render(&out, Bind(plan, valParams{ID: "7"})); err != nil {
		t.Fatalf("render: %v", err)
	}
	if out.String() != "loaded-7" {
		t.Fatalf("output = %q, want the bound value", out.String())
	}
}

// The context-carrying form fails the same way, and reaches the render context.
func TestValErrCtxFailsAndReadsTheContext(t *testing.T) {
	want := errors.New("cancelled")
	body := Builder[valScope]{}
	plan := &Plan[valParams]{Ops: []Op[valParams]{
		ValErrCtx(
			func(ctx context.Context, p valParams) (string, error) { return "", ctx.Err() },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{body.Text(func(p valScope) string { return p.Value })}),
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out strings.Builder
	if err := Render(&out, Bind(plan, valParams{}), WithContext(ctx)); err == nil {
		t.Fatalf("render succeeded on a cancelled context, want %v", want)
	}
}

// A failing binding still decomposes: the sequence walk evaluates nothing, so
// whether the call would fail cannot change the static half.
func TestValErrSplicesItsBodyIntoTheSequence(t *testing.T) {
	body := Builder[valScope]{}
	plan := &Plan[valParams]{Ops: []Op[valParams]{
		ValErr(
			func(p valParams) (string, error) { return p.ID, nil },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{body.Static("<h1>"), body.Text(func(p valScope) string { return p.Value }), body.Static("</h1>")}),
	}}
	if nodes := plan.Sequence().Nodes; len(nodes) != 3 || nodes[0].Kind != SeqStatic {
		t.Fatalf("sequence = %+v, want the body spliced in place", nodes)
	}
}

// The reason the prologue exists: a chain member's loader fails while nothing
// has been written, so the caller is still free to choose the status instead of
// having committed one with the shell.
func TestALeafsLeadingBindingFailsBeforeAnyByte(t *testing.T) {
	want := errors.New("no such record")
	body := Builder[valScope]{}
	leaf := &Plan[valParams]{Ops: []Op[valParams]{
		Builder[valParams]{}.Static(" "),
		ValErr(
			func(p valParams) (string, error) { return "", want },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{body.Static("<h1>"), body.Text(func(p valScope) string { return p.Value })}),
	}}
	var out strings.Builder
	err := Render(&out, Bind(leaf, valParams{ID: "7"}))
	if !errors.Is(err, want) {
		t.Fatalf("render error = %v, want %v", err, want)
	}
	// The static run written before the binding in source order is what the
	// prologue has to beat: nothing at all may reach the writer.
	if out.Len() != 0 {
		t.Fatalf("wrote %q before the loader failed, want nothing", out.String())
	}
}

// The value is computed once. Preparing it and then rendering must not call
// again, which is the difference between this and Plan.Check.
func TestAPreparedBindingIsNotRecomputed(t *testing.T) {
	calls := 0
	body := Builder[valScope]{}
	leaf := &Plan[valParams]{Ops: []Op[valParams]{
		Val(
			func(p valParams) string { calls++; return "loaded-" + p.ID },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{body.Text(func(p valScope) string { return p.Value })}),
	}}
	var out strings.Builder
	if err := Render(&out, Bind(leaf, valParams{ID: "7"})); err != nil {
		t.Fatalf("render: %v", err)
	}
	if calls != 1 {
		t.Fatalf("loader ran %d times, want once", calls)
	}
	if out.String() != "loaded-7" {
		t.Fatalf("output = %q, want the bound value", out.String())
	}
}

// A fragment that is not a chain member is never assembled, so its bindings are
// computed where they run and the render is unaffected.
func TestASlotFragmentIsNotPrepared(t *testing.T) {
	calls := 0
	body := Builder[valScope]{}
	plan := &Plan[valParams]{Ops: []Op[valParams]{
		Val(
			func(p valParams) string { calls++; return p.ID },
			func(p valParams, value string) valScope { return valScope{Outer: p, Value: value} },
			[]Op[valScope]{body.Text(func(p valScope) string { return p.Value })}),
	}}
	fragment := Bind(plan, valParams{ID: "9"})
	var out strings.Builder
	if err := Render(&out, fragment); err != nil {
		t.Fatalf("render: %v", err)
	}
	if calls != 1 || out.String() != "9" {
		t.Fatalf("calls = %d, output = %q", calls, out.String())
	}
}

type wrapperParams struct {
	Title    string
	Children Fragment
}

type wrapperScope struct {
	Outer wrapperParams
	Value string
}

// A wrapper is not prepared, because its parameters are not complete until the
// chain installs the child fragment. Its bindings therefore run where they
// stand — which has to still work, and has to still see the slot.
func TestAWrapperWithABindingRendersAroundItsChild(t *testing.T) {
	calls := 0
	body := Builder[wrapperScope]{}
	layout := &Plan[wrapperParams]{Ops: []Op[wrapperParams]{
		Val(
			func(p wrapperParams) string { calls++; return "banner-" + p.Title },
			func(p wrapperParams, v string) wrapperScope { return wrapperScope{Outer: p, Value: v} },
			[]Op[wrapperScope]{
				body.Static("<header>"),
				body.Text(func(p wrapperScope) string { return p.Value }),
				body.Static("</header>"),
				body.Slot(func(p wrapperScope) Fragment { return p.Outer.Children }, nil),
			}),
	}}
	leaf := &Plan[valParams]{Ops: []Op[valParams]{
		Builder[valParams]{}.Text(func(p valParams) string { return "page-" + p.ID }),
	}}

	var out strings.Builder
	wrapper := layout.BindWrapper(wrapperParams{Title: "home"},
		func(p *wrapperParams, children Fragment) { p.Children = children })
	if err := RenderChain(&out, []Wrapper{wrapper}, Bind(leaf, valParams{ID: "7"})); err != nil {
		t.Fatalf("render: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the layout's loader ran %d times, want once", calls)
	}
	// The slot has to survive. A prepared wrapper would have built its scope
	// before the chain installed the child, and this is what would have gone
	// missing.
	if want := "<header>banner-home</header>page-7"; out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

// Hoisting moves nodes into a binding's body, and the binding splices that body
// back into the sequence. The static half a client caches therefore has to be
// the same tree it would be with the binding written first, or two spellings of
// one page would disagree about their own address.
func TestHoistingDoesNotChangeTheSequence(t *testing.T) {
	body := Builder[valScope]{}
	plain := Builder[valParams]{}
	// The shape hoisting produces: markup that was written before the binding
	// now sits inside its body.
	hoisted := &Plan[valParams]{Ops: []Op[valParams]{
		Val(
			func(p valParams) string { return p.ID },
			func(p valParams, v string) valScope { return valScope{Outer: p, Value: v} },
			[]Op[valScope]{
				body.Static("<p>before</p><h1>"),
				body.Text(func(p valScope) string { return p.Value }),
				body.Static("</h1>"),
			}),
	}}
	// The same page with no binding at all.
	unbound := &Plan[valParams]{Ops: []Op[valParams]{
		plain.Static("<p>before</p><h1>"),
		plain.Text(func(p valParams) string { return p.ID }),
		plain.Static("</h1>"),
	}}
	if got, want := hoisted.Sequence().Address, unbound.Sequence().Address; got != want {
		t.Fatalf("address = %q, want the unbound page's %q", got, want)
	}
}

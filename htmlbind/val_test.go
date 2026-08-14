package htmlbind

import (
	"context"
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

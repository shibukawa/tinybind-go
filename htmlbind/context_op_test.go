package htmlbind

import (
	"context"
	"strings"
	"testing"
)

type ctxKey struct{}

// tokenFrom is the shape a framework's per-request value provider has: it reads
// request-scoped state the caller's middleware installed, and writes nothing.
func tokenFrom(ctx context.Context) string {
	value, _ := ctx.Value(ctxKey{}).(string)
	if value == "" {
		return "no-context"
	}
	return value
}

type ctxParams struct{ Label string }

func TestContextOpsReadTheRenderContext(t *testing.T) {
	ops := Builder[ctxParams]{}
	plan := &Plan[ctxParams]{Ops: []Op[ctxParams]{
		ops.Static("<form"),
		ops.AttrCtx("data-token", func(ctx context.Context, p ctxParams) (string, bool) {
			return Escape(tokenFrom(ctx)), true
		}),
		ops.BoolAttrCtx("hidden", func(ctx context.Context, p ctxParams) bool {
			return tokenFrom(ctx) != "no-context"
		}),
		ops.Static(">"),
		ops.TextCtx(func(ctx context.Context, p ctxParams) string { return tokenFrom(ctx) }),
		ops.IfCtx(func(ctx context.Context, p ctxParams) bool { return tokenFrom(ctx) != "" },
			[]Op[ctxParams]{ops.Static("<b>on</b>")}, nil),
		ops.Static("</form>"),
	}}

	var out strings.Builder
	ctx := context.WithValue(context.Background(), ctxKey{}, "tok-1")
	if err := Render(&out, Bind(plan, ctxParams{}), WithContext(ctx)); err != nil {
		t.Fatal(err)
	}
	want := `<form data-token="tok-1" hidden>tok-1<b>on</b></form>`
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

// TestContextOpsEscapeLikeTheirPlainForms is the constraint that reading the
// request changes nothing about escaping: a hostile token cannot leave its
// attribute or introduce markup.
func TestContextOpsEscapeLikeTheirPlainForms(t *testing.T) {
	ops := Builder[ctxParams]{}
	plan := &Plan[ctxParams]{Ops: []Op[ctxParams]{
		ops.Static("<input value=\""),
		ops.TextCtx(func(ctx context.Context, p ctxParams) string { return tokenFrom(ctx) }),
		ops.Static("\">"),
	}}
	var out strings.Builder
	ctx := context.WithValue(context.Background(), ctxKey{}, `"><script>x()</script>`)
	if err := Render(&out, Bind(plan, ctxParams{}), WithContext(ctx)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "<script>") {
		t.Errorf("a token broke out of its attribute: %s", out.String())
	}
}

// TestContextOpsWithNoContextOption covers the render that supplied none: the
// context always exists, so a context-taking external can never fail for want
// of one the way a registered element's provider must.
func TestContextOpsWithNoContextOption(t *testing.T) {
	ops := Builder[ctxParams]{}
	plan := &Plan[ctxParams]{Ops: []Op[ctxParams]{
		ops.TextCtx(func(ctx context.Context, p ctxParams) string {
			if ctx == nil {
				t.Error("a nil context reached a plan closure")
			}
			return tokenFrom(ctx)
		}),
	}}
	var out strings.Builder
	if err := Render(&out, Bind(plan, ctxParams{})); err != nil {
		t.Fatal(err)
	}
	if out.String() != "no-context" {
		t.Errorf("got %q, want %q", out.String(), "no-context")
	}
}

// TestSlotCtxRendersAFragment covers the html-returning form: the value is
// rendered as a subtree rather than escaped as text, which is what lets a
// framework return a whole hidden input instead of a bare token.
func TestSlotCtxRendersAFragment(t *testing.T) {
	inner := Builder[ctxParams]{}
	field := &Plan[ctxParams]{Ops: []Op[ctxParams]{
		inner.Static(`<input type="hidden" name="csrf" value="`),
		inner.TextCtx(func(ctx context.Context, p ctxParams) string { return tokenFrom(ctx) }),
		inner.Static(`">`),
	}}

	ops := Builder[ctxParams]{}
	plan := &Plan[ctxParams]{Ops: []Op[ctxParams]{
		ops.SlotCtx(func(ctx context.Context, p ctxParams) Fragment {
			return Bind(field, ctxParams{})
		}, nil),
	}}

	var out strings.Builder
	ctx := context.WithValue(context.Background(), ctxKey{}, "tok-2")
	if err := Render(&out, Bind(plan, ctxParams{}), WithContext(ctx)); err != nil {
		t.Fatal(err)
	}
	want := `<input type="hidden" name="csrf" value="tok-2">`
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

// TestContextOpsInsideAnAwaitBoundary covers the position that has its own
// context: a boundary subtree renders under the boundary's context, so work a
// context-taking external starts there is bounded by that boundary.
func TestContextOpsInsideAnAwaitBoundary(t *testing.T) {
	type scope struct {
		Outer ctxParams
		Value string
	}
	inner := Builder[scope]{}
	ops := Builder[ctxParams]{}
	plan := &Plan[ctxParams]{Ops: []Op[ctxParams]{
		Await(
			func(ctx context.Context, p ctxParams) (scope, error) {
				return scope{Outer: p, Value: "settled"}, nil
			},
			func(p ctxParams, err AsyncError) ctxParams { return p },
			[]Op[scope]{
				inner.TextCtx(func(ctx context.Context, s scope) string {
					return s.Value + ":" + tokenFrom(ctx)
				}),
			},
			[]Op[ctxParams]{ops.Static("pending")},
			nil,
		),
	}}

	var out strings.Builder
	ctx := context.WithValue(context.Background(), ctxKey{}, "tok-3")
	if err := Render(&out, Bind(plan, ctxParams{}), WithContext(ctx)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "settled:tok-3") {
		t.Errorf("the boundary subtree did not see the render context: %s", out.String())
	}
}

// TestForCtxReadsTheContext covers the loop list, which is the one context form
// that is a package-level function rather than a builder method.
func TestForCtxReadsTheContext(t *testing.T) {
	type item struct {
		Outer ctxParams
		Item  string
		Index int
	}
	inner := Builder[item]{}
	plan := &Plan[ctxParams]{Ops: []Op[ctxParams]{
		ForCtx(
			func(ctx context.Context, p ctxParams) []string {
				return strings.Split(tokenFrom(ctx), ",")
			},
			func(p ctxParams, value string, index int) item {
				return item{Outer: p, Item: value, Index: index}
			},
			[]Op[item]{inner.Text(func(i item) string { return i.Item })},
		),
	}}
	var out strings.Builder
	ctx := context.WithValue(context.Background(), ctxKey{}, "a,b,c")
	if err := Render(&out, Bind(plan, ctxParams{}), WithContext(ctx)); err != nil {
		t.Fatal(err)
	}
	if out.String() != "abc" {
		t.Errorf("got %q, want %q", out.String(), "abc")
	}
}

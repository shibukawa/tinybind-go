package htmlbind

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// shellPlan is a document shell: it owns the head position, so whatever the
// merge produced is written there and nowhere else.
func shellPlan(head []string) (*Plan[struct{ Children Fragment }], Builder[struct{ Children Fragment }]) {
	type shellParams = struct{ Children Fragment }
	ops := Builder[shellParams]{}
	return &Plan[shellParams]{
		Head: head,
		Ops: []Op[shellParams]{
			ops.Static("<html><head>"),
			ops.MergedHead(),
			ops.Static("</head><body>"),
			ops.Slot(func(p shellParams) Fragment { return p.Children }, nil),
			ops.Static("</body></html>"),
		},
	}, ops
}

func bodyFragment(head []string, text string) Fragment {
	ops := Builder[struct{}]{}
	return Bind(&Plan[struct{}]{Head: head, Ops: []Op[struct{}]{ops.Static(text)}}, struct{}{})
}

func renderShell(t *testing.T, page Fragment, options ...Option) string {
	t.Helper()
	plan, _ := shellPlan(nil)
	shell := BindWrapper(plan, struct{ Children Fragment }{}, func(p *struct{ Children Fragment }, children Fragment) {
		p.Children = children
	})
	var out strings.Builder
	if err := RenderChain(&out, []Wrapper{shell}, page, options...); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestCallerHeadJoinsTheMerge(t *testing.T) {
	page := bodyFragment([]string{`<link rel="stylesheet" href="/a.css">`}, "body")
	got := renderShell(t, page, WithHead(
		HeadTitle("Order 42"),
		HeadMeta(HeadAttr{Name: "name", Value: "description"}, HeadAttr{Name: "content", Value: "an order"}),
	))
	want := `<html><head><link rel="stylesheet" href="/a.css"><title>Order 42</title>` +
		`<meta name="description" content="an order"></head><body>body</body></html>`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestCallerHeadIsInnermost pins the position: a component's contribution comes
// first, so a caller's tag can depend on it.
func TestCallerHeadIsInnermost(t *testing.T) {
	page := bodyFragment([]string{`<link rel="stylesheet" href="/a.css">`}, "body")
	got := renderShell(t, page, WithHead(HeadLink(
		HeadAttr{Name: "rel", Value: "stylesheet"},
		HeadAttr{Name: "href", Value: "/b.css"},
	)))
	if strings.Index(got, "/a.css") > strings.Index(got, "/b.css") {
		t.Errorf("the caller's tag was written before the component's: %s", got)
	}
}

func TestCallerHeadDeduplicatesAgainstComponents(t *testing.T) {
	tag := `<link rel="stylesheet" href="/a.css">`
	page := bodyFragment([]string{tag}, "body")
	got := renderShell(t, page, WithHead(HeadLink(
		HeadAttr{Name: "rel", Value: "stylesheet"},
		HeadAttr{Name: "href", Value: "/a.css"},
	)))
	if strings.Count(got, "/a.css") != 1 {
		t.Errorf("a tag a component already declared was written twice: %s", got)
	}
}

// TestNoCallerHeadIsUnchanged is the unused-is-free rule for this channel.
func TestNoCallerHeadIsUnchanged(t *testing.T) {
	page := bodyFragment([]string{`<link rel="stylesheet" href="/a.css">`}, "body")
	if renderShell(t, page) != renderShell(t, page, WithHead()) {
		t.Error("supplying an empty contribution list changed the output")
	}
}

// TestCallerHeadEscapesValues covers the reason the channel carries values
// rather than markup: nothing a caller passes can become an element.
func TestCallerHeadEscapesValues(t *testing.T) {
	page := bodyFragment(nil, "body")
	got := renderShell(t, page, WithHead(
		HeadTitle(`</title><script>x()</script>`),
		HeadMeta(HeadAttr{Name: "content", Value: `"><script>y()</script>`}),
	))
	if strings.Contains(got, "<script>") {
		t.Errorf("a caller value became markup: %s", got)
	}
}

func TestCallerHeadRejectsInlineScript(t *testing.T) {
	page := bodyFragment(nil, "body")
	plan, _ := shellPlan(nil)
	shell := BindWrapper(plan, struct{ Children Fragment }{}, func(p *struct{ Children Fragment }, children Fragment) {
		p.Children = children
	})
	err := RenderChain(io.Discard, []Wrapper{shell}, page, WithHead(HeadScript(
		HeadAttr{Name: "type", Value: "module"},
	)))
	if !errors.Is(err, ErrHeadNode) {
		t.Fatalf("a script with no src was accepted: %v", err)
	}
	if !strings.Contains(err.Error(), "src") {
		t.Errorf("the diagnostic does not name the cause: %v", err)
	}
}

// TestCallerHeadFailsBeforeTheFirstByte is why the check is at the render entry:
// a rejected contribution must still leave the caller free to send a status.
func TestCallerHeadFailsBeforeTheFirstByte(t *testing.T) {
	page := bodyFragment(nil, "body")
	plan, _ := shellPlan(nil)
	shell := BindWrapper(plan, struct{ Children Fragment }{}, func(p *struct{ Children Fragment }, children Fragment) {
		p.Children = children
	})
	var out strings.Builder
	err := RenderChain(&out, []Wrapper{shell}, page, WithHead(
		HeadMeta(HeadAttr{Name: "bad name", Value: "x"}),
	))
	if err == nil {
		t.Fatal("a malformed attribute name was accepted")
	}
	if out.Len() != 0 {
		t.Errorf("bytes were written before the failure: %q", out.String())
	}
}

func TestCallerHeadNoScript(t *testing.T) {
	page := bodyFragment(nil, "body")
	got := renderShell(t, page, WithHead(HeadNoScript(HeadMeta(
		HeadAttr{Name: "http-equiv", Value: "refresh"},
		HeadAttr{Name: "content", Value: "0; url=/_handoff"},
	))))
	want := `<noscript><meta http-equiv="refresh" content="0; url=/_handoff"></noscript>`
	if !strings.Contains(got, want) {
		t.Errorf("got %s\nwant it to contain %s", got, want)
	}
}

func TestCallerHeadNoScriptRejectsBodyContent(t *testing.T) {
	if _, err := RenderHeadNodes([]HeadNode{HeadNoScript(HeadTitle("x"))}); !errors.Is(err, ErrHeadNode) {
		t.Fatalf("a title inside noscript was accepted: %v", err)
	}
}

// TestRenderHeadNodesForTheFragmentPath covers the caller answering a request
// with no document shell: it can see its own contributions and decide, rather
// than have them silently go nowhere.
func TestRenderHeadNodesForTheFragmentPath(t *testing.T) {
	tags, err := RenderHeadNodes([]HeadNode{HeadTitle("Order 42")})
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "<title>Order 42</title>" {
		t.Errorf("unexpected tags: %q", tags)
	}
}

func TestCallerHeadOnTheAsyncEntry(t *testing.T) {
	page := bodyFragment(nil, "body")
	plan, _ := shellPlan(nil)
	shell := BindWrapper(plan, struct{ Children Fragment }{}, func(p *struct{ Children Fragment }, children Fragment) {
		p.Children = children
	})
	var out strings.Builder
	for _, err := range RenderChainAsync(context.Background(), &out, []Wrapper{shell}, page,
		WithHead(HeadTitle("Order 42"))) {
		if err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(out.String(), "<title>Order 42</title>") {
		t.Errorf("the streaming entry dropped the caller's contribution: %s", out.String())
	}
}

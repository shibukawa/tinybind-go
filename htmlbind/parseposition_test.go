package htmlbind

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// An await boundary commits a fallback the browser parses as the document
// arrives, so where that fallback ends up is decided by the parser and not by
// this package.
//
// It used to be wrapped in an unknown element. In table context the tree
// construction algorithm foster-parents such an element out to just before the
// table and leaves the rows it was wrapping inside — so the marker and its own
// fallback were separated, and a client settling the boundary by that marker
// wrote the finished row outside the table while the fallback stayed in the list
// forever. Unlike the hole in a delta, there is no downstream workaround: a
// caller cannot rewrite a document the browser is parsing as it arrives.
//
// Comment markers are what a table keeps. A template is kept too, but a template
// does not render its content, and a fallback visible without JavaScript is what
// this path exists for.

// tableAwaitPlan holds one await boundary whose fallback is a table row.
func tableAwaitPlan() *Plan[struct{}] {
	builder := Builder[struct{}]{}
	return &Plan[struct{}]{Ops: []Op[struct{}]{
		builder.Static("<table><tbody>"),
		Await(
			func(ctx context.Context, _ struct{}) (string, error) {
				var value string
				return value, Concurrent(ctx, func() error { value = "settled"; return nil })
			},
			func(_ struct{}, err AsyncError) AsyncError { return err },
			[]Op[string]{Builder[string]{}.Static("<tr><td>done</td></tr>")},
			[]Op[struct{}]{builder.Static("<tr><td>loading</td></tr>")},
			nil,
		),
		builder.Static("</tbody></table>"),
	}}
}

// initialPass renders and returns what was written before the first boundary
// settled, which is the document the browser parses.
func initialPass(t *testing.T, plan *Plan[struct{}]) string {
	t.Helper()
	var output bytes.Buffer
	var initial string
	for _, err := range RenderAsync(context.Background(), &output, Bind(plan, struct{}{})) {
		if err != nil {
			t.Fatal(err)
		}
		if initial == "" {
			initial = output.String()
		}
	}
	if initial == "" {
		t.Fatal("nothing was written before the first completion")
	}
	return initial
}

// parentsOf names the element each comment and each row ends up under.
func parentsOf(t *testing.T, fragment string) (comments map[string]string, rows []string) {
	t.Helper()
	document, err := html.Parse(strings.NewReader("<!doctype html><html><body>" + fragment))
	if err != nil {
		t.Fatal(err)
	}
	comments = map[string]string{}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			switch {
			case child.Type == html.CommentNode:
				comments[child.Data] = node.Data
			case child.Type == html.ElementNode && child.Data == "tr":
				rows = append(rows, node.Data)
			}
			walk(child)
		}
	}
	walk(document)
	return comments, rows
}

func TestAnAwaitFallbackInsideATableStaysInsideIt(t *testing.T) {
	comments, rows := parentsOf(t, initialPass(t, tableAwaitPlan()))
	for _, marker := range []string{"tb:tb-1", "/tb:tb-1"} {
		if comments[marker] != "tbody" {
			t.Fatalf("marker %q parsed under %q, want tbody: %v", marker, comments[marker], comments)
		}
	}
	if len(rows) != 1 || rows[0] != "tbody" {
		t.Fatalf("the fallback row parsed under %v, want one row under tbody", rows)
	}
}

// The markers have to survive together with the fallback. Separating them is the
// failure this shape exists to prevent: a marker outside the table and rows
// inside it means the settled content replaces the wrong range.
func TestTheAwaitMarkersBracketTheirOwnFallback(t *testing.T) {
	initial := initialPass(t, tableAwaitPlan())
	open := strings.Index(initial, "<!--tb:tb-1-->")
	fallback := strings.Index(initial, "<tr><td>loading</td></tr>")
	closing := strings.Index(initial, "<!--/tb:tb-1-->")
	if open < 0 || fallback < 0 || closing < 0 {
		t.Fatalf("the boundary did not commit its fallback between its markers:\n%s", initial)
	}
	if !(open < fallback && fallback < closing) {
		t.Fatalf("the fallback is not between the markers:\n%s", initial)
	}
}

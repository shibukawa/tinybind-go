package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// text builds a fragment holding literal markup, standing in for content a
// caller would normally write in a template.
func text(value string) htmlbind.Fragment {
	return htmlbind.Bind(&htmlbind.Plan[struct{}]{
		Ops: []htmlbind.Op[struct{}]{htmlbind.Builder[struct{}]{}.Static(value)},
	}, struct{}{})
}

func TestNamedAndUnnamedSlots(t *testing.T) {
	var output bytes.Buffer
	if err := htmlbind.Render(&output, Page(PageParams{Caption: "Docs"})); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	for _, want := range []string{
		`<div class="head"><em>Guide</em></div>`,
		`<p>body text</p>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("output %q does not contain %q", body, want)
		}
	}
	// The omitted footer slot leaves nothing behind.
	if strings.Contains(body, "<slot") {
		t.Fatalf("slot element reached the output: %q", body)
	}
}

func TestOptionalSlotFallsBackToDefaultContent(t *testing.T) {
	var output bytes.Buffer
	fragment := renderPanel(renderPanelParams{Title: "A&B", Children: text("main")})
	if err := htmlbind.Render(&output, fragment); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	if !strings.Contains(body, `<div class="head"><b>A&amp;B</b></div>`) {
		t.Fatalf("default slot content missing from %q", body)
	}
	if !strings.Contains(body, `<div class="body">main</div>`) {
		t.Fatalf("required slot content missing from %q", body)
	}
}

package pages

import (
	"bytes"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

func TestRenderedOutput(t *testing.T) {
	var output bytes.Buffer
	fragment := Document(DocumentParams{
		Markup:     "<b>raw</b>",
		Css:        "body > p { color: red; }",
		Javascript: "window.ready = true;",
		Payload:    Payload{Message: "<unsafe>&", Count: 2, Enabled: true},
	})
	if err := htmlbind.Render(&output, fragment); err != nil {
		t.Fatal(err)
	}
	// Markup whitespace collapses; the script and style bodies stay verbatim.
	want := " <b>raw</b> <style>body > p { color: red; }</style> " +
		"<script>window.ready = true;</script> " +
		`<script>window.payload = {"message":"\u003cunsafe\u003e\u0026","count":2,"enabled":true};</script>` + " "
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

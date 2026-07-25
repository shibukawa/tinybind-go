package pages

import (
	"strings"
	"testing"
)

func TestRenderedOutput(t *testing.T) {
	var output strings.Builder
	err := Document(&output, DocumentParams{
		Markup:     "<b>raw</b>",
		Css:        "body > p { color: red; }",
		Javascript: "window.ready = true;",
		Payload:    Payload{Message: "<unsafe>&", Count: 2, Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "\n<b>raw</b>\n<style>body > p { color: red; }</style>\n" +
		"<script>window.ready = true;</script>\n" +
		`<script>window.payload = {"message":"\u003cunsafe\u003e\u0026","count":2,"enabled":true};</script>` + "\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

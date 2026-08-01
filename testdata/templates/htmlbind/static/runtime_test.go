package pages

import (
	"bytes"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

func TestRenderedOutput(t *testing.T) {
	var output bytes.Buffer
	if err := htmlbind.Render(&output, Hello(HelloParams{})); err != nil {
		t.Fatal(err)
	}
	// The document body drops its formatting whitespace entirely; the parser
	// discards a run before the doctype anyway.
	want := "<!DOCTYPE html><h1>Hello &amp; welcome</h1>"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

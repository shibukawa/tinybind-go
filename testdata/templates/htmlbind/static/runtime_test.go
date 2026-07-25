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
	want := "\n<!DOCTYPE html>\n<h1>Hello &amp; welcome</h1>\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

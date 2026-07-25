package pages

import (
	"bytes"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

func Decorate(value string, tone Tone) string {
	return string(tone) + ":" + value
}

func TestRenderedOutput(t *testing.T) {
	var output bytes.Buffer
	if err := htmlbind.Render(&output, Label(LabelParams{Value: "<value>", Tone: TonePrimary})); err != nil {
		t.Fatal(err)
	}
	want := "\n<span>Primary:&lt;value&gt;</span>\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

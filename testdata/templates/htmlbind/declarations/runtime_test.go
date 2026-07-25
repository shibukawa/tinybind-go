package pages

import (
	"strings"
	"testing"
)

func Decorate(value string, tone Tone) string {
	return string(tone) + ":" + value
}

func TestRenderedOutput(t *testing.T) {
	var output strings.Builder
	if err := Label(&output, LabelParams{Value: "<value>", Tone: TonePrimary}); err != nil {
		t.Fatal(err)
	}
	want := "\n<span>Primary:&lt;value&gt;</span>\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

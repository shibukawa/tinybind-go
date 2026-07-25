package pages

import (
	"strings"
	"testing"
)

func TestRenderedOutput(t *testing.T) {
	var output strings.Builder
	if err := Card(&output, CardParams{User: User{Name: "A&B"}}); err != nil {
		t.Fatal(err)
	}
	want := `<span class="badge"><strong>A&amp;B</strong><em>member</em></span>`
	if !strings.Contains(output.String(), want) {
		t.Fatalf("output %q does not contain %q", output.String(), want)
	}
}

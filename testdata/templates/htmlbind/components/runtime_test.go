package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

func TestRenderedOutput(t *testing.T) {
	var output bytes.Buffer
	if err := htmlbind.Render(&output, Card(CardParams{User: User{Name: "A&B"}})); err != nil {
		t.Fatal(err)
	}
	want := `<span class="badge"><strong>A&amp;B</strong><em>member</em></span>`
	if !strings.Contains(output.String(), want) {
		t.Fatalf("output %q does not contain %q", output.String(), want)
	}
}

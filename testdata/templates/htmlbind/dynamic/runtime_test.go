package pages

import (
	"bytes"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

func TestRenderedOutput(t *testing.T) {
	nickname := "A & B"
	profileURL, err := url.Parse("https://example.com/profile?q=a&lang=en")
	if err != nil {
		t.Fatal(err)
	}
	user := User{
		Name:       "<Ada>",
		Active:     true,
		Nickname:   &nickname,
		ProfileURL: *profileURL,
		Tags:       []string{"go", "<html>"},
	}
	var output bytes.Buffer
	if err := htmlbind.Render(&output, Profile(ProfileParams{User: user})); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, want := range []string{
		`title="A &amp; B"`,
		`href="https://example.com/profile?q=a&amp;lang=en"`,
		`&lt;Ada&gt;`,
		`<li data-index="0">go</li>`,
		`<li data-index="1">&lt;html&gt;</li>`,
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("output %q does not contain %q", rendered, want)
		}
	}
}

func TestInactiveUserTakesTheElseBranch(t *testing.T) {
	var output bytes.Buffer
	user := User{Name: "Bob", ProfileURL: url.URL{Scheme: "https", Host: "example.com"}}
	if err := htmlbind.Render(&output, Profile(ProfileParams{User: user})); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	if !strings.Contains(rendered, "<p>inactive</p>") {
		t.Fatalf("else branch missing from %q", rendered)
	}
	if !strings.Contains(rendered, "<article hidden>") {
		t.Fatalf("boolean attribute missing from %q", rendered)
	}
	if strings.Contains(rendered, "title=") {
		t.Fatalf("absent optional attribute was emitted: %q", rendered)
	}
}

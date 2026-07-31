package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// A scriptless-client handoff is one noscript refresh in the head. It is the
// only contributed element with element children, and HTML permits it there
// around a link, a style, or a meta.
func TestHeadContributionAcceptsNoscript(t *testing.T) {
	source := `package pages

export component Page(): html {
<head>
<noscript><meta http-equiv="refresh" content="0; url=/_handoff"></noscript>
</head>
<div>page</div>
}
`
	generated, err := htmlbind.Generate("noscript.pw.html", []byte(source), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := `<noscript><meta http-equiv=\"refresh\" content=\"0; url=/_handoff\"></noscript>`
	if !strings.Contains(string(generated), want) {
		t.Errorf("the contribution is missing from the plan head\n%s", generated)
	}
}

func TestHeadNoscriptRejectsBodyContent(t *testing.T) {
	source := `package pages

export component Page(): html {
<head>
<noscript><p>enable scripting</p></noscript>
</head>
<div>page</div>
}
`
	_, err := htmlbind.Generate("noscript.pw.html", []byte(source), htmlbind.GenerateOptions{})
	if err == nil {
		t.Fatal("body content inside a head noscript was accepted")
	}
	if !strings.Contains(err.Error(), "noscript cannot contain p") {
		t.Errorf("the diagnostic does not name the element: %v", err)
	}
}

func TestHeadNoscriptKeepsAttributesStatic(t *testing.T) {
	source := `package pages

export component Page(url: string): html {
<head>
<noscript><meta http-equiv="refresh" content="{url}"></noscript>
</head>
<div>page</div>
}
`
	_, err := htmlbind.Generate("noscript.pw.html", []byte(source), htmlbind.GenerateOptions{})
	if err == nil {
		t.Fatal("a request-dependent attribute inside a head noscript was accepted")
	}
	if !strings.Contains(err.Error(), "static") {
		t.Errorf("the diagnostic does not name the cause: %v", err)
	}
}

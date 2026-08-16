package routetree

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// TestSeamsReachThePageTreePath is the coverage check the context-external seam
// did not have. A page tree is a second compile path, so an option filled only
// where templates are compiled as a package leaves every filesystem route
// without the feature, silently.
//
// See .knowledge requirement:route-package-context-externals.
func TestSeamsReachThePageTreePath(t *testing.T) {
	const page = `messages about

export component Page(): html {<h1>{t title}</h1><p>{lang}</p>}
`
	result := generateTree(t, tree(t, map[string]string{"page.tb.html": page}), func(o *GenerateOptions) {
		o.ImplicitBindings = []htmlbind.ImplicitBinding{{
			Name:     "lang",
			Provider: htmlbind.BindingProvider{Package: "example.com/app/framework", Name: "Lang"},
		}}
		o.Messages = map[string]htmlbind.MessageSymbol{
			"about.title": {Package: "example.com/app/messages", Name: "AboutTitle"},
		}
		o.MessageContextBinding = "lang"
	})
	source := pageSource(t, result.Files)
	for _, want := range []string{
		"messages.AboutTitle(framework.Lang(ctx))",
		`"example.com/app/messages"`,
		`"example.com/app/framework"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("the page tree path did not carry the seam; output lacks %q:\n%s", want, source)
		}
	}
}

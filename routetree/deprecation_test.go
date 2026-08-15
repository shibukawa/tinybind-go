package routetree_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/routetree"
	templatehtml "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// The typed rung still generates and still works. It is reported so a build can
// say so, because this package writes nothing and prints nothing: the caller
// owns the output, so it owns how a warning reaches whoever ran the build.
func TestTheTypedRungIsReportedAsDeprecated(t *testing.T) {
	result, err := routetree.GenerateTree(routetree.GenerateOptions{
		Config: routetree.Config{
			Root:       "../internal/pagesfixture/pages",
			ImportBase: "github.com/shibukawa/tinybind-go/internal/pagesfixture/pages",
		},
		RootPackage: "pages",
		ScriptResolver: func(_ string, scripts []templatehtml.ComponentScript) (routetree.ScriptAnswers, error) {
			answers := routetree.ScriptAnswers{
				Handlers:   map[string]templatehtml.ClientHandlerSet{},
				Parameters: map[string][]string{},
			}
			for _, script := range scripts {
				answers.Handlers[script.Component] = templatehtml.ClientHandlerSet{Resolved: []string{"reload"}}
				answers.Parameters[script.Component] = script.Parameters
			}
			return answers, nil
		},
	})
	if err != nil {
		t.Fatalf("GenerateTree: %v", err)
	}
	byPath := map[string]string{}
	for _, deprecation := range result.Deprecations {
		byPath[deprecation.Path] = deprecation.Message
	}
	// The two typed pages of the fixture are reported.
	for _, want := range []string{"archive", "users"} {
		found := false
		for path, message := range byPath {
			if strings.Contains(path, want) {
				found = true
				if !strings.Contains(message, "{val} binding") {
					t.Errorf("%s: message does not say what to write instead: %q", path, message)
				}
			}
		}
		if !found {
			t.Errorf("the typed page under %s was not reported: %v", want, byPath)
		}
	}
	// The page that loads its own data is not, because it is already the shape
	// the advisory points at.
	for path := range byPath {
		if strings.Contains(path, "records") {
			t.Errorf("a rung 1 page was reported as deprecated: %s", path)
		}
	}
	// Generation is unaffected: a deprecated tree still emits everything.
	if len(result.Files) == 0 {
		t.Fatal("the deprecation stopped generation")
	}
}

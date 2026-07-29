package routetree

import (
	"path/filepath"
	"strings"
	"testing"
)

const actionSource = `package id_

import "net/http"

// Load is the page's own entry point at rung 3, so it shares the handler
// signature without ever being a server action.
func Load(w http.ResponseWriter, r *http.Request) {}

func Rename(w http.ResponseWriter, r *http.Request) {}

func Delete(w http.ResponseWriter, r *http.Request) {}

// unexported, so generated code in another package cannot reach it
func internalOnly(w http.ResponseWriter, r *http.Request) {}

// exported but not handler-shaped
func Helper(id string) string { return id }

// exported, handler-shaped receiver method rather than a function
type Svc struct{}

func (Svc) Handle(w http.ResponseWriter, r *http.Request) {}
`

func actionsIn(t *testing.T, files map[string]string, dir, relDir string) []Action {
	t.Helper()
	root := tree(t, files)
	got, err := DiscoverActions(filepath.Join(root, filepath.FromSlash(dir)), relDir, "id_", "example.com/m/pages/"+relDir, "")
	if err != nil {
		t.Fatalf("DiscoverActions: %v", err)
	}
	return got
}

func actionNames(actions []Action) []string {
	out := make([]string, len(actions))
	for i, action := range actions {
		out[i] = action.Name
	}
	return out
}

func TestDiscoverActionsTakesExportedHandlersOnly(t *testing.T) {
	got := actionsIn(t, map[string]string{
		"users/id_/page.tb.html": "",
		"users/id_/page.go":      actionSource,
	}, "users/id_", "users/id_")

	want := []string{"Delete", "Rename"}
	if names := actionNames(got); strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("actions = %v, want %v", names, want)
	}
}

func TestDiscoverActionsReadsEveryFileOfThePackage(t *testing.T) {
	got := actionsIn(t, map[string]string{
		"users/id_/page.tb.html": "",
		"users/id_/page.go":      "package id_\n\nimport \"net/http\"\n\nfunc Rename(w http.ResponseWriter, r *http.Request) {}\n",
		"users/id_/extra.go":     "package id_\n\nimport \"net/http\"\n\nfunc Publish(w http.ResponseWriter, r *http.Request) {}\n",
		// A test file is not part of the served package surface.
		"users/id_/page_test.go": "package id_\n\nimport \"net/http\"\n\nfunc Leaked(w http.ResponseWriter, r *http.Request) {}\n",
	}, "users/id_", "users/id_")

	want := []string{"Publish", "Rename"}
	if names := actionNames(got); strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("actions = %v, want %v", names, want)
	}
}

func TestDiscoverActionsHonorsAnImportAlias(t *testing.T) {
	got := actionsIn(t, map[string]string{
		"users/id_/page.tb.html": "",
		"users/id_/page.go":      "package id_\n\nimport nh \"net/http\"\n\nfunc Rename(w nh.ResponseWriter, r *nh.Request) {}\n",
	}, "users/id_", "users/id_")

	if names := actionNames(got); len(names) != 1 || names[0] != "Rename" {
		t.Errorf("actions = %v, want [Rename]", names)
	}
}

func TestActionPathCarriesHashAndReadableName(t *testing.T) {
	got := actionsIn(t, map[string]string{
		"users/id_/page.tb.html": "",
		"users/id_/page.go":      "package id_\n\nimport \"net/http\"\n\nfunc Rename(w http.ResponseWriter, r *http.Request) {}\n",
	}, "users/id_", "users/id_")

	action := got[0]
	if want := "/_action/" + action.Hash + "/Rename"; action.Path != want {
		t.Errorf("Path = %q, want %q", action.Path, want)
	}
	if len(action.Hash) != ActionHashLength {
		t.Errorf("Hash = %q, want %d characters", action.Hash, ActionHashLength)
	}
	if action.Pattern() != "POST "+action.Path {
		t.Errorf("Pattern = %q", action.Pattern())
	}
}

func TestActionHashIsDeterministicAndPrefixIndependent(t *testing.T) {
	first := ActionHash("users/id_", "Rename")
	if second := ActionHash("users/id_", "Rename"); first != second {
		t.Errorf("hash is not stable: %q then %q", first, second)
	}
	// The declaring directory and the name are the whole input, so remounting
	// under a different prefix leaves the identity underneath unchanged.
	if other := ActionHash("users/id_", "Delete"); other == first {
		t.Errorf("two names share a hash: %q", first)
	}
	if other := ActionHash("posts/id_", "Rename"); other == first {
		t.Errorf("two directories share a hash: %q", first)
	}
}

func TestValidateActionPrefixRejectsUnusablePrefixes(t *testing.T) {
	root := tree(t, map[string]string{
		"page.tb.html":       "",
		"admin/page.tb.html": "",
	})
	discovered, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	cases := []struct {
		name   string
		prefix string
		want   string
	}{
		{"relative", "_action", "must begin with /"},
		{"dynamic segment", "/a/{id}", "cannot contain a dynamic segment"},
		{"unclean", "/a//b", "not a clean path"},
		{"shadows a route", "/admin", "lies under the action prefix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateActionPrefix(tc.prefix, discovered)
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}

	// The default is safe by construction, because discovery ignores a
	// directory beginning with an underscore.
	if err := ValidateActionPrefix(DefaultActionPrefix, discovered); err != nil {
		t.Errorf("default prefix rejected: %v", err)
	}
}

func TestCheckActionCollisionsNamesBothDeclarations(t *testing.T) {
	actions := []Action{
		{Name: "Rename", Hash: "abc123abc123", File: "a.go", Line: 3},
		{Name: "Other", Hash: "abc123abc123", File: "b.go", Line: 9},
	}
	errs := checkActionCollisions(actions)
	if len(errs) != 1 {
		t.Fatalf("errors = %d, want 1", len(errs))
	}
	message := errs[0].Error()
	for _, want := range []string{"Other", "abc123abc123", "a.go:3"} {
		if !strings.Contains(message, want) {
			t.Errorf("error = %v, want it to mention %q", message, want)
		}
	}
	if errs := checkActionCollisions(actions[:1]); len(errs) != 0 {
		t.Errorf("distinct hashes reported a collision: %v", errs)
	}
}

func TestRegistryRegistersEveryActionEndpoint(t *testing.T) {
	home, analysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	actions := []Action{{
		Name: "Rename", RelDir: "users/id_", Package: "id_",
		ImportPath: "example.com/m/pages/users/id_",
		Hash:       "00369cf962b6", Path: "/_action/00369cf962b6/Rename",
	}}

	source, err := NewEmitter().Registry(&Tree{Routes: []Route{home}}, "pages", []Analysis{analysis}, nil, actions)
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	out := string(source)
	if !strings.Contains(out, `mux.HandleFunc("POST /_action/00369cf962b6/Rename", id_.Rename)`) {
		t.Errorf("endpoint not registered:\n%s", out)
	}
	// The handler owns its whole response, so registration is all there is.
	if !strings.Contains(out, "var Actions = []ActionInfo{") {
		t.Errorf("endpoint table missing:\n%s", out)
	}
	if !strings.Contains(out, `Handler: "Rename"`) {
		t.Errorf("endpoint table does not name the handler:\n%s", out)
	}
}

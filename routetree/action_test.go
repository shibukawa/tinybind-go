package routetree

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
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

func TestRegistryRegistersThePagePostRoute(t *testing.T) {
	users, analysis := templateOnly("/users/{id}", "users/id_", "id_", "example.com/m/pages/users/id_",
		[]Segment{{Name: "id"}}, []Value{{Name: "id", Type: "string"}})
	actions := []Action{
		{
			Name: "Rename", RelDir: "users/id_", Package: "id_",
			ImportPath: "example.com/m/pages/users/id_",
			Hash:       "00369cf962b6", Path: "/_action/00369cf962b6/Rename",
			NativeForm: true,
		},
		{
			Name: "Delete", RelDir: "users/id_", Package: "id_",
			ImportPath: "example.com/m/pages/users/id_",
			Hash:       "11469cf962b7", Path: "/_action/11469cf962b7/Delete",
			NativeForm: true,
		},
	}

	source, err := NewEmitter().Registry(&Tree{Routes: []Route{users}}, "pages", []Analysis{analysis}, nil, actions)
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	out := string(source)
	// A form declaring no action submits to the document URL, so the page pattern
	// is what has to accept the POST. Without this the browser reaches no handler.
	if !strings.Contains(out, `mux.HandleFunc("POST /users/{id}"`) {
		t.Errorf("the page carries no POST route:\n%s", out)
	}
	// One registration serves every handler the page can reach, so a page holding
	// several forms needs no second pattern.
	if !strings.Contains(out, `case "00369cf962b6/Rename":`) ||
		!strings.Contains(out, `case "11469cf962b7/Delete":`) {
		t.Errorf("the dispatcher does not branch on both selectors:\n%s", out)
	}
	if !strings.Contains(out, "httpbind.DispatchAction(w, r, id_.Rename)") {
		t.Errorf("the dispatcher does not run the handler:\n%s", out)
	}
	if !strings.Contains(out, `httpbind.ActionSelector(r, "_action")`) {
		t.Errorf("the dispatcher does not read the selector field:\n%s", out)
	}
	// The direct entry points are unchanged and still registered beside it.
	if !strings.Contains(out, `mux.HandleFunc("POST /_action/00369cf962b6/Rename", id_.Rename)`) {
		t.Errorf("the direct entry point was lost:\n%s", out)
	}
}

func TestRegistryDispatchesALayoutsActionsToo(t *testing.T) {
	// A layout is compiled once and renders under every page below it, so a form
	// its markup declares has to reach a handler from the page that rendered it.
	layout := Layout{RelDir: "users", Package: "users", ImportPath: "example.com/m/pages/users",
		File: "pages/users/layout.tb.html"}
	users, analysis := templateOnly("/users/{id}", "users/id_", "id_", "example.com/m/pages/users/id_",
		[]Segment{{Name: "id"}}, []Value{{Name: "id", Type: "string"}}, layout)
	actions := []Action{{
		Name: "Search", RelDir: "users", Package: "users",
		ImportPath: "example.com/m/pages/users",
		Hash:       "22569cf962b8", Path: "/_action/22569cf962b8/Search",
		NativeForm: true,
	}}

	signatures := map[string]ComponentSignature{
		"users": {Name: "Layout", Slots: []Value{{Name: SlotParamName, Type: "html"}}},
	}
	source, err := NewEmitter().Registry(&Tree{Routes: []Route{users}}, "pages", []Analysis{analysis}, signatures, actions)
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	if out := string(source); !strings.Contains(out, `case "22569cf962b8/Search":`) {
		t.Errorf("a layout's action is unreachable from the page below it:\n%s", out)
	}
}

func TestRegistryLeavesAnActionlessPageWithGetAlone(t *testing.T) {
	home, analysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	source, err := NewEmitter().Registry(&Tree{Routes: []Route{home}}, "pages", []Analysis{analysis}, nil, nil)
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	if out := string(source); strings.Contains(out, `mux.HandleFunc("POST /`) {
		t.Errorf("a page reaching no server function gained a POST route:\n%s", out)
	}
}

func TestRegistryLeavesAButtonOnlyActionWithNoPagePost(t *testing.T) {
	// A bare button has no native submit channel, so a POST on the page pattern
	// would serve nothing and would claim an address the framework-owner guide
	// documents an application as free to register itself.
	users, analysis := templateOnly("/users/{id}", "users/id_", "id_", "example.com/m/pages/users/id_",
		[]Segment{{Name: "id"}}, []Value{{Name: "id", Type: "string"}})
	actions := []Action{{
		Name: "Rename", RelDir: "users/id_", Package: "id_",
		ImportPath: "example.com/m/pages/users/id_",
		Hash:       "00369cf962b6", Path: "/_action/00369cf962b6/Rename",
	}}

	source, err := NewEmitter().Registry(&Tree{Routes: []Route{users}}, "pages", []Analysis{analysis}, nil, actions)
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	out := string(source)
	if strings.Contains(out, `mux.HandleFunc("POST /users/{id}"`) {
		t.Errorf("a button-only action claimed the page pattern:\n%s", out)
	}
	// The direct entry point is what a button's runtime calls, and it is unmoved.
	if !strings.Contains(out, `mux.HandleFunc("POST /_action/00369cf962b6/Rename", id_.Rename)`) {
		t.Errorf("the direct entry point was lost:\n%s", out)
	}
}

func TestScriptResolverAnswersTheCompile(t *testing.T) {
	// The resolver is where a framework that parses JavaScript answers what this
	// module refuses to read. The stub below stands in for that parser.
	root := tree(t, map[string]string{
		"page.tb.html": `export component Page(label: string): html {
<script component>
export function setup({ label }) { return { increment() {} } }
</script>
<div><button on-click="increment">{label}</button></div>
}`,
	})

	var sawBlock string
	files, err := Generate(GenerateOptions{
		Config:      Config{Root: root, ImportBase: "example.com/m/pages"},
		RootPackage: "pages",
		ScriptResolver: func(_ string, scripts []htmlbind.ComponentScript) (ScriptAnswers, error) {
			sawBlock = scripts[0].Script
			return ScriptAnswers{
				Handlers:   map[string]htmlbind.ClientHandlerSet{"Page": {Resolved: []string{"increment"}}},
				Parameters: map[string][]string{"Page": {"label"}},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(sawBlock, "export function setup({ label })") {
		t.Errorf("the resolver did not receive the block as authored: %q", sawBlock)
	}

	var page string
	for _, file := range files {
		if strings.HasSuffix(file.Path, "page_gen.go") {
			page = string(file.Source)
		}
	}
	if page == "" {
		t.Fatal("no component file was generated")
	}
	if !strings.Contains(page, `data-tb-on=\"click:increment\"`) {
		t.Errorf("the handler did not lower:\n%s", page)
	}
	if !strings.Contains(page, `Attr("data-tb-props"`) {
		t.Errorf("the named parameter was not emitted:\n%s", page)
	}
}

func TestNoScriptResolverLeavesTheCompileUnchanged(t *testing.T) {
	root := tree(t, map[string]string{
		"page.tb.html": `export component Page(label: string): html {
<script component>
export function setup({ label }) {}
</script>
<div><button on-click="increment">{label}</button></div>
}`,
	})
	files, err := Generate(GenerateOptions{
		Config:      Config{Root: root, ImportBase: "example.com/m/pages"},
		RootPackage: "pages",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Path, "page_gen.go") {
			continue
		}
		out := string(file.Source)
		// Unchecked, so the handler still lowers; but nothing names a parameter,
		// so no object is emitted.
		if !strings.Contains(out, `data-tb-on=\"click:increment\"`) {
			t.Errorf("an unchecked handler did not lower:\n%s", out)
		}
		if strings.Contains(out, "data-tb-props") {
			t.Errorf("parameters were emitted with none named:\n%s", out)
		}
	}
}

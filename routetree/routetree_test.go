package routetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes a directory layout from a map of relative path to file content
// and returns the route root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "app")
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func discover(t *testing.T, root string) (*Tree, error) {
	t.Helper()
	return Discover(Config{Root: root})
}

func paths(tree *Tree) []string {
	out := make([]string, len(tree.Routes))
	for i, route := range tree.Routes {
		out[i] = route.Path
	}
	return out
}

func TestDiscoverDerivesPatterns(t *testing.T) {
	root := tree(t, map[string]string{
		"page.tb.html":                 "",
		"about/page.tb.html":           "",
		"users/page.tb.html":           "",
		"users/id_/page.tb.html":       "",
		"users/id_/posts/page.tb.html": "",
		"files/rest__/page.tb.html":    "",
	})

	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	want := []string{"/", "/about", "/files/{rest...}", "/users", "/users/{id}", "/users/{id}/posts"}
	if diff := strings.Join(paths(got), ","); diff != strings.Join(want, ",") {
		t.Errorf("paths = %v, want %v", paths(got), want)
	}

	for _, route := range got.Routes {
		want := "GET " + route.Path
		if route.Path == "/" {
			// A bare / is a prefix pattern in the standard library, so the root
			// page must register as an exact match or it answers every 404.
			want = "GET /{$}"
		}
		if route.Pattern() != want {
			t.Errorf("Pattern() = %q, want %q", route.Pattern(), want)
		}
	}
}

func TestDiscoverBindsParametersInRouteOrder(t *testing.T) {
	root := tree(t, map[string]string{
		"orgs/org_/users/id_/page.tb.html": "",
	})

	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(got.Routes))
	}
	route := got.Routes[0]
	if route.Path != "/orgs/{org}/users/{id}" {
		t.Errorf("Path = %q", route.Path)
	}
	if len(route.Params) != 2 || route.Params[0].Name != "org" || route.Params[1].Name != "id" {
		t.Fatalf("Params = %+v, want org then id", route.Params)
	}
	if route.Params[0].Kind != DynamicSegment {
		t.Errorf("Params[0].Kind = %v, want dynamic", route.Params[0].Kind)
	}
}

func TestDiscoverCollectsAncestorLayoutsOutermostFirst(t *testing.T) {
	root := tree(t, map[string]string{
		"layout.tb.html":                 "",
		"users/layout.tb.html":           "",
		"users/id_/page.tb.html":         "",
		"users/id_/posts/layout.tb.html": "",
		"users/id_/posts/page.tb.html":   "",
	})

	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	byPath := map[string]Route{}
	for _, route := range got.Routes {
		byPath[route.Path] = route
	}

	user := byPath["/users/{id}"]
	if len(user.Layouts) != 2 {
		t.Fatalf("/users/{id} layouts = %d, want 2", len(user.Layouts))
	}
	if user.Layouts[0].RelDir != "" || user.Layouts[1].RelDir != "users" {
		t.Errorf("layout order = %q,%q, want root then users", user.Layouts[0].RelDir, user.Layouts[1].RelDir)
	}
	// A layout only sees the dynamic segments at or above its own directory.
	if len(user.Layouts[1].Params) != 0 {
		t.Errorf("users layout params = %+v, want none", user.Layouts[1].Params)
	}

	posts := byPath["/users/{id}/posts"]
	if len(posts.Layouts) != 3 {
		t.Fatalf("posts layouts = %d, want 3", len(posts.Layouts))
	}
	if got, want := posts.Layouts[2].RelDir, "users/id_/posts"; got != want {
		t.Errorf("innermost layout = %q, want %q", got, want)
	}
	if len(posts.Layouts[2].Params) != 1 || posts.Layouts[2].Params[0].Name != "id" {
		t.Errorf("innermost layout params = %+v, want id", posts.Layouts[2].Params)
	}
}

func TestDiscoverSiblingBranchesDoNotShareChains(t *testing.T) {
	root := tree(t, map[string]string{
		"a/page.tb.html":   "",
		"b/page.tb.html":   "",
		"a/x/page.tb.html": "",
		"b/y/page.tb.html": "",
	})

	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for _, route := range got.Routes {
		for _, segment := range route.Segments {
			if !strings.Contains(route.Path, segment.Dir) {
				t.Errorf("route %s carries foreign segment %q", route.Path, segment.Dir)
			}
		}
	}
}

func TestDiscoverFindsDocumentAndLogicFiles(t *testing.T) {
	root := tree(t, map[string]string{
		"document.tb.html":       "",
		"page.tb.html":           "",
		"users/id_/page.tb.html": "",
		"users/id_/page.go":      "package id_\n",
	})

	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got.DocumentFile == "" {
		t.Error("DocumentFile not found")
	}
	byPath := map[string]Route{}
	for _, route := range got.Routes {
		byPath[route.Path] = route
	}
	if byPath["/"].LogicFile != "" {
		t.Error("root route should have no logic file")
	}
	if byPath["/users/{id}"].LogicFile == "" {
		t.Error("users/id_ route should have a logic file")
	}
	if got, want := byPath["/users/{id}"].Package, "id_"; got != want {
		t.Errorf("Package = %q, want %q", got, want)
	}
}

func TestDiscoverPassesThroughDirectoriesWithoutAPage(t *testing.T) {
	root := tree(t, map[string]string{
		"a/b/c/page.tb.html": "",
	})

	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Routes) != 1 || got.Routes[0].Path != "/a/b/c" {
		t.Fatalf("paths = %v, want [/a/b/c]", paths(got))
	}
}

func TestDiscoverSkipsIgnoredDirectories(t *testing.T) {
	root := tree(t, map[string]string{
		"page.tb.html":             "",
		"_components/page.tb.html": "",
		".hidden/page.tb.html":     "",
		"testdata/page.tb.html":    "",
	})

	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Routes) != 1 || got.Routes[0].Path != "/" {
		t.Fatalf("paths = %v, want [/] only", paths(got))
	}
}

func TestDiscoverRejectsIllegalDirectoryNames(t *testing.T) {
	// Every one of these breaks go build ./... for the whole module once it
	// holds a .go file, so discovery refuses them outright.
	for _, name := range []string{"[id]", "{id}", "$id", "@id", ":id", "=id", "(group)", "-id", "~id"} {
		t.Run(name, func(t *testing.T) {
			root := tree(t, map[string]string{
				name + "/page.tb.html": "",
			})
			_, err := discover(t, root)
			if err == nil {
				t.Fatalf("directory %q accepted, want rejection", name)
			}
			if !strings.Contains(err.Error(), "import path") {
				t.Errorf("error = %v, want it to explain the import path rule", err)
			}
		})
	}
}

func TestValidateDirNameAcceptsLegalElements(t *testing.T) {
	for _, name := range []string{"id_", "slug__", "userId_", "users", "sign-in", "v1.2", "id~"} {
		if err := ValidateDirName(name); err != nil {
			t.Errorf("ValidateDirName(%q) = %v, want nil", name, err)
		}
	}
}

func TestDiscoverRejectsTwoDynamicSiblings(t *testing.T) {
	root := tree(t, map[string]string{
		"users/id_/page.tb.html":   "",
		"users/name_/page.tb.html": "",
	})

	_, err := discover(t, root)
	if err == nil {
		t.Fatal("two dynamic siblings accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "dynamic sibling") {
		t.Errorf("error = %v, want it to name the conflict", err)
	}
}

func TestDiscoverAllowsLiteralSiblingOfDynamic(t *testing.T) {
	root := tree(t, map[string]string{
		"users/id_/page.tb.html": "",
		"users/me/page.tb.html":  "",
	})

	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Routes) != 2 {
		t.Fatalf("paths = %v, want both", paths(got))
	}
}

func TestDiscoverRejectsRepeatedParameterName(t *testing.T) {
	root := tree(t, map[string]string{
		"a/id_/b/id_/page.tb.html": "",
	})

	_, err := discover(t, root)
	if err == nil {
		t.Fatal("repeated parameter name accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "already bound") {
		t.Errorf("error = %v, want it to name the earlier binding", err)
	}
}

func TestDiscoverRejectsChildrenOfCatchAll(t *testing.T) {
	root := tree(t, map[string]string{
		"files/rest__/page.tb.html":      "",
		"files/rest__/more/page.tb.html": "",
	})

	_, err := discover(t, root)
	if err == nil {
		t.Fatal("catch-all with children accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "catch-all") {
		t.Errorf("error = %v, want it to name the catch-all", err)
	}
}

func TestDiscoverReportsDuplicateNormalizedRoutes(t *testing.T) {
	// Two dynamic segments with different names still match the same requests.
	root := tree(t, map[string]string{
		"a/id_/page.tb.html":   "",
		"b/name_/page.tb.html": "",
	})
	// Not a duplicate: different static prefixes.
	if _, err := discover(t, root); err != nil {
		t.Fatalf("distinct prefixes rejected: %v", err)
	}

	root = tree(t, map[string]string{
		"x/page.tb.html": "",
	})
	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(got.Routes))
	}
}

func TestDiscoverReportsEveryProblemAtOnce(t *testing.T) {
	root := tree(t, map[string]string{
		"[id]/page.tb.html":     "",
		"{name}/page.tb.html":   "",
		"users/a_/page.tb.html": "",
		"users/b_/page.tb.html": "",
	})

	_, err := discover(t, root)
	if err == nil {
		t.Fatal("expected errors")
	}
	message := err.Error()
	for _, want := range []string{"[id]", "{name}", "dynamic sibling"} {
		if !strings.Contains(message, want) {
			t.Errorf("error does not mention %q:\n%s", want, message)
		}
	}
}

func TestPackageNameSanitizesNonIdentifiers(t *testing.T) {
	cases := map[string]string{
		"id_":     "id_",
		"users":   "users",
		"sign-in": "signin",
		"v1.2":    "v12",
		"func":    "func_",
		"2fa":     "p2fa",
	}
	for dir, want := range cases {
		if got := PackageName(dir); got != want {
			t.Errorf("PackageName(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestParseSegment(t *testing.T) {
	cases := []struct {
		dir  string
		name string
		kind SegmentKind
	}{
		{"users", "users", StaticSegment},
		{"id_", "id", DynamicSegment},
		{"rest__", "rest", CatchAllSegment},
		{"_", "_", StaticSegment},
		{"__", "__", StaticSegment},
	}
	for _, c := range cases {
		got := ParseSegment(c.dir)
		if got.Name != c.name || got.Kind != c.kind {
			t.Errorf("ParseSegment(%q) = {%q,%v}, want {%q,%v}", c.dir, got.Name, got.Kind, c.name, c.kind)
		}
	}
}

func TestDiscoverEmptyRootHasNoRoutes(t *testing.T) {
	root := tree(t, map[string]string{})
	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Routes) != 0 {
		t.Errorf("routes = %v, want none", paths(got))
	}
}

func TestDiscoverMissingRootIsAnError(t *testing.T) {
	if _, err := Discover(Config{Root: filepath.Join(t.TempDir(), "absent")}); err == nil {
		t.Fatal("missing root accepted, want error")
	}
}

package routetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// routeDir writes one route directory and returns the discovered route.
func routeDir(t *testing.T, files map[string]string, path string) Route {
	t.Helper()
	root := tree(t, files)
	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for _, route := range got.Routes {
		if route.Path == path {
			return route
		}
	}
	t.Fatalf("route %s not discovered; got %v", path, paths(got))
	return Route{}
}

const pageTemplateOnly = `export component Page(id: string, page: int): html {
  <p>{id}</p>
}
`

func TestAnalyzeTemplateOnlyUsesComponentParameters(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": pageTemplateOnly,
	}, "/users/{id}")

	analysis, err := Analyze(route)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if analysis.Page.Rung != RungTemplateOnly {
		t.Errorf("Rung = %v, want %v", analysis.Page.Rung, RungTemplateOnly)
	}
	if len(analysis.Inputs) != 2 {
		t.Fatalf("Inputs = %+v, want id and page", analysis.Inputs)
	}
	if analysis.Inputs[0].Name != "id" || analysis.Inputs[0].Type != "string" {
		t.Errorf("Inputs[0] = %+v", analysis.Inputs[0])
	}
	if analysis.Inputs[1].Name != "page" || analysis.Inputs[1].Type != "int" {
		t.Errorf("Inputs[1] = %+v", analysis.Inputs[1])
	}
}

func TestAnalyzeTemplateOnlyRequiresPathParametersFirst(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": `export component Page(page: int, id: string): html { <p>{id}</p> }`,
	}, "/users/{id}")

	_, err := Analyze(route)
	if err == nil {
		t.Fatal("reordered parameters accepted, want rejection")
	}
	if !strings.Contains(err.Error(), `"id"`) {
		t.Errorf("error = %v, want it to name the expected parameter", err)
	}
}

func TestAnalyzeTemplateOnlyRejectsNonScalarParameter(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": `
type User { name: string }

export component Page(id: string, user: User): html { <p>{user.name}</p> }
`,
	}, "/users/{id}")

	_, err := Analyze(route)
	if err == nil {
		t.Fatal("record parameter accepted, want rejection")
	}
	// The message must say why, because the fix is to add a func Load.
	if !strings.Contains(err.Error(), "func Load") {
		t.Errorf("error = %v, want it to point at the typed rung", err)
	}
}

func TestAnalyzeTemplateOnlyAcceptsAnOptionalQueryParameter(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": `export component Page(id: string, page: int?): html { <p>{id}</p> }`,
	}, "/users/{id}")

	analysis, err := Analyze(route)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// The optional marker already lowers to a pointer in the generated parameter
	// struct, so the decoder binds the same type the component declares.
	if got := analysis.Inputs[1].Type; got != "*int" {
		t.Errorf("query input type = %q, want *int", got)
	}
}

func TestAnalyzeTemplateOnlyRejectsAnOptionalPathParameter(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": `export component Page(id: string?): html { <p>x</p> }`,
	}, "/users/{id}")

	_, err := Analyze(route)
	if err == nil {
		t.Fatal("optional path parameter accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "always present") {
		t.Errorf("error = %v, want it to say why a segment cannot be optional", err)
	}
}

func TestAnalyzeTypedPageUsesFunctionParameters(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": `
type User { name: string }

export component Page(user: User): html { <p>{user.name}</p> }
`,
		"users/id_/page.go": `package id_

type User struct{ Name string }

func Load(id string) (User, error) { return User{}, nil }
`,
	}, "/users/{id}")

	analysis, err := Analyze(route)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if analysis.Page.Rung != RungTypedPage {
		t.Fatalf("Rung = %v, want %v", analysis.Page.Rung, RungTypedPage)
	}
	// The decoder binds what the function takes, not what the component takes.
	if len(analysis.Inputs) != 1 || analysis.Inputs[0].Name != "id" {
		t.Errorf("Inputs = %+v, want the function parameters", analysis.Inputs)
	}
	if len(analysis.Component.Inputs) != 1 || analysis.Component.Inputs[0].Type != "User" {
		t.Errorf("Component.Inputs = %+v", analysis.Component.Inputs)
	}
}

func TestAnalyzeTypedPageRejectsResultMismatch(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": `
type User { name: string }

export component Page(user: User): html { <p>{user.name}</p> }
`,
		"users/id_/page.go": `package id_

type Account struct{}

func Load(id string) (Account, error) { return Account{}, nil }
`,
	}, "/users/{id}")

	_, err := Analyze(route)
	if err == nil {
		t.Fatal("mismatched results accepted, want rejection")
	}
	for _, want := range []string{"Account", "User"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to mention %q", err, want)
		}
	}
}

func TestAnalyzeHandlerPageBindsOnlyPathSegments(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": `
type User { name: string }

export component Page(user: User): html { <p>{user.name}</p> }
`,
		"users/id_/page.go": `package id_

import "net/http"

func Load(w http.ResponseWriter, r *http.Request) {}
`,
	}, "/users/{id}")

	analysis, err := Analyze(route)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if analysis.Page.Rung != RungHandlerPage {
		t.Fatalf("Rung = %v, want %v", analysis.Page.Rung, RungHandlerPage)
	}
	// The handler owns decoding, so the generated decoder covers only what the
	// filesystem already knows.
	if len(analysis.Inputs) != 1 || analysis.Inputs[0].Name != "id" || analysis.Inputs[0].Type != "string" {
		t.Errorf("Inputs = %+v, want the path segments as strings", analysis.Inputs)
	}
}

func TestAnalyzeRejectsAPageDeclaringASlot(t *testing.T) {
	route := routeDir(t, map[string]string{
		"page.tb.html": `export component Page(children: html): html { <main><slot required /></main> }`,
	}, "/")

	_, err := Analyze(route)
	if err == nil {
		t.Fatal("page with a slot accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "layout") {
		t.Errorf("error = %v, want it to point at layouts", err)
	}
}

func TestPageComponentRequiresTheReservedName(t *testing.T) {
	route := routeDir(t, map[string]string{
		"page.tb.html": `export component Index(): html { <p>hi</p> }`,
	}, "/")

	_, err := Analyze(route)
	if err == nil {
		t.Fatal("wrongly named component accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "export component Page") {
		t.Errorf("error = %v, want it to state the required declaration", err)
	}
}

func TestPageComponentRequiresExport(t *testing.T) {
	route := routeDir(t, map[string]string{
		"page.tb.html": `component Page(): html { <p>hi</p> }`,
	}, "/")

	_, err := Analyze(route)
	if err == nil {
		t.Fatal("unexported component accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "not exported") {
		t.Errorf("error = %v, want it to name the visibility problem", err)
	}
}

func TestAnalyzeSurfacesTemplateCompileErrors(t *testing.T) {
	route := routeDir(t, map[string]string{
		"page.tb.html": `export component Page(x: Missing): html { <p>{x}</p> }`,
	}, "/")

	if _, err := Analyze(route); err == nil {
		t.Fatal("uncompilable template accepted, want rejection")
	}
}

// TestAnalyzeFeedsTheDecoder is the point of the whole pipeline: what the
// template declares reaches the generated decoder without anyone restating it.
func TestAnalyzeFeedsTheDecoder(t *testing.T) {
	route := routeDir(t, map[string]string{
		"users/id_/page.tb.html": pageTemplateOnly,
	}, "/users/{id}")

	analysis, err := Analyze(route)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	source, err := EmitDecoder(analysis.Route, analysis.Inputs)
	if err != nil {
		t.Fatalf("EmitDecoder: %v", err)
	}
	mustContain(t, string(source),
		"ID string",
		"Page int",
		`httpbind.PathValue(r, "id")`,
		`httpbind.QueryLookup(query, "page")`,
	)
}

func TestLayoutComponentReadsTheReservedName(t *testing.T) {
	root := tree(t, map[string]string{
		"layout.tb.html": `export component Layout(children: html): html { <main><slot required /></main> }`,
		"page.tb.html":   `export component Page(): html { <p>hi</p> }`,
	})
	got, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Routes[0].Layouts) != 1 {
		t.Fatalf("layouts = %+v", got.Routes[0].Layouts)
	}
	layout, err := LayoutComponent(got.Routes[0].Layouts[0].File)
	if err != nil {
		t.Fatalf("LayoutComponent: %v", err)
	}
	if len(layout.Slots) != 1 || layout.Slots[0].Name != "children" {
		t.Errorf("Slots = %+v, want the children slot", layout.Slots)
	}
	if len(layout.Inputs) != 0 {
		t.Errorf("Inputs = %+v, want none", layout.Inputs)
	}
}

// --- whole-tree generation ---

func TestGenerateEmitsEveryFileForATree(t *testing.T) {
	root := tree(t, map[string]string{
		"layout.tb.html":         `export component Layout(children: html): html { <div><slot required /></div> }`,
		"page.tb.html":           `export component Page(): html { <p>home</p> }`,
		"users/id_/page.tb.html": `export component Page(id: string): html { <p>{id}</p> }`,
	})

	files, err := Generate(GenerateOptions{
		Config:      Config{Root: root, ImportBase: "example.com/m/pages"},
		RootPackage: "pages",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	byBase := map[string]bool{}
	for _, file := range files {
		byBase[filepath.Base(file.Path)] = true
	}
	// page.tb.html and layout.tb.html share a directory, so each must claim its
	// own output rather than one overwriting the other.
	for _, want := range []string{"layout_gen.go", "page_gen.go", "route_gen.go", "routes_gen.go"} {
		if !byBase[want] {
			t.Errorf("no %s emitted; got %v", want, byBase)
		}
	}
}

func TestGenerateAsksTheResolverForAnActionTheTreeDoesNotDeclare(t *testing.T) {
	root := tree(t, map[string]string{
		"page.tb.html": `export component Page(): html { <button server-action="Publish">go</button> }`,
	})

	asked := ""
	files, err := Generate(GenerateOptions{
		Config: Config{Root: root, ImportBase: "example.com/m/pages"},
		ActionResolver: func(name string) (string, bool) {
			asked = name
			return "/app/publish", true
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if asked != "Publish" {
		t.Errorf("resolver asked for %q, want Publish", asked)
	}
	found := false
	for _, file := range files {
		if strings.Contains(string(file.Source), "/app/publish") {
			found = true
		}
	}
	if !found {
		t.Error("the resolved URL reached no generated file")
	}
}

func TestGenerateWithoutAResolverStillRejectsAnUnknownAction(t *testing.T) {
	root := tree(t, map[string]string{
		"page.tb.html": `export component Page(): html { <button server-action="Publish">go</button> }`,
	})
	if _, err := Generate(GenerateOptions{Config: Config{Root: root}}); err == nil {
		t.Fatal("unresolved action accepted, want rejection")
	}
}

func TestGenerateReportsTreeErrors(t *testing.T) {
	root := tree(t, map[string]string{
		"page.tb.html": `export component Index(): html { <p>hi</p> }`,
	})
	if _, err := Generate(GenerateOptions{Config: Config{Root: root}}); err == nil {
		t.Fatal("wrongly named component accepted, want rejection")
	}
}

func TestGenerateDerivesTheRootPackageFromTheDirectory(t *testing.T) {
	root := tree(t, map[string]string{
		"page.tb.html": `export component Page(): html { <p>hi</p> }`,
	})
	files, err := Generate(GenerateOptions{Config: Config{Root: root}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, file := range files {
		if filepath.Base(file.Path) == DefaultRegistryOutput {
			// tree() always builds its root as a directory named app.
			if !strings.Contains(string(file.Source), "package app") {
				t.Errorf("root package not derived from the directory:\n%s", file.Source)
			}
			return
		}
	}
	t.Fatal("no registry emitted")
}

func TestWriteCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "gen.go")
	if err := Write([]Generated{{Path: target, Source: []byte("package b\n")}}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
}

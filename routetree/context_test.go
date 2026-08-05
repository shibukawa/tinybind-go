package routetree

import (
	"path/filepath"
	"strings"
	"testing"
)

// generated returns the emitted file with the given base name, which is how
// these tests reach one artifact of a whole-tree run.
func generated(t *testing.T, files []Generated, base string) string {
	t.Helper()
	for _, file := range files {
		if filepath.Base(file.Path) == base {
			return string(file.Source)
		}
	}
	t.Fatalf("no %s emitted", base)
	return ""
}

func generate(t *testing.T, files map[string]string) []Generated {
	t.Helper()
	out, err := Generate(GenerateOptions{
		Config:      Config{Root: tree(t, files), ImportBase: "example.com/m/pages"},
		RootPackage: "pages",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

// A route directory is its own Go package, so an external's implementation sits
// beside the template that calls it. The same declaration receives a context in
// a templates package, and must here too.
func TestGeneratePassesTheContextToASyncExternal(t *testing.T) {
	files := generate(t, map[string]string{
		"page.tb.html": "external CurrentToken(): string\n\nexport component Page(): html { <p>{CurrentToken()}</p> }",
		"externals.go": `package pages

import "context"

func CurrentToken(ctx context.Context) string { return "" }
`,
	})
	source := generated(t, files, "page_gen.go")
	if !strings.Contains(source, "CurrentToken(ctx)") {
		t.Errorf("a sync external declaring a leading context was called without one\n%s", source)
	}
}

// The same defect reached the async form, which is why the reported symptom
// looked like two problems. Both are this one caller.
func TestGeneratePassesTheContextToAnAsyncExternal(t *testing.T) {
	files := generate(t, map[string]string{
		"page.tb.html": `external async RecentMemos(): string[]

export component Page(): html {
{await memos = RecentMemos()}
<p>{memos[0]}</p>
{fallback}
<p>…</p>
{/await}
}`,
		"externals.go": `package pages

import "context"

func RecentMemos(ctx context.Context) ([]string, error) { return nil, nil }
`,
	})
	source := generated(t, files, "page_gen.go")
	if !strings.Contains(source, "RecentMemos(ctx)") {
		t.Errorf("an async external declaring a leading context was called without one\n%s", source)
	}
}

// The choice belongs to whoever writes the implementation, so an external that
// declares no context is still called plainly.
func TestGenerateLeavesAnExternalWithoutAContextAlone(t *testing.T) {
	files := generate(t, map[string]string{
		"page.tb.html": "external CurrentToken(): string\n\nexport component Page(): html { <p>{CurrentToken()}</p> }",
		"externals.go": `package pages

func CurrentToken() string { return "" }
`,
	})
	source := generated(t, files, "page_gen.go")
	if !strings.Contains(source, "CurrentToken()") {
		t.Errorf("a context-free external did not keep its plain call\n%s", source)
	}
	if strings.Contains(source, "CurrentToken(ctx)") {
		t.Errorf("a context was passed to an external that declared none\n%s", source)
	}
}

// A layout is scanned in its own directory rather than the page's, so a route
// does not inherit its ancestor's externals or miss its own.
func TestGenerateScansALayoutsOwnDirectory(t *testing.T) {
	files := generate(t, map[string]string{
		"layout.tb.html": "external Nonce(): string\n\nexport component Layout(children: html): html { <div data-nonce={Nonce()}><slot required /></div> }",
		"externals.go": `package pages

import "context"

func Nonce(ctx context.Context) string { return "" }
`,
		"archive/page.tb.html": `export component Page(): html { <p>archive</p> }`,
	})
	source := generated(t, files, "layout_gen.go")
	if !strings.Contains(source, "Nonce(ctx)") {
		t.Errorf("a layout's own external did not receive the context\n%s", source)
	}
}

// The page that needs the request context most is one with no URL input at all,
// which before this could not use the checked rung.
func TestGenerateCallsAContextOnlyEntryPointWithTheRequestContext(t *testing.T) {
	files := generate(t, map[string]string{
		"page.tb.html": `export component Page(user: string): html { <p>{user}</p> }`,
		"page.go": `package pages

import "context"

func Load(ctx context.Context) (string, error) { return "", nil }
`,
	})
	source := generated(t, files, "routes_gen.go")
	if !strings.Contains(source, "Load(r.Context())") {
		t.Errorf("a typed entry point taking a context was not given the request's\n%s", source)
	}
}

// The context leads and the decoded route inputs follow, so the route-order rule
// is unchanged for everything after it.
func TestGenerateCallsAContextEntryPointWithTheRouteInputsAfterIt(t *testing.T) {
	files := generate(t, map[string]string{
		"users/id_/page.tb.html": `export component Page(user: string): html { <p>{user}</p> }`,
		"users/id_/page.go": `package id_

import "context"

func Load(ctx context.Context, id string) (string, error) { return "", nil }
`,
	})
	source := generated(t, files, "routes_gen.go")
	if !strings.Contains(source, "Load(r.Context(), route.ID)") {
		t.Errorf("the context did not lead the decoded inputs\n%s", source)
	}
}

// An entry point declaring no context keeps the call it already generated.
func TestGenerateLeavesAContextFreeEntryPointAlone(t *testing.T) {
	files := generate(t, map[string]string{
		"users/id_/page.tb.html": `export component Page(user: string): html { <p>{user}</p> }`,
		"users/id_/page.go": `package id_

func Load(id string) (string, error) { return "", nil }
`,
	})
	source := generated(t, files, "routes_gen.go")
	if !strings.Contains(source, "Load(route.ID)") {
		t.Errorf("a context-free entry point did not keep its call\n%s", source)
	}
}

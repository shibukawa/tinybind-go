package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/routetree"
)

// writeActionTree lays out a whole route tree whose page package declares a
// typed server action, so the two phases can be run against each other the way
// a generate command runs them.
func writeActionTree(t *testing.T) (dir, importBase string) {
	t.Helper()
	dir = t.TempDir()
	writeTempModule(t, dir)
	importBase = "tempmod/pages"
	pages := filepath.Join(dir, "pages")
	if err := os.MkdirAll(pages, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(pages, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("page.tb.html", `external LoadName(): string

export component Page(): html { {val name = LoadName()} <p>{name}</p> }`)
	write("page.go", `package pages

import (
	"context"

	"github.com/shibukawa/tinybind-go"
)

type User struct {
	ID   string `+"`json:\"id\"`"+`
	Name string `+"`json:\"name\"`"+`
}

func LoadName() string { return "ada" }

var _ = httpbind.ServerAction(getUser, "fetchUser")

func getUser(ctx context.Context, id string) (User, error) {
	return User{ID: id, Name: "ada"}, nil
}
`)
	tidyTempModule(t, dir)
	return dir, importBase
}

// The two phases have to run in one order and hand one thing to each other.
// This runs them in that order and compiles the result, which is the only way
// to show that the registry and the emitted entry point name one symbol.
func TestTypedActionTreeGeneratesInOrderAndAnswers(t *testing.T) {
	dir, importBase := writeActionTree(t)

	result, err := routetree.GenerateTree(routetree.GenerateOptions{
		Config:      routetree.Config{Root: filepath.Join(dir, "pages"), ImportBase: importBase},
		RootPackage: "pages",
	})
	if err != nil {
		t.Fatalf("GenerateTree: %v", err)
	}

	// The registry names an entry point the next phase writes, so it is held
	// back: the root package would not type-check between the two.
	before, registry := routetree.SplitRegistry(result.Files)
	if len(registry) != 1 {
		t.Fatalf("want one registry file, got %d", len(registry))
	}
	if err := routetree.Write(before); err != nil {
		t.Fatal(err)
	}

	var typed int
	for _, action := range result.Actions {
		if action.Typed {
			typed++
		}
	}
	if typed != 1 {
		t.Fatalf("want one typed action, got %d of %d", typed, len(result.Actions))
	}

	opts := generator.DefaultOptions().WithServerActionsFor(result.Actions, "")
	if len(opts.ServerActions) != 1 {
		t.Fatalf("want one converted action, got %+v", opts.ServerActions)
	}
	if got := opts.ServerActions[0].Wrapper; got != "ActionGetUser" {
		t.Fatalf("wrapper %q; it has to be the one the registry registers", got)
	}
	if _, err := generator.New(opts).Generate(filepath.Join(dir, "pages"), "", ""); err != nil {
		t.Fatalf("binding phase: %v", err)
	}

	// Now the symbol exists, so the registry can land.
	if err := routetree.Write(registry); err != nil {
		t.Fatal(err)
	}

	probe := `package pages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActionAnswers(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	var path string
	for _, a := range Actions {
		if a.Published == "fetchUser" {
			path = a.Path
		}
	}
	if path == "" {
		t.Fatalf("no action published as fetchUser: %+v", Actions)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(` + "`" + `{"id":"3"}` + "`" + `)))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	const want = ` + "`" + `{"id":"3","name":"ada"}` + "`" + `
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "pages", "probe_test.go"), []byte(probe), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("wired tree does not answer: %v\n%s", err, output)
	}
}

// The published name is what a script calls through, and an override is a wire
// name a Go rename must not move.
func TestTypedActionCarriesTheOverriddenPublishedName(t *testing.T) {
	dir, importBase := writeActionTree(t)
	result, err := routetree.GenerateTree(routetree.GenerateOptions{
		Config:      routetree.Config{Root: filepath.Join(dir, "pages"), ImportBase: importBase},
		RootPackage: "pages",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range result.Actions {
		if !action.Typed {
			continue
		}
		if action.Published != "fetchUser" {
			t.Fatalf("published %q, want the declared override", action.Published)
		}
		if action.Name != "getUser" {
			t.Fatalf("name %q; the Go name is the identity and stays as written", action.Name)
		}
		return
	}
	t.Fatal("no typed action discovered")
}

// A raw handler converts to nothing, since nothing is generated around one.
func TestServerActionsForSkipsRawHandlers(t *testing.T) {
	actions := []routetree.Action{
		{Name: "Rename", RelDir: "users", Wrapper: "Rename"},
		{Name: "GetUser", RelDir: "users", Wrapper: "ActionGetUser", Typed: true},
		{Name: "Other", RelDir: "posts", Wrapper: "ActionOther", Typed: true},
	}
	got := generator.ServerActionsFor(actions, "users")
	if len(got) != 1 || got[0].Func != "GetUser" {
		t.Fatalf("got %+v", got)
	}
	if strings.Contains(got[0].Wrapper, "Rename") {
		t.Fatalf("wrapper %q", got[0].Wrapper)
	}
}

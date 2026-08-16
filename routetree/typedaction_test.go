package routetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeActionPackage lays out one route package holding the given Go source.
func writeActionPackage(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const typedActionHeader = `package users

import (
	"context"

	"github.com/shibukawa/tinybind-go"
)

type User struct{ ID string }

`

func typedAction(t *testing.T, body string) Action {
	t.Helper()
	dir := writeActionPackage(t, typedActionHeader+body)
	actions, err := DiscoverActions(dir, "users", "users", "", "")
	return onlyAction(t, actions, err)
}

func onlyAction(t *testing.T, actions []Action, err error) Action {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("want one action, got %d: %+v", len(actions), actions)
	}
	return actions[0]
}

// A function of an arbitrary signature is an action because it was declared.
// The shape filter admits nothing here: GetUser takes no transport types and
// returns two values.
func TestDeclarationAdmitsAnArbitrarySignature(t *testing.T) {
	action := typedAction(t, `
var _ = httpbind.ServerAction(GetUser)

func GetUser(ctx context.Context, id string) (User, error) { return User{}, nil }
`)
	if !action.Typed {
		t.Fatal("want a typed action")
	}
	if action.Name != "GetUser" || action.Published != "getUser" {
		t.Fatalf("names %q / %q", action.Name, action.Published)
	}
	if !action.Signature.TakesContext {
		t.Fatal("want the leading context recognised")
	}
	if len(action.Signature.Params) != 1 || action.Signature.Params[0].Name != "id" || action.Signature.Params[0].Type != "string" {
		t.Fatalf("params %+v", action.Signature.Params)
	}
	if action.Signature.Result != "User" {
		t.Fatalf("result %q", action.Signature.Result)
	}
	// The address is the one a raw action gets, over the same inputs.
	if action.Hash != ActionHash("users", "GetUser") {
		t.Fatalf("hash %q", action.Hash)
	}
}

// A struct parameter is the natural shape rather than a refused one: nothing
// here resolves a type, and the phase that builds the argument struct does
// type-check.
func TestDeclarationTakesAStructParameter(t *testing.T) {
	action := typedAction(t, `
var _ = httpbind.ServerAction(CreateUser)

type CreateUserRequest struct{ Name string }

func CreateUser(req CreateUserRequest) (User, error) { return User{}, nil }
`)
	if action.Signature.TakesContext {
		t.Fatal("no context declared")
	}
	if action.Signature.Params[0].Type != "CreateUserRequest" {
		t.Fatalf("params %+v", action.Signature.Params)
	}
}

func TestDeclarationTakesAnErrorOnlyResult(t *testing.T) {
	action := typedAction(t, `
var _ = httpbind.ServerAction(Touch)

func Touch(id string) error { return nil }
`)
	if action.Signature.Result != "" {
		t.Fatalf("want no value result, got %q", action.Signature.Result)
	}
}

// The published name is a wire name, so a Go rename moves the identifier and
// leaves an overridden published name where it was.
func TestDeclarationOverridesThePublishedName(t *testing.T) {
	action := typedAction(t, `
var _ = httpbind.ServerAction(GetURL, "linkFor")

func GetURL(id string) (User, error) { return User{}, nil }
`)
	if action.Published != "linkFor" {
		t.Fatalf("published %q", action.Published)
	}
}

// An unexported function is reachable: the wrapper is emitted beside it, so the
// registry names the wrapper rather than the function.
func TestDeclarationAdmitsAnUnexportedFunction(t *testing.T) {
	action := typedAction(t, `
var _ = httpbind.ServerAction(getUser)

func getUser(id string) (User, error) { return User{}, nil }
`)
	if action.Name != "getUser" || action.Published != "getUser" {
		t.Fatalf("names %q / %q", action.Name, action.Published)
	}
}

// A handler-shaped function is still an action by existing, so the two rules
// stand beside each other.
func TestTheShapeFilterStillAdmitsWithoutADeclaration(t *testing.T) {
	dir := writeActionPackage(t, `package users

import "net/http"

func Rename(w http.ResponseWriter, r *http.Request) {}
`)
	actions, err := DiscoverActions(dir, "users", "users", "", "")
	action := onlyAction(t, actions, err)
	if action.Typed {
		t.Fatal("a handler-shaped function is not typed")
	}
	if action.Published != "rename" {
		t.Fatalf("published %q", action.Published)
	}
}

func TestDeclarationDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{
			"not a function",
			"var _ = httpbind.ServerAction(User{})",
			"not an expression",
		},
		{
			"names nothing declared here",
			"var _ = httpbind.ServerAction(Missing)",
			"not a function declared in this package",
		},
		{
			"the page entry point",
			"var _ = httpbind.ServerAction(Load)\n\nfunc Load(id string) (User, error) { return User{}, nil }",
			"page entry point",
		},
		{
			"too many results",
			"var _ = httpbind.ServerAction(Three)\n\nfunc Three() (User, User, error) { return User{}, User{}, nil }",
			"returns one value and an error",
		},
		{
			"a single non-error result",
			"var _ = httpbind.ServerAction(One)\n\nfunc One() User { return User{} }",
			"a single result must be an error",
		},
		{
			"a second result that is not an error",
			"var _ = httpbind.ServerAction(Two)\n\nfunc Two() (User, User) { return User{}, User{} }",
			"second result must be an error",
		},
		{
			"an unnamed parameter",
			"var _ = httpbind.ServerAction(Anon)\n\nfunc Anon(string) (User, error) { return User{}, nil }",
			"needs a name",
		},
		{
			"a computed published name",
			"var _ = httpbind.ServerAction(Named, name)\n\nvar name = \"x\"\n\nfunc Named() error { return nil }",
			"must be a string literal",
		},
		{
			// A handler-shaped function returns nothing, so declaring one is
			// caught by the result rule rather than by the collision guard, and
			// the result rule names the actual reason it cannot be typed.
			"declared and handler-shaped at once",
			"var _ = httpbind.ServerAction(Both)\n\nfunc Both(w http.ResponseWriter, r *http.Request) {}",
			"returns one value and an error",
		},
		{
			"declared twice",
			"var _ = httpbind.ServerAction(Once)\n\nvar _ = httpbind.ServerAction(Once)\n\nfunc Once() error { return nil }",
			"admitted once",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header := typedActionHeader
			if strings.Contains(tc.body, "http.ResponseWriter") {
				header = strings.Replace(header, `"context"`, "\"context\"\n\t\"net/http\"", 1)
			}
			dir := writeActionPackage(t, header+tc.body)
			_, err := DiscoverActions(dir, "users", "users", "", "")
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q missing %q", err, tc.want)
			}
		})
	}
}

// The registry registers a typed action under the generated entry point rather
// than under the declared function. That is what lets the function be
// unexported and what makes its signature free: the registry names a symbol of
// a fixed shape either way.
func TestRegistryRegistersTheGeneratedEntryPoint(t *testing.T) {
	dir := writeActionPackage(t, typedActionHeader+`
var _ = httpbind.ServerAction(getUser)

func getUser(id string) (User, error) { return User{}, nil }
`)
	actions, err := DiscoverActions(dir, "users", "users", "example.com/app/users", "")
	if err != nil {
		t.Fatal(err)
	}
	action := onlyAction(t, actions, err)
	if action.Wrapper != "ActionGetUser" {
		t.Fatalf("wrapper %q", action.Wrapper)
	}

	e := NewEmitter()
	source, err := e.Registry(&Tree{}, "app", nil, nil, actions)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		// The registration names the wrapper, in the action's own package.
		"users.ActionGetUser)",
		// The table carries the published name a script calls through.
		`Published: "getUser"`,
		"Typed: true",
	} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("registry missing %q:\n%s", want, source)
		}
	}
	if strings.Contains(string(source), "users.getUser)") {
		t.Fatalf("the declared function must not be registered directly:\n%s", source)
	}
}

// A raw handler is still registered as itself, so nothing about the shipped
// shape moves.
func TestRegistryStillRegistersARawHandlerDirectly(t *testing.T) {
	dir := writeActionPackage(t, `package users

import "net/http"

func Rename(w http.ResponseWriter, r *http.Request) {}
`)
	actions, err := DiscoverActions(dir, "users", "users", "example.com/app/users", "")
	if err != nil {
		t.Fatal(err)
	}
	e := NewEmitter()
	source, err := e.Registry(&Tree{}, "app", nil, nil, actions)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "users.Rename)") {
		t.Fatalf("registry missing the raw registration:\n%s", source)
	}
	if !strings.Contains(string(source), "Typed: false") {
		t.Fatalf("registry should mark a raw action untyped:\n%s", source)
	}
}

// The entry point generated for a typed action is an exported function of
// exactly the transport types returning nothing, in a route package — the raw
// admission rule to the letter. Read back on the next run it became a second
// action with its own hash, address and published name, beside the one it
// exists to serve.
func TestGeneratedSourceIsNotDiscoveredAsAnAction(t *testing.T) {
	dir := writeActionPackage(t, typedActionHeader+`
var _ = httpbind.ServerAction(getUser)

func getUser(id string) (User, error) { return User{}, nil }
`)
	// What the binding phase writes beside it on a previous run.
	if err := os.WriteFile(filepath.Join(dir, "action_gen.go"), []byte(`// Code generated by tinybind; DO NOT EDIT.

package users

import "net/http"

func ActionGetUser(w http.ResponseWriter, r *http.Request) {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	actions, err := DiscoverActions(dir, "users", "users", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("want the declared action alone, got %d: %+v", len(actions), actions)
	}
	if actions[0].Name != "getUser" || !actions[0].Typed {
		t.Fatalf("got %+v", actions[0])
	}
}

// A framework brands its own generated output, and this module does not know
// that header until it is told. It is registered through the handler shape, the
// way every other pass takes it — never by a caller naming this module's own
// prefix to avoid rediscovering this module's output.
func TestAFrameworksOwnGeneratedHeaderIsSkippedWhenRegistered(t *testing.T) {
	const branded = `// Code generated by Popcorn Wave via TinyBind; DO NOT EDIT.

package users

import "net/http"

func Branded(w http.ResponseWriter, r *http.Request) {}
`
	dir := writeActionPackage(t, "package users\n")
	if err := os.WriteFile(filepath.Join(dir, "branded_gen.go"), []byte(branded), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unregistered, it is an ordinary file and its handler is an action.
	actions, err := DiscoverActions(dir, "users", "users", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Name != "Branded" {
		t.Fatalf("an unregistered header is not recognized: %+v", actions)
	}

	shape := DefaultHandlerShape()
	shape.GeneratedHeaders = []string{"Code generated by Popcorn Wave"}
	actions, err = DiscoverActionsWith(dir, "users", "users", "", "", shape)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Fatalf("a registered header must be skipped: %+v", actions)
	}
}

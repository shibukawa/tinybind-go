package routetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func logicFile(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), DefaultLogicFile)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func inspect(t *testing.T, source string) *PageFunc {
	t.Helper()
	fn, err := InspectLogic(logicFile(t, source))
	if err != nil {
		t.Fatalf("InspectLogic: %v", err)
	}
	return fn
}

func TestInspectLogicNoFileIsTemplateOnly(t *testing.T) {
	fn, err := InspectLogic("")
	if err != nil {
		t.Fatalf("InspectLogic: %v", err)
	}
	if fn.Rung != RungTemplateOnly {
		t.Errorf("Rung = %v, want %v", fn.Rung, RungTemplateOnly)
	}
}

func TestInspectLogicFileWithoutPageIsTemplateOnly(t *testing.T) {
	fn := inspect(t, `package id_

func helper() string { return "" }
`)
	if fn.Rung != RungTemplateOnly {
		t.Errorf("Rung = %v, want %v", fn.Rung, RungTemplateOnly)
	}
}

func TestInspectLogicTypedPage(t *testing.T) {
	fn := inspect(t, `package id_

type User struct{}
type Order struct{}

func Load(id string, page int) (User, []Order, error) { return User{}, nil, nil }
`)
	if fn.Rung != RungTypedPage {
		t.Fatalf("Rung = %v, want %v", fn.Rung, RungTypedPage)
	}
	if len(fn.Params) != 2 || fn.Params[0].Name != "id" || fn.Params[0].Type != "string" {
		t.Fatalf("Params = %+v", fn.Params)
	}
	if fn.Params[1].Name != "page" || fn.Params[1].Type != "int" {
		t.Fatalf("Params = %+v", fn.Params)
	}
	// The trailing error is the contract, not a result the template renders.
	if len(fn.Results) != 2 || fn.Results[0].Type != "User" || fn.Results[1].Type != "[]Order" {
		t.Fatalf("Results = %+v", fn.Results)
	}
}

func TestInspectLogicExpandsGroupedParameters(t *testing.T) {
	fn := inspect(t, `package id_

func Load(org, id string) (error) { return nil }
`)
	if len(fn.Params) != 2 {
		t.Fatalf("Params = %+v, want two", fn.Params)
	}
	if fn.Params[0].Name != "org" || fn.Params[1].Name != "id" {
		t.Errorf("Params = %+v", fn.Params)
	}
	if fn.Params[0].Type != "string" || fn.Params[1].Type != "string" {
		t.Errorf("grouped type not applied to both: %+v", fn.Params)
	}
}

func TestInspectLogicHandlerPage(t *testing.T) {
	fn := inspect(t, `package id_

import "net/http"

func Load(w http.ResponseWriter, r *http.Request) {}
`)
	if fn.Rung != RungHandlerPage {
		t.Errorf("Rung = %v, want %v", fn.Rung, RungHandlerPage)
	}
}

func TestInspectLogicHandlerPageWithAliasedImport(t *testing.T) {
	fn := inspect(t, `package id_

import nethttp "net/http"

func Load(w nethttp.ResponseWriter, r *nethttp.Request) {}
`)
	if fn.Rung != RungHandlerPage {
		t.Errorf("Rung = %v, want %v", fn.Rung, RungHandlerPage)
	}
}

func TestInspectLogicRejectsSignatureThatIsNeitherShape(t *testing.T) {
	_, err := InspectLogic(logicFile(t, `package id_

func Load(id string) string { return id }
`))
	if err == nil {
		t.Fatal("signature accepted, want rejection")
	}
	message := err.Error()
	// The message must name both contracts, because the author is one edit away
	// from either.
	for _, want := range []string{"error", "http.ResponseWriter"} {
		if !strings.Contains(message, want) {
			t.Errorf("error does not mention %q: %s", want, message)
		}
	}
}

func TestInspectLogicRejectsHandlerShapeWithoutTheImport(t *testing.T) {
	// Without the net/http import these are two ordinary named types, so the
	// declaration is read as a typed Page and fails on its missing error.
	_, err := InspectLogic(logicFile(t, `package id_

func Load(w http.ResponseWriter, r *http.Request) {}
`))
	if err == nil {
		t.Fatal("accepted, want rejection")
	}
}

func TestInspectLogicReportsParseErrors(t *testing.T) {
	if _, err := InspectLogic(logicFile(t, "package id_\n\nfunc Load( {")); err == nil {
		t.Fatal("unparsable file accepted, want error")
	}
}

// route builds a route with the given dynamic parameter names.
func routeWithParams(path string, names ...string) Route {
	route := Route{Path: path}
	for _, name := range names {
		route.Params = append(route.Params, Segment{Dir: name + "_", Name: name, Kind: DynamicSegment})
	}
	return route
}

func TestValidateAcceptsMatchingSignature(t *testing.T) {
	fn := inspect(t, `package id_

func Load(id string, page int) (User, []Order, error) { return User{}, nil, nil }
`)
	component := []Value{{Name: "user", Type: "User"}, {Name: "orders", Type: "[]Order"}}
	if errs := Validate(routeWithParams("/users/{id}", "id"), fn, component); len(errs) != 0 {
		t.Fatalf("Validate = %v, want none", errs)
	}
}

func TestValidateRequiresPathParametersFirst(t *testing.T) {
	fn := inspect(t, `package id_

func Load(page int, id string) (User, error) { return User{}, nil }
`)
	errs := Validate(routeWithParams("/users/{id}", "id"), fn, nil)
	if len(errs) == 0 {
		t.Fatal("reordered parameters accepted, want rejection")
	}
	if !strings.Contains(errs[0].Error(), `"id"`) {
		t.Errorf("error = %v, want it to name the expected parameter", errs[0])
	}
}

func TestValidateRequiresEnoughParameters(t *testing.T) {
	fn := inspect(t, `package id_

func Load() (User, error) { return User{}, nil }
`)
	errs := Validate(routeWithParams("/orgs/{org}/users/{id}", "org", "id"), fn, nil)
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "org, id") {
		t.Errorf("error = %v, want it to list the missing parameters", errs[0])
	}
}

func TestValidateRejectsComplexInputTypes(t *testing.T) {
	fn := inspect(t, `package id_

type Filter struct{}

func Load(id string, filter Filter) (User, error) { return User{}, nil }
`)
	errs := Validate(routeWithParams("/users/{id}", "id"), fn, nil)
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "query parameter") || !strings.Contains(errs[0].Error(), "Filter") {
		t.Errorf("error = %v, want it to name the offending query parameter", errs[0])
	}
}

func TestValidateRejectsComplexPathType(t *testing.T) {
	fn := inspect(t, `package id_

type ID struct{}

func Load(id ID) (User, error) { return User{}, nil }
`)
	errs := Validate(routeWithParams("/users/{id}", "id"), fn, nil)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "path parameter") {
		t.Fatalf("errs = %v, want one path parameter error", errs)
	}
}

func TestValidateRejectsResultMismatch(t *testing.T) {
	fn := inspect(t, `package id_

func Load(id string) (User, error) { return User{}, nil }
`)
	component := []Value{{Name: "user", Type: "User"}, {Name: "orders", Type: "[]Order"}}
	errs := Validate(routeWithParams("/users/{id}", "id"), fn, component)
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "2 parameter") {
		t.Errorf("error = %v, want it to state the component arity", errs[0])
	}
}

func TestValidateRejectsResultTypeMismatch(t *testing.T) {
	fn := inspect(t, `package id_

func Load(id string) (Account, error) { return Account{}, nil }
`)
	component := []Value{{Name: "user", Type: "User"}}
	errs := Validate(routeWithParams("/users/{id}", "id"), fn, component)
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	for _, want := range []string{"Account", "User", "user"} {
		if !strings.Contains(errs[0].Error(), want) {
			t.Errorf("error = %v, want it to mention %q", errs[0], want)
		}
	}
}

func TestValidateSkipsComponentCheckWhenNotSupplied(t *testing.T) {
	fn := inspect(t, `package id_

func Load(id string) (User, []Order, error) { return User{}, nil, nil }
`)
	if errs := Validate(routeWithParams("/users/{id}", "id"), fn, nil); len(errs) != 0 {
		t.Fatalf("Validate = %v, want none", errs)
	}
}

func TestValidateIgnoresOtherRungs(t *testing.T) {
	for _, source := range []string{
		"package id_\n",
		"package id_\n\nimport \"net/http\"\n\nfunc Load(w http.ResponseWriter, r *http.Request) {}\n",
	} {
		fn := inspect(t, source)
		if errs := Validate(routeWithParams("/users/{id}", "id"), fn, nil); len(errs) != 0 {
			t.Errorf("Validate(%v) = %v, want none", fn.Rung, errs)
		}
	}
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	fn := inspect(t, `package id_

type Filter struct{}

func Load(wrong string, filter Filter) (User, error) { return User{}, nil }
`)
	errs := Validate(routeWithParams("/users/{id}", "id"), fn, nil)
	if len(errs) < 2 {
		t.Fatalf("errs = %v, want the name and the type problem", errs)
	}
}

package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// getUserAction is the request's own example, read back as the input this phase
// takes: routetree reads the declaration, this phase builds the argument struct
// and the codecs.
func getUserAction() generator.ServerAction {
	return generator.ServerAction{
		Func:         "GetUser",
		TakesContext: true,
		Params:       []generator.ServerActionParam{{Name: "id", Type: "string"}},
		Result:       "User",
	}
}

func emitActionSource(t *testing.T, body string, actions ...generator.ServerAction) (string, string) {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package main

import "context"

var _ = context.Background

type User struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

` + body + `

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)

	opts := generator.DefaultOptions()
	opts.ServerActions = actions
	plan, err := generator.New(opts).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	return dir, string(code)
}

func TestServerActionWrapperIsEmitted(t *testing.T) {
	_, source := emitActionSource(t, `
func GetUser(ctx context.Context, id string) (User, error) { return User{ID: id}, nil }`,
		getUserAction())

	mustContainAll(t, source,
		"type actionGetUserInput struct",
		"Id string `json:\"id\"`",
		"func decodeactionGetUserInputBytes(data []byte) (actionGetUserInput, error)",
		"func ActionGetUser(w http.ResponseWriter, r *http.Request)",
		"httpbind.ReadActionBody(r)",
		"GetUser(r.Context(), input.Id)",
		// The wrapper names the generated encoder rather than reaching a
		// runtime registry, which is what removes the missing-codec 500.
		"appendUserJSON((*buf)[:0], out)",
		"httpbind.WriteJSONBytes(w, http.StatusOK, *buf)",
	)
	mustContainNone(t, source, "httpbind.Write[", "jsonbind.EncodeJSON")
}

// The result type gets its encoder because the declaration asked for it. No
// call site in the package names it, and the one the generator writes is inside
// a file every analysis is required to skip.
func TestServerActionResultTypeGetsItsEncoder(t *testing.T) {
	_, source := emitActionSource(t, `
func GetUser(ctx context.Context, id string) (User, error) { return User{ID: id}, nil }`,
		getUserAction())
	mustContainAll(t, source, "func appendUserJSON(dst []byte, v User) []byte")
}

func TestServerActionWithoutAContext(t *testing.T) {
	_, source := emitActionSource(t, `
func GetUser(id string) (User, error) { return User{ID: id}, nil }`,
		generator.ServerAction{
			Func:   "GetUser",
			Params: []generator.ServerActionParam{{Name: "id", Type: "string"}},
			Result: "User",
		})
	mustContainAll(t, source, "GetUser(input.Id)")
	mustContainNone(t, source, "r.Context()")
}

// A struct parameter is the ordinary case: the argument struct holds it as a
// nested field and the existing decoder machinery reads it.
func TestServerActionTakesAStructParameter(t *testing.T) {
	_, source := emitActionSource(t, `
type CreateUserRequest struct {
	Name string `+"`json:\"name\"`"+`
}

func CreateUser(req CreateUserRequest) (User, error) { return User{Name: req.Name}, nil }`,
		generator.ServerAction{
			Func:   "CreateUser",
			Params: []generator.ServerActionParam{{Name: "req", Type: "CreateUserRequest"}},
			Result: "User",
		})
	mustContainAll(t, source,
		"Req CreateUserRequest `json:\"req\"`",
		"func decodeCreateUserRequestBytes",
		"CreateUser(input.Req)",
	)
}

// An error-only action produces no body, so it answers no content rather than
// an empty document.
func TestServerActionReturningOnlyAnErrorAnswersNoContent(t *testing.T) {
	_, source := emitActionSource(t, `
func Touch(id string) error { return nil }`,
		generator.ServerAction{
			Func:   "Touch",
			Params: []generator.ServerActionParam{{Name: "id", Type: "string"}},
		})
	mustContainAll(t, source,
		"if err := Touch(input.Id); err != nil",
		"w.WriteHeader(http.StatusNoContent)",
	)
	mustContainNone(t, source, "GetBuffer")
}

// The declared function may be unexported, because the wrapper sits beside it
// and the registry names the wrapper.
func TestServerActionWrapsAnUnexportedFunction(t *testing.T) {
	_, source := emitActionSource(t, `
func getUser(id string) (User, error) { return User{ID: id}, nil }`,
		generator.ServerAction{
			Func:   "getUser",
			Params: []generator.ServerActionParam{{Name: "id", Type: "string"}},
			Result: "User",
		})
	mustContainAll(t, source, "func ActionGetUser(", "getUser(input.Id)")
}

// The generated entry point has to compile and answer, which is what proves the
// pieces name each other correctly rather than merely appearing.
func TestServerActionEntryPointAnswers(t *testing.T) {
	dir, code := emitActionSource(t, `
func GetUser(ctx context.Context, id string) (User, error) { return User{ID: id, Name: "ada"}, nil }`,
		getUserAction())
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := `package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnswers(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_action/x/GetUser", strings.NewReader(` + "`" + `{"id":"3"}` + "`" + `))
	ActionGetUser(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	const want = ` + "`" + `{"id":"3","name":"ada"}` + "`" + `
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(probe), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated entry point does not answer: %v\n%s\n%s", err, output, code)
	}
}

func TestServerActionRefusesAnUnparseableParameterType(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"),
		[]byte("package main\n\nfunc Touch(id string) error { return nil }\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.ServerActions = []generator.ServerAction{{
		Func:   "Touch",
		Params: []generator.ServerActionParam{{Name: "id", Type: "func("}},
	}}
	_, err := generator.New(opts).Analyze(dir)
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "not a type expression") {
		t.Fatalf("unexpected error: %v", err)
	}
}

package fixture

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/sqlbind"

	"tempmod/pw"
)

func TestHTMLComponentIsWriterShaped(t *testing.T) {
	var out strings.Builder
	if err := UserPage(&out, UserPageParams{User: User{Name: "a<b"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<p>a&lt;b</p>") {
		t.Fatalf("rendered %q", out.String())
	}
}

func TestWriteHTMLAcceptsGeneratedComponent(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/page", nil)
	if err := pw.WriteHTML(recorder, request, UserPage, UserPageParams{User: User{Name: "Ada"}}); err != nil {
		t.Fatal(err)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(recorder.Body.String(), "<p>Ada</p>") {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestSQLPublicAPIIsContextOnly(t *testing.T) {
	if _, err := FindUser(context.Background(), 1); !errors.Is(err, sqlbind.ErrNoSQLExecutor) {
		t.Fatalf("FindUser without executor = %v", err)
	}
	statement, err := BuildFindUser(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(statement.Args) != 1 || statement.Args[0] != 7 {
		t.Fatalf("statement = %#v", statement)
	}
}

func TestRequestBindingIsGenerated(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Ada"}`))
	request.Header.Set("Content-Type", "application/json")
	createUser(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"id":3`) {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

// TestConfigRegistrationsResolve relies on the runtime contract of
// pw.RegisterConfig and pw.SubCommand: both panic when the generated
// definition is missing, or when the registered subcommand name and help do
// not match the call site. Completing both calls therefore proves the
// definitions were generated with the right name and help.
func TestConfigRegistrationsResolve(t *testing.T) {
	if LoadAppConfig() == nil {
		t.Fatal("AppConfig binding did not allocate a target")
	}
	if GenerateConfigOptions() != nil {
		t.Fatal("generate-config must not be the selected subcommand under go test")
	}
}

func TestRoutesRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
}

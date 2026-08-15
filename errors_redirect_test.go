package httpbind_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/htmlbind"
)

type redirectParams struct{ ID string }

type redirectScope struct {
	Outer redirectParams
	Value string
}

// A redirect travels the error return, because a caller that returns values
// rather than holding a ResponseWriter has no other channel. It emits a
// Location and no problem document: the browser is being sent somewhere, not
// told what went wrong.
func TestRedirectWritesALocation(t *testing.T) {
	recorder := httptest.NewRecorder()
	httpbind.WriteError(recorder, httptest.NewRequest(http.MethodGet, "/old", nil),
		httpbind.Redirect("/new"))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "/new" {
		t.Fatalf("Location = %q, want /new", got)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("wrote a body: %q", recorder.Body.String())
	}
}

// The default is 303, which is what a page wants after a POST, and the other
// four are available.
func TestRedirectStatusChoices(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		recorder := httptest.NewRecorder()
		httpbind.WriteError(recorder, httptest.NewRequest(http.MethodGet, "/", nil),
			httpbind.Redirect("/new", status))
		if recorder.Code != status {
			t.Fatalf("status = %d, want %d", recorder.Code, status)
		}
	}
	// A status no client would follow is refused where it is written rather
	// than emitted and left to fail in the browser.
	recorder := httptest.NewRecorder()
	httpbind.WriteError(recorder, httptest.NewRequest(http.MethodGet, "/", nil),
		httpbind.Redirect("/new", 200))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the refusal", recorder.Code)
	}
	if recorder.Header().Get("Location") != "" {
		t.Fatalf("a refused redirect still set a Location")
	}
}

// The point of decision:value-binding-hoisting: a template's own loader chooses
// the response, from inside the component, with nothing written.
func TestAFailingBindingChoosesTheResponse(t *testing.T) {
	for name, fail := range map[string]error{
		"a redirect":  httpbind.Redirect("/sign-in"),
		"a not found": httpbind.NotFound(httpbind.Problem{Code: "absent", Message: "no such record"}),
		"a forbidden": httpbind.Forbidden(httpbind.Problem{Code: "denied", Message: "not yours"}),
	} {
		t.Run(name, func(t *testing.T) {
			body := htmlbind.Builder[redirectScope]{}
			leaf := &htmlbind.Plan[redirectParams]{Ops: []htmlbind.Op[redirectParams]{
				htmlbind.Builder[redirectParams]{}.Static("<main>"),
				htmlbind.ValErr(
					func(p redirectParams) (string, error) { return "", fail },
					func(p redirectParams, v string) redirectScope { return redirectScope{Outer: p, Value: v} },
					[]htmlbind.Op[redirectScope]{body.Text(func(p redirectScope) string { return p.Value })}),
			}}
			var out strings.Builder
			err := htmlbind.Render(&out, htmlbind.Bind(leaf, redirectParams{ID: "7"}))
			if !errors.Is(err, fail) {
				t.Fatalf("render error = %v, want %v", err, fail)
			}
			if out.Len() != 0 {
				t.Fatalf("wrote %q before the loader failed", out.String())
			}
			recorder := httptest.NewRecorder()
			httpbind.WriteError(recorder, httptest.NewRequest(http.MethodGet, "/", nil), err)
			if recorder.Code == http.StatusOK {
				t.Fatalf("the failure did not reach the response")
			}
		})
	}
}

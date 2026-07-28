package pagesfixture

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/tinybind-go/internal/pagesfixture/pages"
)

// The framework-owner guide tells readers to put hand-registered routes and
// generated page routes on one mux. These tests are what that claim rests on.

func post(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, target, nil))
	return rec
}

// TestComposedRoutersShareOnePathUnderDifferentMethods is the documented shape:
// a form target registered by hand beside the generated page it posts from.
func TestComposedRoutersShareOnePathUnderDifferentMethods(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handled-By", "registered")
		w.Header().Set("X-ID", r.PathValue("id"))
	})
	pages.Register(mux)

	if rec := post(t, mux, "/users/alice"); rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body = %s", rec.Code, rec.Body)
	} else if got := rec.Header().Get("X-Handled-By"); got != "registered" {
		t.Errorf("POST reached %q, want the hand-registered handler", got)
	} else if got := rec.Header().Get("X-ID"); got != "alice" {
		t.Errorf("hand-registered handler saw id = %q", got)
	}

	rec := get(t, mux, "/users/alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
	if body := rec.Body.String(); body == "" || rec.Header().Get("X-Handled-By") != "" {
		t.Errorf("GET did not reach the generated page: %q", body)
	}
}

func TestComposedRoutersAreOrderIndependent(t *testing.T) {
	// Registering the generated routes first must work as well as last.
	mux := http.NewServeMux()
	pages.Register(mux)
	mux.HandleFunc("POST /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handled-By", "registered")
	})

	if rec := post(t, mux, "/users/bob"); rec.Header().Get("X-Handled-By") != "registered" {
		t.Errorf("POST did not reach the hand-registered handler: %d", rec.Code)
	}
	if rec := get(t, mux, "/users/bob"); rec.Code != http.StatusOK {
		t.Errorf("GET status = %d", rec.Code)
	}
}

// TestGeneratedRootDoesNotSwallowHandRegisteredPrefixes checks the consequence
// of registering the root page as /{$}: a hand-registered subtree still wins its
// own paths, and an unmatched path is still a 404.
func TestGeneratedRootDoesNotSwallowHandRegisteredPrefixes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handled-By", "registered")
	})
	pages.Register(mux)

	if rec := get(t, mux, "/api/users"); rec.Header().Get("X-Handled-By") != "registered" {
		t.Errorf("hand-registered subtree lost to the generated root: %d", rec.Code)
	}
	if rec := get(t, mux, "/"); rec.Code != http.StatusOK || rec.Header().Get("X-Handled-By") != "" {
		t.Errorf("generated root page not reached: %d", rec.Code)
	}
	if rec := get(t, mux, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unmatched path = %d, want 404", rec.Code)
	}
}

// TestExactDuplicatePatternPanics records the one way composition fails. The
// standard library panics on a duplicate pattern, so a hand-registered route
// that collides with a generated one is a startup crash rather than a silent
// override — worth knowing before it happens in production.
func TestExactDuplicatePatternPanics(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {})

	defer func() {
		if recover() == nil {
			t.Fatal("duplicate pattern accepted; the guide claims it panics")
		}
	}()
	pages.Register(mux)
}

// TestMethodNotAllowedSurvivesComposition confirms the stdlib still answers 405
// for a path that exists under another method, rather than 404, once both
// routers share the mux.
func TestMethodNotAllowedSurvivesComposition(t *testing.T) {
	mux := http.NewServeMux()
	pages.Register(mux)

	if rec := post(t, mux, "/about"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /about = %d, want 405", rec.Code)
	}
}

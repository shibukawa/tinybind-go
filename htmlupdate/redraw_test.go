package htmlupdate_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// The Render function stands in for generated code: a typed decoder that
// refuses a value it cannot parse rather than substituting a zero.
const cardKind = "UserCard@8Qv3n1"

func cardRegistry(t *testing.T) *htmlupdate.Registry {
	t.Helper()
	registry := &htmlupdate.Registry{}
	registry.Register(htmlupdate.Reloadable{
		KindID: cardKind,
		Render: func(r *http.Request, instanceID string, values url.Values) (htmlbind.Fragment, error) {
			page, err := strconv.Atoi(values.Get("page"))
			if err != nil {
				return htmlbind.Fragment{}, errors.New("page must be an integer")
			}
			// A registered component authorizes its own inputs, because the
			// caller supplies them. This one refuses a page nobody may see.
			if page > 10 {
				return htmlbind.Fragment{}, errors.New("forbidden page")
			}
			return htmlbind.Bind(badgePlan, badgeParams{ID: instanceID, Count: page}), nil
		},
	})
	return registry
}

// buildRequest carries the build identity the page was rendered by, which a
// real page gets from its runtime script tag.
func buildRequest(path string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
	return request
}

func redrawServer(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	options.Mount(mux, cardRegistry(t))
	return mux
}

func TestRedrawRendersOneComponent(t *testing.T) {
	path := options.RedrawPath(cardKind, "card-1", url.Values{"page": {"2"}})
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, buildRequest(path))
	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type = %q", got)
	}
	// The URL alone identifies the response and the content is usually
	// per-user, so a shared cache must never hold it.
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	body := read(t, response)
	// The replacement keeps the instance id, or the region stops being
	// addressable after the first redraw.
	if !strings.Contains(body, `id="card-1"`) || !strings.Contains(body, ">2<") {
		t.Fatalf("unexpected markup %q", body)
	}
}

// Nothing is registered implicitly: being renderable is not enough, because
// publishing an endpoint has to be deliberate.
func TestUnregisteredComponentHasNoEndpoint(t *testing.T) {
	path := options.RedrawPath("Secret@abc", "x", url.Values{"page": {"1"}})
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, buildRequest(path))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

// A template edit changes the kind hash, so a page loaded before a deploy asks
// for a kind that no longer exists and must reload rather than render under
// changed semantics.
func TestStaleKindIsNotFound(t *testing.T) {
	path := options.RedrawPath("UserCard@olderhash", "card-1", url.Values{"page": {"1"}})
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, buildRequest(path))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

// The parameters are public input, so the component's own checks are what
// decides, not the fact that the browser sent them.
func TestRedrawParametersAreValidatedAndAuthorized(t *testing.T) {
	for name, values := range map[string]url.Values{
		"undecodable": {"page": {"many"}},
		"missing":     {},
		"refused":     {"page": {"999"}},
	} {
		t.Run(name, func(t *testing.T) {
			path := options.RedrawPath(cardKind, "card-1", values)
			recorder := httptest.NewRecorder()
			redrawServer(t).ServeHTTP(recorder, buildRequest(path))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}
		})
	}
}

// A GET carries every argument in the URL, so the length has a bound.
func TestOversizedRedrawQueryIsRejected(t *testing.T) {
	path := options.RedrawPath(cardKind, "card-1", url.Values{"page": {"1"}}) +
		"&pad=" + strings.Repeat("x", htmlupdate.MaxQueryBytes)
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, buildRequest(path))
	if recorder.Code != http.StatusRequestURITooLong {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestMalformedRedrawPathIsNotFound(t *testing.T) {
	for _, path := range []string{
		htmlupdate.DefaultPathPrefix + "/redraw/",
		htmlupdate.DefaultPathPrefix + "/redraw/" + cardKind,
		htmlupdate.DefaultPathPrefix + "/redraw/" + cardKind + "/",
		htmlupdate.DefaultPathPrefix + "/redraw/" + cardKind + "/a/b",
	} {
		recorder := httptest.NewRecorder()
		redrawServer(t).ServeHTTP(recorder, buildRequest(path))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("path %q gave %d, want 404", path, recorder.Code)
		}
	}
}

// The kind covers a component's name, parameters, and compiled markup but not
// its package, so two identical templates in different packages collide.
// Keeping the last registration would serve a component that looks the same but
// calls its own package's external functions, so registration refuses instead.
func TestDuplicateKindIsRefused(t *testing.T) {
	defer func() {
		message, ok := recover().(string)
		if !ok || !strings.Contains(message, cardKind) {
			t.Fatalf("want a panic naming the kind, got %v", message)
		}
	}()
	registry := cardRegistry(t)
	registry.Register(htmlupdate.Reloadable{KindID: cardKind, Render: nil})
	t.Fatal("a repeated kind must not overwrite silently")
}

func TestRegisteringWithoutAKindIsRefused(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a component with no kind must be refused")
		}
	}()
	(&htmlupdate.Registry{}).Register(htmlupdate.Reloadable{})
}

// A kind is stable across builds on purpose, so it cannot say whether the page
// asking is current. The build identity does, and it covers every change a kind
// cannot see: a component this one calls, an external function, the runtime.
func TestRedrawFromAnotherBuildIsRefused(t *testing.T) {
	path := options.RedrawPath(cardKind, "card-1", url.Values{"page": {"2"}})
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("X-Tinybind-Build", "older-revision")
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
}

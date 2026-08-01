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
	// The content is usually per-user, so no shared cache may hold it. It is
	// still revalidatable, because no-store would forbid the conditional
	// request the ETag exists for.
	if got := response.Header.Get("Cache-Control"); got != htmlupdate.DefaultRedrawCacheControl {
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
		"&pad=" + strings.Repeat("x", htmlupdate.DefaultMaxQueryBytes)
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, buildRequest(path))
	if recorder.Code != http.StatusRequestURITooLong {
		t.Fatalf("status = %d", recorder.Code)
	}
}

// An unchanged redraw costs a 304 rather than its whole markup, which is what
// the endpoint was designed for and what a fixed no-store had made impossible.
func TestUnchangedRedrawAnswers304(t *testing.T) {
	path := options.RedrawPath(cardKind, "card-1", url.Values{"page": {"2"}})
	server := redrawServer(t)

	first := httptest.NewRecorder()
	server.ServeHTTP(first, buildRequest(path))
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag, so nothing can be revalidated")
	}
	// The response depends on the build that rendered the page, so a cache
	// holding it has to key on that too.
	if !strings.Contains(first.Header().Get("Vary"), "Build") {
		t.Fatalf("Vary = %q, want the build header", first.Header().Get("Vary"))
	}

	conditional := buildRequest(path)
	conditional.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	server.ServeHTTP(second, conditional)
	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 carried a body: %q", second.Body.String())
	}

	// A different render is a different tag, so the browser gets the new bytes.
	changed := buildRequest(options.RedrawPath(cardKind, "card-1", url.Values{"page": {"3"}}))
	changed.Header.Set("If-None-Match", etag)
	third := httptest.NewRecorder()
	server.ServeHTTP(third, changed)
	if third.Code != http.StatusOK {
		t.Fatalf("status = %d, want the changed render", third.Code)
	}
}

// A 304 confirms a guess, and a redraw usually renders low-entropy per-user
// content, so the tag is keyed for the same reason a frame validator is.
func TestRedrawETagIsKeyed(t *testing.T) {
	tag := func(key string) string {
		opts := htmlupdate.Options{Key: []byte(key)}
		mux := http.NewServeMux()
		opts.Mount(mux, cardRegistry(t))
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, buildRequest(opts.RedrawPath(cardKind, "card-1", url.Values{"page": {"2"}})))
		return recorder.Header().Get("ETag")
	}
	if tag("one key") == tag("another key") {
		t.Fatal("the same render digests identically under two keys, so a guess confirms across deployments")
	}
}

// A deployment whose redraws are public, or whose proxy needs different terms,
// supplies its own policy.
func TestRedrawCachePolicyIsConfigurable(t *testing.T) {
	custom := options
	custom.RedrawCacheControl = "public, max-age=60"
	mux := http.NewServeMux()
	custom.Mount(mux, cardRegistry(t))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, buildRequest(custom.RedrawPath(cardKind, "card-1", url.Values{"page": {"2"}})))
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

// countingRouter is a router that is not *http.ServeMux, which is the whole
// point: a framework with its own mux could not call Mount at all.
type countingRouter struct {
	patterns []string
	mux      *http.ServeMux
}

func (c *countingRouter) Handle(pattern string, handler http.Handler) {
	c.patterns = append(c.patterns, pattern)
	c.mux.Handle(pattern, handler)
}

func TestMountAcceptsAnyRouter(t *testing.T) {
	router := &countingRouter{mux: http.NewServeMux()}
	options.Mount(router, cardRegistry(t))
	if len(router.patterns) != 2 {
		t.Fatalf("registered %v, want the runtime asset and the redraw endpoint", router.patterns)
	}
	recorder := httptest.NewRecorder()
	router.mux.ServeHTTP(recorder, buildRequest(options.RedrawPath(cardKind, "card-1", url.Values{"page": {"2"}})))
	if recorder.Code != http.StatusOK {
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
// Failing at startup and panicking are not the same requirement: a caller
// running its own validation pass collects every problem and reports them
// together, which a panic on the first one makes impossible.
func TestDuplicateKindIsRefused(t *testing.T) {
	registry := cardRegistry(t)
	err := registry.Register(htmlupdate.Reloadable{KindID: cardKind, Render: nil})
	if err == nil {
		t.Fatal("a repeated kind must not overwrite silently")
	}
	if !strings.Contains(err.Error(), cardKind) {
		t.Fatalf("want an error naming the kind, got %v", err)
	}
	// The registration that was already there stands, so a refused duplicate
	// leaves a working endpoint rather than a half-replaced one.
	recorder := httptest.NewRecorder()
	path := options.RedrawPath(cardKind, "card-1", url.Values{"page": {"2"}})
	mux := http.NewServeMux()
	options.Mount(mux, registry)
	mux.ServeHTTP(recorder, buildRequest(path))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the first registration to stand", recorder.Code)
	}
}

func TestRegisteringWithoutAKindIsRefused(t *testing.T) {
	if err := (&htmlupdate.Registry{}).Register(htmlupdate.Reloadable{}); err == nil {
		t.Fatal("a component with no kind must be refused")
	}
}

// A caller with nowhere to return an error still has the abort one line away.
func TestMustRegisterPanicsOnADuplicate(t *testing.T) {
	defer func() {
		message, ok := recover().(string)
		if !ok || !strings.Contains(message, cardKind) {
			t.Fatalf("want a panic naming the kind, got %v", message)
		}
	}()
	cardRegistry(t).MustRegister(htmlupdate.Reloadable{KindID: cardKind})
	t.Fatal("MustRegister must not accept a duplicate")
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

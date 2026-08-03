package htmlupdate_test

import (
	"encoding/base64"
	"encoding/json"
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

// The stylesheet of a component appearing for the first time is not in the live
// document head, and its markup landing before the sheet does is the flash of
// unstyled content the navigation delta added its own head field to prevent. A
// redraw's body is the bare subtree, so the contribution travels beside it.
const styledKind = "StyledCard@2Rq9x0"

var styledHead = []string{`<link rel="stylesheet" href="/public/generated/cards.style.abc123.css">`}

var styledAsset = htmlbind.Asset{
	ID:   "cards.style.abc123",
	Type: htmlbind.AssetTypeStyle,
	URL:  "/public/generated/cards.style.abc123.css",
}

func styledRegistry(t *testing.T) *htmlupdate.Registry {
	t.Helper()
	registry := &htmlupdate.Registry{}
	if err := registry.Register(htmlupdate.Reloadable{
		KindID: styledKind,
		Head:   styledHead,
		Assets: []htmlbind.Asset{styledAsset},
		Render: func(r *http.Request, instanceID string, values url.Values) (htmlbind.Fragment, error) {
			return htmlbind.Bind(badgePlan, badgeParams{ID: instanceID, Count: 1}), nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestRedrawCarriesTheComponentHead(t *testing.T) {
	mux := http.NewServeMux()
	options.Mount(mux, styledRegistry(t))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, buildRequest(options.RedrawPath(styledKind, "card-1", nil)))
	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	encoded := response.Header.Get("X-Tinybind-Head")
	if encoded == "" {
		t.Fatal("a component contributing head must say so")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("head header is not decodable: %v", err)
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		t.Fatalf("head header is not a tag list: %v", err)
	}
	if len(tags) != 1 || tags[0] != styledHead[0] {
		t.Fatalf("head = %q, want the component's contribution", tags)
	}
	// The body stays the bare subtree: no envelope, so the endpoint is still
	// what curl shows and a client parses what it already parsed.
	body := recorder.Body.String()
	if !strings.HasPrefix(strings.TrimSpace(body), "<span") {
		t.Fatalf("body is not a bare fragment: %q", body)
	}
}

// A component contributing no head produces the response it produced before the
// field existed.
func TestRedrawWithoutHeadIsUnchanged(t *testing.T) {
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, buildRequest(options.RedrawPath(cardKind, "card-1", url.Values{"page": {"2"}})))
	if got := recorder.Header().Get("X-Tinybind-Head"); got != "" {
		t.Fatalf("head header = %q, want none", got)
	}
}

// The required set is readable without a request and without a render, so a
// document shell built once at startup covers every redraw the deployment will
// serve. That is what makes the guarantee a caller gives a checkable one.
func TestRegistryPublishesWhatARedrawRequires(t *testing.T) {
	registry := styledRegistry(t)
	if err := registry.Register(htmlupdate.Reloadable{
		KindID: "Second@1a2b3c",
		// The same stylesheet, because two components of one generation unit
		// share a bundle. It must appear once.
		Head:   styledHead,
		Assets: []htmlbind.Asset{styledAsset},
		Render: func(r *http.Request, instanceID string, values url.Values) (htmlbind.Fragment, error) {
			return htmlbind.Bind(badgePlan, badgeParams{ID: instanceID}), nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if got := registry.RequiredHead(); len(got) != 1 || got[0] != styledHead[0] {
		t.Fatalf("required head = %q, want one shared tag", got)
	}
	if got := registry.RequiredAssets(); len(got) != 1 || got[0] != styledAsset {
		t.Fatalf("required assets = %+v, want one shared file", got)
	}
	if got := cardRegistry(t).RequiredHead(); got != nil {
		t.Fatalf("a registry of components contributing nothing requires %q", got)
	}
}

// An oversized head is a fact about the templates rather than about a request,
// so it is discovered at startup instead of by a proxy dropping the header in
// production.
func TestOversizedHeadIsRefusedAtRegistration(t *testing.T) {
	huge := make([]string, 0, 64)
	for i := 0; i < 64; i++ {
		huge = append(huge, `<link rel="stylesheet" href="/public/generated/`+strings.Repeat("x", 60)+strconv.Itoa(i)+`.css">`)
	}
	err := (&htmlupdate.Registry{}).Register(htmlupdate.Reloadable{
		KindID: "Huge@000000",
		Head:   huge,
		Render: func(r *http.Request, instanceID string, values url.Values) (htmlbind.Fragment, error) {
			return htmlbind.Fragment{}, nil
		},
	})
	if err == nil {
		t.Fatal("an oversized head must be refused while a caller can still act on it")
	}
	if !strings.Contains(err.Error(), "RequiredHead") {
		t.Fatalf("the failure must name the way out, got %v", err)
	}
}

// The token reaches this package by two channels because a browser has two: the
// runtime sends a header on everything it fetches, and a form submitted without
// script carries the hidden field instead.
func TestCSRFTokenIsReadFromEitherChannel(t *testing.T) {
	header := httptest.NewRequest(http.MethodPost, "/send", nil)
	header.Header.Set("X-CSRF-Token", "tok")
	if got := options.CSRFToken(header); got != "tok" {
		t.Fatalf("header channel = %q", got)
	}

	form := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader("_csrf=tok&body=x"))
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := options.CSRFToken(form); got != "tok" {
		t.Fatalf("form channel = %q", got)
	}

	// The header wins, so an ordinary fetch never pays for parsing a body it
	// does not have.
	both := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader("_csrf=from-body"))
	both.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	both.Header.Set("X-CSRF-Token", "from-header")
	if got := options.CSRFToken(both); got != "from-header" {
		t.Fatalf("want the header to win, got %q", got)
	}
}

func TestVerifyCSRF(t *testing.T) {
	request := func(token string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/send", nil)
		if token != "" {
			r.Header.Set("X-CSRF-Token", token)
		}
		return r
	}
	if err := options.VerifyCSRF(request("tok"), "tok"); err != nil {
		t.Fatalf("a matching token must pass: %v", err)
	}
	if err := options.VerifyCSRF(request("other"), "tok"); !errors.Is(err, htmlupdate.ErrCSRFMismatch) {
		t.Fatalf("want a mismatch, got %v", err)
	}
	if err := options.VerifyCSRF(request(""), "tok"); !errors.Is(err, htmlupdate.ErrCSRFMissing) {
		t.Fatalf("want a missing token, got %v", err)
	}
	// A session lookup that quietly returned nothing would otherwise disable the
	// whole control for exactly the requests that most need it.
	if err := options.VerifyCSRF(request("tok"), ""); !errors.Is(err, htmlupdate.ErrCSRFMissing) {
		t.Fatalf("an empty expectation must be refused, got %v", err)
	}
}

// The token belongs to a session and the options belong to the process, so it
// reaches the runtime through the script tag rather than through the config.
func TestScriptTagCarriesTheSessionToken(t *testing.T) {
	// The header name travels either way, because it is a name rather than a
	// secret; the token itself does not.
	if got := options.ScriptTag(); strings.Contains(got, `"csrf"`) || strings.Contains(got, "tok") {
		t.Fatalf("the plain tag carries no session token: %s", got)
	}
	tagged := options.ScriptTagFor("tok")
	if !strings.Contains(tagged, "csrf") || !strings.Contains(tagged, "tok") {
		t.Fatalf("the token never reached the tag: %s", tagged)
	}
	if got := options.RuntimeConfigFor("tok"); got.CSRF != "tok" || got.CSRFHeader != htmlupdate.DefaultCSRFHeaderName {
		t.Fatalf("config = %+v", got)
	}
}

// The generator writes the field and this reads it back, and nothing links the
// two at compile time, so a renamed field has to reach both.
func TestConfiguredCSRFFieldNameIsRead(t *testing.T) {
	renamed := options
	renamed.CSRFFieldName = "authenticity_token"
	request := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader("authenticity_token=tok"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := renamed.CSRFToken(request); got != "tok" {
		t.Fatalf("the configured field was not read, got %q", got)
	}
}

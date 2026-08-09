package htmlupdate_test

import (
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

// redrawInto is what a caller writes now: the entry computes an answer and the
// caller sends it. This package sets no header and no status of its own, so a
// test that wants the response has to write it, exactly as a handler does.
func redrawInto(w http.ResponseWriter, o htmlupdate.Options, r *http.Request,
	reg *htmlupdate.Registry, opts ...htmlbind.Option) bool {
	answer, ok := o.Redraw(r, reg, opts...)
	if !ok {
		return false
	}
	// The policy and the conditional answer are both the caller's now. This one
	// keeps a redraw out of every shared cache and lets a private one
	// revalidate, which is what this package used to choose for everybody.
	w.Header().Set("Cache-Control", redrawCachePolicy)
	if answer.NotModified(r) {
		htmlupdate.ApplyTo(answer.Header, w)
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	_, _ = answer.WriteTo(w)
	return true
}

// redrawCachePolicy is this test caller's choice, not a package default.
var redrawCachePolicy = "private, no-cache"

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

// redrawTarget is the page URL these tests answer redraws from. Any URL works —
// that is the point — and a page URL is the one a deployment actually uses.
const redrawTarget = "/dashboard"

// redrawRequest builds the published redraw shape: the component in headers, the
// URL the caller's own.
func redrawRequest(kindID, instanceID string, values url.Values) *http.Request {
	path := redrawTarget
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	request := buildRequest(path)
	request.Header.Set("X-Tinybind-Render", "redraw")
	request.Header.Set("X-Tinybind-Kind", kindID)
	request.Header.Set("X-Tinybind-Instance", instanceID)
	return request
}

// redrawServer is a caller's own page handler, in the shape the guide shows:
// the redraw branch first, the page behind it.
func redrawServer(t *testing.T) http.Handler {
	t.Helper()
	return redrawServerWith(t, options)
}

func redrawServerWith(t *testing.T, opts htmlupdate.Options) http.Handler {
	t.Helper()
	registry := cardRegistry(t)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redrawInto(w, opts, r, registry) {
			return
		}
		_, _ = w.Write([]byte("<!doctype html><title>the page</title>"))
	})
}

// redrawBody decodes the response every update path now returns: the region's
// own fragment, a hole where each nested boundary sits, the head it contributes,
// and the manifest entry a redraw used to leave stale.
func redrawBody(t *testing.T, response *http.Response) deltaBody {
	t.Helper()
	var body deltaBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode redraw: %v", err)
	}
	return body
}

// redrawHTML is the markup of the operation targeting instance.
func redrawHTML(t *testing.T, response *http.Response, instance string) string {
	t.Helper()
	for _, operation := range redrawBody(t, response).Operations {
		if operation.ID == instance {
			return operation.HTML
		}
	}
	t.Fatalf("no operation targets %q", instance)
	return ""
}

func TestRedrawRendersOneComponent(t *testing.T) {
	request := redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}})
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, request)
	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	// The body is the shape every other update path returns, so one client
	// applies them all. It was a bare fragment with its head in a header, which
	// made the redraw the only response in this package with a form of its own.
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q", got)
	}
	// The content is usually per-user, so no shared cache may hold it. It is
	// still revalidatable, because no-store would forbid the conditional
	// request the ETag exists for.
	if got := response.Header.Get("Cache-Control"); got != redrawCachePolicy {
		t.Fatalf("Cache-Control = %q", got)
	}
	body := redrawBody(t, response)
	if len(body.Operations) != 1 || body.Operations[0].ID != "card-1" {
		t.Fatalf("want one operation naming the instance, got %+v", body.Operations)
	}
	// The replacement keeps the instance id, or the region stops being
	// addressable after the first redraw.
	if markup := body.Operations[0].HTML; !strings.Contains(markup, `id="card-1"`) || !strings.Contains(markup, ">2<") {
		t.Fatalf("unexpected markup %q", markup)
	}
	// A reloadable component is an update boundary, so the client held a
	// validator this replacement just made wrong. Returning the new one is what
	// keeps the next navigation delta from re-sending a region already right.
	if len(body.Manifest) != 1 || body.Manifest[0].ID != "card-1" || body.Manifest[0].Frame == "" {
		t.Fatalf("a redraw must return the instance's new validator, got %+v", body.Manifest)
	}
}

// Nothing is registered implicitly: being renderable is not enough, because
// publishing an endpoint has to be deliberate.
func TestUnregisteredComponentHasNoEndpoint(t *testing.T) {
	request := redrawRequest("Secret@abc", "x", url.Values{"page": {"1"}})
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

// A template edit changes the kind hash, so a page loaded before a deploy asks
// for a kind that no longer exists and must reload rather than render under
// changed semantics.
func TestStaleKindIsNotFound(t *testing.T) {
	request := redrawRequest("UserCard@olderhash", "card-1", url.Values{"page": {"1"}})
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, request)
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
			request := redrawRequest(cardKind, "card-1", values)
			recorder := httptest.NewRecorder()
			redrawServer(t).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}
		})
	}
}

// A GET carries every argument in the URL, so the length has a bound.
func TestOversizedRedrawQueryIsRejected(t *testing.T) {
	request := redrawRequest(cardKind, "card-1", url.Values{
		"page": {"1"},
		"pad":  {strings.Repeat("x", htmlupdate.DefaultMaxQueryBytes)},
	})
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestURITooLong {
		t.Fatalf("status = %d", recorder.Code)
	}
}

// An unchanged redraw costs a 304 rather than its whole markup, which is what
// the endpoint was designed for and what a fixed no-store had made impossible.
func TestUnchangedRedrawAnswers304(t *testing.T) {
	request := redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}})
	server := redrawServer(t)

	first := httptest.NewRecorder()
	server.ServeHTTP(first, request)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag, so nothing can be revalidated")
	}
	// The response depends on the build that rendered the page, and on which
	// component it is, since the URL no longer says.
	vary := strings.Join(first.Header().Values("Vary"), ",")
	for _, axis := range []string{"Build", "Kind", "Instance"} {
		if !strings.Contains(vary, axis) {
			t.Fatalf("Vary = %q, want the %s header", vary, axis)
		}
	}

	conditional := request
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
	changed := redrawRequest(cardKind, "card-1", url.Values{"page": {"3"}})
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
		recorder := httptest.NewRecorder()
		redrawInto(recorder, opts, redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}}), cardRegistry(t))
		return recorder.Header().Get("ETag")
	}
	if tag("one key") == tag("another key") {
		t.Fatal("the same render digests identically under two keys, so a guess confirms across deployments")
	}
}

// A deployment whose redraws are public, or whose proxy needs different terms,
// supplies its own policy — which it now does by writing it, since this package
// writes none.
func TestRedrawCachePolicyIsTheCallers(t *testing.T) {
	answer, ok := options.Redraw(redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}}), cardRegistry(t))
	if !ok {
		t.Fatal("Redraw did not answer the request")
	}
	if got := answer.Header.Get("Cache-Control"); got != "" {
		t.Fatalf("this package chose a cache policy: %q", got)
	}
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = answer.WriteTo(recorder)
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
	options.Mount(router)
	// The runtime asset, and nothing else: a redraw is answered from the
	// caller's own handler at the caller's own URL, so this package mounts none.
	if len(router.patterns) != 1 {
		t.Fatalf("registered %v, want the runtime asset alone", router.patterns)
	}
	recorder := httptest.NewRecorder()
	router.mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, options.RuntimePath(), nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
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
	redrawInto(recorder, options, redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}}), registry)
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
	recorder := httptest.NewRecorder()
	redrawInto(recorder, options, redrawRequest(styledKind, "card-1", nil), styledRegistry(t))
	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	// The head travels in the body now. It was packed as base64 of JSON in a
	// header, and bounded at registration so a proxy could not drop it; a field
	// in a body needs neither, and the redraw stops being the one response in
	// this package with a shape of its own.
	body := redrawBody(t, response)
	if len(body.Head) != 1 || !strings.Contains(body.Head[0], "cards.style") {
		t.Fatalf("a component contributing head must say so, got %+v", body.Head)
	}
	if response.Header.Get("X-Tinybind-Head") != "" {
		t.Fatal("the head header is gone; the body carries it")
	}
}

// A component contributing nothing leaves the field out, so a project using no
// component styles sees no head in its redraw responses at all.
func TestRedrawWithoutHeadIsUnchanged(t *testing.T) {
	recorder := httptest.NewRecorder()
	redrawInto(recorder, options, redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}}), cardRegistry(t))
	if strings.Contains(recorder.Body.String(), `"head"`) {
		t.Fatalf("body carries an empty head field: %s", recorder.Body.String())
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

// The head used to be bounded at registration, because it travelled in a header
// and a proxy could drop an oversized one in production with nothing to look at.
// It travels in the body now, so there is no bound and nothing to refuse.
// Registry.RequiredHead is unaffected and is still what a deployment puts in its
// document shell, which is the only way nothing is fetched mid-swap.

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

// A redraw addressed at a page's own URL is the published shape. The component
// travels in headers, so the caller keeps the address and the redraw inherits
// whatever protects that page.
func callerRedraw(t *testing.T, target, instance string, values url.Values, registry *htmlupdate.Registry) *http.Response {
	t.Helper()
	path := target
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	request := buildRequest(path)
	request.Header.Set("X-Tinybind-Render", "redraw")
	request.Header.Set("X-Tinybind-Kind", cardKind)
	request.Header.Set("X-Tinybind-Instance", instance)
	recorder := httptest.NewRecorder()
	// The shape a caller writes: branch inside its own handler, after its own
	// authorization, and fall through to the page when this is not a redraw.
	if !redrawInto(recorder, options, request, registry) {
		_, _ = recorder.WriteString("<!doctype html><title>the page</title>")
	}
	return recorder.Result()
}

func TestRedrawAnswersAtThePageURL(t *testing.T) {
	response := callerRedraw(t, "/dashboard", "card-1", url.Values{"page": {"2"}}, cardRegistry(t))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	markup := redrawHTML(t, response, "card-1")
	if !strings.Contains(markup, `id="card-1"`) {
		t.Fatalf("want the instance's own subtree, got %q", markup)
	}
	// The region's own fragment and nothing else: no page markup leaked in from
	// the handler this redraw was answered inside.
	if strings.Contains(markup, "<!doctype") {
		t.Fatalf("the page was rendered alongside the redraw: %q", markup)
	}
}

// Two components redrawing at one URL must never be answered from each other's
// cached response. The URL no longer says which component the bytes belong to,
// so the cache keys have to.
func TestRedrawAtAPageURLVariesOnTheComponent(t *testing.T) {
	response := callerRedraw(t, "/dashboard", "card-1", url.Values{"page": {"2"}}, cardRegistry(t))
	vary := response.Header.Values("Vary")
	for _, required := range []string{"X-Tinybind-Kind", "X-Tinybind-Instance", "X-Tinybind-Render", "X-Tinybind-Build"} {
		found := false
		for _, value := range vary {
			if strings.Contains(value, required) {
				found = true
			}
		}
		if !found {
			t.Fatalf("Vary %v does not carry %s", vary, required)
		}
	}
}

// An ordinary request for the page is not a redraw, so the caller renders it —
// and declares the redraw's Vary axes anyway, because the two responses share a
// URL and a cache that learned only one of them would answer the other from it.
// This package computes those axes; declaring them on a response it did not
// produce is the caller's, and this is the shape that does it.
func TestAPageRequestFallsThroughWithItsCacheKeysDeclared(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	recorder := httptest.NewRecorder()
	// A page handler serving redraws at this URL declares their axes before it
	// branches, since the branch it takes is what a cache must key on.
	htmlupdate.ApplyTo(options.RedrawHeaders(request), recorder)
	if redrawInto(recorder, options, request, cardRegistry(t)) {
		t.Fatal("a request with no redraw header must not be answered as one")
	}
	if vary := recorder.Header().Values("Vary"); len(vary) == 0 {
		t.Fatal("the page response must still declare the redraw cache keys")
	}
}

// A page from another build asking at its own URL gets that page, not a refusal.
// It is one round trip rather than a 409 and then a reload, and the page is what
// the caller was about to render anyway.
func TestAStalePageRedrawingAtItsOwnURLGetsThePage(t *testing.T) {
	request := redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}})
	request.Header.Set("X-Tinybind-Build", "an-older-revision")
	recorder := httptest.NewRecorder()
	if redrawInto(recorder, options, request, cardRegistry(t)) {
		t.Fatal("a stale redraw at a page URL must fall through to the page")
	}
	// And the caller's handler then serves the page, as it would for any
	// ordinary request.
	served := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(served, request)
	if !strings.Contains(served.Body.String(), "<!doctype") {
		t.Fatalf("want the page, got %q", served.Body.String())
	}
}

// Naming no component is a bad request rather than a fall-through: the mode says
// this is a redraw, so the caller is not going to render a page for it.
func TestARedrawNamingNoComponentIsRefused(t *testing.T) {
	request := buildRequest("/dashboard")
	request.Header.Set("X-Tinybind-Render", "redraw")
	recorder := httptest.NewRecorder()
	if !redrawInto(recorder, options, request, cardRegistry(t)) {
		t.Fatal("a redraw naming nothing must be answered rather than fall through")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

// An unregistered kind is a 404 at either address, because the component is not
// published at all.
func TestAnUnknownKindAtThePageURLIs404(t *testing.T) {
	request := buildRequest("/dashboard")
	request.Header.Set("X-Tinybind-Render", "redraw")
	request.Header.Set("X-Tinybind-Kind", "NotRegistered@0000")
	request.Header.Set("X-Tinybind-Instance", "card-1")
	recorder := httptest.NewRecorder()
	if !redrawInto(recorder, options, request, cardRegistry(t)) {
		t.Fatal("an unknown kind must be answered rather than fall through")
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

// The version on a redraw is the caller's, exactly as on every other mode.
func TestARedrawCarriesTheCallersVersion(t *testing.T) {
	for _, header := range []string{"redraw", "redraw;v=1", "redraw;v=99"} {
		request := buildRequest("/dashboard?page=2")
		request.Header.Set("X-Tinybind-Render", header)
		request.Header.Set("X-Tinybind-Kind", cardKind)
		request.Header.Set("X-Tinybind-Instance", "card-1")
		recorder := httptest.NewRecorder()
		if !redrawInto(recorder, options, request, cardRegistry(t)) {
			t.Fatalf("header %q was not read as a redraw", header)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("header %q gave status %d", header, recorder.Code)
		}
	}
}

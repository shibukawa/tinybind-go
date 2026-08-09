package htmlupdate_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// The templates below stand in for generated code. Writing the plans by hand
// keeps this package's tests independent of the template compiler while
// exercising exactly the values it emits.

type documentParams struct{ Children htmlbind.Fragment }

var documentOps = htmlbind.Builder[documentParams]{}

var documentPlan = &htmlbind.Plan[documentParams]{
	Ops: []htmlbind.Op[documentParams]{
		documentOps.Static("<!doctype html><html><head></head><body>"),
		documentOps.Slot(func(p documentParams) htmlbind.Fragment { return p.Children }, nil),
		documentOps.Static("</body></html>"),
	},
}

type layoutParams struct {
	Section  string
	Children htmlbind.Fragment
}

var layoutOps = htmlbind.Builder[layoutParams]{}

var layoutPlan = &htmlbind.Plan[layoutParams]{
	Boundary: &htmlbind.Boundary[layoutParams]{
		ComponentID: "Layout@v1",
		Attr:        "data-tb-id",
		Input:       func(p layoutParams) string { return delta.CanonString(p.Section) },
	},
	Ops: []htmlbind.Op[layoutParams]{
		layoutOps.Static("<main"),
		layoutOps.BoundaryAttr(),
		layoutOps.Static("><h1>"),
		layoutOps.Text(func(p layoutParams) string { return p.Section }),
		layoutOps.Static("</h1>"),
		layoutOps.Slot(func(p layoutParams) htmlbind.Fragment { return p.Children }, nil),
		layoutOps.Static("</main>"),
	},
}

type pageParams struct{ Query string }

var pageOps = htmlbind.Builder[pageParams]{}

var pagePlan = &htmlbind.Plan[pageParams]{
	Boundary: &htmlbind.Boundary[pageParams]{
		ComponentID: "Page@v1",
		Attr:        "data-tb-id",
		Input:       func(p pageParams) string { return delta.CanonString(p.Query) },
	},
	Ops: []htmlbind.Op[pageParams]{
		pageOps.Static("<p"),
		pageOps.BoundaryAttr(),
		pageOps.Static(">results for "),
		pageOps.Text(func(p pageParams) string { return p.Query }),
		pageOps.Static("</p>"),
	},
}

// ServeRuntime is set because these tests exercise the served asset and the
// script tag. It is off by default now: the browser half belongs to the caller,
// so serving one is asked for rather than inherited.
var options = htmlupdate.Options{Key: []byte("test key"), ServeRuntime: true}

// server renders the same route in whichever mode the request asks for, which
// is the whole point of the negotiation.
func server() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrappers := []htmlbind.Wrapper{
			htmlbind.BindWrapper(documentPlan, documentParams{}, func(target *documentParams, children htmlbind.Fragment) {
				target.Children = children
			}),
			htmlbind.BindWrapper(layoutPlan, layoutParams{Section: r.URL.Query().Get("section")}, func(target *layoutParams, children htmlbind.Fragment) {
				target.Children = children
			}),
		}
		leaf := htmlbind.Bind(pagePlan, pageParams{Query: r.URL.Query().Get("q")})
		// The package writes bytes and nothing else, so a caller sets what the
		// response has to say about itself and adds its own cache policy on top.
		htmlupdate.ApplyTo(options.Headers(r, wrappers, leaf), w)
		if options.Negotiate(r).Mode != htmlupdate.ModeDocument {
			w.Header().Set("Cache-Control", "no-store")
		}
		if err := options.Render(w, r, wrappers, leaf); err != nil {
			t := http.StatusInternalServerError
			http.Error(w, http.StatusText(t), t)
		}
	})
}

func get(t *testing.T, url string, headers map[string]string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, url, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	server().ServeHTTP(recorder, request)
	return recorder.Result()
}

func fetchDelta(t *testing.T, url string, known delta.Manifest) (*http.Response, deltaBody) {
	t.Helper()
	headers := map[string]string{
		"X-Tinybind-Render": "navigation;v=" + strconv.Itoa(clientVersion),
		"X-Tinybind-Build":  htmlupdate.BuildID(),
	}
	if len(known.Instances) > 0 {
		headers["X-Tinybind-Manifest"] = htmlupdate.EncodeManifest(known)
	}
	response := get(t, url, headers)
	var body deltaBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	return response, body
}

// deltaManifest is the empty manifest a first request carries.
func deltaManifest() delta.Manifest { return delta.Manifest{} }

// fetchDeltaWithSequences is fetchDelta for a client that says it can walk a
// sequence tree, so the response sends values in place of markup.
func fetchDeltaWithSequences(t *testing.T, url string) (*http.Response, deltaBody) {
	t.Helper()
	response := get(t, url, map[string]string{
		"X-Tinybind-Render":    "navigation;v=" + strconv.Itoa(clientVersion),
		"X-Tinybind-Build":     htmlupdate.BuildID(),
		"X-Tinybind-Sequences": "1",
	})
	var body deltaBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	return response, body
}

// clientVersion is the wire version this test's client claims. The package
// neither defines nor judges one, so a test picks its own exactly as a caller
// does.
const clientVersion = 1

type deltaBody struct {
	Operations []struct {
		Kind       string   `json:"kind"`
		ID         string   `json:"id"`
		HTML       string   `json:"html"`
		Boundaries []string `json:"boundaries"`
		Seq        string   `json:"seq"`
		Values     []string `json:"values"`
	} `json:"ops"`
	Manifest []struct {
		ID       string `json:"id"`
		Frame    string `json:"frame"`
		Children string `json:"children"`
	} `json:"manifest"`
	Head []string `json:"head"`
}

// validators is what a client returns on its next request: everything the last
// response's manifest gave it, including the children digest a list needs so its
// rows can move without their parent being replaced to say so. Returning less
// than a response gave is what a client does not do.
func (b deltaBody) validators() delta.Manifest {
	var known delta.Manifest
	for _, instance := range b.Manifest {
		known.Instances = append(known.Instances, delta.Instance{
			ID: instance.ID, FrameValidator: instance.Frame, ChildrenValidator: instance.Children,
		})
	}
	return known
}

// A request without the header must be indistinguishable from an ordinary page,
// which is what keeps crawlers and clients without the runtime working.
func TestPlainRequestGetsTheDocument(t *testing.T) {
	response := get(t, "/search?q=go&section=Docs", nil)
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type = %q", got)
	}
	body := read(t, response)
	for _, want := range []string{"<!doctype html>", `<main data-tb-id="c1"`, `<p data-tb-id="c2"`, "results for go"} {
		if !strings.Contains(body, want) {
			t.Fatalf("document %q is missing %q", body, want)
		}
	}
}

// Vary is what stops a shared cache from handing a delta body to a browser
// asking for a page.
func TestEveryResponseVariesOnTheRenderHeader(t *testing.T) {
	for _, response := range []*http.Response{
		get(t, "/search?q=go", nil),
		get(t, "/search?q=go", map[string]string{"X-Tinybind-Render": "navigation;v=" + strconv.Itoa(clientVersion), "X-Tinybind-Build": htmlupdate.BuildID()}),
	} {
		if got := response.Header.Values("Vary"); len(got) != 2 ||
			got[0] != "X-Tinybind-Render" || got[1] != "X-Tinybind-Build" {
			t.Fatalf("Vary = %q", got)
		}
	}
}

// The first update has no validators, so it legitimately returns everything.
func TestFirstDeltaReturnsEveryBoundary(t *testing.T) {
	response, body := fetchDelta(t, "/search?q=go&section=Docs", delta.Manifest{})
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q", got)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	// Every boundary is its own fragment. The layout carries a hole where the
	// page sits rather than the page's bytes, so a later render that changes only
	// the page replaces the page alone — and one that changes only the layout
	// leaves the page's DOM, and the state inside it, untouched.
	if len(body.Operations) != 2 || body.Operations[0].ID != "c1" || body.Operations[1].ID != "c2" {
		t.Fatalf("want a fragment per boundary, outermost first, got %+v", body.Operations)
	}
	layout := body.Operations[0]
	if strings.Contains(layout.HTML, "results for go") {
		t.Fatalf("the layout fragment carries the page's bytes: %q", layout.HTML)
	}
	if !strings.Contains(layout.HTML, `data-tb-id="c2"`) {
		t.Fatalf("the layout fragment has no hole for the page: %q", layout.HTML)
	}
	// The list is what tells a hole to fill from a hole to retain; nothing in the
	// markup does. Here it is filled, because the client holds no page yet.
	if strings.Join(layout.Boundaries, ",") != "c2" {
		t.Fatalf("boundaries = %v", layout.Boundaries)
	}
	if len(body.Manifest) != 2 {
		t.Fatalf("want both instances in the manifest, got %+v", body.Manifest)
	}
}

// The milestone's target case: only the search parameter changed, so only the
// page travels and the layout markup is never resent.
func TestSearchParameterChangeSendsOnlyThePage(t *testing.T) {
	_, first := fetchDelta(t, "/search?q=go&section=Docs", delta.Manifest{})
	_, second := fetchDelta(t, "/search?q=rust&section=Docs", first.validators())
	if len(second.Operations) != 1 {
		t.Fatalf("want one operation, got %+v", second.Operations)
	}
	operation := second.Operations[0]
	if operation.ID != "c2" || operation.Kind != "replace" {
		t.Fatalf("want the page replaced, got %+v", operation)
	}
	if !strings.Contains(operation.HTML, "results for rust") {
		t.Fatalf("page payload is stale: %q", operation.HTML)
	}
	if strings.Contains(operation.HTML, "<main") {
		t.Fatalf("layout markup must not be resent: %q", operation.HTML)
	}
}

// An unchanged render sends no markup at all, which is the payoff.
func TestUnchangedRenderSendsNoOperations(t *testing.T) {
	_, first := fetchDelta(t, "/search?q=go&section=Docs", delta.Manifest{})
	_, second := fetchDelta(t, "/search?q=go&section=Docs", first.validators())
	if len(second.Operations) != 0 {
		t.Fatalf("want no operations, got %+v", second.Operations)
	}
	if len(second.Manifest) != 2 {
		t.Fatalf("an empty delta must still carry the state: %+v", second.Manifest)
	}
}

// A layout change replaces the layout, and the page inside it is not sent
// separately.
func TestLayoutChangeReplacesTheAncestorOnly(t *testing.T) {
	_, first := fetchDelta(t, "/search?q=go&section=Docs", delta.Manifest{})
	_, second := fetchDelta(t, "/search?q=go&section=Guides", first.validators())
	if len(second.Operations) != 1 || second.Operations[0].ID != "c1" {
		t.Fatalf("want only the layout replaced, got %+v", second.Operations)
	}
}

// Every unusable request must still produce a working page rather than an
// error, because that is what lets later milestones stay incomplete safely.
func TestUnusableRequestsFallBackToTheDocument(t *testing.T) {
	tests := []struct{ name, header string }{
		{"unknown mode", "boundary;v=" + strconv.Itoa(clientVersion)},
		// A buffered entry cannot hold a delivery stream open, so a live request
		// reaching one takes the same fallback every unrecognized condition takes.
		{"live at a buffered entry", "live;v=" + strconv.Itoa(clientVersion)},
		{"unknown mode with no version at all", "boundary"},
		{"empty", ""},
		{"separator with no mode", ";v=1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The build matches, so anything that still falls back does so
			// because of the mode rather than because of the build.
			response := get(t, "/search?q=go", map[string]string{
				"X-Tinybind-Render": test.header,
				"X-Tinybind-Build":  htmlupdate.BuildID(),
			})
			if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Fatalf("content type = %q, want a document", got)
			}
			if !strings.Contains(read(t, response), "<!doctype html>") {
				t.Fatal("want a complete document")
			}
		})
	}
}

// The wire version belongs to the caller, so this package carries it and echoes
// it without ever deciding what it means. A client claiming a version this
// package has never heard of is served its delta, because the only thing that
// could make the two incompatible is the build, and the build matches.
func TestTheVersionIsCarriedRatherThanJudged(t *testing.T) {
	for _, header := range []string{
		"navigation;v=" + strconv.Itoa(clientVersion+1),
		"navigation;v=9999",
		// A version this package cannot parse is read as none rather than as a
		// reason to refuse: refusing would cost the page its update over a field
		// this package does not interpret.
		"navigation;v=banana",
		"navigation",
	} {
		t.Run(header, func(t *testing.T) {
			response := get(t, "/search?q=go", map[string]string{
				"X-Tinybind-Render": header,
				"X-Tinybind-Build":  htmlupdate.BuildID(),
			})
			if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Fatalf("content type = %q, want a delta", got)
			}
			// The echo carries the client's own number back, or no number when
			// the client sent none.
			want := "navigation"
			if _, rest, found := strings.Cut(header, ";v="); found {
				if _, err := strconv.Atoi(rest); err == nil {
					want = header
				}
			}
			if got := response.Header.Get("X-Tinybind-Render"); got != want {
				t.Fatalf("echoed %q, want %q", got, want)
			}
		})
	}
}

// A version of zero is indistinguishable from no version, so it echoes bare.
// Stated as its own case because the alternative — inventing a number to echo —
// is what this package stopped doing.
func TestAZeroVersionEchoesBare(t *testing.T) {
	response := get(t, "/search?q=go", map[string]string{
		"X-Tinybind-Render": "navigation;v=0",
		"X-Tinybind-Build":  htmlupdate.BuildID(),
	})
	if got := response.Header.Get("X-Tinybind-Render"); got != "navigation" {
		t.Fatalf("echoed %q, want a bare mode name", got)
	}
}

// An oversized manifest is dropped rather than rejected, costing response bytes
// instead of failing the request.
func TestOversizedManifestIsIgnored(t *testing.T) {
	response := get(t, "/search?q=go", map[string]string{
		"X-Tinybind-Render":   "navigation;v=" + strconv.Itoa(clientVersion),
		"X-Tinybind-Build":    htmlupdate.BuildID(),
		"X-Tinybind-Manifest": "c1:" + strings.Repeat("x", htmlupdate.DefaultMaxManifestBytes),
	})
	var body deltaBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Operations) != 2 {
		t.Fatalf("dropped hints must yield a full delta, got %+v", body.Operations)
	}
}

// A stale validator is a hint, not authority: the server recomputes and the
// client converges.
func TestStaleValidatorsYieldAFullDelta(t *testing.T) {
	_, body := fetchDelta(t, "/search?q=go&section=Docs", delta.Manifest{Instances: []delta.Instance{
		{ID: "c1", FrameValidator: "stale"}, {ID: "c2", FrameValidator: "stale"},
	}})
	if len(body.Operations) != 2 {
		t.Fatalf("every stale boundary must be re-sent, got %+v", body.Operations)
	}
}

func TestManifestHeaderRoundTrips(t *testing.T) {
	manifest := delta.Manifest{Instances: []delta.Instance{
		{ID: "c1", FrameValidator: "aaa"},
		{ID: "c2", FrameValidator: "bbb"},
	}}
	decoded := htmlupdate.DecodeManifest(htmlupdate.EncodeManifest(manifest))
	if len(decoded.Instances) != 2 || decoded.Instances[1].ID != "c2" || decoded.Instances[1].FrameValidator != "bbb" {
		t.Fatalf("round trip lost data: %+v", decoded.Instances)
	}
	if got := htmlupdate.DecodeManifest("garbage,,c1:,:x").Instances; len(got) != 0 {
		t.Fatalf("malformed pairs must be skipped, got %+v", got)
	}
}

// A different key must not validate another deployment's digests, which is what
// makes a key rotation force complete renders.
func TestValidatorsAreKeyed(t *testing.T) {
	render := func(key string) delta.Manifest {
		wrappers := []htmlbind.Wrapper{htmlbind.BindWrapper(layoutPlan, layoutParams{Section: "Docs"},
			func(target *layoutParams, children htmlbind.Fragment) { target.Children = children })}
		diff, err := delta.RenderDelta([]byte(key), delta.Manifest{}, wrappers, htmlbind.Bind(pagePlan, pageParams{Query: "go"}))
		if err != nil {
			t.Fatal(err)
		}
		return diff.Manifest
	}
	first, second := render("key one"), render("key two")
	if first.Instances[0].FrameValidator == second.Instances[0].FrameValidator {
		t.Fatal("validators must depend on the key")
	}
}

func TestRuntimeIsServedImmutably(t *testing.T) {
	path := options.RuntimePath()
	if !strings.Contains(path, htmlupdate.RuntimeVersion()) {
		t.Fatalf("runtime path %q lacks its content version", path)
	}
	if !strings.HasPrefix(path, htmlupdate.DefaultPathPrefix+"/") {
		t.Fatalf("runtime path %q is outside the endpoint namespace", path)
	}
	recorder := httptest.NewRecorder()
	options.RuntimeHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if got := response.Header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control = %q", got)
	}
	body := read(t, response)
	if !strings.Contains(body, "createPartialUpdateRuntime") {
		t.Fatalf("runtime body looks wrong: %q", body)
	}
	// The reference client carries no protocol version, because the wire version
	// is the caller's. A version compiled in here would be this module versioning
	// a contract only half of which lives in a file the caller replaces.
	if strings.Contains(body, "VERSION") {
		t.Fatal("the reference client must carry no protocol version")
	}
	if tag := options.ScriptTag(); !strings.Contains(tag, path) {
		t.Fatalf("script tag %q does not load %q", tag, path)
	}
}

func read(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// Every framework endpoint shares one configurable namespace, so a deployment
// can route, cache, or protect the whole surface with a single rule.
func TestEndpointNamespaceIsConfigurable(t *testing.T) {
	custom := htmlupdate.Options{Key: options.Key, PathPrefix: "internal/tb", ServeRuntime: true}
	path := custom.RuntimePath()
	if !strings.HasPrefix(path, "/internal/tb/") {
		t.Fatalf("runtime path = %q, want the configured namespace", path)
	}
	// The namespace reaches the browser in the asset URL and nowhere else. It
	// used to travel in the configuration too, because the client built the
	// redraw URL from it; a redraw is addressed by header now, so the client has
	// nothing left to build and the field was dropped rather than kept as decor.
	tag := custom.ScriptTag()
	if !strings.Contains(tag, path) {
		t.Fatalf("script tag %q does not load %q", tag, path)
	}
	if strings.Contains(tag, "prefix") {
		t.Fatalf("script tag %q still carries a path prefix the client cannot use", tag)
	}
	mux := http.NewServeMux()
	custom.Mount(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, buildRequest(path))
	if recorder.Code != http.StatusOK {
		t.Fatalf("mounted runtime status = %d", recorder.Code)
	}
	for _, form := range []string{"internal/tb", "/internal/tb", "/internal/tb/"} {
		got := htmlupdate.Options{PathPrefix: form}.RuntimePath()
		if got != path {
			t.Fatalf("prefix %q produced %q, want %q", form, got, path)
		}
	}
}

// A boundary the browser holds that the new render does not produce must be
// taken off the screen. Nothing in the delta says where it was, so the
// outermost boundary is replaced, which removes it along with everything else
// that moved.
//
// Without this a shorter chain leaves the old innermost region on screen
// whenever the boundary above it happened to render identical markup.
func TestDisappearingBoundaryIsNotLeftOnScreen(t *testing.T) {
	deep := []htmlbind.Wrapper{
		htmlbind.BindWrapper(layoutPlan, layoutParams{Section: "Docs"},
			func(target *layoutParams, children htmlbind.Fragment) { target.Children = children }),
		htmlbind.BindWrapper(layoutPlan, layoutParams{Section: "Docs"},
			func(target *layoutParams, children htmlbind.Fragment) { target.Children = children }),
	}
	shallow := deep[:1]
	leaf := htmlbind.Bind(pagePlan, pageParams{Query: "go"})

	first, err := delta.RenderDelta(options.Key, delta.Manifest{}, deep, leaf)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Manifest.Instances) != 3 {
		t.Fatalf("want three boundaries, got %+v", first.Manifest.Instances)
	}
	second, err := delta.RenderDelta(options.Key, first.Manifest, shallow, leaf)
	if err != nil {
		t.Fatal(err)
	}
	if _, still := second.Manifest.Find("c2"); still {
		t.Fatal("the fixture no longer exercises a disappearing boundary")
	}
	if len(second.Operations) == 0 {
		t.Fatal("a disappearing boundary must produce an operation")
	}
	if second.Operations[0].InstanceID != "c0" {
		t.Fatalf("want the outermost boundary replaced, got %+v", second.Operations)
	}
}

// A page rendered by another build holds state this binary cannot vouch for,
// and none of that is visible in a validator, so the build is compared rather
// than guessed at.
func TestAnotherBuildGetsTheDocument(t *testing.T) {
	for name, header := range map[string]string{
		"another build": "older-revision",
		"absent":        "",
	} {
		t.Run(name, func(t *testing.T) {
			headers := map[string]string{"X-Tinybind-Render": "navigation;v=" + strconv.Itoa(clientVersion)}
			if header != "" {
				headers["X-Tinybind-Build"] = header
			}
			response := get(t, "/search?q=go", headers)
			if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Fatalf("content type = %q, want a document", got)
			}
		})
	}
}

// A clean build keeps its identity across restarts, so a rolling restart of one
// release does not throw away every client's state.
func TestBuildIDIsStableWithinAProcess(t *testing.T) {
	if htmlupdate.BuildID() == "" {
		t.Fatal("a build identity is always available")
	}
	if htmlupdate.BuildID() != htmlupdate.BuildID() {
		t.Fatal("the identity must not change while the process runs")
	}
}

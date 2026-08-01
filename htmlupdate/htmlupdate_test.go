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
		Input:       func(p layoutParams) string { return htmlbind.CanonString(p.Section) },
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
		Input:       func(p pageParams) string { return htmlbind.CanonString(p.Query) },
	},
	Ops: []htmlbind.Op[pageParams]{
		pageOps.Static("<p"),
		pageOps.BoundaryAttr(),
		pageOps.Static(">results for "),
		pageOps.Text(func(p pageParams) string { return p.Query }),
		pageOps.Static("</p>"),
	},
}

var options = htmlupdate.Options{Key: []byte("test key")}

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

func delta(t *testing.T, url string, known map[string]string) (*http.Response, deltaBody) {
	t.Helper()
	headers := map[string]string{
		"X-Tinybind-Render": "navigation;v=" + strconv.Itoa(htmlupdate.Version),
		"X-Tinybind-Build":  htmlupdate.BuildID(),
	}
	if len(known) > 0 {
		var manifest htmlbind.Manifest
		for id, frame := range known {
			manifest.Instances = append(manifest.Instances, htmlbind.Instance{ID: id, FrameValidator: frame})
		}
		headers["X-Tinybind-Manifest"] = htmlupdate.EncodeManifest(manifest)
	}
	response := get(t, url, headers)
	var body deltaBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	return response, body
}

type deltaBody struct {
	Version    int `json:"v"`
	Operations []struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
		HTML string `json:"html"`
	} `json:"ops"`
	Manifest []struct {
		ID    string `json:"id"`
		Frame string `json:"frame"`
	} `json:"manifest"`
}

func (b deltaBody) validators() map[string]string {
	out := map[string]string{}
	for _, instance := range b.Manifest {
		out[instance.ID] = instance.Frame
	}
	return out
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
		get(t, "/search?q=go", map[string]string{"X-Tinybind-Render": "navigation;v=1", "X-Tinybind-Build": htmlupdate.BuildID()}),
	} {
		if got := response.Header.Values("Vary"); len(got) != 2 ||
			got[0] != "X-Tinybind-Render" || got[1] != "X-Tinybind-Build" {
			t.Fatalf("Vary = %q", got)
		}
	}
}

// The first update has no validators, so it legitimately returns everything.
func TestFirstDeltaReturnsEveryBoundary(t *testing.T) {
	response, body := delta(t, "/search?q=go&section=Docs", nil)
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q", got)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if len(body.Operations) != 1 || body.Operations[0].ID != "c1" {
		t.Fatalf("want the outermost boundary replaced once, got %+v", body.Operations)
	}
	// The layout replacement contains the page, so sending the page too would
	// target a node that no longer exists.
	if !strings.Contains(body.Operations[0].HTML, `<p data-tb-id="c2"`) {
		t.Fatalf("ancestor replacement must contain its descendants: %q", body.Operations[0].HTML)
	}
	if len(body.Manifest) != 2 {
		t.Fatalf("want both instances in the manifest, got %+v", body.Manifest)
	}
}

// The milestone's target case: only the search parameter changed, so only the
// page travels and the layout markup is never resent.
func TestSearchParameterChangeSendsOnlyThePage(t *testing.T) {
	_, first := delta(t, "/search?q=go&section=Docs", nil)
	_, second := delta(t, "/search?q=rust&section=Docs", first.validators())
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
	_, first := delta(t, "/search?q=go&section=Docs", nil)
	_, second := delta(t, "/search?q=go&section=Docs", first.validators())
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
	_, first := delta(t, "/search?q=go&section=Docs", nil)
	_, second := delta(t, "/search?q=go&section=Guides", first.validators())
	if len(second.Operations) != 1 || second.Operations[0].ID != "c1" {
		t.Fatalf("want only the layout replaced, got %+v", second.Operations)
	}
}

// Every unusable request must still produce a working page rather than an
// error, because that is what lets later milestones stay incomplete safely.
func TestUnusableRequestsFallBackToTheDocument(t *testing.T) {
	tests := []struct{ name, header string }{
		{"future version", "navigation;v=" + strconv.Itoa(htmlupdate.Version+1)},
		{"past version", "navigation;v=0"},
		{"unknown mode", "boundary;v=1"},
		{"malformed", "navigation"},
		{"empty", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := get(t, "/search?q=go", map[string]string{"X-Tinybind-Render": test.header})
			if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Fatalf("content type = %q, want a document", got)
			}
			if !strings.Contains(read(t, response), "<!doctype html>") {
				t.Fatal("want a complete document")
			}
		})
	}
}

// An oversized manifest is dropped rather than rejected, costing response bytes
// instead of failing the request.
func TestOversizedManifestIsIgnored(t *testing.T) {
	response := get(t, "/search?q=go", map[string]string{
		"X-Tinybind-Render":   "navigation;v=1",
		"X-Tinybind-Build":    htmlupdate.BuildID(),
		"X-Tinybind-Manifest": "c1:" + strings.Repeat("x", htmlupdate.DefaultMaxManifestBytes),
	})
	var body deltaBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Operations) != 1 {
		t.Fatalf("dropped hints must yield a full delta, got %+v", body.Operations)
	}
}

// A stale validator is a hint, not authority: the server recomputes and the
// client converges.
func TestStaleValidatorsYieldAFullDelta(t *testing.T) {
	_, body := delta(t, "/search?q=go&section=Docs", map[string]string{"c1": "stale", "c2": "stale"})
	if len(body.Operations) != 1 || body.Operations[0].ID != "c1" {
		t.Fatalf("want the outermost boundary replaced, got %+v", body.Operations)
	}
}

func TestManifestHeaderRoundTrips(t *testing.T) {
	manifest := htmlbind.Manifest{Instances: []htmlbind.Instance{
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
	render := func(key string) htmlbind.Manifest {
		wrappers := []htmlbind.Wrapper{htmlbind.BindWrapper(layoutPlan, layoutParams{Section: "Docs"},
			func(target *layoutParams, children htmlbind.Fragment) { target.Children = children })}
		delta, err := htmlbind.RenderDelta([]byte(key), htmlbind.Manifest{}, wrappers, htmlbind.Bind(pagePlan, pageParams{Query: "go"}))
		if err != nil {
			t.Fatal(err)
		}
		return delta.Manifest
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
	if !strings.Contains(body, "window.tinybind") {
		t.Fatalf("runtime body looks wrong: %q", body)
	}
	// The runtime hardcodes the protocol version, so a bump that forgets the
	// script would silently disable every update.
	if !strings.Contains(body, "VERSION = "+strconv.Itoa(htmlupdate.Version)) {
		t.Fatal("runtime protocol version disagrees with the server")
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
	custom := htmlupdate.Options{Key: options.Key, PathPrefix: "internal/tb"}
	path := custom.RuntimePath()
	if !strings.HasPrefix(path, "/internal/tb/") {
		t.Fatalf("runtime path = %q, want the configured namespace", path)
	}
	// The runtime learns the prefix from its own script tag, so one shared
	// asset serves any namespace without being rebuilt.
	tag := custom.ScriptTag()
	if !strings.Contains(tag, `data-tinybind-prefix="/internal/tb"`) {
		t.Fatalf("script tag %q does not carry the namespace", tag)
	}
	mux := http.NewServeMux()
	custom.Mount(mux, nil)
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

	first, err := htmlbind.RenderDelta(options.Key, htmlbind.Manifest{}, deep, leaf)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Manifest.Instances) != 3 {
		t.Fatalf("want three boundaries, got %+v", first.Manifest.Instances)
	}
	second, err := htmlbind.RenderDelta(options.Key, first.Manifest, shallow, leaf)
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
			headers := map[string]string{"X-Tinybind-Render": "navigation;v=1"}
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

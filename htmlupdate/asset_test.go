package htmlupdate_test

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// A framework that already ships a runtime cannot put a second one on the same
// document: that would be two boundary id spaces, two build identities, and two
// script tags with nothing deciding which owns a region. So it merges ours into
// its own asset, and merging needs the bytes.
//
// Without them the only way to merge is to keep a copy, and a copy is not a
// version-pinned dependency: it drifts on upgrade with nothing in the build
// failing, and a drifted browser runtime is a dead page rather than a compile
// error.
func TestRuntimeSourceIsReadable(t *testing.T) {
	source := htmlupdate.RuntimeSource()
	if len(source) == 0 {
		t.Fatal("runtime source is empty")
	}
	if !strings.Contains(string(source), "createPartialUpdateRuntime") {
		t.Fatal("runtime source does not expose the factory a merging caller constructs from")
	}
	// The bytes carry no naming choice, which is what lets one asset serve
	// every deployment and a merge need no build step.
	for _, name := range []string{
		`"data-tb-id"`,
		`"data-tinybind-preserve"`,
		`"data-tinybind-ignore"`,
		`"X-Tinybind-Render"`,
		`"X-Tinybind-Manifest"`,
		`"X-Tinybind-Build"`,
		"window.tinybind",
	} {
		if strings.Contains(string(source), name) {
			t.Fatalf("runtime still hardcodes %s, so a caller cannot own that name", name)
		}
	}
	// Callers get a copy; mutating it must not reach the served asset.
	source[0] = 'X'
	if htmlupdate.RuntimeSource()[0] == 'X' {
		t.Fatal("RuntimeSource hands out the embedded bytes rather than a copy")
	}
}

// The asset form is what a caller serving its own files needs: the bytes, the
// identity that makes an immutable URL honest, and the media type.
func TestRuntimeAssetCarriesItsIdentity(t *testing.T) {
	asset := options.RuntimeAsset()
	if asset.Version != htmlupdate.RuntimeVersion() {
		t.Fatalf("asset version %q disagrees with RuntimeVersion", asset.Version)
	}
	if !strings.HasPrefix(asset.ContentType, "text/javascript") {
		t.Fatalf("content type = %q", asset.ContentType)
	}
	if !strings.Contains(asset.FileName, asset.Version) {
		t.Fatalf("file name %q does not carry the digest, so an immutable cache would go stale", asset.FileName)
	}
	named := htmlupdate.Options{RuntimeFileName: "popcorn", ServeRuntime: true}
	if !strings.HasPrefix(named.RuntimeAsset().FileName, "popcorn.") {
		t.Fatalf("file name = %q, want the caller's name", named.RuntimeAsset().FileName)
	}
	if !strings.Contains(named.RuntimePath(), "/popcorn.") {
		t.Fatalf("runtime path = %q, want the caller's name", named.RuntimePath())
	}
}

// A caller owning the runtime gets no asset route and no script tag. A tag
// pointing at an asset this build does not serve is worse than no tag.
func TestCallerOwnedRuntimeIsNotServed(t *testing.T) {
	owned := htmlupdate.Options{Key: options.Key, CallerOwnsRuntime: true}
	if tag := owned.ScriptTag(); tag != "" {
		t.Fatalf("script tag = %q, want none", tag)
	}
	mux := http.NewServeMux()
	owned.Mount(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, owned.RuntimePath(), nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want the asset route to be absent", recorder.Code)
	}
	// Redraw is unaffected: owning the runtime is not owning the endpoints, and
	// a redraw is answered from the caller's handler rather than a mounted route.
	redraw := httptest.NewRecorder()
	if !redrawInto(redraw, owned, redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}}), cardRegistry(t)) {
		t.Fatal("redraw stopped working when the runtime asset was disowned")
	}
}

var configAttr = regexp.MustCompile(`data-config="([^"]*)"`)

// The server and the browser cannot disagree about a name, because the server
// writes every one of them onto the tag the browser reads.
func TestScriptTagCarriesEveryName(t *testing.T) {
	custom := htmlupdate.Options{
		Key:                 options.Key,
		PathPrefix:          "internal/pw",
		HeaderPrefix:        "X-Popcorn",
		DataAttributePrefix: "pw",
		GlobalName:          "popcorn",
		ServeRuntime:        true,
	}
	match := configAttr.FindStringSubmatch(custom.ScriptTag())
	if match == nil {
		t.Fatalf("script tag %q carries no configuration", custom.ScriptTag())
	}
	var config htmlupdate.RuntimeConfig
	if err := json.Unmarshal([]byte(html.UnescapeString(match[1])), &config); err != nil {
		t.Fatal(err)
	}
	want := custom.RuntimeConfig()
	if config != want {
		t.Fatalf("tag config = %+v, want %+v", config, want)
	}
	if config.Header != "X-Popcorn" || config.Attr != "pw" || config.Global != "popcorn" {
		t.Fatalf("configuration did not follow the options: %+v", config)
	}
	if config.Build == "" {
		t.Fatal("build identity is missing, so every request would be answered as stale")
	}
}

// A prefix that names an element has to start with a letter, which the
// attribute-only rule never required. Finding that in a browser is expensive,
// so it is reported at startup instead.
func TestValidateReportsUnusableNames(t *testing.T) {
	if err := options.Validate(); err != nil {
		t.Fatalf("default options are invalid: %v", err)
	}
	for _, bad := range []htmlupdate.Options{
		{DataAttributePrefix: "9pw"},
		{DataAttributePrefix: "PW"},
		{DataAttributePrefix: "pw-"},
		{HeaderPrefix: "X Popcorn"},
		{MaxQueryBytes: -1},
	} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("%+v was accepted", bad)
		}
	}
	// Every problem is reported, not only the first, because a caller
	// validating at startup wants the whole list.
	both := htmlupdate.Options{DataAttributePrefix: "PW", MaxManifestBytes: -1}
	if err := both.Validate(); err == nil || !strings.Contains(err.Error(), "MaxManifestBytes") ||
		!strings.Contains(err.Error(), "lowercase") {
		t.Fatalf("want both problems reported, got %v", err)
	}
}

// A framework naming its own prefix gets one naming system in the document,
// including the placeholder element and the identifier allocation that used to
// be literals.
func TestPrefixReachesTheWholeDocument(t *testing.T) {
	custom := htmlupdate.Options{Key: options.Key, DataAttributePrefix: "pw"}
	if got := custom.RuntimeConfig().Attr; got != "pw" {
		t.Fatalf("runtime attr = %q", got)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/page", nil)
	page := htmlbind.Bind(badgePlan, badgeParams{ID: "cart", Count: 2})
	if err := custom.Render(recorder, request, nil, page); err != nil {
		t.Fatal(err)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "tb-boundary") {
		t.Fatalf("the default placeholder name survived the override:\n%s", body)
	}
}

// Who serves the browser runtime has to be answered at startup, because the
// wrong answer is invisible at run time: a build that serves none and owns none
// compiles, starts, renders every page correctly, and then does nothing when a
// boundary should update. Every other unusable option is wrong in a way somebody
// notices.
func TestRuntimeOwnershipMustBeStated(t *testing.T) {
	if err := (htmlupdate.Options{Key: options.Key}).Validate(); err == nil {
		t.Fatal("options naming no runtime owner were accepted, so a dead page would reach production")
	}
	// Both is refused too: a document carrying two runtimes has two boundary id
	// spaces and two build identities, and nothing decides which owns a region.
	both := htmlupdate.Options{Key: options.Key, ServeRuntime: true, CallerOwnsRuntime: true}
	if err := both.Validate(); err == nil {
		t.Fatal("a build claiming both was accepted")
	}
	for _, valid := range []htmlupdate.Options{
		{Key: options.Key, ServeRuntime: true},
		{Key: options.Key, CallerOwnsRuntime: true},
	} {
		if err := valid.Validate(); err != nil {
			t.Fatalf("%+v was refused: %v", valid, err)
		}
	}
}

// The default ships no browser asset: no route, no script tag. A caller that
// wants the reference client asks for it.
func TestTheDefaultServesNoRuntime(t *testing.T) {
	quiet := htmlupdate.Options{Key: options.Key, CallerOwnsRuntime: true}
	if tag := quiet.ScriptTag(); tag != "" {
		t.Fatalf("script tag = %q, want none", tag)
	}
	router := &countingRouter{mux: http.NewServeMux()}
	quiet.Mount(router)
	for _, pattern := range router.patterns {
		if strings.Contains(pattern, "/runtime/") {
			t.Fatalf("the runtime asset was routed anyway: %v", router.patterns)
		}
	}
	// The bytes stay in the module, so a caller opting back in never copies
	// them and a merged asset never drifts from what this build expects.
	if len(htmlupdate.RuntimeSource()) == 0 {
		t.Fatal("RuntimeSource must still return the reference client")
	}
}

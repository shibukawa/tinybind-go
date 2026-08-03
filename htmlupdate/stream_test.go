package htmlupdate_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

func streamServer() http.Handler {
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
		if err := options.RenderStream(w, r, wrappers, leaf); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
		}
	})
}

func streamRecords(t *testing.T, url string, known map[string]string) (*http.Response, []map[string]any) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, url, nil)
	request.Header.Set("X-Tinybind-Render", "navigation;v="+strconv.Itoa(clientVersion))
	request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
	if len(known) > 0 {
		var manifest htmlbind.Manifest
		for id, frame := range known {
			manifest.Instances = append(manifest.Instances, htmlbind.Instance{ID: id, FrameValidator: frame})
		}
		request.Header.Set("X-Tinybind-Manifest", htmlupdate.EncodeManifest(manifest))
	}
	recorder := httptest.NewRecorder()
	streamServer().ServeHTTP(recorder, request)
	response := recorder.Result()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(read(t, response)), "\n") {
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("record %q: %v", line, err)
		}
		records = append(records, item)
	}
	return response, records
}

// A stream opens with the head, so contributions install before any markup that
// depends on them, and closes with a terminator, which is the only way a client
// can tell a finished render from a truncated one.
func TestStreamIsFramedByHeadAndTerminator(t *testing.T) {
	response, records := streamRecords(t, "/search?q=go&section=Docs", nil)
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-ndjson") {
		t.Fatalf("content type = %q", got)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if len(records) < 3 {
		t.Fatalf("want head, operations, and a terminator, got %+v", records)
	}
	if records[0]["r"] != "head" {
		t.Fatalf("first record = %+v", records[0])
	}
	// The head record carries the build, which is the compatibility axis this
	// package still operates, and no version, which is the caller's.
	if records[0]["build"] != htmlupdate.BuildID() {
		t.Fatalf("head must carry the build identity, got %+v", records[0])
	}
	if _, carried := records[0]["v"]; carried {
		t.Fatalf("head must carry no version field, got %+v", records[0])
	}
	if records[len(records)-1]["r"] != "end" {
		t.Fatalf("last record = %+v", records[len(records)-1])
	}
}

// Each record carries its own manifest entry, because a trailing manifest
// cannot be written before the operations it describes.
func TestEveryRecordCarriesItsValidator(t *testing.T) {
	_, records := streamRecords(t, "/search?q=go&section=Docs", nil)
	seen := map[string]bool{}
	for _, item := range records {
		if item["r"] != "op" {
			continue
		}
		if item["frame"] == "" || item["frame"] == nil {
			t.Fatalf("operation without a validator: %+v", item)
		}
		seen[item["id"].(string)] = true
	}
	// Both boundaries must be represented, or the client's manifest would lose
	// the one that did not change.
	for _, id := range []string{"c1", "c2"} {
		if !seen[id] {
			t.Fatalf("instance %s missing from the stream: %+v", id, records)
		}
	}
}

// An unchanged boundary restates its validator without markup, so the client
// can rebuild its whole manifest from what it received.
func TestUnchangedBoundaryStreamsValidatorOnly(t *testing.T) {
	_, first := streamRecords(t, "/search?q=go&section=Docs", nil)
	known := map[string]string{}
	for _, item := range first {
		if item["r"] == "op" {
			known[item["id"].(string)] = item["frame"].(string)
		}
	}
	_, second := streamRecords(t, "/search?q=rust&section=Docs", known)
	for _, item := range second {
		if item["r"] != "op" {
			continue
		}
		html, _ := item["html"].(string)
		if item["id"] == "c1" && html != "" {
			t.Fatalf("the unchanged layout must carry no markup: %+v", item)
		}
		if item["id"] == "c2" && !strings.Contains(html, "results for rust") {
			t.Fatalf("the changed page must carry its markup: %+v", item)
		}
	}
}

// Nothing about the negotiation changes, so a client that cannot stream still
// gets a document.
func TestStreamFallsBackToTheDocument(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/search?q=go", nil)
	recorder := httptest.NewRecorder()
	streamServer().ServeHTTP(recorder, request)
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type = %q", got)
	}
	if !strings.Contains(recorder.Body.String(), "<!doctype html>") {
		t.Fatal("want a complete document")
	}
}

// A producer drives the stream directly, which is the seam an asynchronous
// render sequence plugs into: it writes each boundary as it settles and closes.
func TestProducerDrivesTheStream(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream := options.OpenStream(recorder, []string{"<title>Live</title>"})
	stream.Replace("c1", `<main id="c1">first</main>`, "f1")
	if !stream.Sent("c1") {
		t.Fatal("a written instance must be reported as sent")
	}
	stream.Unchanged("c2", "f2")
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(recorder.Body.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("want head, two operations, and a terminator, got %q", lines)
	}
	var head, last map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &head); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[3]), &last); err != nil {
		t.Fatal(err)
	}
	if head["r"] != "head" || last["r"] != "end" {
		t.Fatalf("framing = %q", lines)
	}
}

// A failure after the response committed cannot change the status, so it is
// reported in band and still terminates the stream.
func TestProducerReportsLateFailureInBand(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream := options.OpenStream(recorder, nil)
	stream.Replace("c1", `<main id="c1">partial</main>`, "f1")
	stream.Fail("boundary failed")
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(recorder.Body.String()), "\n")
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatal(err)
	}
	if last["r"] != "end" || last["error"] != "boundary failed" {
		t.Fatalf("want a terminating error record, got %v", last)
	}
	// Close after Fail must not write a second terminator, or the client would
	// see the stream end twice.
	if got := strings.Count(recorder.Body.String(), `"r":"end"`); got != 1 {
		t.Fatalf("terminator written %d times", got)
	}
}

// asyncPage stands in for a generated component with an await boundary: the
// initial pass writes the fallback, and the settled subtree follows.
type asyncParams struct{ Query string }

type asyncScope struct{ Value string }

var asyncOps = htmlbind.Builder[asyncParams]{}
var asyncScopeOps = htmlbind.Builder[asyncScope]{}

var asyncPlan = &htmlbind.Plan[asyncParams]{
	Boundary: &htmlbind.Boundary[asyncParams]{
		ComponentID: "Async@v1",
		Attr:        "data-tb-id",
		Input:       func(p asyncParams) string { return htmlbind.CanonString(p.Query) },
	},
	HasAwaitBlock: true,
	Ops: []htmlbind.Op[asyncParams]{
		asyncOps.Static("<section"),
		asyncOps.BoundaryAttr(),
		asyncOps.Static(">"),
		htmlbind.Await(
			func(ctx context.Context, p asyncParams) (asyncScope, error) {
				return asyncScope{Value: "settled " + p.Query}, nil
			},
			func(asyncParams, htmlbind.AsyncError) asyncScope { return asyncScope{} },
			[]htmlbind.Op[asyncScope]{asyncScopeOps.Text(func(s asyncScope) string { return s.Value })},
			[]htmlbind.Op[asyncParams]{asyncOps.Static("loading")},
			nil,
		),
		asyncOps.Static("</section>"),
	},
}

// A slow region reaches the browser with its fallback, and its replacement
// follows on the same stream, so one dependency delays only itself.
func TestStreamCarriesAwaitCompletions(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaf := htmlbind.Bind(asyncPlan, asyncParams{Query: r.URL.Query().Get("q")})
		if err := options.RenderStreamAsync(r.Context(), w, r, nil, leaf); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
		}
	})
	request := httptest.NewRequest(http.MethodGet, "/search?q=go", nil)
	request.Header.Set("X-Tinybind-Render", "navigation;v="+strconv.Itoa(clientVersion))
	request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	var kinds []string
	var settled map[string]any
	for _, line := range strings.Split(strings.TrimSpace(recorder.Body.String()), "\n") {
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("record %q: %v", line, err)
		}
		kinds = append(kinds, item["r"].(string))
		if item["r"] == "await" {
			settled = item
		}
		if item["r"] == "op" {
			// The region travels with its fallback, not with the settled value,
			// because that is what the browser can paint immediately.
			if html, _ := item["html"].(string); html != "" && !strings.Contains(html, "loading") {
				t.Fatalf("the initial pass should carry the fallback, got %q", html)
			}
		}
	}
	if len(kinds) < 3 || kinds[0] != "head" || kinds[len(kinds)-1] != "end" {
		t.Fatalf("framing = %q", kinds)
	}
	if settled == nil {
		t.Fatalf("no completion in %q", kinds)
	}
	if html, _ := settled["html"].(string); !strings.Contains(html, "settled go") {
		t.Fatalf("completion = %v", settled)
	}
	if id, _ := settled["id"].(string); id == "" {
		t.Fatal("a completion must name the placeholder it replaces")
	}
}

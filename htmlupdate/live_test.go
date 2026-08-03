package htmlupdate_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// livePage stands in for a generated component holding one live boundary: the
// document render settles the first value in place, and a live request keeps
// pulling.
type liveParams struct{ Values []string }

var liveOps = htmlbind.Builder[liveParams]{}

var livePlan = &htmlbind.Plan[liveParams]{
	Boundary: &htmlbind.Boundary[liveParams]{
		ComponentID: "Feed@v1",
		Attr:        "data-tb-id",
		Input:       func(p liveParams) string { return htmlbind.CanonString(strings.Join(p.Values, ",")) },
	},
	HasAwaitBlock: true,
	HasLiveBlock:  true,
	Ops: []htmlbind.Op[liveParams]{
		liveOps.Static("<section"),
		liveOps.BoundaryAttr(),
		liveOps.Static(">"),
		htmlbind.Live(
			func(ctx context.Context, p liveParams) []htmlbind.LiveBinding[string] {
				return []htmlbind.LiveBinding[string]{
					func(deliver func(func(*string), error) bool) error {
						for _, value := range p.Values {
							if !deliver(func(scope *string) { *scope = value }, nil) {
								return nil
							}
						}
						return nil
					},
				}
			},
			func(liveParams) string { return "" },
			func(_ liveParams, err htmlbind.AsyncError) htmlbind.AsyncError { return err },
			[]htmlbind.Op[string]{htmlbind.Builder[string]{}.Text(func(value string) string { return value })},
			[]htmlbind.Op[liveParams]{liveOps.Static("pending")},
			nil,
		),
		liveOps.Static("</section>"),
	},
}

// staticPlan is the same shape with no live boundary, which is what proves a
// page that has none is unchanged by any of this.
var staticPlan = &htmlbind.Plan[liveParams]{
	Boundary: &htmlbind.Boundary[liveParams]{
		ComponentID: "Static@v1",
		Attr:        "data-tb-id",
		Input:       func(liveParams) string { return "" },
	},
	Ops: []htmlbind.Op[liveParams]{
		liveOps.Static("<section"),
		liveOps.BoundaryAttr(),
		liveOps.Static(">still</section>"),
	},
}

func liveServer(plan *htmlbind.Plan[liveParams], values ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaf := htmlbind.Bind(plan, liveParams{Values: values})
		if err := options.RenderLiveStream(r.Context(), w, r, nil, leaf); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
		}
	})
}

func requestMode(t *testing.T, handler http.Handler, mode string) (*http.Response, []map[string]any) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/feed", nil)
	if mode != "" {
		request.Header.Set("X-Tinybind-Render", mode+";v="+strconv.Itoa(htmlupdate.Version))
		request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(read(t, response)), "\n") {
		if line == "" || !strings.HasPrefix(line, "{") {
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

// A live connection is not a navigation. The two differ in duration and in
// termination, and a caller sharing one name for them cannot route, time out, or
// bound them separately.
func TestLiveModeHasItsOwnToken(t *testing.T) {
	response, _ := requestMode(t, liveServer(livePlan, "one", "two"), "live")
	if got := response.Header.Get("X-Tinybind-Render"); got != "live;v="+strconv.Itoa(htmlupdate.Version) {
		t.Fatalf("served mode = %q, want the live token echoed", got)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-ndjson") {
		t.Fatalf("content type = %q", got)
	}
}

// The same route in the navigation mode describes the page and finishes. A
// navigation that kept subscriptions open would never terminate, because a live
// source has no settle.
func TestNavigationOnALiveRouteTerminates(t *testing.T) {
	response, records := requestMode(t, liveServer(livePlan, "one", "two"), "navigation")
	if got := response.Header.Get("X-Tinybind-Render"); got != "navigation;v="+strconv.Itoa(htmlupdate.Version) {
		t.Fatalf("served mode = %q", got)
	}
	last := records[len(records)-1]
	if last["r"] != "end" {
		t.Fatalf("last record = %+v", last)
	}
	// The route owns a live boundary, so the terminator hands off rather than
	// declaring the screen finished.
	if last["reason"] != "live_pending" {
		t.Fatalf("terminator = %+v, want a handoff to the live mode", last)
	}
}

// A page with no live boundary must cost no speculative request. A live request
// re-executes the whole route, so a client that cannot tell one from the other
// pays a page execution per screen that will never deliver anything.
func TestAPageWithNoLiveBoundaryAnnouncesNothing(t *testing.T) {
	handler := liveServer(staticPlan)
	response, records := requestMode(t, handler, "navigation")
	if got := response.Header.Get("X-Tinybind-Live"); got != "" {
		t.Fatalf("a static page must carry no live marker, got %q", got)
	}
	if last := records[len(records)-1]; last["reason"] != "final" {
		t.Fatalf("terminator = %+v, want final", last)
	}
	document, _ := requestMode(t, handler, "")
	if got := document.Header.Get("X-Tinybind-Live"); got != "" {
		t.Fatalf("a static document must carry no live marker, got %q", got)
	}
}

// The document a browser loads cannot read a body field, so the handoff travels
// as a header there.
func TestDocumentCarriesTheHandoffMarker(t *testing.T) {
	response, _ := requestMode(t, liveServer(livePlan, "one"), "")
	if got := response.Header.Get("X-Tinybind-Live"); got != "1" {
		t.Fatalf("live marker = %q, want the document to announce it", got)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type = %q, want a document", got)
	}
}

// Every delivery carries the whole state of its region, so a live stream is
// self-sufficient and never restates a manifest the document render already
// established. Restating validators it will not replace buys nothing.
func TestLiveStreamCarriesDeliveriesWithoutRestatingTheManifest(t *testing.T) {
	_, records := requestMode(t, liveServer(livePlan, "first", "second"), "live")
	if records[0]["r"] != "head" {
		t.Fatalf("first record = %+v", records[0])
	}
	// The opening record names the binary, so a client reconnecting into a
	// redeployed server reloads instead of applying deliveries addressed at a
	// document it is no longer showing.
	if records[0]["build"] != htmlupdate.BuildID() {
		t.Fatalf("head must carry the build, got %+v", records[0])
	}
	var deliveries []string
	for _, item := range records {
		if item["r"] == "op" && item["html"] == "" {
			t.Fatalf("a live stream must not restate an unchanged boundary: %+v", item)
		}
		if item["r"] == "await" {
			deliveries = append(deliveries, item["html"].(string))
		}
	}
	if len(deliveries) != 2 {
		t.Fatalf("want one record per delivery, got %q", deliveries)
	}
	if !strings.Contains(deliveries[0], "first") || !strings.Contains(deliveries[1], "second") {
		t.Fatalf("deliveries = %q", deliveries)
	}
	if last := records[len(records)-1]; last["reason"] != "done" {
		t.Fatalf("a stream whose sources finished must say so, got %+v", last)
	}
}

// A healthy close at the server's own lifetime bound is not a fault. Without the
// distinction a client backs off on both, which stalls a working screen every
// time the server rotates a connection.
func TestRetryTerminatorNamesAHealthyClose(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream := options.OpenLiveStream(recorder, nil)
	stream.Replace("c1", `<main id="c1">now</main>`, "")
	if err := stream.Retry(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	var last map[string]any
	lines := strings.Split(strings.TrimSpace(recorder.Body.String()), "\n")
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatal(err)
	}
	if last["reason"] != "retry" {
		t.Fatalf("terminator = %+v", last)
	}
	// The server is the only party that knows it is shedding load or rolling a
	// deploy, so it is the only one that can spread the return before anything
	// fails.
	if last["retryMs"] != float64(2000) {
		t.Fatalf("retry hint = %+v", last)
	}
	// Close after Retry must not write a second terminator.
	if got := strings.Count(recorder.Body.String(), `"r":"end"`); got != 1 {
		t.Fatalf("terminator written %d times", got)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(recorder.Body.String(), `"r":"end"`); got != 1 {
		t.Fatalf("Close after Retry wrote another terminator: %q", recorder.Body.String())
	}
}

// An entry that does not serve live answers a live request as a navigation and
// terminates, so a client that opened one learns at once rather than holding a
// connection that will never deliver.
func TestALiveRequestToANonLiveEntryTerminates(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaf := htmlbind.Bind(livePlan, liveParams{Values: []string{"one"}})
		if err := options.RenderStreamAsync(r.Context(), w, r, nil, leaf); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
		}
	})
	response, records := requestMode(t, handler, "live")
	if got := response.Header.Get("X-Tinybind-Render"); got != "navigation;v="+strconv.Itoa(htmlupdate.Version) {
		t.Fatalf("served mode = %q, want the navigation it actually served", got)
	}
	if last := records[len(records)-1]; last["r"] != "end" {
		t.Fatalf("last record = %+v", last)
	}
}

// A shutdown or a rolling deploy cancels the request context, and the sequence
// ends silently. Closing that done would tell every open screen that its sources
// finished and it should stop, so a deploy would leave them frozen until somebody
// reloaded.
func TestCancelledLiveStreamClosesRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaf := htmlbind.Bind(livePlan, liveParams{Values: []string{"one"}})
		if err := options.RenderLiveStream(ctx, w, r, nil, leaf); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
		}
	})
	cancel()
	_, records := requestMode(t, handler, "live")
	last := records[len(records)-1]
	if last["r"] != "end" {
		t.Fatalf("last record = %+v", last)
	}
	if last["reason"] != "retry" {
		t.Fatalf("terminator = %+v, want the client told to come back", last)
	}
}

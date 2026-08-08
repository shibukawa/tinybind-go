package htmlupdate_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// badge stands in for a generated component whose root carries an author id.
type badgeParams struct {
	ID    string
	Count int
}

var badgeOps = htmlbind.Builder[badgeParams]{}

// The boundary is what generation emits for a reloadable component: it names its
// own instance from the declared id, so it is an update boundary wherever it
// renders and a delta can compare the region a redraw can replace.
var badgePlan = &htmlbind.Plan[badgeParams]{
	Boundary: &htmlbind.Boundary[badgeParams]{
		ComponentID: "Badge@v1",
		Attr:        "data-tb-id",
		Instance:    func(p badgeParams) string { return p.ID },
		Input:       func(p badgeParams) string { return delta.CanonInt(p.Count) },
	},
	Ops: []htmlbind.Op[badgeParams]{
		badgeOps.Static("<span"),
		badgeOps.Attr("id", func(p badgeParams) (string, bool) { return htmlbind.Escape(p.ID), true }),
		badgeOps.BoundaryAttr(),
		badgeOps.Static(">"),
		badgeOps.Text(func(p badgeParams) string { return strconv.Itoa(p.Count) }),
		badgeOps.Static("</span>"),
	},
}

// styledPlan is a component that brings a stylesheet, standing in for one an
// action reveals for the first time: a validation summary, a panel that was not
// on the page before.
var styledPlan = &htmlbind.Plan[badgeParams]{
	Head:        []string{`<link rel="stylesheet" href="/badge.css">`},
	HeadSources: []string{"Badge"},
	Boundary:    badgePlan.Boundary,
	Ops:         badgePlan.Ops,
}

// An action may render a region the document never carried, so its stylesheet
// is not in the live head and its markup landing first flashes unstyled. That
// is the failure the navigation delta added a head field to prevent, and the
// action response had never carried one.
func TestActionResponseCarriesHead(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := options.WriteUpdate(recorder, actionRequest(), []htmlupdate.Update{
		htmlupdate.Replace("cart", htmlbind.Bind(styledPlan, badgeParams{ID: "cart", Count: 1})),
		// A second region declaring the same sheet emits one tag, which is the
		// htmlbind.MergeHead rule applied across the written set.
		htmlupdate.Replace("mini", htmlbind.Bind(styledPlan, badgeParams{ID: "mini", Count: 1})),
	})
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Head []string `json:"head"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Head) != 1 {
		t.Fatalf("head = %v, want one deduplicated tag", body.Head)
	}
	if !strings.Contains(body.Head[0], "/badge.css") {
		t.Fatalf("head = %v", body.Head)
	}
}

// A component that brings nothing leaves the field out, so a project using no
// component styles gets the response it got before.
func TestActionResponseOmitsAnEmptyHead(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := options.WriteUpdate(recorder, actionRequest(), []htmlupdate.Update{
		htmlupdate.Replace("cart", htmlbind.Bind(badgePlan, badgeParams{ID: "cart", Count: 1}))}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(recorder.Body.String(), `"head"`) {
		t.Fatalf("body carries an empty head field: %s", recorder.Body.String())
	}
}

// api is an ordinary JSON endpoint that additionally knows how to answer with
// the regions its action changed. One branch point decides which.
func api(count int, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if options.WantsUpdate(r) {
			_ = options.WriteUpdateStatus(w, r, status, []htmlupdate.Update{
				htmlupdate.Replace("cart", htmlbind.Bind(badgePlan, badgeParams{ID: "cart", Count: count}))})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]int{"count": count})
	})
}

// actionRequest is the request an action entry now takes. The two tests that
// drive the writer directly have no handler around them, so they build one.
func actionRequest() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/cart/add", nil)
}

func post(t *testing.T, handler http.Handler, headers map[string]string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/cart/add", nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

var actionHeader = map[string]string{
	"X-Tinybind-Render": "action;v=" + strconv.Itoa(clientVersion),
	"X-Tinybind-Build":  htmlupdate.BuildID(),
}

// Without the header the endpoint stays an ordinary API, which is what keeps a
// non-browser client and a page without the runtime working.
func TestActionWithoutTheHeaderStaysOrdinary(t *testing.T) {
	response := post(t, api(3, http.StatusOK), nil)
	body := read(t, response)
	if !strings.Contains(body, `"count":3`) {
		t.Fatalf("want the ordinary JSON body, got %q", body)
	}
	if response.Header.Get("X-Tinybind-Render") != "" {
		t.Fatal("an ordinary response must not claim to be an update")
	}
}

// One round trip performs the action and returns the regions it changed.
func TestActionReturnsTheChangedRegions(t *testing.T) {
	response := post(t, api(3, http.StatusOK), actionHeader)
	// The echo names the served mode and no version. The action path could read
	// the claimed version now that it takes the request, and deliberately does
	// not: the version is the caller's field, and echoing it here would be a
	// wire change this round did not ask for.
	if got := response.Header.Get("X-Tinybind-Render"); got != "action" {
		t.Fatalf("render header = %q", got)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	var body deltaBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Operations) != 1 {
		t.Fatalf("want one operation, got %+v", body.Operations)
	}
	operation := body.Operations[0]
	if operation.ID != "cart" || operation.Kind != "replace" {
		t.Fatalf("unexpected operation %+v", operation)
	}
	// The replacement has to keep the id, or the region becomes unaddressable
	// after the first update.
	if !strings.Contains(operation.HTML, `id="cart"`) || !strings.Contains(operation.HTML, ">3<") {
		t.Fatalf("unexpected markup %q", operation.HTML)
	}
	// An action changes state rather than reporting a render, so it carries no
	// manifest and must not disturb the navigation validators the client holds.
	if len(body.Manifest) != 0 {
		t.Fatalf("an action must not restate the manifest, got %+v", body.Manifest)
	}
}

// A rejected submission returns its real status and still carries the regions
// showing why, which is the whole reason the client ignores status.
func TestActionKeepsItsStatus(t *testing.T) {
	response := post(t, api(0, http.StatusUnprocessableEntity), actionHeader)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", response.StatusCode)
	}
	var body deltaBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Operations) != 1 {
		t.Fatalf("a failed action must still be able to rewrite a region, got %+v", body.Operations)
	}
}

// An action that changed where the user belongs says so rather than guessing
// which regions to rewrite.
func TestActionCanNavigate(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = options.WriteNavigate(w, "/orders/17")
	})
	response := post(t, handler, actionHeader)
	var body struct {
		Navigate string `json:"navigate"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Navigate != "/orders/17" {
		t.Fatalf("navigate = %q", body.Navigate)
	}
}

func TestWantsUpdateRejectsWhatItCannotServe(t *testing.T) {
	// Only the mode decides. A version this package never chose cannot make an
	// action request unservable, so the rejected set is the wrong mode and the
	// missing header.
	for _, header := range []string{"", "navigation;v=1", "navigation", ";v=1"} {
		request := httptest.NewRequest(http.MethodPost, "/cart/add", nil)
		if header != "" {
			request.Header.Set("X-Tinybind-Render", header)
		}
		request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
		if options.WantsUpdate(request) {
			t.Fatalf("header %q was accepted", header)
		}
	}
	// The same request under any version, and under none, is servable.
	for _, header := range []string{"action", "action;v=0", "action;v=" + strconv.Itoa(clientVersion+1)} {
		request := httptest.NewRequest(http.MethodPost, "/cart/add", nil)
		request.Header.Set("X-Tinybind-Render", header)
		request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
		if !options.WantsUpdate(request) {
			t.Fatalf("header %q was refused", header)
		}
	}
	// The right render header from a page another build rendered is refused
	// too, because its regions are not ones this binary can hand back.
	stale := httptest.NewRequest(http.MethodPost, "/cart/add", nil)
	stale.Header.Set("X-Tinybind-Render", "action;v="+strconv.Itoa(clientVersion))
	stale.Header.Set("X-Tinybind-Build", "older-revision")
	if options.WantsUpdate(stale) {
		t.Fatal("a page from another build was accepted")
	}
}

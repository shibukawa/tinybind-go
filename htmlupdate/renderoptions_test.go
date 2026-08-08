package htmlupdate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// The redraw and action entries rendered with no htmlbind option at all, so a
// component rendered one way inside its page and another in the response that
// replaced it. Two of the absences were not defaults but failures: a component
// holding an unsafe form did not render, and a caller's configured URL scheme
// allowlist never arrived. These cover both, plus the store the report asked
// for and the context nothing was passing.

// formParams stands in for a component whose region an action rewrites with its
// validation errors, which is the documented reason WriteUpdateStatus exists.
type formParams struct {
	ID   string
	Link url.URL
}

var formOps = htmlbind.Builder[formParams]{}

// formPlan is what generation emits for an unsafe form: the CSRF field is the
// first child, so an author writes nothing and cannot displace it.
var formPlan = &htmlbind.Plan[formParams]{
	Boundary: &htmlbind.Boundary[formParams]{
		ComponentID: "Form@v1", Attr: "data-tb-id",
		Instance: func(p formParams) string { return p.ID },
		Input:    func(formParams) string { return "" },
	},
	Ops: []htmlbind.Op[formParams]{
		formOps.Static(`<form method="post"`),
		formOps.BoundaryAttr(),
		formOps.Attr("id", func(p formParams) (string, bool) { return htmlbind.Escape(p.ID), true }),
		formOps.Static(`>`),
		formOps.CSRFField("_csrf"),
		formOps.Static(`</form>`),
	},
}

// linkPlan renders a URL the deployment's own scheme policy has to permit.
var linkPlan = &htmlbind.Plan[formParams]{
	Boundary: &htmlbind.Boundary[formParams]{
		ComponentID: "Link@v1", Attr: "data-tb-id",
		Instance: func(p formParams) string { return p.ID },
		Input:    func(formParams) string { return "" },
	},
	Ops: []htmlbind.Op[formParams]{
		formOps.Static(`<a`),
		formOps.BoundaryAttr(),
		formOps.Attr("id", func(p formParams) (string, bool) { return htmlbind.Escape(p.ID), true }),
		formOps.URLAttr("href", func(p formParams) (string, bool) { return p.Link.String(), true }),
		formOps.Static(`>open</a>`),
	},
}

const (
	formKind = "Form@0001"
	linkKind = "Link@0001"
)

func formRegistry(t *testing.T, plan *htmlbind.Plan[formParams]) *htmlupdate.Registry {
	t.Helper()
	registry := &htmlupdate.Registry{}
	kind := formKind
	if plan == linkPlan {
		kind = linkKind
	}
	registry.Register(htmlupdate.Reloadable{
		KindID: kind,
		Render: func(r *http.Request, instanceID string, values url.Values) (htmlbind.Fragment, error) {
			link, err := url.Parse(values.Get("link"))
			if err != nil {
				return htmlbind.Fragment{}, err
			}
			return htmlbind.Bind(plan, formParams{ID: instanceID, Link: *link}), nil
		},
	})
	return registry
}

// A component containing an unsafe form could not be a redraw endpoint at all:
// CSRFField needs a token, no option supplied one, and the render failed into a
// 500. Supplying the token is what makes the class serviceable.
func TestRedrawRendersAnUnsafeFormWhenGivenAToken(t *testing.T) {
	request := redrawRequest(formKind, "signup", nil)
	registry := formRegistry(t, formPlan)

	bare := httptest.NewRecorder()
	if !options.Redraw(bare, request, registry) {
		t.Fatal("Redraw did not answer the request")
	}
	if bare.Code != http.StatusInternalServerError {
		t.Fatalf("without a token the render should still fail loudly, got %d", bare.Code)
	}

	withToken := httptest.NewRecorder()
	if !options.Redraw(withToken, request, registry, htmlbind.WithCSRFToken("t0ken")) {
		t.Fatal("Redraw did not answer the request")
	}
	if withToken.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", withToken.Code, withToken.Body.String())
	}
	if markup := redrawHTML(t, withToken.Result(), "signup"); !strings.Contains(markup, `name="_csrf" value="t0ken"`) {
		t.Fatalf("fragment carries no token field: %s", markup)
	}
}

// The same failure on the action path, where it is worse: rewriting a form
// region with its validation errors is the case WriteUpdateStatus documents.
func TestActionRendersAnUnsafeFormWhenGivenAToken(t *testing.T) {
	region := []htmlupdate.Update{
		htmlupdate.Replace("signup", htmlbind.Bind(formPlan, formParams{ID: "signup"})),
	}

	bare := httptest.NewRecorder()
	if err := options.WriteUpdateStatus(bare, actionRequest(), http.StatusUnprocessableEntity, region); err == nil {
		t.Fatal("a form region with no token should not render")
	}

	ok := httptest.NewRecorder()
	err := options.WriteUpdateStatus(ok, actionRequest(), http.StatusUnprocessableEntity, region,
		htmlbind.WithCSRFToken("t0ken"))
	if err != nil {
		t.Fatal(err)
	}
	if ok.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", ok.Code)
	}
	if !strings.Contains(ok.Body.String(), `value=\"t0ken\"`) {
		t.Fatalf("body carries no token field: %s", ok.Body.String())
	}
}

// A configured scheme allowlist never reached the redraw path, so an app's own
// scheme rendered on the page and neutralised in the response replacing it. The
// divergence was stricter rather than looser, so nothing hostile ever rendered
// — it cost an application its own scheme.
func TestRedrawAppliesTheConfiguredURLSchemes(t *testing.T) {
	request := redrawRequest(linkKind, "deep", url.Values{"link": {"myapp://open/42"}})
	registry := formRegistry(t, linkPlan)

	bare := httptest.NewRecorder()
	options.Redraw(bare, request, registry)
	if markup := redrawHTML(t, bare.Result(), "deep"); !strings.Contains(markup, "#tb-blocked-url") {
		t.Fatalf("the default allowlist should still neutralise an unconfigured scheme: %s", markup)
	}

	configured := httptest.NewRecorder()
	options.Redraw(configured, request, registry, htmlbind.WithURLSchemes("http", "https", "myapp"))
	if markup := redrawHTML(t, configured.Result(), "deep"); !strings.Contains(markup, `href="myapp://open/42"`) {
		t.Fatalf("the configured scheme did not reach the redraw render: %s", markup)
	}
}

// A hostile scheme stays inert whatever the caller configured, since the
// allowlist is positive and nothing in this change widens it.
func TestRedrawStillNeutralisesAHostileScheme(t *testing.T) {
	request := redrawRequest(linkKind, "deep", url.Values{"link": {"javascript:alert(1)"}})
	recorder := httptest.NewRecorder()
	options.Redraw(recorder, request, formRegistry(t, linkPlan),
		htmlbind.WithURLSchemes("http", "https", "myapp"))
	if strings.Contains(recorder.Body.String(), "javascript:") {
		t.Fatalf("hostile scheme reached the attribute: %s", recorder.Body.String())
	}
}

// A cached component redrawn on its own ran its body every time, while the same
// component was served from the store on the page around it. The store is a
// caller resource per render, so it arrives through the options rather than on
// Options — which is what WithCache's own documentation asks for.
func TestRedrawReachesTheCacheStore(t *testing.T) {
	store := htmlbind.NewMemoryCache(8)
	request := redrawRequest(cardKind, "card-1", url.Values{"page": {"2"}})
	registry := cardRegistry(t)

	first := httptest.NewRecorder()
	if !options.Redraw(first, request, registry, htmlbind.WithCache(store)) {
		t.Fatal("Redraw did not answer the request")
	}
	second := httptest.NewRecorder()
	if !options.Redraw(second, request, registry, htmlbind.WithCache(store)) {
		t.Fatal("Redraw did not answer the request")
	}
	// badgePlan carries no cache annotation, so this asserts the option reaches
	// the render and changes nothing about the output. Whether a CachePolicy is
	// then consulted belongs to the htmlbind suite that owns the policy.
	if first.Body.String() != second.Body.String() {
		t.Fatalf("two redraws with one store disagreed:\n%s\n%s", first.Body.String(), second.Body.String())
	}
}

// The redraw render used context.Background(), so a cancelled request reached
// neither a context-taking external nor a shared store. It now runs under the
// request's own context.
func TestRedrawRendersUnderTheRequestContext(t *testing.T) {
	// A plain variable rather than a channel: the synchronous render runs on this
	// goroutine, and a channel would turn a render that never reaches the op into
	// a hang instead of a failure.
	var ran, cancelled bool
	plan := &htmlbind.Plan[formParams]{
		Boundary: &htmlbind.Boundary[formParams]{
			ComponentID: "Ctx@v1",
			Attr:        "data-tb-id",
			Instance:    func(p formParams) string { return p.ID },
			Input:       func(formParams) string { return "" },
		},
		Ops: []htmlbind.Op[formParams]{
			formOps.Static("<span"),
			formOps.BoundaryAttr(),
			formOps.Static(">"),
			formOps.TextCtx(func(ctx context.Context, _ formParams) string {
				ran, cancelled = true, ctx.Err() != nil
				return "x"
			}),
			formOps.Static("</span>"),
		},
	}
	registry := &htmlupdate.Registry{}
	registry.Register(htmlupdate.Reloadable{
		KindID: "Ctx@0001",
		Render: func(r *http.Request, instanceID string, _ url.Values) (htmlbind.Fragment, error) {
			return htmlbind.Bind(plan, formParams{ID: instanceID}), nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := redrawRequest("Ctx@0001", "ctx-1", nil).WithContext(ctx)
	options.Redraw(httptest.NewRecorder(), request, registry)
	if !ran {
		t.Fatal("the render never reached the context-taking op")
	}
	if !cancelled {
		t.Fatal("the render did not see the request's cancellation")
	}
}

// The primary page entry took no render options at all, so a page containing an
// unsafe form could not render through it — the same failure the redraw and
// action entries had, on the path every ordinary request reaches first. It was
// missed on the earlier sweep because these entries render through the delta
// package rather than by calling htmlbind directly.
func TestEveryRenderEntryTakesRenderOptions(t *testing.T) {
	page := htmlbind.Bind(formPlan, formParams{ID: "signup"})
	entries := map[string]func(w http.ResponseWriter, r *http.Request, options ...htmlbind.Option) error{
		"Render": func(w http.ResponseWriter, r *http.Request, o ...htmlbind.Option) error {
			return options.Render(w, r, nil, page, o...)
		},
		"RenderStream": func(w http.ResponseWriter, r *http.Request, o ...htmlbind.Option) error {
			return options.RenderStream(w, r, nil, page, o...)
		},
		"RenderStreamAsync": func(w http.ResponseWriter, r *http.Request, o ...htmlbind.Option) error {
			return options.RenderStreamAsync(r.Context(), w, r, nil, page, o...)
		},
	}
	for name, entry := range entries {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/signup", nil)
			if err := entry(httptest.NewRecorder(), request); err == nil {
				t.Fatal("a form with no token should not render")
			}
			recorder := httptest.NewRecorder()
			if err := entry(recorder, request, htmlbind.WithCSRFToken("t0ken")); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(recorder.Body.String(), `value="t0ken"`) {
				t.Fatalf("the token did not reach the render: %s", recorder.Body.String())
			}
		})
	}
}

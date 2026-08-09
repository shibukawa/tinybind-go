package fasthttpupdate_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/fasthttpupdate"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
	"github.com/shibukawa/tinybind-go/htmlupdate"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The two shells are one implementation behind two transports, so what these
// tests assert is that the pair agree, not that either matches a golden file. A
// golden file would only prove each side still does what it did yesterday,
// which is the property that was never in doubt.
//
// Every entry compared here returns a Response the caller sends. That is what
// made them portable: the coupling was a parameter type, not a behaviour.

type badgeParams struct {
	ID    string
	Count int
}

var badgeOps = htmlbind.Builder[badgeParams]{}

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

const cardKind = "UserCard@8Qv3n1"

// The Options are declared once as a literal and converted, which is what an
// authored source file looks like after the import rewrite: the same field
// names, the same values, a different package behind the name.
var netOptions = htmlupdate.Options{Key: []byte("k"), ServeRuntime: true, BuildID: "b1"}

var fastOptions = fasthttpupdate.Options{Key: []byte("k"), ServeRuntime: true, BuildID: "b1"}

// render is the generated decoder stood in for: typed, and refusing a value it
// cannot parse rather than substituting a zero. It takes a context on both
// sides, which is why one registration serves both.
func render(_ context.Context, instanceID string, values url.Values) (htmlbind.Fragment, error) {
	page, err := strconv.Atoi(values.Get("page"))
	if err != nil {
		return htmlbind.Fragment{}, errors.New("page must be an integer")
	}
	if page > 10 {
		return htmlbind.Fragment{}, errors.New("forbidden page")
	}
	return htmlbind.Bind(badgePlan, badgeParams{ID: instanceID, Count: page}), nil
}

func registry(t *testing.T) *htmlupdate.Registry {
	t.Helper()
	reg := &htmlupdate.Registry{}
	if err := reg.Register(htmlupdate.Reloadable{KindID: cardKind, Render: render}); err != nil {
		t.Fatal(err)
	}
	return reg
}

// headerPair is one request expressed for both transports. Building them from
// one description is what keeps a divergence in the result attributable to the
// runtimes rather than to the fixtures.
type headerPair struct {
	method  string
	target  string
	headers map[string]string
	body    string
}

func (p headerPair) netHTTP() *http.Request {
	var reader *strings.Reader
	if p.body != "" {
		reader = strings.NewReader(p.body)
	} else {
		reader = strings.NewReader("")
	}
	r := httptest.NewRequest(p.method, p.target, reader)
	if p.body != "" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for name, value := range p.headers {
		r.Header.Set(name, value)
	}
	return r
}

func (p headerPair) fast() *fasthttp.RequestCtx {
	var fr fasthttp.Request
	fr.SetRequestURI(p.target)
	fr.Header.SetMethod(p.method)
	if p.body != "" {
		fr.Header.SetContentType("application/x-www-form-urlencoded")
		fr.SetBody([]byte(p.body))
	}
	for name, value := range p.headers {
		fr.Header.Set(name, value)
	}
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&fr, nil, nil)
	return ctx
}

func redraw(kind, instance string, values url.Values) headerPair {
	target := "/dashboard"
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	return headerPair{method: http.MethodGet, target: target, headers: map[string]string{
		"X-Tinybind-Render":   "redraw",
		"X-Tinybind-Build":    "b1",
		"X-Tinybind-Kind":     kind,
		"X-Tinybind-Instance": instance,
	}}
}

func sameResponse(t *testing.T, what string, want htmlupdate.Response, got fasthttpupdate.Response) {
	t.Helper()
	if want.Status != got.Status {
		t.Errorf("%s status: net/http %d, fasthttp %d", what, want.Status, got.Status)
	}
	if string(want.Body) != string(got.Body) {
		t.Errorf("%s body differs\n net/http: %s\n fasthttp: %s", what, want.Body, got.Body)
	}
	sameHeader(t, what, want.Header, got.Header)
	switch {
	case want.Failure == nil && got.Failure != nil:
		t.Errorf("%s: fasthttp reported a failure and net/http did not: %v", what, got.Failure)
	case want.Failure != nil && got.Failure == nil:
		t.Errorf("%s: net/http reported a failure and fasthttp did not: %v", what, want.Failure)
	case want.Failure != nil && want.Failure.Kind != got.Failure.Kind:
		t.Errorf("%s failure kind: net/http %v, fasthttp %v", what, want.Failure.Kind, got.Failure.Kind)
	}
}

func sameHeader(t *testing.T, what string, want, got http.Header) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%s header set differs\n net/http: %v\n fasthttp: %v", what, want, got)
		return
	}
	for name, values := range want {
		if strings.Join(got[name], "|") != strings.Join(values, "|") {
			t.Errorf("%s header %s: net/http %v, fasthttp %v", what, name, values, got[name])
		}
	}
}

func TestRedrawParity(t *testing.T) {
	reg := registry(t)
	for _, tc := range []struct {
		name string
		req  headerPair
	}{
		{"answered", redraw(cardKind, "card-1", url.Values{"page": {"3"}})},
		{"unknown component", redraw("Nope@0000", "card-1", url.Values{"page": {"3"}})},
		{"undecodable argument", redraw(cardKind, "card-1", url.Values{"page": {"x"}})},
		{"refused by the component", redraw(cardKind, "card-1", url.Values{"page": {"99"}})},
		{"names no component", redraw(cardKind, "", url.Values{"page": {"3"}})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, wantOK := netOptions.Redraw(tc.req.netHTTP(), reg)
			got, gotOK := fastOptions.Redraw(tc.req.fast(), reg)
			if wantOK != gotOK {
				t.Fatalf("answered: net/http %v, fasthttp %v", wantOK, gotOK)
			}
			sameResponse(t, "redraw", want, got)
		})
	}
}

// A redraw digests its own body, so an entity tag that differed would mean the
// two transports rendered different bytes — which is the failure this whole
// arrangement exists to make impossible.
func TestRedrawETagParity(t *testing.T) {
	reg := registry(t)
	req := redraw(cardKind, "card-1", url.Values{"page": {"3"}})
	want, _ := netOptions.Redraw(req.netHTTP(), reg)
	got, _ := fastOptions.Redraw(req.fast(), reg)
	if want.Header.Get("ETag") == "" {
		t.Fatal("net/http redraw carried no entity tag")
	}
	if want.Header.Get("ETag") != got.Header.Get("ETag") {
		t.Errorf("entity tag differs: net/http %q, fasthttp %q",
			want.Header.Get("ETag"), got.Header.Get("ETag"))
	}
}

func TestActionParity(t *testing.T) {
	action := headerPair{method: http.MethodPost, target: "/cart", headers: map[string]string{
		"X-Tinybind-Render": "action",
		"X-Tinybind-Build":  "b1",
	}}
	if !netOptions.WantsUpdate(action.netHTTP()) {
		t.Fatal("net/http did not recognize the action request")
	}
	if !fastOptions.WantsUpdate(action.fast()) {
		t.Fatal("fasthttp did not recognize the action request")
	}

	updates := []htmlupdate.Update{
		htmlupdate.Replace("cart", htmlbind.Bind(badgePlan, badgeParams{ID: "cart", Count: 2})),
	}
	want, err := netOptions.WriteUpdateStatus(action.netHTTP(), http.StatusUnprocessableEntity, updates)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fastOptions.WriteUpdateStatus(action.fast(), http.StatusUnprocessableEntity, updates)
	if err != nil {
		t.Fatal(err)
	}
	sameResponse(t, "action", want, got)
}

// A stale build gets the document rather than a delta, on both sides. It is the
// one compatibility axis this package judges, so a transport that judged it
// differently would serve one backend's clients regions they cannot apply.
func TestNegotiateBuildSkewParity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build string
		want  htmlupdate.Mode
	}{
		{"current build", "b1", htmlupdate.ModeNavigation},
		{"another build", "b0", htmlupdate.ModeDocument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := headerPair{method: http.MethodGet, target: "/page?q=1", headers: map[string]string{
				"X-Tinybind-Render": "navigation",
				"X-Tinybind-Build":  tc.build,
			}}
			want := netOptions.Negotiate(req.netHTTP())
			got := fastOptions.Negotiate(req.fast())
			if want.Mode != got.Mode {
				t.Errorf("mode: net/http %v, fasthttp %v", want.Mode, got.Mode)
			}
			if want.Mode != tc.want {
				t.Errorf("net/http mode = %v, want %v", want.Mode, tc.want)
			}
		})
	}
}

// A POST claiming a render mode is a client error rather than a delta, because
// a render request must stay side-effect free. Reading the method is one of the
// six things the reader exists for, so a transport spelling it differently
// would show up here.
func TestNegotiateRefusesUnsafeMethodParity(t *testing.T) {
	req := headerPair{method: http.MethodPost, target: "/page", headers: map[string]string{
		"X-Tinybind-Render": "navigation",
		"X-Tinybind-Build":  "b1",
	}}
	if mode := netOptions.Negotiate(req.netHTTP()).Mode; mode != htmlupdate.ModeDocument {
		t.Errorf("net/http mode = %v, want document", mode)
	}
	if mode := fastOptions.Negotiate(req.fast()).Mode; mode != htmlupdate.ModeDocument {
		t.Errorf("fasthttp mode = %v, want document", mode)
	}
}

func TestHeadersParity(t *testing.T) {
	req := redraw(cardKind, "card-1", url.Values{"page": {"3"}})
	sameHeader(t, "redraw headers",
		netOptions.RedrawHeaders(req.netHTTP()), fastOptions.RedrawHeaders(req.fast()))
	sameHeader(t, "headers",
		netOptions.Headers(req.netHTTP(), nil, htmlbind.Fragment{}),
		fastOptions.Headers(req.fast(), nil, htmlbind.Fragment{}))

	nav := headerPair{method: http.MethodGet, target: "/page", headers: map[string]string{
		"X-Tinybind-Render": "navigation",
		"X-Tinybind-Build":  "b1",
	}}
	sameHeader(t, "stream headers",
		netOptions.StreamHeaders(nav.netHTTP(), nil, htmlbind.Fragment{}),
		fastOptions.StreamHeaders(nav.fast(), nil, htmlbind.Fragment{}))
	sameHeader(t, "live headers",
		netOptions.LiveHeaders(nav.netHTTP(), nil, htmlbind.Fragment{}),
		fastOptions.LiveHeaders(nav.fast(), nil, htmlbind.Fragment{}))
}

// The token reaches the server by two channels because a browser has two, and
// the pair have to agree about both or a form that submits without script works
// on one backend and not the other.
func TestCSRFParity(t *testing.T) {
	header := headerPair{method: http.MethodPost, target: "/cart", headers: map[string]string{
		"X-CSRF-Token": "tok-header",
	}}
	if got := netOptions.CSRFToken(header.netHTTP()); got != "tok-header" {
		t.Fatalf("net/http header token = %q", got)
	}
	if got := fastOptions.CSRFToken(header.fast()); got != "tok-header" {
		t.Errorf("fasthttp header token = %q, want tok-header", got)
	}

	field := headerPair{method: http.MethodPost, target: "/cart", body: "_csrf=tok-field&other=1"}
	if got := netOptions.CSRFToken(field.netHTTP()); got != "tok-field" {
		t.Fatalf("net/http field token = %q", got)
	}
	if got := fastOptions.CSRFToken(field.fast()); got != "tok-field" {
		t.Errorf("fasthttp field token = %q, want tok-field", got)
	}

	if err := netOptions.VerifyCSRF(field.netHTTP(), "tok-field"); err != nil {
		t.Fatalf("net/http verify: %v", err)
	}
	if err := fastOptions.VerifyCSRF(field.fast(), "tok-field"); err != nil {
		t.Errorf("fasthttp verify: %v", err)
	}
	if err := fastOptions.VerifyCSRF(field.fast(), "other"); !errors.Is(err, htmlupdate.ErrCSRFMismatch) {
		t.Errorf("fasthttp mismatch = %v, want ErrCSRFMismatch", err)
	}
}

// fasthttp's own FormValue falls back to the query arguments and net/http's
// PostFormValue does not. A token accepted from a query string is a token in
// access logs and referrers, so the narrower meaning is the one that has to
// survive the port — and this is the test that says so.
func TestCSRFIgnoresQueryParity(t *testing.T) {
	inQuery := headerPair{method: http.MethodPost, target: "/cart?_csrf=tok-query"}
	if got := netOptions.CSRFToken(inQuery.netHTTP()); got != "" {
		t.Fatalf("net/http read a token from the query: %q", got)
	}
	if got := fastOptions.CSRFToken(inQuery.fast()); got != "" {
		t.Errorf("fasthttp read a token from the query: %q, want none", got)
	}
}

// The runtime asset is addressed by a digest of its own bytes, so a deployment
// that serves one backend and negotiates against the other still agrees about
// the URL. Different bytes here would be two runtimes claiming one address.
func TestRuntimeAssetParity(t *testing.T) {
	if netOptions.RuntimePath() != fastOptions.RuntimePath() {
		t.Errorf("runtime path: net/http %q, fasthttp %q",
			netOptions.RuntimePath(), fastOptions.RuntimePath())
	}
	if netOptions.ScriptTag() != fastOptions.ScriptTag() {
		t.Errorf("script tag differs\n net/http: %s\n fasthttp: %s",
			netOptions.ScriptTag(), fastOptions.ScriptTag())
	}
	if string(netOptions.RuntimeAsset().Source) != string(fastOptions.RuntimeAsset().Source) {
		t.Error("the two backends serve different runtime bytes")
	}
}

// WriteTo is the one thing each shell implements rather than shares, so what it
// puts on the wire is compared rather than assumed.
func TestWriteToParity(t *testing.T) {
	reg := registry(t)
	req := redraw(cardKind, "card-1", url.Values{"page": {"3"}})

	answer, _ := netOptions.Redraw(req.netHTTP(), reg)
	recorder := httptest.NewRecorder()
	if _, err := answer.WriteTo(recorder); err != nil {
		t.Fatal(err)
	}

	fastAnswer, _ := fastOptions.Redraw(req.fast(), reg)
	ctx := req.fast()
	if _, err := fastAnswer.WriteTo(ctx); err != nil {
		t.Fatal(err)
	}

	if recorder.Code != ctx.Response.StatusCode() {
		t.Errorf("status: net/http %d, fasthttp %d", recorder.Code, ctx.Response.StatusCode())
	}
	if recorder.Body.String() != string(ctx.Response.Body()) {
		t.Errorf("body differs\n net/http: %s\n fasthttp: %s", recorder.Body, ctx.Response.Body())
	}
	for name, values := range recorder.Header() {
		want := strings.Join(values, "|")
		var got []string
		ctx.Response.Header.VisitAll(func(key, value []byte) {
			if http.CanonicalHeaderKey(string(key)) == name {
				got = append(got, string(value))
			}
		})
		if strings.Join(got, "|") != want {
			t.Errorf("written header %s: net/http %q, fasthttp %q", name, want, strings.Join(got, "|"))
		}
	}
}

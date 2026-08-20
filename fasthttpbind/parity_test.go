package fasthttpbind_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The two runtimes are separate implementations of one contract, so the tests
// that matter compare them against each other rather than against a golden
// file: a golden file only proves each side matches what it did yesterday.

type req struct {
	Name  string
	Page  int
	Org   string
	Token string
	Sess  string
}

const (
	uri  = "/orgs/acme/users?name=ada&page=7"
	body = `{"name":"grace","extra":"ignored"}`
)

// bindNetHTTP and bindFast are hand-written stand-ins for what the generator
// emits. They are deliberately the same statements with the same call names, to
// show the shape a rewrite has to preserve: only the argument lists differ.
func bindNetHTTP(r *http.Request) (req, error) {
	var out req
	q := httpbind.Queries(r)
	if v, ok := httpbind.QueryLookup(q, "name"); ok {
		out.Name = v
	}
	if v, ok := httpbind.QueryLookup(q, "page"); ok {
		n, err := httpbind.ParseInt(v)
		if err != nil {
			return out, httpbind.BindError("page", "query", "invalid int")
		}
		out.Page = n
	}
	out.Org = httpbind.PathValue(r, "org")
	out.Token = httpbind.HeaderValue(r, "Authorization")
	if v, ok := httpbind.CookieValue(r, "session"); ok {
		out.Sess = v
	}
	return out, nil
}

func bindFast(ctx *fasthttp.RequestCtx) (req, error) {
	var out req
	q := fasthttpbind.Queries(ctx)
	if v, ok := fasthttpbind.QueryLookup(q, "name"); ok {
		out.Name = v
	}
	if v, ok := fasthttpbind.QueryLookup(q, "page"); ok {
		n, err := fasthttpbind.ParseInt(v)
		if err != nil {
			return out, fasthttpbind.BindError("page", "query", "invalid int")
		}
		out.Page = n
	}
	out.Org = fasthttpbind.PathValue(ctx, "org")
	out.Token = fasthttpbind.HeaderValue(ctx, "Authorization")
	if v, ok := fasthttpbind.CookieValue(ctx, "session"); ok {
		out.Sess = v
	}
	return out, nil
}

func newNetHTTP(t *testing.T) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, uri, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer t0k")
	r.AddCookie(&http.Cookie{Name: "session", Value: "s3ss"})
	r.SetPathValue("org", "acme")
	return r
}

func newFast(t *testing.T) *fasthttp.RequestCtx {
	t.Helper()
	var fr fasthttp.Request
	fr.SetRequestURI(uri)
	fr.Header.SetMethod(http.MethodPost)
	fr.Header.SetContentType("application/json")
	fr.Header.Set("Authorization", "Bearer t0k")
	fr.Header.SetCookie("session", "s3ss")
	fr.SetBody([]byte(body))

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&fr, nil, nil)
	// fasthttp has no routing, so the path value arrives the way a router
	// would leave it rather than from the transport.
	ctx.SetUserValue("org", "acme")
	return ctx
}

func TestBindParity(t *testing.T) {
	want, err := bindNetHTTP(newNetHTTP(t))
	if err != nil {
		t.Fatalf("net/http bind: %v", err)
	}
	got, err := bindFast(newFast(t))
	if err != nil {
		t.Fatalf("fasthttp bind: %v", err)
	}
	if got != want {
		t.Errorf("bound value differs\n net/http: %+v\n fasthttp: %+v", want, got)
	}
	if want.Name != "ada" || want.Page != 7 || want.Org != "acme" ||
		want.Token != "Bearer t0k" || want.Sess != "s3ss" {
		t.Errorf("net/http bind did not read the request as expected: %+v", want)
	}
}

func TestReadJSONBodyParity(t *testing.T) {
	want, err := httpbind.ReadJSONBody(newNetHTTP(t))
	if err != nil {
		t.Fatalf("net/http ReadJSONBody: %v", err)
	}
	got, err := fasthttpbind.ReadJSONBody(newFast(t))
	if err != nil {
		t.Fatalf("fasthttp ReadJSONBody: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("body differs: net/http %q fasthttp %q", want, got)
	}
}

// The raw body points into pooled memory on fasthttp, so this is the test that
// the copy in ReadJSONBodyOwned is actually happening — the variant a binder
// whose raw spans outlive the bind is emitted against.
func TestReadJSONBodyOwnedDoesNotAliasPooledBody(t *testing.T) {
	ctx := newFast(t)
	owned, err := fasthttpbind.ReadJSONBodyOwned(ctx)
	if err != nil {
		t.Fatalf("ReadJSONBodyOwned: %v", err)
	}
	before := string(owned)

	// Scribble over the request body the way reuse of a pooled ctx would.
	ctx.Request.SetBody(bytes.Repeat([]byte("X"), len(body)))
	ctx.Request.Reset()

	if after := string(owned); after != before {
		t.Errorf("owned body aliased the pool: %q became %q", before, after)
	}
}

func TestWriteErrorParity(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"validation", httpbind.Validation(httpbind.Field("page", "query", "invalid int"))},
		{"not_found", httpbind.NotFound(httpbind.Problem{Code: "no_user", Message: "user not found"})},
		{"internal_is_hidden", httpbind.Internal(errDetail{})},
		{"bare", httpbind.BadRequest(httpbind.Problem{})},
		{"redirect", httpbind.Redirect("/sign-in")},
		{"permanent_redirect", httpbind.Redirect("/moved", 308)},
		{"refused_redirect_status", httpbind.Redirect("/nowhere", 200)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			httpbind.WriteError(rec, newNetHTTP(t), tc.err)

			ctx := newFast(t)
			fasthttpbind.WriteError(ctx, tc.err)

			if got, want := ctx.Response.StatusCode(), rec.Code; got != want {
				t.Errorf("status: fasthttp %d, net/http %d", got, want)
			}
			if got, want := string(ctx.Response.Header.ContentType()), rec.Header().Get("Content-Type"); got != want {
				t.Errorf("content-type: fasthttp %q, net/http %q", got, want)
			}
			if got, want := ctx.Response.Body(), rec.Body.Bytes(); !bytes.Equal(got, want) {
				t.Errorf("body differs\n net/http: %s\n fasthttp: %s", want, got)
			}
			// A redirect is the one error that answers with a header rather
			// than a document, so the header it answers with is part of the
			// parity rather than beside it.
			if got, want := string(ctx.Response.Header.Peek("Location")), rec.Header().Get("Location"); got != want {
				t.Errorf("location: fasthttp %q, net/http %q", got, want)
			}
			if tc.name == "internal_is_hidden" && bytes.Contains(ctx.Response.Body(), []byte("secret")) {
				t.Error("5xx body leaked the internal cause")
			}
		})
	}
}

type errDetail struct{}

func (errDetail) Error() string { return "secret internal detail" }

func TestWriteErrorNilWritesNothing(t *testing.T) {
	ctx := newFast(t)
	fasthttpbind.WriteError(ctx, nil)
	if n := len(ctx.Response.Body()); n != 0 {
		t.Errorf("nil error wrote %d bytes", n)
	}
}

func TestWriteJSONBytesParity(t *testing.T) {
	payload := []byte(`{"id":"u_1"}`)

	rec := httptest.NewRecorder()
	if err := httpbind.WriteJSONBytes(rec, http.StatusCreated, payload); err != nil {
		t.Fatalf("net/http WriteJSONBytes: %v", err)
	}
	ctx := newFast(t)
	if err := fasthttpbind.WriteJSONBytes(ctx, http.StatusCreated, payload); err != nil {
		t.Fatalf("fasthttp WriteJSONBytes: %v", err)
	}
	if got, want := ctx.Response.StatusCode(), rec.Code; got != want {
		t.Errorf("status: fasthttp %d, net/http %d", got, want)
	}
	if got, want := ctx.Response.Body(), rec.Body.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("body differs: fasthttp %q, net/http %q", got, want)
	}
}

func TestFormAndContentTypeParity(t *testing.T) {
	const form = "a=1&b=two&a=dup"

	r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var fr fasthttp.Request
	fr.SetRequestURI("/x")
	fr.Header.SetMethod(http.MethodPost)
	fr.Header.SetContentType("application/x-www-form-urlencoded")
	fr.SetBody([]byte(form))
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&fr, nil, nil)

	if httpbind.IsJSONRequest(r) != fasthttpbind.IsJSONRequest(ctx) {
		t.Error("IsJSONRequest disagrees")
	}

	want, err := httpbind.ParseFormMap(r)
	if err != nil {
		t.Fatalf("net/http ParseFormMap: %v", err)
	}
	got, err := fasthttpbind.ParseFormMap(ctx)
	if err != nil {
		t.Fatalf("fasthttp ParseFormMap: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("form size differs: fasthttp %v, net/http %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("form[%q]: fasthttp %q, net/http %q", k, got[k], v)
		}
	}
}

func TestMissingRegistrationIsAnError(t *testing.T) {
	type unregistered struct{ A string }
	if _, err := fasthttpbind.Bind[unregistered](newFast(t)); err == nil {
		t.Fatal("expected an error for an unregistered request type")
	}
	if err := fasthttpbind.Write[unregistered](newFast(t), unregistered{}); err == nil {
		t.Fatal("expected an error for an unregistered response type")
	}
}

func TestRegisteredBinderAndWriterRoundTrip(t *testing.T) {
	fasthttpbind.RegisterBind[req](bindFast)
	fasthttpbind.RegisterWrite[req](func(ctx *fasthttp.RequestCtx, v req) error {
		return fasthttpbind.WriteJSONBytes(ctx, http.StatusOK, []byte(`{"name":"`+v.Name+`"}`))
	})

	ctx := newFast(t)
	got, err := fasthttpbind.Bind[req](ctx)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "ada" {
		t.Errorf("Name = %q, want %q", got.Name, "ada")
	}
	if err := fasthttpbind.Write[req](ctx, got); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if want := `{"name":"ada"}` + "\n"; string(ctx.Response.Body()) != want {
		t.Errorf("body = %q, want %q", ctx.Response.Body(), want)
	}
}

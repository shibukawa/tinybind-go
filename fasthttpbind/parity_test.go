package fasthttpbind_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinybind-go/jsonbind"
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

// ParseBytes is one of the value-source helpers the generated binder calls
// under the httpbind qualifier, so the two runtimes have to answer alike or a
// fasthttp build binds a blob differently from a net/http one.
func TestParseBytesParity(t *testing.T) {
	for _, s := range []string{"", "AQID", "AQIDBA==", "-_8", "not base64!!"} {
		mine, myErr := httpbind.ParseBytes(s)
		theirs, theirErr := fasthttpbind.ParseBytes(s)
		if (myErr == nil) != (theirErr == nil) {
			t.Fatalf("%q: net/http err %v, fasthttp err %v", s, myErr, theirErr)
		}
		if !bytes.Equal(mine, theirs) {
			t.Fatalf("%q: net/http %v, fasthttp %v", s, mine, theirs)
		}
	}
}

// The array accessor has to agree across transports the way the scalar one
// does: same values, same order, and the same refusal to count an empty one.
func TestQueryLookupAllParity(t *testing.T) {
	const raw = "tag=a&q=go&tag=&tag=b%20c&tag[]=x"
	for _, key := range []string{"tag", "q", "tag[]", "absent"} {
		r := httptest.NewRequest(http.MethodGet, "/?"+raw, nil)
		want := httpbind.QueryLookupAll(httpbind.Queries(r), key)

		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/?" + raw)
		got := fasthttpbind.QueryLookupAll(fasthttpbind.Queries(ctx), key)

		if len(got) != len(want) {
			t.Fatalf("key=%q: fasthttp %q, net/http %q", key, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("key=%q element %d: fasthttp %q, net/http %q", key, i, got[i], want[i])
			}
		}
	}
}

// queryParityCases are the spellings the two runtimes used to answer
// differently: the driver's own parser admits a pair carrying a semicolon and a
// pair whose percent escape is broken, and url.ParseQuery drops both. Every one
// of these is a query a client writes, so the difference was the client's to
// pick — which is why both surfaces now split the raw query through
// bindcore.ParseQuery instead of each reading its own transport's idea of it.
var queryParityCases = []string{
	"a=1", "a=1&a=2", "a", "a=", "=1",
	"a=1;b=2", "tag=a;b&tag=c",
	"a=%zz", "a=%", "a=x%2", "%zz=1",
	"a%2Fb=c", "a+b=c+d", "a=%E3%81%82", "&&a=1", "a=1&", "%41=%42",
}

func TestQueryParity(t *testing.T) {
	for _, raw := range queryParityCases {
		for _, key := range []string{"a", "b", "tag", "A B", "a/b"} {
			checkQueryParity(t, raw, key)
		}
	}
}

// FuzzQueryParity keeps the two surfaces reading one query the same way. It
// drives both through their own transport's URI parsing rather than setting
// RawQuery by hand, so a divergence it reports is one a request can produce.
func FuzzQueryParity(f *testing.F) {
	for _, raw := range queryParityCases {
		f.Add(raw, "a")
	}
	f.Fuzz(func(t *testing.T, raw, key string) {
		// A fragment marker and a raw control byte never survive a request line
		// intact, so feeding them compares the two URI parsers rather than the
		// two query views.
		for i := 0; i < len(raw); i++ {
			if raw[i] < 0x21 || raw[i] > 0x7e || raw[i] == '#' {
				return
			}
		}
		if _, err := url.ParseRequestURI("/x?" + raw); err != nil {
			return
		}
		checkQueryParity(t, raw, key)
	})
}

func checkQueryParity(t *testing.T, raw, key string) {
	t.Helper()
	target := "/x?" + raw

	r := httptest.NewRequest(http.MethodGet, target, nil)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI(target)

	wantValue, wantOK := httpbind.QueryLookup(httpbind.Queries(r), key)
	gotValue, gotOK := fasthttpbind.QueryLookup(fasthttpbind.Queries(ctx), key)
	if gotValue != wantValue || gotOK != wantOK {
		t.Fatalf("%s key=%q: net/http=(%q,%v) fasthttp=(%q,%v)", target, key, wantValue, wantOK, gotValue, gotOK)
	}

	// The single-shot accessor answers off the same split, so it has to agree
	// with the pre-parsed one on both sides.
	if v, ok := fasthttpbind.QueryValue(ctx, key); v != wantValue || ok != wantOK {
		t.Fatalf("%s key=%q: fasthttp QueryValue=(%q,%v), QueryLookup=(%q,%v)", target, key, v, ok, wantValue, wantOK)
	}
	if v, ok := httpbind.QueryValue(r, key); v != wantValue || ok != wantOK {
		t.Fatalf("%s key=%q: net/http QueryValue=(%q,%v), QueryLookup=(%q,%v)", target, key, v, ok, wantValue, wantOK)
	}

	want := httpbind.QueryLookupAll(httpbind.Queries(r), key)
	got := fasthttpbind.QueryLookupAll(fasthttpbind.Queries(ctx), key)
	if len(got) != len(want) {
		t.Fatalf("%s key=%q: net/http=%q fasthttp=%q", target, key, want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s key=%q: net/http=%q fasthttp=%q", target, key, want, got)
		}
	}
}

// methodResp carries its own encoder and registers nothing, which is the
// jsonbind.GenerateCodec shape: the method is the only way to serialize it.
// Write must answer with it on both transports — it used to work on net/http
// and report missing_codec here.
type methodResp struct{ N int }

func (v methodResp) AppendJSONTo(dst []byte) []byte {
	dst = append(dst, `{"n":`...)
	dst = jsonbind.AppendInt(dst, int64(v.N))
	return append(dst, '}')
}

func TestWriteAppenderCarryingTypeParity(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	if err := httpbind.Write(rec, r, methodResp{N: 7}); err != nil {
		t.Fatalf("net/http Write: %v", err)
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	if err := fasthttpbind.Write(ctx, methodResp{N: 7}); err != nil {
		t.Fatalf("fasthttp Write: %v", err)
	}

	if got, want := string(ctx.Response.Body()), rec.Body.String(); got != want {
		t.Fatalf("bodies differ: fasthttp %q, net/http %q", got, want)
	}
	if got, want := string(ctx.Response.Header.ContentType()), rec.Header().Get("Content-Type"); got != want {
		t.Fatalf("content types differ: fasthttp %q, net/http %q", got, want)
	}
}

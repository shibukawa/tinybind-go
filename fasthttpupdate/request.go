package fasthttpupdate

import (
	"context"
	"net/url"

	"github.com/shibukawa/tinybind-go/internal/updatecore"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// request is *fasthttp.RequestCtx seen through the reader the update entries
// take. It is the same six accessors htmlupdate wraps *http.Request in, which
// is the whole of what the read-only half needed from a transport.
//
// Every string leaves through a []byte conversion, which copies, so no value an
// entry returns can alias pooled request memory.
type request struct{ ctx *fasthttp.RequestCtx }

func reader(ctx *fasthttp.RequestCtx) updatecore.Reader { return request{ctx} }

func (q request) Header(name string) string {
	if q.ctx == nil {
		return ""
	}
	return string(q.ctx.Request.Header.Peek(name))
}

func (q request) Method() string {
	if q.ctx == nil {
		return ""
	}
	return string(q.ctx.Method())
}

func (q request) RawQuery() string {
	if q.ctx == nil {
		return ""
	}
	return string(q.ctx.URI().QueryString())
}

// Query parses the raw query rather than reading QueryArgs, so a repeated name
// and a name with no value mean here what they mean on net/http. The decoders a
// redraw registration calls treat both as errors, and a second parser reaching
// a different answer is exactly the divergence this package exists to avoid.
func (q request) Query() url.Values {
	values, err := url.ParseQuery(q.RawQuery())
	if err != nil {
		// url.URL.Query discards the error and returns what parsed, and a redraw
		// whose arguments did not parse is refused by the decoder either way.
		return values
	}
	return values
}

// FormValue reads the request body and never the query.
//
// fasthttp's own FormValue falls back to the query arguments; net/http's
// PostFormValue does not, and this is the CSRF channel. A token accepted from a
// query string is a token in access logs and referrers, so the narrower
// net/http meaning is the one reproduced here.
func (q request) FormValue(name string) string {
	if q.ctx == nil {
		return ""
	}
	if value := q.ctx.PostArgs().Peek(name); value != nil {
		return string(value)
	}
	// A multipart submission carries its fields in the form rather than in
	// PostArgs, and net/http's ParseMultipartForm merges them into PostForm, so
	// reading only PostArgs would lose the token for exactly the forms that
	// upload something.
	form, err := q.ctx.MultipartForm()
	if err != nil || form == nil {
		return ""
	}
	if values := form.Value[name]; len(values) > 0 {
		return values[0]
	}
	return ""
}

// Context is the RequestCtx itself, which satisfies context.Context.
//
// It is passed through rather than copied, and that is inside the lifetime
// rule: the entries hand it to a render that completes before the handler
// returns, and nothing here stores it or gives it to a goroutine.
func (q request) Context() context.Context {
	if q.ctx == nil {
		return context.Background()
	}
	return q.ctx
}

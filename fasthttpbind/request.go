package fasthttpbind

import (
	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Request-side accessors. Each one keeps the name and the argument shape of its
// net/http counterpart, differing only in the transport type, so a generated
// binder reads the same on either backend.
//
// Every string returned here is produced by converting a pooled byte slice,
// which copies. That is the whole safety argument: no value handed to a binder
// can survive into another request.

// Queries returns the parsed query arguments. Generated binders call this once
// per request and resolve each field with QueryLookup.
func Queries(ctx *fasthttp.RequestCtx) *fasthttp.Args {
	if ctx == nil {
		return nil
	}
	return ctx.QueryArgs()
}

// QueryLookup returns the first value for key from pre-parsed query values.
// A key present with an empty value reports ("", true), matching net/http.
func QueryLookup(q *fasthttp.Args, key string) (string, bool) {
	if q == nil || !q.Has(key) {
		return "", false
	}
	return string(q.Peek(key)), true
}

// QueryValue returns the first query parameter value for key.
func QueryValue(ctx *fasthttp.RequestCtx, key string) (string, bool) {
	return QueryLookup(Queries(ctx), key)
}

// PathValue returns the path value for key.
//
// fasthttp has no routing of its own, so the value comes from whatever the
// router stored as a user value rather than from the transport.
func PathValue(ctx *fasthttp.RequestCtx, key string) string {
	if ctx == nil {
		return ""
	}
	switch v := ctx.UserValue(key).(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	return ""
}

// HeaderValue returns a request header.
func HeaderValue(ctx *fasthttp.RequestCtx, key string) string {
	if ctx == nil {
		return ""
	}
	return string(ctx.Request.Header.Peek(key))
}

// CookieValue returns a cookie value if present.
func CookieValue(ctx *fasthttp.RequestCtx, name string) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v := ctx.Request.Header.Cookie(name)
	if v == nil {
		return "", false
	}
	return string(v), true
}

func contentType(ctx *fasthttp.RequestCtx) string {
	if ctx == nil {
		return ""
	}
	return bindcore.MediaType(string(ctx.Request.Header.ContentType()))
}

// IsJSONRequest reports whether the request body should be treated as JSON.
func IsJSONRequest(ctx *fasthttp.RequestCtx) bool {
	return bindcore.IsJSONMediaType(contentType(ctx))
}

// IsFormRequest reports application/x-www-form-urlencoded.
func IsFormRequest(ctx *fasthttp.RequestCtx) bool {
	return contentType(ctx) == "application/x-www-form-urlencoded"
}

// IsMultipartRequest reports multipart/form-data.
func IsMultipartRequest(ctx *fasthttp.RequestCtx) bool {
	return contentType(ctx) == "multipart/form-data"
}

// ReadJSONObject splits a JSON object body into its raw fields.
//
// The returned Object holds subslices of the document, so the pooled body is
// copied first: a generated binder may hand those raw bytes straight into a
// json.RawMessage rest map, which would otherwise outlive the request.
func ReadJSONObject(ctx *fasthttp.RequestCtx) (*jsonbind.Object, error) {
	if ctx == nil {
		return jsonbind.EmptyObject(), nil
	}
	body := ctx.PostBody()
	limit := MaxJSONBodyBytes()
	if int64(len(body)) > limit {
		return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "JSON body too large"}, jsonbind.ErrBodyTooLarge)
	}
	if jsonbind.IsBlank(body) {
		return jsonbind.EmptyObject(), nil
	}
	owned := make([]byte, len(body))
	copy(owned, body)
	obj, err := jsonbind.ParseObject(owned)
	if err != nil {
		if je, ok := jsonbind.AsError(err); ok && je.Message == "JSON value must be an object" {
			return nil, BadRequest(Problem{Code: "json_parse", Message: "JSON body must be an object"}, err)
		}
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid JSON body"}, err)
	}
	return obj, nil
}

// ParseFormMap parses an urlencoded form body into a flat map (first value wins).
func ParseFormMap(ctx *fasthttp.RequestCtx) (map[string]string, error) {
	args := ctx.PostArgs()
	out := make(map[string]string, args.Len())
	args.VisitAll(func(key, value []byte) {
		k := string(key)
		if _, seen := out[k]; seen {
			return
		}
		out[k] = string(value)
	})
	return out, nil
}

// ParseMultipartMap parses a multipart/form-data body into scalar form fields
// (first value wins) and named file parts (first file wins per field name).
//
// The body is capped at MaxMultipartBodyBytes(). fasthttp has already read the
// body by the time a handler runs, so this bound is a policy check rather than
// the memory guarantee; the memory guarantee belongs to the server's own
// per-request limit.
func ParseMultipartMap(ctx *fasthttp.RequestCtx) (form map[string]string, files map[string]File, err error) {
	limit := MaxMultipartBodyBytes()
	if limit > 0 && int64(ctx.Request.Header.ContentLength()) > limit {
		return nil, nil, PayloadTooLarge(Problem{
			Code:    "payload_too_large",
			Message: "multipart body too large",
		}, nil)
	}
	mf, err := ctx.MultipartForm()
	if err != nil {
		return nil, nil, bindcore.MultipartParseError(err, bindcore.IsMessageTooLarge(err))
	}
	form = make(map[string]string)
	files = make(map[string]File)
	if mf == nil {
		return form, files, nil
	}
	for k, vs := range mf.Value {
		if len(vs) > 0 {
			form[k] = vs[0]
		}
	}
	for k, fhs := range mf.File {
		if len(fhs) == 0 {
			continue
		}
		f, ferr := bindcore.FileFromHeader(fhs[0], limit)
		if ferr != nil {
			if ferr == bindcore.ErrFileTooLarge || bindcore.IsMessageTooLarge(ferr) {
				return nil, nil, PayloadTooLarge(Problem{
					Code:    "payload_too_large",
					Message: "multipart file too large",
				}, ferr)
			}
			return nil, nil, BindError(k, "payload", "unreadable file")
		}
		files[k] = f
	}
	return form, files, nil
}

// ReadBody dispatches on the request content type and reads the body at most
// once on behalf of a generated binder. wantForm/wantFiles mirror which body
// kinds the binder's fields can consume; a request whose content type matches
// none of them yields all-nil results without error.
func ReadBody(ctx *fasthttp.RequestCtx, wantForm, wantFiles bool) (*jsonbind.Object, map[string]string, map[string]File, error) {
	if IsJSONRequest(ctx) {
		obj, err := ReadJSONObject(ctx)
		if err != nil {
			return nil, nil, nil, err
		}
		return obj, nil, nil, nil
	}
	if wantForm || wantFiles {
		if IsFormRequest(ctx) {
			m, err := ParseFormMap(ctx)
			if err != nil {
				return nil, nil, nil, err
			}
			return nil, m, nil, nil
		}
		if IsMultipartRequest(ctx) {
			m, files, err := ParseMultipartMap(ctx)
			if err != nil {
				return nil, nil, nil, err
			}
			if !wantFiles {
				files = nil
			}
			return nil, m, files, nil
		}
	}
	return nil, nil, nil, nil
}

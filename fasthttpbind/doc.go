// Package fasthttpbind is the fasthttp half of the binding runtime. It
// declares the same names as the net/http runtime over *fasthttp.RequestCtx,
// so generated code imports it under the httpbind alias and its call selectors
// read the same on either transport:
//
//	import httpbind "github.com/shibukawa/tinybind-go/fasthttpbind"
//
// Nothing here is written by hand in an application. Generation emits every
// call, which is why the declarations reuse the net/http names instead of
// taking a prefix that would only matter to a human reader.
//
// # Pooled memory
//
// A RequestCtx and every byte slice reachable from it are pooled and reused
// once the handler returns. Every value this package hands back is copied out,
// including the JSON document a binder parses. Nothing returned by Bind may
// alias the request.
//
// # Error bytes
//
// WriteError derives its document through the same shared code the net/http
// runtime uses, so the two transports emit identical problem bodies for
// identical errors.
package fasthttpbind

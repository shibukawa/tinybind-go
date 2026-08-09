// Package fasthttpupdate is the fasthttp half of the partial-update surface. It
// declares the same names as htmlupdate over *fasthttp.RequestCtx, so generated
// code imports it under the htmlupdate alias and its call selectors read the
// same on either transport:
//
//	import htmlupdate "github.com/shibukawa/tinybind-go/fasthttpupdate"
//
// # What is here, and what is not
//
// The entries that read a request and write nothing through it are all here:
// the action pair, the redraw, the sequence, the CSRF reads, the negotiation,
// and every header computation. Each returns a Response the caller sends, which
// is what made them portable — they were never coupled to the transport by
// behaviour, only by a parameter type.
//
// The entries that write as they go are not here, and will not be until the
// flusher inversion is done: Render, RenderStream, RenderStreamAsync,
// RenderLiveStream, OpenStream, and OpenLiveStream all hold a response open
// inside a handler that has not returned, which is precisely what fasthttp
// moves into SetBodyStreamWriter. A handler calling one is refused at generation
// with the occurrence named, rather than silently losing its delivery.
//
// # Pooled memory
//
// A RequestCtx and every byte slice reachable from it are pooled and reused
// once the handler returns. Every value read here is produced by converting a
// pooled byte slice, which copies, so nothing a Response carries can alias the
// request. The context is the one value passed through rather than copied: it
// reaches a render that completes before the handler returns, and nothing here
// retains it.
package fasthttpupdate

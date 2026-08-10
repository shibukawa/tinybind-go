# fasthttp Backend User Guide

You do not write fasthttp code. You write the same `net/http` handlers
[httpbind.md](httpbind.md) describes, and generation derives a second copy of
them against fasthttp. One authored package produces two builds, selected by a
build tag, and the compiler checks both.

That only works while the derivation can account for every use of the writer
and the request. It usually can. When it cannot, generation stops and names the
line, because there is no adapter to quietly absorb the handler it could not
rewrite. Knowing which of your handlers fall on which side is the first thing to
find out, and the last section of this guide is about that.

## What you get

- `Bind`, `Write`, `WriteStatus`, `WriteError` and the request accessors, over
  `*fasthttp.RequestCtx`, under the same names
- Binders and writers generated for the same models, registered with the
  fasthttp runtime
- Your handlers, rewritten: two transport parameters collapse into one context
- Route registration on a fasthttp router, from the same `net/http`
  registrations discovery already reads
- The same OpenAPI document; it derives from the field plan, not the transport
- The whole partial-update surface, including the streamed and live renders —
  see [the section below](#partial-updates-on-fasthttp) for the one behavioural
  difference

What does not change is the source you maintain. There is one authoring form,
and it is the net/http one.

## Before you generate: where the tag boundary falls

A build tag excludes a whole file. That single fact decides your file layout.

Your handlers are replaced under the fasthttp tag, so the file holding them is
excluded from that build. Anything sharing that file goes with it — including a
type declaration both builds need. Put transport handlers in files of their own:

```
api/
├── models.go      // untagged: request and response structs
├── handlers.go    //go:build !fasthttp — the handlers, and the ServeMux wiring
└── service.go     // untagged: whatever has no writer and no request in it
```

The `!fasthttp` tag on `handlers.go` is yours to write; generation does not add
it. Generation does check the arrangement and warns when a file mixes handlers
with declarations both builds need.

Wiring is the one thing you write twice. `handlers.go` builds a `ServeMux`;
a small tagged file builds the fasthttp server. Everything between them is
either shared or derived.

## Generating

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir ./api -backend fasthttp
```

Five files come out:

| File | Constraint | Contents |
| --- | --- | --- |
| `tinybind_gen.go` | `!fasthttp` | net/http binders and writers |
| `tinybind_fasthttp_gen.go` | `fasthttp` | fasthttp binders and writers |
| `tinybind_transport_gen.go` | `fasthttp` | your handlers, derived |
| `tinybind_routes_gen.go` | `fasthttp` | route registration |
| `tinybind_openapi_gen.go` | none | unchanged |

The two binder files register the same models against different runtimes, which
is why neither may compile without the other being excluded. Omit `-backend` and
none of this happens: you get exactly the output you got before the flag
existed, byte for byte.

## Running it

```go
//go:build fasthttp

package main

import (
	"github.com/shibukawa/tinygodriver/fasthttp"
	"github.com/shibukawa/tinygodriver/fasthttprouter"

	_ "github.com/shibukawa/tinygodriver/netdev" // TinyGo socket layer; a no-op on host Go

	"example.com/app/api"
)

func main() {
	r := router.New()
	api.RegisterRoutes(r)
	_ = fasthttp.ListenAndServe(":8080", r.Handler)
}
```

```bash
go build -tags fasthttp ./...
```

The router is the fork tinygodriver carries beside its fasthttp fork. It has to
be: a handler taking the fork's `RequestCtx` is not a handler taking
`valyala/fasthttp`'s, so the upstream router will not accept generated code.
Both spell a named parameter `{name}`, so your route patterns transfer
unchanged; a catch-all `{rest...}` becomes `{rest:*}`.

## Three differences worth knowing

**Values are copied out of the request.** A `RequestCtx` and every byte slice
reachable from it are pooled and reused once your handler returns. Everything
`Bind` hands back is copied, including the JSON document it parses, so a bound
value stays valid afterwards. Do not defeat this by keeping the context past the
handler — a goroutine holding one reads whatever request occupies that slot next.

**Streaming inverts.** `WriteStream` takes a callback on both transports:

```go
httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error {
	if err := s.Write(ChatEvent{Type: "delta", Delta: "hi"}); err != nil {
		return err
	}
	return s.Write(ChatEvent{Type: "done"})
})
```

On fasthttp the callback runs after your handler returns, so an error inside it
has no way back to handler code. Both transports therefore route it to the
handler you install with `SetStreamErrorHandler`, and both close the stream for
you — which is what keeps a JSON array document terminated when the callback
fails halfway through. The held-stream entry is gone rather than
deprecated: one that still compiled would be a call site with no fasthttp
counterpart, found at deploy rather than at build.

**WebSockets invert the same way, and cost less here.** `WebSocket` takes a
callback on both transports, and the callback body is one piece of source:

```go
_ = httpbind.WebSocket(w, r, func(s *httpbind.Socket[ClientMsg, ServerMsg]) error {
	for {
		in, err := s.Read()
		if err != nil {
			return err
		}
		if err := s.Write(ServerMsg{Type: "message", Text: in.Text}); err != nil {
			return err
		}
	}
})
```

The return value is the handshake error only; anything the callback returns is
post-101 and goes to `SetStreamErrorHandler`. As with a stream, the callback
outlives the handler here, so it must not read the context — capture first.

What fasthttp saves is the layer underneath. `RequestCtx.Hijack` is a
synchronous handoff, so the upgrade works on TinyGo with no help. The
`net/http` backend needs `tinygodriver/httpserver` in front of it, because
TinyGo's own server cannot complete an upgrade at all. fasthttp closes the
connection when the callback returns, which is exactly the callback shape's
contract, so `KeepHijackedConns` stays off.

**Some capabilities are gone.** fasthttp implements no HTTP/2. Under TinyGo it
cannot terminate TLS either, so put a terminator in front. TinyGo is supported
only in the sense that the package compiles; there is no size or throughput
commitment there, and the fasthttp fork is in fact larger than `net/http` on
that toolchain.

## Finding out what will refuse

Ask before you commit. The report writes nothing and exits zero:

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate \
    -dir ./api -backend fasthttp -transport-report
```

```
handlers.go:31:2: createUser passes r to otel.Attach, whose transport arguments are
  undeclared; remedy: move the call behind a function taking neither the writer nor
  the request, or register it as a call pattern declaring its transport slots (unknown_call)
handlers.go:78:1: listUsers calls renderError, which is not transformable;
  handlers.go:72:6 reads r.URL, which no rewrite covers; remedy: fix the refusal
  reported below this one (inherited)
3 handler(s) would be refused by the fasthttp backend
```

Each line ends with the classification, and each carries the position of the
occurrence rather than of the declaration. An inherited refusal names every hop
down to the line that actually caused it.

Adoption is all-or-nothing, so seeing the whole bill at once matters more than
usual. Refusals also cluster: one shared error helper commonly accounts for most
of a package, and fixing it clears every handler that called it.

A handler is admitted when every occurrence of the writer and the request is one
the rewriter recognizes — an argument to a runtime call, an argument to another
function being rewritten, or `r.Context()`, which maps to the context because a
`RequestCtx` satisfies `context.Context`. Assignment to `_` is fine. Anything
else refuses:

| Kind | What it means |
| --- | --- |
| `unknown_call` | the value reaches a function outside the package — tracing, metrics, sessions |
| `unknown_selector` | a field or method with no entry in the rewrite table, such as `r.RemoteAddr` |
| `escapes` | assigned, stored, returned, captured by a closure, or its address taken |
| `type_assertion` | `w.(http.Flusher)` and friends; use `WriteStream` instead |
| `inherited` | refused only because a callee is; the chain names the real occurrence |

Every refusal prints the position of the occurrence rather than of the
declaration, and the remedy that clears it.

## Partial updates on fasthttp

The update surface splits by what it needs from the transport, and the larger
half carries over unchanged.

Everything that reads a request and returns a `Response` you send works: the
action pair, the redraw, the sequence, `Negotiate`, the CSRF reads, and every
header computation. Your handler is the same handler — `options.WantsUpdate(r)`
becomes `options.WantsUpdate(ctx)`, and nothing else about the branch moves.

```go
func addToCart(w http.ResponseWriter, r *http.Request) {
    if !options.WantsUpdate(r) {
        htmlupdate.Redirect(w, r, "/cart", http.StatusSeeOther)
        return
    }
    answer, err := options.WriteUpdate(r, updates)
    if err != nil {
        httpbind.WriteError(w, r, err)
        return
    }
    _, _ = answer.WriteTo(w)
}
```

Use `htmlupdate.Redirect` rather than `http.Redirect`. It is the same call —
it delegates straight to the standard library on net/http — but fasthttp
redirects through a method on its context, and a rewrite that turns a function
call into a method call is not something the transform does. A handler calling
`http.Redirect` is refused by name.

Two things need saying about layout.

Your `Options` value has to be built inside a handler, or declared in a tagged
file pair. It is the one type each backend redeclares, and a package-level `var`
is a declaration rather than a function, so the transform does not rewrite it
and the tag excludes the file it lives in.

Your `Registry` does not have that problem, and neither do the component
registrations generation emits. A `Registry`, a `Reloadable`, an `Update` and a
`Failure` are one type on both backends, so a helper building one lives in an
untagged file and is compiled by both halves.

### The streaming entries take a callback

`OpenStream` and `OpenLiveStream` are gone. `WriteStream` and `WriteLiveStream`
replace them, and they take the producer rather than handing a stream back:

```go
options.WriteStream(w, r, head, func(stream *htmlupdate.DeltaStream) error {
    stream.Replace("feed", markup, entry)
    return nil
})
```

The reason is fasthttp: it writes a streamed body from a callback that runs
*after* your handler returned, so a stream you hold across statements has no
transcription there. Two things fall out of the shape that are worth having on
either backend — the entry closes the stream whether or not your producer
succeeded, so a forgotten `Close` can no longer send a truncated response, and
an error you return is reported in band and then sent to
`SetStreamErrorHandler` instead of being discarded.

`Render`, `RenderStream`, `RenderStreamAsync` and `RenderLiveStream` keep their
signatures. Everything each of them reads from the request is read before the
first record, so a failure before that is still an ordinary error you can turn
into a status; after it, the status is committed and the failure travels in the
terminator.

One difference is real and you should know it before you rely on live delivery.
**fasthttp has no per-request cancellation** — its `Done` channel closes on
server shutdown and nothing else. On net/http a live stream ends promptly when
the client disconnects, because the request context is cancelled. On fasthttp a
departed client is noticed when a record fails to write, so the stream ends at
its next delivery instead; a subscription that never delivers again holds its
resources until the server stops. Pass a context carrying your own bound if that
matters.

For the same reason, a `*fasthttp.RequestCtx` handed to `RenderLiveStream` as
its cancellation context is replaced with one carrying the shutdown signal. A
rewritten handler always does this — the transform collapses `r` and
`r.Context()` to the same identifier — and the value is pooled, so reading it
after the handler returned would read another request.

## What is not done yet

Per-route body limits do not transfer. `http.MaxBytesHandler` bounds one
handler, while fasthttp bounds the server, and the generated header hook that
would restore per-route limits is not written yet. If you rely on a route-level
limit, wait for it or set `Server.MaxRequestBodySize` to your smallest and
accept that as the ceiling everywhere.

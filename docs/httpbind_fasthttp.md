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
fails halfway through. `NewStream` still exists and is deprecated; it has no
fasthttp transcription.

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

## What is not done yet

Per-route body limits do not transfer. `http.MaxBytesHandler` bounds one
handler, while fasthttp bounds the server, and the generated header hook that
would restore per-route limits is not written yet. If you rely on a route-level
limit, wait for it or set `Server.MaxRequestBodySize` to your smallest and
accept that as the ceiling everywhere.

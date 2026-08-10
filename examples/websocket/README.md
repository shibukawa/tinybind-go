# websocket

A typed chat room, served beside ordinary REST routes on one port, under both
compilers.

```bash
go run ./examples/websocket
# open http://localhost:8080/ in two tabs
```

```bash
tinygo build -o wschat ./examples/websocket && ./wschat
```

## What it shows

**One socket type per direction.** `ClientMsg` is what the browser sends,
`ServerMsg` is what comes back, and the variants inside each are told apart by
a `type` field — the same shape a stream's events use. Neither type argument is
spelled at the call site: generation recovers both from the closure parameter.

Regenerating produces exactly three registrations, which is the point:

```go
jsonbind.RegisterDecode[ClientMsg](decodeClientMsgBytes)  // inbound: decoded, never encoded
jsonbind.RegisterEncode[ServerMsg](encodeServerMsg)       // outbound: encoded, never decoded
httpbind.RegisterWrite[HealthResponse](writeHealthResponse)
```

Each type gets the one codec its direction needs. A decoder that is never
called is dead weight in a TinyGo binary, so `generate.go` deliberately does
not pass `-generate-all`.

**Broadcasting is a handful of lines.** [hub.go](hub.go) holds a map of sockets
and writes to all of them from whichever connection's goroutine is handling a
message. That works because `Socket.Write` takes a lock the library's own
control frames share. Written against a raw gorilla connection the same feature
needs a per-connection outbound channel and a writer goroutine, because two
goroutines writing one connection interleave frames with no error and no
diagnostic.

The hub itself is application code on purpose. httpbind owns one connection
each — its read limit, its deadlines, its close handshake — and ships no
registry, because who may hear what is a design question rather than a
transport one.

**The socket is one route among many.** `/healthz` is an ordinary
`httpbind.Write` handler on the same mux and the same port.

**The handshake failure is yours to log, not to answer.** `WebSocket` returns
the handshake error only, and the refusal response — RFC 9457 Problem Details,
like every other refusal — is already written by then:

```console
$ curl -i -H 'Origin: https://evil.example' ... /ws
HTTP/1.1 403 Forbidden
Content-Type: application/problem+json

{"type":"about:blank","title":"Forbidden","status":403,"detail":"origin not allowed","code":"websocket_origin"}
```

Cross-origin is refused by default. A socket that accepts any origin is CSRF
with a connection attached.

## The one line that is not ordinary net/http

```go
return httpserver.Serve(ln, srv)
```

TinyGo's own `net/http` server cannot complete an upgrade: it starts a
background read before the handler and cancels it by moving the read deadline
into the past, which netdev cannot do to a `recv()` already in flight, so
`Hijack` blocks forever with no error and no log line. `httpserver` routes
upgrades around that and hands everything else to a real `http.Server`. Under
host Go it calls `srv.Serve(ln)` and nothing else.

Forget it and the entry answers 500 rather than hanging — but the fix is the
line, not the error. The fasthttp backend needs none of this, because
`RequestCtx.Hijack` is a synchronous handoff.

## A wire detail

Every field of a message is on the wire, including the ones a given variant
does not use. The JSON codec reads a `json` tag for the field's name and
ignores `omitempty`, so the tags here do not spell it. Carrying fields a
variant does not need is the admitted cost of typing a direction with one
struct; the alternative puts the discriminator's spelling inside the library
rather than in your protocol.

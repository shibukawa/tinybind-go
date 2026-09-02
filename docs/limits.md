# Size and shape limits

Every bound a request can run into, in one place: what it is, what it defaults
to, how to change it, and what the client sees when it is exceeded.

They were scattered across the body-type sections that own them, which reads
fine while you are configuring one of them and badly when you are answering
"what does this service accept?" — the question an operator asks once and a
security review asks every time.

## At a glance

| Limit | Default | Change it with | Exceeded |
|---|---|---|---|
| JSON body | 1 MiB | `SetMaxJSONBodyBytes`, or `jsonbind.DecodeJSONLimit` per call | 413 |
| JSON nesting depth | 90 | `jsonbind.SetMaxNestingDepth` | 400 |
| CBOR body | 1 MiB | `SetMaxCBORBodyBytes` | 413 |
| multipart body | 1 MiB | `SetMaxMultipartBodyBytes` | 413 |
| multipart file part | the multipart body limit | same knob | 413 |
| multipart RAM before spilling | 32 MiB, capped by the body limit | fixed (`DefaultMultipartMaxMemory`) | — |
| WebSocket message | 1 MiB | `SocketOptions.ReadLimit` | connection closes |
| fixed-length array or `[N]byte` field | the length the Go type declares | the type | 400 |
| urlencoded form body | **not this module's** — see [what the transport owns](#what-the-transport-owns) | | |

The setters are process-wide and live below both transport runtimes, so calling
one at startup configures net/http and fasthttp alike:

```go
func main() {
	httpbind.SetMaxJSONBodyBytes(4 << 20)      // 4 MiB
	httpbind.SetMaxMultipartBodyBytes(8 << 20) // 8 MiB
	httpbind.SetMaxCBORBodyBytes(2 << 20)      // 2 MiB
	httpbind.SetSocketDefaults(httpbind.SocketOptions{ReadLimit: 256 << 10})
	// ...
}
```

Each setter takes bytes, and a non-positive value restores the default rather
than removing the limit. There is no "unlimited": a bound nobody chose is a
bound the client chooses.

## Request bodies

### JSON — 1 MiB

`httpbind.SetMaxJSONBodyBytes(n)` / `httpbind.MaxJSONBodyBytes()`. Outside HTTP,
`jsonbind.DecodeJSONLimit(r, n)` bounds one call without touching the process
default.

On net/http the limit bounds the read itself, so a body arriving without a
Content-Length — or with a lying one — still stops at the cap. On fasthttp the
server has already read the body by the time a binder runs, so the cap is
checked against what arrived and the read itself was bounded by the server's
`MaxRequestBodySize`; set that too if you lower this one and want the bytes
refused earlier.

`jsonbind` returns a transport-neutral error; the generated binder maps it to
413.

### JSON nesting depth — 90

`jsonbind.DefaultMaxNestingDepth`, raised with `jsonbind.SetMaxNestingDepth`.

Unlike the others this is not a size but a shape. Reading is recursive — a
nested value is a nested call, and a generated decoder calls the next one down —
so the document's depth is the goroutine's stack depth. Without the bound a
one-megabyte body of `[` would be half a million frames and tens of megabytes
of stack per request, which is a body size limit doing nothing at all about
memory.

The default is set by the smallest stack the parser runs on rather than by the
host. TinyGo's goroutine stacks are fixed, and its wasm targets start at 64 KiB,
on which the parser overflows at about a hundred open brackets — and a wasm
overflow is detected only at exit, so the request that caused it gets a wrong
answer, not an error. Ninety stays under that with room for the frames around
the parser. Nothing an application produces comes close; hand-written JSON does
not nest ninety deep.

A document deeper than the bound is a parse error, which generated binders map
to 400 like any other malformed body. A host with a growable stack can raise the
bound at startup — `jsonbind.SetMaxNestingDepth(10000)` is what `encoding/json`
allows. A TinyGo target should raise it only together with `-stack-size`.

### CBOR — 1 MiB

`httpbind.SetMaxCBORBodyBytes(n)` / `httpbind.MaxCBORBodyBytes()`, honoured by
both transport runtimes. Present only in builds generated with CBOR enabled;
see the optional-CBOR-bodies section of [the httpbind guide](httpbind.md).

### multipart/form-data — 1 MiB

`httpbind.SetMaxMultipartBodyBytes(n)` / `httpbind.MaxMultipartBodyBytes()`.

One value bounds three things: the whole body, each file part read out of it,
and — as a ceiling, never a floor — how much of the form is held in RAM before
parts spill to temp files. That last one is `DefaultMultipartMaxMemory`, 32 MiB,
and the body limit caps it, so at the default nothing spills at all.

Content-Length is checked first when the client sent one. On net/http the body
is then wrapped so a missing or wrong Content-Length cannot get past the cap; on
fasthttp the server's own `MaxRequestBodySize` bounded the read before the
handler was reached.

A body over the limit and a single file part over it are both 413. An
unreadable part is a 400 naming the field.

### application/x-www-form-urlencoded — not bounded here

`SetMaxMultipartBodyBytes` does not reach it. The urlencoded body is parsed by
the transport, so what bounds it is the transport's own limit: 10 MB on
net/http, `MaxRequestBodySize` on fasthttp. If a small multipart limit is part
of your threat model, set the transport limit as well — otherwise the same
payload arrives an order of magnitude larger by changing one header.

## Shape limits

A field declared as a fixed-length Go array bounds what may decode into it.

```go
type Board struct {
	Cells [9]string `json:"cells"`
}
```

A short array fills what arrived and leaves the rest at the zero value: a length
the Go type states is not a length the document has to restate. A long one is an
error carrying `jsonbind.ErrArrayTooLong`, because storing the first `N` and
dropping the tail is a decoder silently losing data — the failure a declared
length exists to prevent. A `[N]byte` field decoded from base64 follows the same
two-ended rule.

Neither the number of query parameters nor the number of form fields is bounded
here; both are bounded by the body and URL limits the transport enforces.

## WebSocket messages — 1 MiB

`SocketOptions.ReadLimit`, per process with `httpbind.SetSocketDefaults` or per
endpoint with `httpbind.WebSocketWith`. A peer that sends more has its
connection closed rather than its message truncated.

It sits beside the lifecycle bounds — `IdleTimeout`, `PingInterval`,
`WriteTimeout` — which are documented with the socket surface in
[the WebSocket section of the httpbind guide](httpbind.md#websockets). A zero
field takes the process default, and a process default left zero takes the
constant, so nothing reaches the driver unbounded.

## What the transport owns

These are not this module's to set, and they sit underneath everything above.
A limit here that is looser than the ones above is not a hole, but it is the
number that decides how many bytes the process has already touched by the time
a binder gets to refuse them.

| | net/http | fasthttp |
|---|---|---|
| whole request body | unbounded unless you wrap it | `Server.MaxRequestBodySize`, 4 MiB |
| urlencoded form body | 10 MB, fixed | `Server.MaxRequestBodySize` |
| request headers | `Server.MaxHeaderBytes`, 1 MB | `Server.ReadBufferSize`, 4 KiB |
| request line and URL | `Server.MaxHeaderBytes` | `Server.ReadBufferSize` |

## Related

- [httpbind guide](httpbind.md) — the body types these limits apply to
- [jsonbind guide](jsonbind.md) — JSON outside HTTP
- [fasthttp backend](httpbind_fasthttp.md) — what differs on the other runtime

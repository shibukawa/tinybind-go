# The update wire contract

This is the normative description of what `htmlupdate` puts on the wire and what a browser client must do with it. It exists so that a client can be written against a specification rather than against `htmlupdate/runtime.js`.

The browser half of this design belongs to the caller. That is only true if the caller can see the protocol, so everything a client needs is here; the bundled `runtime.js` is one conforming implementation and holds no privileged knowledge.

**Conformance language.** *Must* is required for correctness. *Must not* marks the cases where getting it wrong produces silent damage rather than an error. *May* is genuinely optional.

## What this contract does not carry

**There is no protocol version.** The server defines none, sends none, and compares none.

A version number is a way for two parties to detect that they disagree. Here both parties are the caller: the caller writes the client, deploys it, and deploys the server that answers it. A version owned by this module would version a contract only half of which lives in it, and the coordinated release it forces is exactly what putting the client with the caller was meant to avoid.

The mode token still accepts a `;v=N` suffix. It is *your* field: the server parses it, carries it, echoes it back on the response, and never judges it. Use it if you version your own wire; omit it and every token is a bare mode name.

What the server does compare is the **build identity**, described below. It is strictly stronger: it changes when a template, a Go function a template calls, the browser client, or any dependency changes.

## Header namespace

Every header derives from one configured prefix, default `X-Tinybind`. Written `<P>` below.

| Header | Direction | Meaning |
| --- | --- | --- |
| `<P>-Render` | request and response | The mode. See [Modes](#modes). |
| `<P>-Build` | request and response | The build identity. See [Build identity](#build-identity). |
| `<P>-Manifest` | request | Validators the client already holds. See [Manifest](#manifest). |
| `<P>-Kind` | request | The component a redraw addresses. |
| `<P>-Instance` | request | The instance a redraw addresses. |
| `<P>-Live` | response | Present with the value `1` when the composition owns a live boundary. |
| `<P>-Head` | response | A redraw's head contribution. See [Redraw response](#redraw-response). |

The CSRF header is **not** derived from this prefix. It defaults to `X-CSRF-Token`, because that names a convention every framework's middleware already recognises rather than anything this protocol owns.

A client must read the prefix from its configuration rather than compiling these names in. A deployment that changes the prefix and a client that did not is a page where nothing works and nothing reports why.

## Modes

The `<P>-Render` value is:

```text
token   = mode [ ";" "v=" 1*DIGIT ]
```

Whitespace around the mode and around the version is ignored. A version that is absent, empty, or unparseable is treated as absent; it is never a reason to refuse a request.

**Request modes** — a client sends one of these:

| Mode | Method | Meaning |
| --- | --- | --- |
| `navigation` | `GET` or `HEAD` | Return the changed boundaries of the target route. |
| `live` | `GET` or `HEAD` | Return the deliveries of this route's live boundaries, held open. |
| `redraw` | `GET` or `HEAD` | Return one registered component's subtree. |
| `action` | any | Return the regions a mutating request changed. |

`action` is negotiated separately from the other three, because it is the only one that is not side-effect free: it carries ambient credentials and therefore requires the CSRF token, and it is the only mode a non-`GET` may use.

**Any other token, and an absent header, resolves to a complete HTML document.** This is a total function on the mode name: a stale client, a truncated header, a proxy that dropped one, and a mode a future version introduces all produce a working page rather than an error. Nothing in this protocol answers an unrecognised request with a failure status.

A response echoes the mode it served in `<P>-Render`, carrying back whatever version the request claimed. A client **must** check it: a shared cache or a proxy may have answered a delta request with the document body, and applying that as a delta is how a page fills with markup that means nothing. Compare the mode; ignore the version unless it is yours.

## Build identity

`<P>-Build` carries the identity of the binary that rendered the page. The client reads it from its configuration — the module's own script tag carries it in `data-config` — and sends it on every update request.

The server compares it for exact equality. **On a mismatch the server answers as though the request had no render header at all**: a complete document for `navigation` and `live`, a fall-through to the caller's page handler for `redraw`, and a refusal for `action`. There is no dedicated stale-page status — at the URL a redraw is served from, the page itself is the right answer and it costs one round trip instead of two.

The client's obligation is the same in every case: **fall back to an ordinary full navigation.** The page then reloads and arrives holding the new client, which is the mechanism that makes deploying the two halves independently safe.

A streamed response repeats the build in its opening record, so a client that consumes records without inspecting response headers can still detect a server that was redeployed under an open connection.

## Manifest

A navigation request may carry the validators it already holds:

```text
<P>-Manifest: <instanceId>:<validator>,<instanceId>:<validator>,…
```

Both halves are opaque tokens: base64url, no padding, no character needing escaping. Pairs are comma-separated. A malformed pair is skipped rather than failing the request.

**Oversize rule.** A value longer than the configured bound (default 8192 bytes) is *dropped, not rejected*: the server answers as though the client sent nothing, which costs a larger delta rather than a failed request. A client must therefore never assume a delta is minimal.

A validator is a hint and nothing else. The server never derives arguments, identity, or access decisions from one.

## Delta records

`navigation` and `live` may be answered either buffered or streamed. A client must handle both, deciding on the response `Content-Type`.

### Buffered

`Content-Type: application/json; charset=utf-8`, and a single object:

```json
{
  "ops": [ { "kind": "replace", "id": "c2", "html": "<p …>" } ],
  "manifest": [ { "id": "c1", "frame": "8Qv…" } ],
  "head": ["<link …>"],
  "navigate": "/orders/17",
  "live": true
}
```

Every field except `ops` is optional and absent when empty. There is no version field.

### Streamed

`Content-Type: application/x-ndjson; charset=utf-8`, one JSON object per line. The `r` field names the record kind.

| `r` | Fields | Meaning |
| --- | --- | --- |
| `head` | `head`, `build` | Opens the stream. Always first. |
| `op` | `kind`, `id`, `html`, `frame` | One boundary. `html` absent means unchanged: record the validator, apply nothing. |
| `await` | `id`, `html` | An async boundary that settled. `id` addresses a *placeholder*, not an instance. |
| `end` | `reason`, `error`, `retryMs` | The terminator. |

`await` and `op` address different namespaces. A placeholder id names a hole inside a region the client already installed; an instance id names a boundary. A client that looks one up in the other's namespace finds nothing and silently drops the update.

**Operation kinds.** Currently only `replace`. A client **must** ignore an operation kind it does not recognise rather than guessing, and must not treat one as a reason to abandon the stream.

### The terminator

A stream that ends without an `end` record is **truncated**. A client must treat it as a failure: discard the manifest it accumulated during that stream and fall back. Trusting a partial manifest means telling the server it holds regions it never received, and the server will then omit them forever.

| `reason` | Meaning | Client action |
| --- | --- | --- |
| `final` | The route is fully described. | Stop. Open no live connection. |
| `live_pending` | The route owns live boundaries. | Open a live connection. |
| `failed` | Ended on an unrecovered failure; `error` says what. | Stop. Content already applied stays. |
| `done` | A live stream whose every source finished. | Stop. Do not reconnect. |
| `retry` | A live stream the server closed healthy: a lifetime bound, a shutdown, a rebalance. | Reconnect promptly. |

`retry` **must not** spend a backoff attempt. Nothing failed, and treating a healthy rollover as a fault stalls a working screen every time the server rotates a connection. `retryMs`, when present, is the server's own hint; it is the only party that knows it is shedding load.

## Head operations

`head` is an array of ready-to-write tag strings, in the buffered body and in the stream's opening record.

**Ordering is normative: head must be installed before the markup that needs it.** A delta reuses the live document shell, so a component reachable for the first time has no stylesheet on the page; markup landing first paints unstyled.

A client must deduplicate against what the document already has, and must treat `<title>` as a singleton that replaces rather than accumulates — otherwise history entries and assistive technology see the old page.

## Redraw

### Request

```text
GET <any url you serve it from>?<declared parameters>
<P>-Render: redraw
<P>-Kind: <kind>
<P>-Instance: <instance id>
<P>-Build: <build>
```

The kind and the instance are headers so the URL is the caller's. A redraw served at the page's own URL inherits that page's authorization; on a reserved path it needs a second path pattern kept in step with the first, and nothing forces two such rules to agree. This is the only redraw addressing: there is no reserved path form to fall back to.

The client reads the kind from the target element's `data-<attr>-kind` attribute rather than being told it. Every render of a reloadable component emits it, including a redraw's own replacement — without that a region would be redrawable exactly once.

Parameters go in the query string, and the generated decoder is strict: an unknown name, an undecodable value, a repeated one, or a missing one is an error rather than a zero value. This is why the kind and instance are not query parameters — they would reserve two names an author could then not declare.

### Redraw response

The body is the shape every other update path returns: `ops`, `head`, and `manifest`, as `application/json`. The region travels as one `replace` operation naming the instance, with a hole where each nested boundary sits; each of those arrives as its own operation in the same response.

It was a bare HTML fragment with its head in a header, which made the redraw the one response in this package with a form of its own — and left it nowhere to return the validator its own replacement had just made stale. The manifest entry is what closes that: a client stores the returned validator rather than dropping the one it held.

- `Content-Type: application/json; charset=utf-8`
- The head is the `head` field of the body, a JSON array of ready-to-write tags, absent when the component contributes nothing. It used to travel as base64 of JSON in a `<P>-Head` header, bounded at registration so a proxy could not drop an oversized one; a field in a body needs neither the packing nor the bound, and both are gone.
- `ETag` over the rendered bytes. Matching it against `If-None-Match` — list form and `W/` prefix included — and answering `304` is the server author's, since a `304` is a cache decision and this module makes none. `Response.NotModified` does the comparison so the decision is the only part left to make.
- `Vary` includes the render and build headers, and — when the redraw is served at a URL that also serves a page — the kind and instance headers. Without those two, two components redrawing on one page are one cache entry and either may be answered with the other's markup.
- No `Cache-Control`. `private, no-cache` is the sensible one for a per-user region that still wants its `ETag` revalidated, and a server author sets it.

**Failure statuses:**

| Status | Meaning | Client action |
| --- | --- | --- |
| `400` | The query was refused by the decoder, or no component was named. | Fall back. |
| `404` | This deployment publishes no such kind — usually a page loaded before a deploy. | Fall back: the *page* is stale, not the region. |
| `414` | The query is past the configured bound. | Fall back. |
| `500` | The component could not render. | Fall back. |

## Action response

`action` returns the same buffered body shape as a navigation: `ops`, and `head` for a region the document never carried — a validation summary, a panel that was not there before.

It carries **no** `manifest`: an action rewrites regions by target id and must leave navigation state alone.

**The status is not a signal to skip applying.** A rejected submission returns `4xx` and the regions it carries *are* the validation errors. A client that applies only on `2xx` throws away exactly the output the user needs to see.

`navigate` in the body means the action changed where the user belongs; leave the page rather than guessing which regions to rewrite.

## Client obligations

A conforming client must not break these, whatever else it does.

1. **Trigger a streaming swap from the commit marker, never from the template.** A template's start tag can arrive in its own chunk; an observer watching the template will swap in empty content and destroy the fallback. The marker element is written after the template's closing tag and therefore cannot appear before the template is complete. Any API is fine — the marker's connected callback, a mutation observer — as long as the *trigger source* is the marker. This failure only appears once a proxy, a TLS record, or a compressing encoder splits the bytes, so it survives development untouched.

2. **Apply each record at most once.** A superseded request must not overwrite newer state: track a ticket per target and drop a response whose ticket is stale.

3. **Install head before the markup that needs it.**

4. **Fall back to an ordinary full navigation on every failure path.** A non-2xx, a served mode that is not what was asked for, a body that does not parse, a missing target, a build mismatch, a truncated stream, an aborted request. This is the invariant the whole design rests on: it is what lets every other part be deliberately incomplete without being incorrect.

5. **Never treat an unrecognised operation kind, record kind, or terminator reason as fatal.** Ignore what you do not know and keep the fallback available.

6. **Send the CSRF token on every `action` request**, in the configured header. `navigation`, `live`, and `redraw` are side-effect-free `GET`s and need none.

## Checking an implementation

`htmlupdate/testdata/runtime_harness.js` drives a client against a stubbed DOM under node, covering header construction, validator bookkeeping, supersession, head installation, the terminator reasons, and the fallback paths. It tests observable wire behaviour — the requests a client issues, the responses it consumes, the resulting DOM — rather than any JavaScript entry surface, so it is not a second contract in another language.

If you implement this specification and find it under-determined anywhere, that is a defect in this document.

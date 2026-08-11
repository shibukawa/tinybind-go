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
| `<P>-Sequences` | request | Present when the client can walk a sequence tree. See [Sequences](#sequences-sending-values-instead-of-markup). |
| `<P>-Sequence-Address` | request | Which sequence tree to serve. |

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
| `sequence` | `GET` or `HEAD` | Return one fragment's static half, named by `<P>-Sequence-Address`. |

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
<P>-Manifest: <instanceId>:<frame>[:<children>[:<parent>]],…
```

Every field is an opaque token: base64url, no padding, no character needing escaping. Entries are comma-separated, fields within an entry colon-separated. A malformed entry is skipped rather than failing the request.

`frame` digests the boundary's own bytes; `children` digests the ids of its nested boundaries, in order; `parent` names the boundary enclosing it. The last two are omitted when empty, so a leaf boundary with no parent is still `id:frame`. A client that stores only the frame makes every list look reordered on the request after next, and makes a removal fall back to replacing the outermost boundary.

**Oversize rule.** A value longer than the configured bound (default 8192 bytes) is *dropped, not rejected*: the server answers as though the client sent nothing, which costs a larger delta rather than a failed request. A client must therefore never assume a delta is minimal.

A validator is a hint and nothing else. The server never derives arguments, identity, or access decisions from one.

## Delta records

`navigation` and `live` may be answered either buffered or streamed. A client must handle both, deciding on the response `Content-Type`.

### Buffered

`Content-Type: application/json; charset=utf-8`, and a single object:

```json
{
  "ops": [ { "kind": "replace", "id": "c2", "html": "<p …>", "boundaries": ["c3"] } ],
  "manifest": [ { "id": "c1", "frame": "8Qv…", "children": "R1p…", "parent": "c0" } ],
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
| `op` | `kind`, `id`, `html` or `seq`+`values`, `boundaries`, `frame`, `children`, `parent` | One boundary. No `kind` and no markup means unchanged: record the validators, apply nothing. |
| `await` | `id`, `html` | An async boundary that settled. `id` addresses an *await marker*, not an instance. |
| `signal` | `name`, `data` | An instruction a live source emitted. Addresses no region. See [Signals](#signals). |
| `end` | `reason`, `error`, `retryMs` | The terminator. |

`await` and `op` address different namespaces. An await id names a marker pair inside a region the client already installed; an instance id names a boundary. A client that looks one up in the other's namespace finds nothing and silently drops the update.

An await boundary is bracketed in the document by a comment pair, `<!--<prefix>:<id>-->` and `<!--/<prefix>:<id>-->`, with the committed fallback between them. Settling replaces that range. Comments rather than an element because the fallback has to be visible — so it cannot be inside a `template` — and has to stay where it was written, which an unknown element in a table does not.

**Operation kinds.** `replace` and `children`. A client **must** ignore an operation kind it does not recognise rather than guessing, and must not treat one as a reason to abandon the stream.

**Dispatch on `kind`, not on whether `html` is present.** A `children` operation carries no markup at all, and a `replace` may carry `seq`+`values` instead. Every stream path in the reference implementation got this wrong once, and the result was a list emptied rather than reordered.

**Holes.** A `replace` fragment stops at its nested boundaries, leaving one `<template>` per nested boundary carrying that boundary's instance attribute, and `boundaries` names them in order. A hole whose id also carries an operation in this response is filled from it; one that does not is a region the client already holds and moves its live node into. Nothing in the markup distinguishes the two, which is what the list is for.

The hole is a `template` because that is the one element the parser keeps where it was written and never renders. A client **must** parse a fragment inside a `template` element for the same reason: a `<tr>` parsed anywhere else loses its tags, and an unknown element inside a table is moved out to just before it.

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

## Signals

A live source can say *something happened* as well as *this region now shows X*. It yields a signal in the error position of its sequence; the runtime classifies it ahead of every failure path and forwards it, and the stream carries one `signal` record:

```json
{"r":"signal","name":"app.toast","data":{"text":"saved"}}
```

`name` is a dispatch key. `data` is the payload the source encoded, or absent when the signal carries none.

A signal **addresses no region**. It carries no `id`, no `frame`, no `children`, no `parent`, and no revision, because nothing on screen is being replaced. It is dispatched, not applied.

**A client resolves `name` against a table it registered while the page loaded, and against nothing else.** Not `eval`, not `new Function`, not `import()`, not a lookup of a global by that name, not an attribute handler it writes. This is the whole point of the record: the set of things the server can ask for is fixed at build time and is exactly what the table holds, so a page keeps a `script-src` with neither `unsafe-eval` nor `unsafe-inline` and is still directed. The server varies the payload, never the instruction.

| Situation | Client action |
| --- | --- |
| `name` is registered | Call it once, with the parsed payload. |
| `name` is not registered | **Ignore it** and keep reading. A server ahead of its client is ordinary; a screen that stops over an instruction it does not understand is worse than one that misses it. |
| The handler throws | Catch, report through your own diagnostics, keep applying. A bug in a toast handler must not stop deliveries from landing. |
| The record is malformed | Drop it and keep reading. A signal carries no revision, so skipping one desynchronizes nothing. |
| The request was aborted | Never dispatch, even if the bytes arrived. |

**Names beginning `tb.` are reserved.** A client may register a handler for one; an application may never emit one, and the server rejects an attempt at the source. Reserved names are for notices the client's own runtime produces about itself, and a handler trusts one precisely because application data could not have forged it.

**Reservation is layered.** Each layer that produces signals of its own reserves a prefix and guards it in the constructor it exports: this module holds `tb.`, a framework built on it holds its own — `pw.`, say — and an application uses what neither has taken. The module does not hold anyone else's prefix, because `NewSignal` is called at a yield site inside a source and is not render-scoped, so it can reach no configured value; a layer that owns a namespace owns the one wrapper that guards it. Dispatch is indifferent to all of this: a name is resolved by byte-for-byte lookup, so a further namespace needs no client change. A prefix constrains who may **emit**, never how a name **resolves**.

Registration happens before the live request is issued. Signals are **best-effort**: they are not queued, not replayed on reconnect, and never acknowledged. An instruction that must be seen exactly once does not belong on this channel.

The payload is **data**, not markup and not code. A handler that assigns it to `innerHTML`, passes it to a DOM sink that parses markup, or builds a selector or URL from it without escaping reopens the injection this record closed.

### Lifecycle signals — a reference vocabulary

Everything above is a signal the *server* sent. A client also knows things the server cannot see: that a completion is now in the DOM, that a live connection opened, that a document was cut off. Today an application learns those by observing the DOM or patching the runtime.

The names below are a **reference vocabulary** for dispatching them through the same table, so an application registers once and does not care which side noticed. They are *suffixes*: a client dispatches them under its own reserved prefix — `pw.boundary_settled`, and so on — and reuses the suffix verbatim so one moment reads the same across implementations. `tb.` is reserved for them, and nothing in this module emits one.

Implementing this set requires **no server code**. Every fact below is already on the wire.

| Suffix | Fires | Carries |
| --- | --- | --- |
| `document_committed` | The document terminator was read. | `reason`: `final`, `live_pending`, or `failed`. |
| `document_truncated` | Parsing finished with **no** terminator. | — |
| `boundary_settled` | An `await` completion is in the DOM. | The await marker `id`. |
| `live_opened` | The live stream began yielding. | Whether this was a first subscribe or a reconnect. |
| `live_closed` | The live stream ended, by any of the three routes. | `reason`, and `retryMs` when the server sent one. |
| `delivery_applied` | A live delivery's operations are in the DOM. | The instance `id` and its `frame` validator. |
| `navigation_applied` | A navigation delta is applied. | The URL now displayed. |
| `directive_received` | A `navigate` or `reload` directive arrived. | Which one, and the target for a navigate. |

`document_committed` covers all three document-side terminator reasons rather than splitting into a name per outcome; `document_truncated` is the separate case because a terminator saying `failed` is a response that ended and said so, while no terminator is a response that was cut.

`delivery_applied` carries the frame validator and **not** a revision: no revision exists on this wire, and specifying one would make this the single name that needs server work. A handler wants to know which region changed; ordering is the apply layer's problem, not the handler's.

**Fire after applying, never before.** The use is a handler that reads or decorates what just arrived, and firing first hands it the previous DOM. `document_truncated` is the exception, describing an absence with nothing to follow.

Dispatch synchronously, in record order. A signal a source emitted before a delivery then fires before that delivery's `delivery_applied`, which is the only thing that makes "highlight what just arrived" expressible.

## Sequences: sending values instead of markup

A fragment's static half — its literal text — is identical in every render, so it can travel once per client instead of once per render. A client that can walk one says so:

```text
<P>-Sequences: 1
```

An operation then carries an address and the values that fill it **instead of** `html`, wherever that is smaller:

```json
{"kind":"replace","id":"panel","seq":"Yb3_x…","values":["Inbox","30","data-tb-id","r0"],"boundaries":["r0"]}
```

The choice is per operation and per response. A fragment of two elements costs more as an address plus its values than as markup, so an operation carries one half or the other and never both. A client must handle either on every operation.

The tree behind an address is fetched separately, in `sequence` mode, at a URL the server author chooses:

```text
<P>-Render: sequence
<P>-Sequence-Address: Yb3_x…
```

The answer is `application/json` and echoes `<P>-Render: sequence`. An address this server has never rendered is `404`, and the client asks for markup instead — a sequence is an optimisation over something still available, never a thing a screen depends on. A request naming no address is `400`.

A sequence derives from the template rather than from the request, which makes it the one response on this wire that is not per user. It is addressed by a digest of its own content, so it survives a build change and a template edit produces a new address rather than a new body at the old one.

**Walking the tree.** Consume one value per hole, one per conditional (which branch ran), one per loop (how many times), and one per component call (whether it opened a boundary or rendered inline). Concatenate and parse once.

**Escaping never leaves the server.** Values arrive already escaped, exactly as they would have been written into HTML. Concatenate and parse; apply no escaper and judge no value. The URL scheme allowlist in particular stays on the server.

## Head operations

`head` is an array of ready-to-write tag strings, in the buffered body and in the stream's opening record.

**Ordering is normative: head must be installed before the markup that needs it.** A delta reuses the live document shell, so a component reachable for the first time has no stylesheet on the page; markup landing first paints unstyled.

A client must deduplicate against what the document already has, and must treat `<title>` as a singleton that replaces rather than accumulates — otherwise history entries and assistive technology see the old page.

**The head is only ever added to.** Nothing here retires a tag, which is why a script installed through it has already evaluated and owns whatever it registered for the life of the document. A script that must be released when a region leaves is the next section.

## Scoped scripts

A component can declare a script of its own, beside its head block:

```text
<script component>
  export function setup(el) { … }
</script>
```

It is extracted to a content-hashed file like any other, and its head reference is an ordinary `type="module"` tag. What is different is that the module reports **who owns it**. Each asset carries a scope:

```go
htmlbind.Asset{ID: "…", Type: "text/javascript", URL: "/public/generated/…", Scope: "pages.counter.Counter"}
```

An **empty** `Scope` is document lifetime — the file evaluates once and is never released, which is what a head contribution has always been. A **named** one is the package-qualified identity of the component that declared the block. It is the same string `ComponentID` carries; the short declared name is used nowhere, because two `Counter` components in two directories are one name and two declarations.

**Every rendered instance of that component is marked with it:**

```html
<li data-tb-component="pages.counter.Counter">…</li>
```

Match `Scope` against `data-<P>-component` to find the elements an asset belongs to. The marker is static markup, not an instruction, which is what makes it dependable:

- it lands on an **ordinary component call**, which opens no update boundary and therefore carries no `data-<P>-id` and appears in no manifest — and a component rendered many times inside a page is exactly that call;
- it lands on a **first load**, which holds no manifest at all, the manifest being a header the client sends back;
- it is in the body, so it compresses, and it is not subject to the manifest's oversize rule.

**It names a declaration, not an instance.** Two `Counter` elements carry the same marker and nothing on the wire tells them apart. That is still enough to run a lifecycle, because both questions a lifecycle asks are answered locally:

- *What do I start?* Every element carrying a marker that matches an asset's `Scope`.
- *What do I release?* Whatever sits inside the region you are about to replace. A `replace` fragment stops at its nested boundaries, so at the moment an operation for instance `X` arrives, the subtree under `X` is still the one you mounted against: scan it for markers, run their teardowns, and only then apply the markup. That is what satisfies the release-before-the-markup-lands rule above, and it asks nothing of the server — which elements are about to be destroyed is something only the client can know, since it owns the apply loop.

A `children` operation carries no markup and moves live nodes, so anything mounted inside one survives untouched. That is correct rather than a gap: the element did not go away.

What the marker does **not** give you is a way for the *server* to address one instance — a redraw endpoint for a single `Counter`, say. That needs the component to be an update boundary, which an ordinary call does not become, and it is a different feature from the lifecycle.

This module publishes the owner and **calls nothing**. What a scoped script exports, when it is started, and when it is released are the client's; the rules below are what a client must not break, not an API this module specifies.

**Run per live instance, not once per document.** The asset set is deliberately conservative — it reports what a composition *could* require, including a component below a slot that never rendered — so it is a catalogue, not a mount list. An instance that did not render has no attribute and no manifest entry, and nothing should start for it.

**Release before the incoming markup lands.** When a delta removes or replaces an instance, whatever that instance's script registered is released first. Doing it afterwards means the teardown runs against DOM that is already gone.

**Diff the chain across a navigation; do not tear it down.** A common prefix of the composition chain stays mounted. Only the tail below the divergence is released, innermost first, and the new tail started, outermost first. The server reads composition order from `MergeAssets`, which is outermost first, and each layer's own set from `Assets()` on that `Wrapper` or `Fragment`. Build the chain from the per-layer sets: the merged one is flat and cannot tell you where a layer ends.

**A throwing start or release must not stop the apply loop.** Catch it, report through your own diagnostics, and keep applying — the same rule a signal handler follows.

Do not rebuild this on top of the head: re-adding a `<script type="module">` tag does not re-evaluate it, because a module is keyed by its resolved URL in a per-document module map. That map is also why code shared by two scoped scripts should stay an ordinary import of one URL. It is fetched and evaluated once, so bundling is what would duplicate it.

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
- `ETag` over the rendered bytes. It is the one header a server author cannot compute without rendering the component twice, which is why this module still produces it. Matching it against `If-None-Match` — list form and `W/` prefix included — and answering `304` is the server author's, since a `304` is a cache decision and this module makes none. `Response.NotModified` does the comparison so the decision is the only part left to make.
- `Vary` includes the render and build headers, and — when the redraw is served at a URL that also serves a page — the kind and instance headers. Without those two, two components redrawing on one page are one cache entry and either may be answered with the other's markup.
- No `Cache-Control`. `private, no-cache` is the sensible one for a per-user region that still wants its `ETag` revalidated, and a server author sets it.

**Failure statuses:**

| Status | Meaning | Client action |
| --- | --- | --- |
| `400` | The query was refused by the decoder, or no component was named. | Fall back. |
| `404` | This deployment publishes no such kind — usually a page loaded before a deploy. | Fall back: the *page* is stale, not the region. |
| `414` | The query is past the configured bound. | Fall back. |
| `500` | The component could not render. | Fall back. |

A refusal carries the same `Vary` axes an answer would have. Without them a `404` — heuristically cacheable with no `Cache-Control` — can be stored and served to a request for the page at the same URL.

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

7. **Resolve a signal name against your own registration table and nothing else.** No `eval`, no `new Function`, no `import()`, no global lookup by name. A dynamic fallback for an unregistered name is the code execution the record exists to avoid, reached by another route.

8. **Release a scoped script's registrations when its instance goes.** A script whose asset names an owner is bound to that component's live instances; if a delta removes or replaces one and nothing is released, every listener, observer, and timer it installed survives and the next instance adds its own on top. The symptom is a handler firing twice, which still looks like it works.

## Checking an implementation

`htmlupdate/testdata/runtime_harness.js` drives a client against a stubbed DOM under node, covering header construction, validator bookkeeping, supersession, head installation, the terminator reasons, and the fallback paths. It tests observable wire behaviour — the requests a client issues, the responses it consumes, the resulting DOM — rather than any JavaScript entry surface, so it is not a second contract in another language.

Two parts of this document have no wire form and so no harness coverage: the lifecycle vocabulary and the scoped-script rules. Both are dispatched by the client about itself, and both are specified here because a second implementation cannot infer them from the bytes. Checking them means testing your own runtime.

If you implement this specification and find it under-determined anywhere, that is a defect in this document.

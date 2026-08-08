# Reloadable Components User Guide

A reloadable component is an `htmlbind` component whose rendered region the browser can refresh on its own. When the page's inputs change, the server re-runs the render, compares each region against what the browser already holds, and sends back only the regions whose markup actually changed.

One URL answers in three ways. Without an update header it serves the ordinary complete document, so a browser without the runtime, a crawler, and `curl` all see exactly what they saw before. With one, the same URL serves a delta for a navigation, or a re-render of a single component.

This is the same idea as a page component that takes search parameters as arguments: the page is a function of the request, and changing one argument should not cost a full page load.

> [!IMPORTANT]
> This feature ships in milestones. Everything below describes the finished behavior; see "[Availability](#availability)" for what works today. Nothing here changes how existing templates render: update support activates only for requests that ask for it.

## What is automated

- Marking each layout and page in a composition as an update boundary
- Emitting a stable instance identity on each boundary's root element
- Computing a canonical digest of a component's declared inputs
- Computing a digest of a boundary's own rendered markup, excluding its children
- Comparing a fresh render against the digests the browser sent back
- Selecting the outermost changed boundaries and sending only those
- Negotiating document and navigation rendering from one request header
- Publishing a registered component as a redraw endpoint
- Serving the browser runtime that applies the result
- Falling back to an ordinary navigation whenever anything is unrecognized

You do not compute hashes, assign IDs, or write update endpoints. Your handler renders the same chain it always did.

## What you provide

1. A composition of `.tb.html` components: a document shell, layouts, and a page
2. A handler that renders that chain through `htmlupdate`
3. A validator key
4. A route serving the browser runtime, and a script tag loading it
5. Browser code that asks for an update, or a link the runtime intercepts
6. A DOM `id` on each instance you want to redraw, and an authorization check inside it

## How a delta happens

```text
1st request   GET /search?q=go
              -> complete HTML, each boundary carrying data-tb-id

update        GET /search?q=rust
              X-Tinybind-Render: navigation
              X-Tinybind-Build: 3f9c2a…
              X-Tinybind-Manifest: c1:8Qv…,c2:Lm4…
              -> { ops: [ replace c2 with <p …>results for rust</p> ],
                   manifest: [ c1:8Qv…, c2:Nk9… ] }

apply         the runtime swaps that one element and stores the new digests
```

The layout markup never travels the second time. Its digest is unchanged, so the server knows the browser already has it.

## Update boundaries

Every layout and page in a rendered chain is a boundary automatically. Nothing is declared, because a layout chain is exactly the structure a partial update wants: an outer frame that usually survives, wrapping an inner region that usually changes.

An ordinary component call is **not** a boundary. Without that rule a list of five hundred rows would put five hundred entries in every request. Marking an arbitrary component as a boundary is an explicit opt-in, and only an exported component is eligible: becoming a boundary publishes an identity into the DOM and into the protocol, so a file-private implementation detail must not be addressable from outside.

The document shell is never a boundary. A partial update reuses the existing document, so replacing the shell would defeat the purpose.

### The single root element rule

A boundary must render exactly one root element, because its identity lives in an attribute on that element and every operation targets that node.

This does not mean adding a wrapper. A table-row component whose root is `<tr>` is fine; a component returning two sibling `<tr>` elements is not. A doctype, a comment, whitespace, and a hoisted `head` declaration are ignored when counting.

While boundaries are automatic, a component that cannot satisfy the rule is simply not a boundary and still compiles. Once boundaries become explicit, an opt-in that cannot be satisfied becomes a generation error.

## Two digests, two jobs

A boundary carries two digests, and the difference matters.

The **input validator** covers the component's declared parameters, canonically encoded and tagged with the component's generated version. It is a cache key and a diagnostic aid.

The **frame validator** covers the boundary's own rendered bytes, excluding the output of nested boundaries. It is the authority for omitting a boundary.

The reason for two is that equal inputs do not prove equal output. A component may read the clock, a database, a locale, or the request. Only a component that explicitly opts into output caching may skip its own render on an input match; every other boundary is rendered and then compared. **A delta skips transmission, never execution.**

Excluding children from the frame is what makes a layout reusable: change a page parameter and the layout's own markup hashes identically, so the layout stays in the DOM while its child is replaced.

Digests are keyed with a secret you supply. An unkeyed hash of low-entropy content would let anyone confirm a guess by comparing digests. Rotating the key does not break anything; it makes comparisons miss, so the next response is a complete render.

## Rendering modes

| Mode | Trigger | Response |
| --- | --- | --- |
| Document | No render header | Complete HTML |
| Navigation | `X-Tinybind-Render: navigation` | Changed boundaries of the target route |
| Redraw | `X-Tinybind-Render: redraw` plus `X-Tinybind-Kind` and `X-Tinybind-Instance` | That one component's subtree |
| Live delivery | `X-Tinybind-Render: live` on a second connection | A record stream of deliveries, held open |

Every mode is a header, so the URL is never part of the negotiation. That includes redraw, which used to be addressed by a reserved path; see [Addressing a redraw](#addressing-a-redraw).

The module writes no protocol version and compares none. A token may carry a `;v=N` your client adds — the server parses it, echoes it back, and never judges it — because the browser half belongs to you and so does its wire version. The compatibility axis the module does operate is `X-Tinybind-Build`: it changes when a template, a Go function a template calls, the browser client, or a dependency changes, which is everything a version could have caught and more. A build mismatch answers with a complete document, and the page reloads holding the new client.

The last row belongs to a different feature and is listed here because it shares this negotiation. A [live source](htmlbind.md#live-sources) settles in place during the document render, so the document response finishes; the runtime then opens a second connection that carries deliveries for as long as the sources produce. When that connection drops — a proxy timeout, a sleeping laptop — the regions would freeze with no signal, so the runtime reopens it. Reconnecting is the same request again.

Live is its own token rather than a navigation held open, because the two differ in exactly the ways a deployment has to act on. A navigation ends when the route has been described; a live response ends when every source finishes or when the server closes it at a lifetime bound. Sharing a name meant neither could be routed, timed out, or bounded separately, and a served-mode log could not tell an hours-long subscription from ordinary traffic.

The same route answers both. `options.RenderLiveStream` serves a navigation by settling live boundaries in place and finishing, and serves a live request by keeping the subscriptions open — one entry, so the reconnect path and the render path are the same code. `options.RenderStreamAsync` serves only the first; a live request reaching it is answered as a navigation and terminated, so a client learns at once instead of holding a connection that will never deliver.

That resumption is unusually cheap, and for a structural reason: **every live delivery carries the whole state of its region rather than an increment.** A missed value costs nothing, so there is no cursor, no event log, no replay, and no equivalent of `Last-Event-ID`. Boundary ids are allocated by position, so a re-render reproduces the ids already on screen. The reconnected region paints current state immediately instead of showing a placeholder.

A delivery therefore carries no validator. A validator exists so an unchanged boundary can be skipped, and a delivery is a region the server already decided to repaint; the opening delta of a live response still carries them, so a client that skips unchanged boundaries can, and one that ignores them entirely — as a client holding the document it just loaded reasonably does — loses nothing.

### Knowing whether to open one

A live request re-executes the route, its layouts, and its page. A client that cannot tell a live screen from a static one therefore pays a whole page execution per screen that will never deliver anything, so the server says which it is:

- the document and delta responses carry `X-Tinybind-Live: 1`
- a navigation delta body carries `"live": true`
- a streamed navigation terminates with `{"r":"end","reason":"live_pending"}` rather than `"final"`

All three are absent when the composition owns no live boundary, so a page that has none is byte-identical to what it was before any of this existed. In Go the same fact is `htmlbind.HasLiveBlock(wrappers, leaf)`, readable before rendering starts.

### Why a stream ended

A close alone cannot say. The server deliberately closes healthy responses at their lifetime bound, so a client that read every ending as a failure would back a working screen off further on every rotation until it reloaded. The terminator names which it is:

| `reason` | Mode | The client should |
| --- | --- | --- |
| `final` | navigation | Stop. Nothing more is coming. |
| `live_pending` | navigation | Open a live request. |
| `failed` | either | Stop. The screen is known incomplete; recovery is the caller's policy. |
| `done` | live | Stop. Every source finished. |
| `retry` | live | Reconnect promptly, spending no attempt. |

A `retry` record may carry `retryMs`, the server's own hint. A client's backoff can only react to a failure; the server is the only party that knows it is shedding load or rolling a deploy, and so the only one that can spread the return before anything fails. `stream.Retry(d)` writes it.

**Closing a healthy live response is yours to do.** This package never ends one on its own: how long a subscription may live, how many one session may hold, and how long a boundary may sit idle are budgets only your deployment knows. `Retry` is the record that makes your close readable as healthy, so the client comes straight back instead of backing off:

```go
if time.Since(opened) > lifetime {
    return stream.Retry(0)   // or Retry(d) to spread the return yourself
}
```

An unbounded response is not merely long-lived. It also never re-checks authorization, never rolls onto a new deploy, and never rebalances — those are what a bounded lifetime buys, and none of them happen until you set one.

The one ending this package does own is its own: a cancelled request context — a shutdown, a rolling deploy, a deadline you set — closes `retry` rather than `done`. Without that a deploy would tell every open screen its sources had finished, and they would sit frozen until somebody reloaded.

The opening record carries `build`. A stream another build opened describes a document the page is no longer showing, and no number of retries changes that, so the runtime reloads rather than reconnecting.

A stream that ends with no terminator at all is truncation, which is a fault: the runtime backs off exponentially with jitter and falls back to a full page load after a configured number of attempts.

The header is deliberately not `Accept`: shared caches normalize or drop `Vary: Accept`, and one URL must stay one cacheable document resource. It is deliberately not a query parameter either, because that would change canonical, shareable, and logged URLs.

Every response varies on the render header. Delta responses are `no-store`, since they carry per-document validators.

Anything unrecognized — an unknown mode, a truncated header, a proxy that stripped one — resolves to a complete document rather than an error. That rule is what lets each milestone stay incomplete without ever being incorrect.

## Build identity, and the version that isn't there

**This module owns no protocol version.** It defines none, sends none, and compares none. The mode token accepts a `;v=N` you add and echoes it back untouched, because the browser half is yours and so is its wire version; a version owned here would version a contract only half of which lives here, and force the coordinated deploy that separating the halves was meant to avoid.

What the server does compare is `X-Tinybind-Build`, the identity of the binary that rendered the page — the version control revision it was stamped with, or a per-process value from a dirty or unstamped tree so that every development restart invalidates.

It is strictly stronger than a protocol number. It moves when a template changes, when a Go function a template calls changes, when the browser client changes, and when a dependency changes. A component's own kind cannot see any of that: it hashes one component's compiled plan and misses everything around it.

A mismatch serves a complete document rather than an error, which is what makes a rolling deploy safe: a page rendered by the old build whose next request reaches a new server falls back cleanly and comes back holding the new client.

The build identity is also mixed into every validator, so two builds can never produce digests that compare equal even if the header were stripped in transit.

The full wire description — every header, every record, every status, and what a conforming client must not get wrong — is in [The update wire contract](httpbind_update_wire_contract.md). Read that if you are writing your own client.

For how to call every entry — render, redraw, action, sequence — see [The update surface](httpbind_update_surface.md).

## Delta operations

A delta carries the outermost changed boundaries only. A descendant of a replaced boundary is already inside that replacement, so sending it again would target a node that no longer exists.

- `replace` — swap a boundary's element for new markup
- `insert`, `remove`, `move` — structural changes in a keyed list
- `replace` with retain holes — replace an ancestor while moving unchanged descendants into it, so their DOM state survives

The response also carries head operations. A component appearing for the first time brings stylesheet and script links that are not in the live document head, and its markup must not be applied until they are installed; otherwise the region flashes unstyled. Stylesheets that fail to load do not block the update indefinitely.

## Client API

```js
await tinybind.update("/search?q=rust");   // re-render the current route
await tinybind.navigate("/guides/intro");  // move to another route
await tinybind.redraw("card-1", { page: 2 }); // re-render one component
await tinybind.apply(response);            // install what an action returned
```

`tinybind` is the default name, not a fixed one. `GlobalName` renames it, so a framework building on this module gives its users its own name rather than a dependency's. A framework merging this runtime into its own asset calls `createPartialUpdateRuntime(config)` instead and never loads a second script.

Links and GET forms are intercepted for same-origin navigation; put `data-tb-ignore` on an element or an ancestor to return it to the browser. That attribute follows the configured prefix like every other one, so a project using `pw` writes `data-pw-ignore`. A form's fields become the query, so a search form refines the page it is on and replaces the URL rather than stacking a history entry per submit. Non-GET submission is left to the browser, which is what makes post-redirect-get work unchanged.

`subscribe` delivers `start`, `applied`, `superseded`, `fellBack`, and `redrawn`, carrying outcomes rather than component arguments — enough for a progress indicator, an analytics call, or a third-party widget that must reinitialize after its region was replaced. A failing subscriber cannot break the update it is watching. Modified clicks, `target`, `download`, and cross-origin URLs are left to the browser.

History, scroll, and focus behave as they do for a real navigation: the URL is pushed after the response commits so a failed delta leaves history untouched, scroll position is restored on back and forward, and focus is preserved or moved to a documented landmark rather than silently lost.

Every failure path — network error, timeout, a response that is not a delta, a protocol mismatch — performs the ordinary browser navigation to the same URL. A user action is never lost.

Without JavaScript, links and forms simply work as they always have.

## Redrawing one component

A navigation delta answers "what changed on this page?". Redrawing answers a different question: "render this one region again, with these values." It needs none of the machinery above — no manifest, no validators, no comparison.

You declare the component reloadable, give the instance an ordinary DOM `id` at the call site, and call it:

```text
export reloadable component UserCard(id: string, userId: int): html {
<article class="card">…</article>
}
```

```text
<UserCard id="card-1" userId={current.ID} />
```

The declaration generates a typed query decoder and a registration value; the application installs it, so publishing is visible in Go as well as in the template:

```go
registry := &htmlupdate.Registry{}
if err := registry.Register(pages.UserCardReloadable); err != nil {
    return err // a duplicate kind, which must not be discovered in production
}
options.Mount(mux)
```

Registering a repeated kind fails, because the kind covers a component's name, parameters, and markup but not its package: silently keeping the last one would serve a component that looks the same but calls another package's external functions. The failure is returned rather than raised, so a caller running a startup validation pass collects every problem and reports them together; `MustRegister` is there for a caller with nowhere to return one.

`Mount` takes any router with a `Handle(string, http.Handler)` method and registers the runtime asset, which is the only endpoint this package still owns. It takes no registry: a redraw is answered from your handler at your URL.

The `id` parameter is required and is filled from the `X-Tinybind-Instance` header, not the query. The framework writes it and the component's kind onto the root element on every render, so a region stays addressable and redrawable after a redraw replaced it. A reloadable component must be exported and single-rooted, and every other parameter must be a type a query string carries deterministically — a record, a slice, and `html` are refused at generation time. Unlike an automatic boundary these are errors, because the author asked for the endpoint.

```js
await tinybind.redraw("card-1", { userId: 42 });
```

That becomes a plain GET to the page the component sits on:

```text
GET /dashboard?userId=42
X-Tinybind-Render: redraw
X-Tinybind-Kind: UserCard@8Qv3n1
X-Tinybind-Instance: card-1
X-Tinybind-Build: 3f9c2a…
```

The server runs **only that component**, with the arguments from the query string, and returns its subtree as a single root element carrying the same `id`. There is nothing to reconstruct, so there is no capability token, no signing key, and no page execution.

### Addressing a redraw

The component travels in headers, so the URL is yours. Branch on it inside your own page handler:

```go
mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
    if !authorized(r) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    if options.Redraw(w, r, registry) {
        return // it was a redraw, and it inherited the check above
    }
    // ordinary page render
})
```

That placement is the point. Path protection is configured by path pattern, so a redraw served from a reserved path needs its own pattern maintained in parallel with the one protecting the page the component sits on — two rules that must agree and that nothing forces to agree. At the page's own URL the redraw inherits that protection automatically, and placed after the handler's own checks it inherits those too, not merely the middleware's.

`Redraw` returns `false` with nothing written when the request is not a redraw, so the same handler serves the page. A request from a page another build rendered is one of those cases: at a page URL the right answer to a stale redraw is that page, which you are about to render anyway, so it costs one round trip rather than a refusal and then a reload.

`Redraw` and the action entries take `htmlbind.Option`s, and you should pass the same ones your page render gets. Without them a component renders one way inside its page and another in the response that replaces it: a configured URL scheme allowlist does not arrive, so an application's own scheme neutralises to the blocked marker; a cached component runs its body every time; and a component holding an unsafe form **does not render at all**, because the CSRF field needs a token and a failed render is a 500.

```go
render := []htmlbind.Option{
    htmlbind.WithCSRFToken(session.CSRFToken(r)),
    htmlbind.WithCache(store),
    htmlbind.WithURLSchemes("http", "https", "myapp"),
}
if options.Redraw(w, r, registry, render...) {
    return
}
```

The boundary prefix and the build identity are supplied from your `Options` and need no passing, and the request's own context goes in ahead of yours, so a cancelled request stops the work its externals started.

Because the page response and the redraw response now share a URL, `Redraw` declares `Vary` on the render, build, kind, and instance headers whichever one it turns out to serve. Without the kind and instance there, two components redrawing on one page would be a single cache entry and either could be answered with the other's markup.

There is no reserved redraw path any more. `RedrawHandler`, `RedrawPath`, and the `Mount` registration are gone: an endpoint whose address the caller cannot choose was the defect, and keeping a second addressing alive meant publishing two shapes in one contract.

Nothing is lost. A deployment that wants a dedicated route writes one, and it is the same few lines whatever URL it picks:

```go
mux.HandleFunc("GET /internal/redraw/{kind}/{instance}", func(w http.ResponseWriter, r *http.Request) {
    r.Header.Set("X-Tinybind-Render", "redraw")
    r.Header.Set("X-Tinybind-Kind", r.PathValue("kind"))
    r.Header.Set("X-Tinybind-Instance", r.PathValue("instance"))
    options.Redraw(w, r, registry)
})
```

Pass `{ url: … }` as `redraw`'s third argument to address it from the browser. Doing this puts the parallel-path-pattern problem back, which is why it is a handler you write rather than one the module ships.

The `id` is yours, not the framework's. Reloading a region is a deliberate act, so naming it should be too, and `getElementById` already solves lookup. Write it at the call site rather than inside the component, or every instance would share one id; in a loop, compose it from the item key as you would anyway.

The kind is a hash of the component's parameters and compiled markup. Its job is versioning: editing the template changes it, so a page loaded before a deploy names a kind this deployment does not publish, gets a 404, and falls back to a full reload rather than rendering under changed semantics.

It distinguishes types as a side effect — two components sharing a name but differing in parameters or markup get different kinds — but it does **not** cover the package. Two templates identical in name, parameters, and markup collide, and the collision matters because identical plan text still resolves its external calls per package. Registering the same kind twice therefore fails at startup rather than overwriting.

> [!WARNING]
> **Registering a component publishes an HTTP endpoint, and its parameters become untrusted input.** Anyone can call `?userId=999`. A component's arguments used to be values a page had already authenticated, authorized, and derived; a registered one receives whatever the caller sends. A component that only formats values handed to it is safe to register. One that loads a record by identifier must check ownership or visibility itself, exactly as an ordinary handler would. Registration is the review point.

Because a redraw is a GET, it must be repeatable with no observable effect: it is retried on supersession and may be answered from a cache. Per-user output must be marked private — and note that the URL no longer identifies the response on its own, which is why the kind and instance are `Vary` axes.

### Assets a redraw needs

A redraw swaps markup into a page this endpoint never rendered, so unlike a navigation it cannot merge into a head it owns. A component whose stylesheet is not already on the page therefore renders unstyled — the flash the navigation delta added its own head field to prevent.

Two things close that, and a deployment wants both.

**Put them in the shell.** The registration publishes what each component contributes, and the registry unions it:

```go
shell := layout.LayoutParams{Head: registry.RequiredHead()}
```

Reading it needs no request and no render, so a shell built once at startup covers every redraw the deployment will ever serve. `registry.RequiredAssets()` is the same set as identities rather than markup — an `ID`, a media type, and a URL — for a caller deciding where each file is served or which to preload.

**The response carries it anyway.** A redraw whose component contributes head sets `X-Tinybind-Head`, base64 of the tag list, and the runtime installs anything missing and waits for stylesheets before swapping. The body stays the bare subtree — no envelope, so the endpoint is still what `curl` shows — and a component contributing no head sends no header at all.

In a deployment that did the first, the second does nothing: every tag is already present and none is installed. It exists so the failure mode is a slower swap rather than an unstyled one.

The header is bounded, and the bound is checked at registration rather than at request time. A component's head is a static declaration, so an oversized one is a fact about the templates; `Register` refuses it and names `RequiredHead` as the way out, which is a startup failure instead of a proxy silently dropping a header in production.

> [!TIP]
> If the state can live in the URL, put it there and use a navigation delta instead. Redrawing earns its keep for widget-local state that should not appear in a shareable URL, or for a region whose inputs the browser genuinely owns.

### Why there is no third mode

The two modes divide cleanly: inputs the browser owns are a redraw, inputs the server must derive are a navigation. A middle option — re-run the handler, patch one parameter, return one region — looks appealing and does not work.

The patch reaches only the target component, so it cannot influence the data fetch that produced that component's other inputs. Patch a sort order into a table and the page's `ORDER BY` never sees it; the same rows come back. The mode is confined to presentation below the fetch, which is a contract that reads far more general than it is.

It is also avoidable from both sides. State that must reach the query belongs in the URL, where a navigation already handles it and the user gets a shareable, bookmarkable, back-navigable page. State that must stay out of the URL means the component should fetch its own data — which is exactly the condition for registering it as reloadable.

## Acting and refreshing in one round trip

Acting and then re-fetching is two round trips for one gesture, and the second one re-derives what the handler already knew. A mutating endpoint can instead return the regions its action changed:

```go
func addToCart(w http.ResponseWriter, r *http.Request) {
    count, err := cart.Add(r.Context(), itemID)
    if err != nil {
        httpbind.WriteError(w, r, err)
        return
    }
    if options.WantsUpdate(r) {
        _ = options.WriteUpdate(w, r, []htmlupdate.Update{
            htmlupdate.Replace("cart", CartBadge(CartBadgeParams{ID: "cart", Count: count})),
            htmlupdate.Replace("row-"+itemID, ItemRow(ItemRowParams{ID: "row-" + itemID, Item: item})),
        }, htmlbind.WithCSRFToken(session.CSRFToken(r)))
        return
    }
    httpbind.Write(w, r, result) // the endpoint's ordinary JSON
}
```

```js
const response = await fetch("/cart/add", {
  method: "POST",
  headers: { ...tinybind.updateHeaders(), "X-CSRF-Token": token },
  body,
});
await tinybind.apply(response);
```

Without the render header the endpoint answers the way it always did — JSON for an API, a redirect for a form handler — so a non-browser client and a page without the runtime are unaffected. `WantsUpdate` is the single branch point, which is what keeps the two paths from drifting apart.

The body is the same shape a redraw returns, so the client applies both with one code path. `WriteNavigate` replaces the region list when the action changed where the user belongs.

**The client applies the regions whatever the status says.** A rejected submission returns 4xx and the regions it carries *are* the validation errors — showing them is the point. This is the opposite of a redraw, where a non-2xx means the render failed and the runtime falls back.

Unlike a redraw, this adds no trust surface: the handler authorized the action and picked the components in Go, so no parameter arrives from the caller. It does mutate, so CSRF protection applies and the response is never cacheable.

## Streaming

A delta can arrive as a stream of records rather than one buffered body, so each region applies as soon as it is written instead of when the response ends. Serve it with `options.RenderStream` in place of `options.Render`; negotiation is unchanged, so a client that cannot stream still gets a document.

```text
{"r":"head","v":1,"head":["<link …>"]}
{"r":"op","kind":"replace","id":"c2","html":"<p …>","frame":"Nk9…"}
{"r":"op","id":"c1","frame":"8Qv…"}
{"r":"end"}
```

`options.RenderStreamAsync` adds await boundaries to the same stream: a region travels with its fallback and its replacement follows, so a slow dependency delays only itself. `options.RenderLiveStream` keeps live subscriptions open on it.

Two record kinds share the stream because both mean a region is ready. An `op` addresses a boundary by its instance; an `await` addresses a placeholder inside one that was already installed.

Each record carries its own manifest entry, because a trailing manifest cannot be written before the operations it describes. An unchanged boundary still appears, carrying a validator and no markup, so the client can rebuild its whole manifest from what it received.

The stream ends with an explicit terminator, and that is the only way to tell a finished render from a truncated one. A stream that stops without it leaves the state unknown: applied operations are not rolled back, the manifest is discarded, and the next request is a complete document.

Once the first record is written the status is fixed, so the choice between a delta and a complete document is made before anything is sent.

## Consistency

One delta response is a single consistent render of the boundaries it covers. The document as a whole is not: after independent redraws, regions may come from different points in time.

This is a documented boundary, not a bug. A value that must agree across regions belongs in one boundary, or in an ancestor of both.

Responses are fenced. A superseded navigation's response is discarded unapplied, a boundary response whose base revision is stale is rejected, and an asynchronous completion for a superseded revision is dropped. Out-of-order responses cannot restore older state.

Because a response may be discarded after the server rendered it, **re-rendering must be repeatable and free of side effects**. Mutations belong in ordinary handlers.

## Preserving browser state

Replacing a region destroys what the browser owns inside it: focus and text selection, an in-progress IME composition, a file input's selection, media playback position, and custom element internals.

The runtime avoids this where it can — an unchanged boundary is never touched, retain holes move live nodes instead of recreating them, focus is restored, and an update is deferred while an IME composition is active. Where it cannot, mark the region as preserved and the runtime moves the existing node instead of accepting the server's version.

The server render is authoritative for form values, so a boundary containing an uncontrolled input loses user typing when it is replaced. Keep such inputs outside the boundary, mark them preserved, or accept the reset deliberately.

### How much of a region gets touched

Whole replacement is the blunt instrument, and it is the right one for a full navigation, where the page changes anyway. It is the wrong one for a search-parameter update or a live delivery in a region holding a form.

The end state is a **static-dynamic split**. Templates here are already compiled to an instruction list, so which parts of a component are fixed markup and which are dynamic holes is known at generation time. The client holds the static skeleton — the component kind hash is a content address, so a skeleton is immutable, permanently cacheable, and invalidated by a deploy — and an update sends only the values of the holes that changed. Applying is then exact rather than heuristic: each hole has a known position, its surroundings are never touched, and form values, focus, media, and event listeners survive by construction rather than by rule.

That is also what relaxes the live-region restriction. Today a live region forbids form controls because every delivery replaces the subtree. Once a hole whose value did not change emits nothing, the rule becomes "do not make a control's value a live hole" — a far more natural constraint than banning the elements.

Either of those needs to know which existing row corresponds to which new one, which is what a **list key** is for:

```text
{for item in items}<li key={item.ID}>{item.Label}</li>{/for}
```

Positional matching still converges on the right markup without one. What it gets wrong is everything the markup does not describe: insert a row at the top and focus, an open menu, a checked box, and a playing video each shift one row across, and transitions replay. The key must come from the data — a content hash loses identity exactly when the content changes, which is when identity matters most — and a keyed loop body needs a single root element, the same constraint an update boundary has and for the same reason.

Between here and there, **morphing** — walking the old and new trees and mutating in place, as Turbo and LiveView do — buys most of the benefit without the author marking anything, at the cost of heuristics that go wrong in keyless lists. Preserved islands remain useful at every stage, because a third-party widget the server does not own cannot be patched by any of these.

### Form controls are a separate problem

An island says "do not touch this subtree." That is the wrong tool for a `<select>` whose options depend on an upstream choice: the option list *must* change, and the current selection should often survive anyway.

The reason is that client state comes in two kinds. Focus, text selection, IME composition, scroll offset, and media position belong to a **node** — they survive only if that element survives. A chosen option, a checked box, and typed text are keyed to a **value** — they mean something independently of the node that carried them, so they can outlive a replacement.

Transferring the second kind needs no author markup and no bookkeeping, because HTML already separates what the server said from what the user did: `value` against `defaultValue`, `checked` against `defaultChecked`, `selected` against `defaultSelected`.

The procedure is four distinct comparisons:

1. **Dirty detection** — the element's `value` against its own `defaultValue`. A pair that differs means the user touched it. Both sides are already in the DOM, so nothing has to be snapshotted.
2. **Control identity** — the recorded control against the controls in the updated DOM. The element may have been recreated, so this matches on `name`, falling back to position.
3. **Applicability** — the recorded user value against the new set of options or radio inputs. Needed only for `select` and radio groups; text and checkboxes are unconstrained.
4. **Assertion change** — the old default against the new default.

That last comparison is the tie-break. A **changed** default is the server asserting a new value, and it wins. An unchanged default is the server expressing no opinion, so the user's value stays. Treating silence as an assertion would clear typed text on every unrelated update.

This runs by default on every region an update touches, with no way to turn it off and no way for one response to discard user state. Losing typed text is the failure users notice, and an opt-in would mean either every author remembers it or every author's forms break.

When a recorded value no longer applies it is reported through a browser alert. That is deliberately provisional: silent data loss is the worse outcome today, but a value falling out of an option set is often ordinary application behavior, so the alert will eventually give way to an event the application can subscribe to and decide about.

Always-on leaves one gap. The server cannot express "clear this back to a default that did not change" — emptying a form renders the same empty default it rendered before, which the tie-break reads as silence, so the user's text stays. No amount of comparing defaults distinguishes the two situations, because the markup is identical. In practice it rarely bites: a form submit is not a GET, so the runtime leaves it to the browser and the response is a complete document with no region to reconcile. Post-redirect-get clears a form through an ordinary page load, outside this rule entirely. What remains unexpressible is clearing a form through a GET update.

A file input is the exception to all of this: its selection cannot be restored by value at all, so it belongs in a preserved island or outside the region.

### Preserved islands

Some regions the server cannot patch at all: a third-party widget, a canvas, a media element it does not own. Mark one and the runtime moves the live node into the replacement instead of accepting the server's version, so the node and everything the browser attached to it survive:

```html
<div data-tb-preserve="player">…</div>
```

The marker follows the configured prefix, so a project using `pw` writes `data-pw-preserve`.

The key matches the region across renders. A key with no counterpart in the replacement is a new region, so the server's version stands rather than being handed an unrelated node.

## Configuration

Generation:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -data-attribute-prefix tb
```

`-data-attribute-prefix` names the generated attributes, producing `data-tb-id` by default. The same prefix names the placeholder element and the boundary identifiers, so a project setting `pw` gets `data-pw-id` on its boundaries and `<pw-boundary id="pw-1">` for its placeholders — one naming system rather than two. Set the same value on `Options.DataAttributePrefix`, because the generator writes those attributes and the runtime reads them.

Serving. Every name the protocol puts in a document is configurable, and everything the framework owns lives inside the two namespaces:

```go
options := htmlupdate.Options{
    Key:                 validatorKey, // required for non-public pages
    PathPrefix:          "/_tb",       // every framework endpoint
    HeaderPrefix:        "X-Tinybind", // every framework header
    DataAttributePrefix: "tb",         // every attribute, element, and identifier
    GlobalName:          "tinybind",   // what the runtime installs itself as
    MaxManifestBytes:    8 << 10,      // oversized hints are dropped, not rejected
    MaxQueryBytes:       4 << 10,      // an oversized redraw query is rejected
    ServeRuntime:        true,         // serve the reference browser client
}

if err := options.Validate(); err != nil {
    return err // a prefix that cannot name an element, or no runtime owner
}

options.Mount(mux) // the runtime asset, which is the only endpoint left

mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
    wrappers := []htmlbind.Wrapper{BindDocument(...), BindLayout(...)}
    leaf := Page(PageParams{Query: r.URL.Query().Get("q")})
    if err := options.Render(w, r, wrappers, leaf); err != nil {
        http.Error(w, "render failed", http.StatusInternalServerError)
    }
})
```

One prefix for every endpoint means one routing rule, one cache rule, and one access rule covers the whole surface. `Mount` registers them all; nothing the framework owns appears outside that prefix.

`options.Render` negotiates the mode, sets `Vary`, and writes either the document or the delta. Everything else about the response stays yours, as elsewhere in this module.

The runtime is served at a content-hashed path under the same prefix, so it is immutably cacheable and a deploy invalidates it. `options.ScriptTag()` returns the element that loads it, and that element carries the whole configuration, so one shared runtime asset works for any set of names without being rebuilt. Nothing at all is compiled into the bytes — not even a protocol version.

### Who serves the browser runtime

**You must say.** Set exactly one of `ServeRuntime` and `CallerOwnsRuntime`; `Validate` refuses both and neither at startup.

That is not ceremony. Every other misconfiguration here is wrong in a way somebody notices, and this one is not: a build that serves no runtime and claims no ownership compiles, starts, renders every page correctly, and then does nothing at all when a boundary should update. No error, no log line, no failed request — just a page that quietly stopped working. So the question is asked once, at startup, where the answer is cheap.

`ServeRuntime` serves the reference client at a content-hashed path under your prefix and makes `ScriptTag` return the element that loads it. That is the whole of what a project using this module directly needs.

`CallerOwnsRuntime` says you ship your own, and then `Mount` registers no asset route and `ScriptTag` returns nothing — a tag pointing at an asset this build does not serve is worse than no tag.

A framework that already ships a browser runtime takes the second, because two runtimes on one document means two boundary id spaces, two build identities, and two script tags with nothing deciding which owns a region. Take the bytes and merge them:

```go
options := htmlupdate.Options{Key: validatorKey, CallerOwnsRuntime: true}

source := htmlupdate.RuntimeSource()      // the bytes, naming-independent
asset := options.RuntimeAsset()           // bytes, digest, media type, file name
config := options.RuntimeConfig()         // what the merged runtime must be given
```

Your merged asset calls `createPartialUpdateRuntime(config)` with a config carrying the same names, and installs the result wherever it likes.

Taking a copy of `runtime.js` instead is the shape to avoid: a copy is not a version-pinned dependency, so it drifts on upgrade with nothing in your build failing, and a drifted browser runtime is a silently dead page rather than a compile error. `RuntimeSource` exists so you never have to.

### Writing your own client

`htmlupdate/runtime.js` is reference code. Everything it does is a consequence of the wire contract, so a client written from that contract alone is conforming whether or not it resembles this one — and it is worth reading in that light rather than as an implementation to subclass.

What such a client must not get wrong, whatever else it does:

- **Trigger a streaming swap from the commit marker, never from the template.** A template's start tag can arrive in its own chunk, and an observer watching the template will swap in empty content and destroy the fallback. The marker cannot appear before the template is complete. This only shows up once a proxy, a TLS record, or a compressing encoder splits the bytes, so it survives development.
- **Apply each record at most once**, and treat a stream with no terminator as truncated: discard the manifest rather than trusting a partial one.
- **Install head contributions before the markup that needs them**, or a region whose stylesheet just arrived paints unstyled.
- **Fall back to an ordinary full navigation on every failure path** — a non-2xx, a body that is not what the mode promised, a missing target, a build that does not match. That fallback is what lets every other part of this design be incomplete without being incorrect.

The version, if you want one, is yours: add `;v=N` to the mode token and the server will echo it back untouched. The module compares only `X-Tinybind-Build`.

### Reporting failures

The redraw endpoint has to write a response, because it owns the URL. It does not decide what a failure looks like:

```go
options.OnFailure = func(w http.ResponseWriter, r *http.Request, f htmlupdate.Failure) {
    logger.ErrorContext(r.Context(), "redraw failed", "kind", f.Kind, "err", f.Err)
    problem.Write(w, r, f.Status, f.Kind.String()) // or htmlupdate.WriteFailure(w, f)
}
```

Every refused redraw arrives here — a malformed path, an unpublished kind, a page from another build, an oversized query, a rejected argument, a failed render — with the status and body the package would have written, and the underlying cause where there is one. Without a hook it writes those defaults itself, unchanged.

A redraw response carries an `ETag` over its rendered bytes and `private, no-cache`, so an unchanged region costs a `304` instead of its whole markup. `RedrawCacheControl` replaces that policy for a deployment whose redraws are public or whose proxy needs different terms.

The HTTP layer lives in `htmlupdate` rather than `htmlbind`, because the render runtime stays free of `net/http` so generated template code keeps working on TinyGo and WebAssembly targets.

## Availability

| Capability | Status |
| --- | --- |
| Boundaries, identities, and both validators | Available |
| Manifest comparison and change detection | Available |
| Mode negotiation, build identity, `Vary` | Available |
| Navigation delta with `replace`, buffered | Available |
| Browser runtime `update()` and URL replacement | Available |
| Cross-route navigation, history, scroll, focus | Available |
| Head and asset synchronization | Available |
| Head on the redraw response, and the required-asset set | Available |
| Head of a fragment supplied through a slot | Available |
| Link interception and `navigate()` | Available |
| Form state reconciliation | Available |
| Registered component redraw endpoint | Available |
| Structural operations, list keys, retain holes | Planned |
| Preserved islands | Available |
| Lifecycle events and GET form interception | Available |
| Streamed delta records | Available |
| Await completions on the same stream | Available |
| Live delivery and reconnection, on the `live` token | Available |
| Handoff marker and terminator reasons | Available |
| Acting and refreshing in one round trip | Available |

| Morphing or static-dynamic application | Planned |

Available capabilities are limited to same-route updates: the composition does not change, so no head synchronization is needed and history only replaces the query string.

## Limits and non-goals

- Diffing is per boundary. There is no attribute-level or text-node-level diff; a changed boundary is replaced whole.
- A boundary must render deterministically for equal inputs and equal state. A region embedding a timestamp never matches and is resent every time.
- Manifest size grows with boundary count, so boundaries belong at meaningful regions rather than on every list row.
- Update state lives in the browser, not on the server. Nothing needs session affinity, restarts lose nothing, and a stale hint degrades to a complete document.
- This is not a client-side framework. There is no component state in the browser, no virtual DOM, and no client-side routing table; the server remains the single source of truth for markup.

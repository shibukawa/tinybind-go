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
              X-Tinybind-Render: navigation;v=1
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
| Navigation | `X-Tinybind-Render: navigation;v=N` | Changed boundaries of the target route |
| Redraw | `GET <prefix>/redraw/<kind>/<id>?args` | That one component's subtree |
| Live reconnect | `X-Tinybind-Render: live;v=N` after a dropped stream | Resumed deliveries for the page's live regions |

The last row belongs to a different feature and is listed here because it shares this negotiation. A [live source](htmlbind.md#live-sources) delivers over one chunked response; when that connection drops — a proxy timeout, a sleeping laptop — the regions freeze with no signal. Reconnecting re-requests the same page in a live mode and resumes.

That resumption is unusually cheap, and for a structural reason: **every live delivery carries the whole state of its region rather than an increment.** A missed value costs nothing, so there is no cursor, no event log, no replay, and no equivalent of `Last-Event-ID`. Boundary ids are allocated by position, so a re-render reproduces the ids already on screen. The reconnected region paints current state immediately instead of showing a placeholder.

The header is deliberately not `Accept`: shared caches normalize or drop `Vary: Accept`, and one URL must stay one cacheable document resource. It is deliberately not a query parameter either, because that would change canonical, shareable, and logged URLs.

Every response varies on the render header. Delta responses are `no-store`, since they carry per-document validators.

Anything unrecognized — an unknown mode, a version the server does not speak, a truncated header, a proxy that stripped one — resolves to a complete document rather than an error. That rule is what lets each milestone stay incomplete without ever being incorrect.

## Protocol version

The version identifies the wire contract: attribute names, manifest fields, operation kinds, and how validators are built. It is a single integer owned by the framework, not a project setting, and it is mixed into every digest.

It does **not** identify your templates. A template edit changes that component's version inside its validators; the protocol number stays put.

Client and server must match exactly. A mismatch serves a complete document, which is also what makes a rolling deploy safe: a page rendered by the old version whose next request reaches a new server falls back cleanly.

## Delta operations

A delta carries the outermost changed boundaries only. A descendant of a replaced boundary is already inside that replacement, so sending it again would target a node that no longer exists.

- `replace` — swap a boundary's element for new markup
- `insert`, `remove`, `move` — structural changes in a keyed list
- `replace` with retain holes — replace an ancestor while moving unchanged descendants into it, so their DOM state survives

The response also carries head operations. A component appearing for the first time brings stylesheet and script links that are not in the live document head, and its markup must not be applied until they are installed; otherwise the region flashes unstyled. Stylesheets that fail to load do not block the update indefinitely.

## Client API

```js
await window.tinybind.update("/search?q=rust");   // re-render the current route
await window.tinybind.navigate("/guides/intro");  // move to another route
await window.tinybind.redraw("card-1", { page: 2 }); // re-render one component
await window.tinybind.apply(response);            // install what an action returned
```

Links and GET forms are intercepted for same-origin navigation; put `data-tinybind-ignore` on an element or an ancestor to return it to the browser. A form's fields become the query, so a search form refines the page it is on and replaces the URL rather than stacking a history entry per submit. Non-GET submission is left to the browser, which is what makes post-redirect-get work unchanged.

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
registry.Register(pages.UserCardReloadable)
options.Mount(mux, registry)
```

The `id` parameter is required and is filled from the path, not the query. The framework writes it and the component's kind onto the root element on every render, so a region stays addressable and redrawable after a redraw replaced it. A reloadable component must be exported and single-rooted, and every other parameter must be a type a query string carries deterministically — a record, a slice, and `html` are refused at generation time. Unlike an automatic boundary these are errors, because the author asked for the endpoint.

```js
await window.tinybind.redraw("card-1", { userId: 42 });
```

That becomes a plain GET:

```text
GET /_tb/redraw/UserCard@8Qv3n1/card-1?userId=42
```

The server runs **only that component**, with the arguments from the query string, and returns its subtree as a single root element carrying the same `id`. There is nothing to reconstruct, so there is no capability token, no signing key, and no page execution.

The `id` is yours, not the framework's. Reloading a region is a deliberate act, so naming it should be too, and `getElementById` already solves lookup. Write it at the call site rather than inside the component, or every instance would share one id; in a loop, compose it from the item key as you would anyway.

The path segment after the component name is a hash of its parameters and compiled markup. Its job is versioning: editing the template changes the URL, so a page loaded before a deploy gets a 404 and falls back to a full reload rather than rendering under changed semantics.

It distinguishes types as a side effect — two components sharing a name but differing in parameters or markup get different kinds — but it does **not** cover the package. Two templates identical in name, parameters, and markup collide, and the collision matters because identical plan text still resolves its external calls per package. Registering the same kind twice therefore fails at startup rather than overwriting.

> [!WARNING]
> **Registering a component publishes an HTTP endpoint, and its parameters become untrusted input.** Anyone can call `?userId=999`. A component's arguments used to be values a page had already authenticated, authorized, and derived; a registered one receives whatever the caller sends. A component that only formats values handed to it is safe to register. One that loads a record by identifier must check ownership or visibility itself, exactly as an ordinary handler would. Registration is the review point.

Because a redraw is a GET, it must be repeatable with no observable effect: it is retried on supersession and may be answered from a cache. Per-user output must be marked private, since the URL alone identifies the response.

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
        _ = options.WriteUpdate(w,
            htmlupdate.Replace("cart", CartBadge(CartBadgeParams{ID: "cart", Count: count})),
            htmlupdate.Replace("row-"+itemID, ItemRow(ItemRowParams{ID: "row-" + itemID, Item: item})),
        )
        return
    }
    httpbind.Write(w, r, result) // the endpoint's ordinary JSON
}
```

```js
const response = await fetch("/cart/add", {
  method: "POST",
  headers: { ...window.tinybind.updateHeaders(), "X-CSRF-Token": token },
  body,
});
await window.tinybind.apply(response);
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
<div data-tinybind-preserve="player">…</div>
```

The key matches the region across renders. A key with no counterpart in the replacement is a new region, so the server's version stands rather than being handed an unrelated node.

## Configuration

Generation:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -data-attribute-prefix tb
```

`-data-attribute-prefix` names the generated attributes, producing `data-tb-id` by default. Override it only if your markup already uses that prefix; the browser runtime hardcodes the names, so an override needs a runtime built to match.

Serving. Two namespaces are configurable, and everything the framework owns lives inside them:

```go
options := htmlupdate.Options{
    Key:              validatorKey, // required for non-public pages
    PathPrefix:       "/_tb",       // every framework endpoint
    HeaderPrefix:     "X-Tinybind", // every framework header
    MaxManifestBytes: 8 << 10,      // oversized hints are dropped, not rejected
}

options.Mount(mux) // installs the runtime asset, and later the redraw endpoint

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

The runtime is served at a content-hashed path under the same prefix, so it is immutably cacheable and a deploy invalidates it. `options.ScriptTag()` returns the element that loads it, and that element carries the prefix, so one shared runtime asset works for any namespace without being rebuilt. The header names are different: the runtime hardcodes those, so overriding `HeaderPrefix` needs a runtime built to match.

The HTTP layer lives in `htmlupdate` rather than `htmlbind`, because the render runtime stays free of `net/http` so generated template code keeps working on TinyGo and WebAssembly targets.

## Availability

| Capability | Status |
| --- | --- |
| Boundaries, identities, and both validators | Available |
| Manifest comparison and change detection | Available |
| Mode negotiation, protocol version, `Vary` | Available |
| Navigation delta with `replace`, buffered | Available |
| Browser runtime `update()` and URL replacement | Available |
| Cross-route navigation, history, scroll, focus | Available |
| Head and asset synchronization | Available |
| Link interception and `navigate()` | Available |
| Form state reconciliation | Available |
| Registered component redraw endpoint | Available |
| Structural operations, list keys, retain holes | Planned |
| Preserved islands | Available |
| Lifecycle events and GET form interception | Available |
| Streamed delta records | Available |
| Await completions on the same stream | Available |
| Live delivery and reconnection | Available |
| Acting and refreshing in one round trip | Available |

| Morphing or static-dynamic application | Planned |

Available capabilities are limited to same-route updates: the composition does not change, so no head synchronization is needed and history only replaces the query string.

## Limits and non-goals

- Diffing is per boundary. There is no attribute-level or text-node-level diff; a changed boundary is replaced whole.
- A boundary must render deterministically for equal inputs and equal state. A region embedding a timestamp never matches and is resent every time.
- Manifest size grows with boundary count, so boundaries belong at meaningful regions rather than on every list row.
- Update state lives in the browser, not on the server. Nothing needs session affinity, restarts lose nothing, and a stale hint degrades to a complete document.
- This is not a client-side framework. There is no component state in the browser, no virtual DOM, and no client-side routing table; the server remains the single source of truth for markup.

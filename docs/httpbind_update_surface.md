# The update surface: a usage guide

**Audience:** someone building a framework on `tinybind-go`, wiring the HTML update endpoints into their own router and their own browser runtime.

This is how to use it. For why it is shaped this way, see [`httpbind_render_modes.md`](httpbind_render_modes.md); for the exact bytes on the wire, [`httpbind_update_wire_contract.md`](httpbind_update_wire_contract.md).

---

## The shape in one paragraph

One URL answers several ways. A request with no render header gets the complete document, so a crawler and a browser with no runtime are unaffected. With the header, the same handler returns only the regions that changed — as markup, or as the values that fill a static shape the client already holds. Every entry is something you call from inside your own handler; this package mounts no routes and owns no URLs.

## Setting up

```go
var update = htmlupdate.Options{
    Key:          []byte(os.Getenv("TB_VALIDATOR_KEY")), // authenticates validators
    ServeRuntime: true,                                  // or CallerOwnsRuntime
}

func main() {
    if err := update.Validate(); err != nil {
        log.Fatal(err) // reports every misconfiguration at once
    }
}
```

`Key` matters: a validator published to a browser must be keyed, or anyone who can reach the endpoint can confirm a guess about what a region says by comparing digests. Rotating it forces complete documents, which is the intended effect of a rotation.

Exactly one of `ServeRuntime` and `CallerOwnsRuntime` must be set. Neither compiles fine and then serves pages that silently stop updating, which is why `Validate` refuses it.

## Rendering a page

Your handler renders the chain and lets `Options.Render` decide what the request asked for:

```go
mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
    if !authorized(r) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    wrappers := []htmlbind.Wrapper{
        htmlbind.BindWrapper(documentPlan, documentParams{}, setChildren),
        htmlbind.BindWrapper(layoutPlan, layoutParams{Section: r.URL.Query().Get("section")}, setChildren),
    }
    leaf := htmlbind.Bind(pagePlan, pageParams{Query: r.URL.Query().Get("q")})
    if err := update.Render(w, r, wrappers, leaf); err != nil {
        http.Error(w, http.StatusText(500), 500)
    }
})
```

`Render` buffers. For a page with `await` boundaries, use `RenderStreamAsync` so a slow boundary delays only itself:

```go
err := update.RenderStreamAsync(r.Context(), w, r, wrappers, leaf, renderOptions(r)...)
```

## Render options, and why you must pass them

Every entry that renders a fragment takes `[]htmlbind.Option`. **Pass the same ones everywhere.** Without them a component renders one way inside its page and another in the response that replaces it:

```go
func renderOptions(r *http.Request) []htmlbind.Option {
    return []htmlbind.Option{
        htmlbind.WithCSRFToken(session.CSRFToken(r)),
        htmlbind.WithCache(store),
        htmlbind.WithURLSchemes("http", "https", "myapp"),
    }
}
```

- **`WithCSRFToken` is not optional in practice.** A component containing an unsafe form emits a CSRF field, and without a token that render *fails* — a 500, not a form with an empty token. Use `WithoutCSRFToken()` for a render with no session behind it: a mail body, a static export, a golden test.
- **`WithURLSchemes`** — without it a `url` carrying your own scheme neutralises to `#tb-blocked-url`.
- **`WithCache`** — a `@cache` component runs its body every time without it.

The boundary prefix, the build identity, and the request's context are supplied from your `Options` and need no passing. Your options come last, so you can still override the context.

## Redraw: the browser asks for one region again

Register the components you publish. Registration is the review point — it puts an HTTP endpoint in front of the component's parameters, and anyone can supply them:

```go
var registry = &htmlupdate.Registry{}

func init() {
    registry.MustRegister(pages.CounterReloadable)
}
```

> A component that only formats values handed to it is safe to register. One that loads a record by identifier **must check ownership itself**, exactly as an ordinary handler would.

Branch on it inside the page handler, so the redraw inherits that handler's own checks:

```go
mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
    if !authorized(r) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    if update.Redraw(w, r, registry, renderOptions(r)...) {
        return // it was a redraw, and it inherited the check above
    }
    // ordinary page render
})
```

Put `Registry.RequiredHead()` in your document shell at startup. A redraw rewrites a region of a page this endpoint never rendered, so it cannot install a stylesheet before the markup that needs it:

```go
shell := documentParams{Head: registry.RequiredHead()}
```

## Actions: one round trip that changes state and refreshes

```go
func addToCart(w http.ResponseWriter, r *http.Request) {
    count, err := cart.Add(r.Context(), itemID)
    if err != nil {
        httpbind.WriteError(w, r, err)
        return
    }
    if update.WantsUpdate(r) {
        _ = update.WriteUpdate(w, r, []htmlupdate.Update{
            htmlupdate.Replace("cart", CartBadge(CartBadgeParams{ID: "cart", Count: count})),
        }, renderOptions(r)...)
        return
    }
    httpbind.Write(w, r, result) // the endpoint's ordinary JSON
}
```

`WriteUpdateStatus` is the same with an explicit status, so a rejected submission returns 422 **and** the region showing why. That is the case the CSRF option above matters most for: the region being rewritten is a form.

## Sequences: sending values instead of markup

A fragment's static half — its literal text — is identical in every render. Set one request header and it stops travelling:

```
X-Tinybind-Sequences: 1
```

An operation then carries an address and the values that fill it, wherever that is smaller than the markup:

```json
{"kind":"replace","id":"panel","seq":"Yb3_x…","values":["Inbox","30","data-tb-id","r0", …]}
```

Ask for the tree behind an address the same way you answer a redraw:

```go
mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
    if update.Sequence(w, r) {
        return
    }
    if update.Redraw(w, r, registry, renderOptions(r)...) {
        return
    }
    // ordinary page render
})
```

A sequence is the one response here that is **not per user** — it derives from the template, not from the request — so it is served `public, max-age=31536000, immutable` and a shared cache may hold it across users and across builds. An address this process has never rendered is answered 404, and the client asks for markup instead.

**Whether it pays depends on the shape.** Measured on a hundred-row panel: values are 40% of the markup taken whole, but a small fragment costs *more* as an address plus values than as markup — which is why the choice is made per fragment, and why the split is never a loss.

## What the client must do

Four rules the record shape cannot enforce.

**1. Do not set `If-None-Match` yourself.** Issue an ordinary fetch and let the browser revalidate — it reconstructs the full body from its store, so head, live marker, and manifest always reach you. A conditional you issue returns a bodiless 304 and loses all three.

**2. On a same-document history navigation, send the manifest describing the DOM currently on screen** — not the one stored with the entry you are moving to. Going A → B → A and returning A's manifest while the screen shows B makes every boundary compare equal, and nothing is sent.

**3. Abort superseded requests.** A pending redraw for an instance, before issuing another for it; every pending redraw, before applying navigation records. A response whose request you aborted is never applied, even if its bytes arrived. Without this, a search box redrawing per keystroke leaves the shorter query's result under the longer query's input.

**4. Across a navigation into a live page:** abort the outgoing page's live request, apply the navigation records, *then* open a new live request. In that order.

## Applying a response

Three record shapes, one apply path.

**`replace`** — swap the region's markup. It carries a **hole** where each nested boundary sits, and `boundaries` names them:

```json
{"kind":"replace","id":"panel","html":"<section …><tb-boundary data-tb-id=\"r0\" …></tb-boundary></section>","boundaries":["r0","r1"]}
```

A hole whose id **also carries an operation** in this response is filled from it. A hole that does **not** is a region you already hold — lift its live node out before the swap and move it into the hole. That is what keeps the focus, the form values, and the playing media inside it.

**`children`** — the region's own markup is unchanged and its nested boundaries are now these, in this order. No markup at all:

```json
{"kind":"children","id":"panel","boundaries":["r0","r1","r2"]}
```

Keep what the list keeps, moving what moved; drop what it omits; fill what arrives as its own operation. This is how a list says a row was appended without re-sending the list.

**Dispatch on `kind`, not on whether `html` is present.** A `children` record carries none, and every stream path in this package got that wrong once.

**`seq` + `values`** — the same fragment, split. Walk the tree consuming one value per hole, one per conditional (which branch), one per loop (how many times), one per component call (boundary or inline). Concatenate and parse once.

**Escaping never leaves this module.** Values arrive already escaped, exactly as they would have been written into HTML. You concatenate and parse; you apply no escaper and judge no value. The URL scheme allowlist in particular stays here.

## Failures

An update that could not be produced answers `application/problem+json`:

```json
{"type":"about:blank","title":"Bad Request","status":400,
 "detail":"invalid redraw arguments","code":"invalid_arguments",
 "errors":[{"field":"page","location":"query","message":"is not an integer"}]}
```

**The media type is the discriminator.** `application/json` is an update to apply — including a non-2xx one, since a 422 carrying the validation errors is a *successful* update. `application/problem+json` is a request that produced none; apply nothing and fall back.

`code` carries the failure kind, because a stale page and a failed render are one status to a proxy and different events to whoever is on call. Take over the response with `Options.OnFailure` to log it, trace it, or write your own body.

## Headers

| header | direction | meaning |
| --- | --- | --- |
| `X-Tinybind-Render` | request | the mode: `navigation`, `live`, `redraw`, `action`, `sequence`; echoed on the response |
| `X-Tinybind-Build` | request | the build the page was rendered by; a mismatch answers with a complete document |
| `X-Tinybind-Manifest` | request | `id:frame:children:parent` entries, comma separated |
| `X-Tinybind-Kind` / `-Instance` | request | which component a redraw addresses |
| `X-Tinybind-Sequences` | request | this client can walk a sequence tree |
| `X-Tinybind-Sequence-Address` | request | which sequence to serve |
| `X-Tinybind-Live` | response | this composition owns a live boundary; open a live request |

The prefix is `Options.HeaderPrefix`. Everything composes from it, so renaming it needs no rebuilt runtime.

## What this package will not do

- **Choose your wire version.** Add your own field beside the emitted shape; the build identity is the only compatibility axis this package operates.
- **Ship your browser runtime.** Set `CallerOwnsRuntime` and merge `RuntimeSource` into your own asset, or write your own against the wire contract.
- **Mount a route.** Every entry is one you call. The URL a redraw or a sequence is served at is yours, which is what lets it inherit the page handler's authorization instead of needing a second path pattern kept in step with the first.

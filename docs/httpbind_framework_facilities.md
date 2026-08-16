# Framework Facilities

This document is the index of what tinybind offers a framework built **on top of**
it, organized as requirements: what each facility is for, what it costs, whether
it is built, and what is still specified rather than shipped.

The other framework guides are task-shaped — routing in
[httpbind_frameworkowner.md](httpbind_frameworkowner.md), the page router in
[httpbind_discovered_router.md](httpbind_discovered_router.md), the boundary
protocol and client runtime in
[htmlbind_frameworkowner.md](htmlbind_frameworkowner.md). This one is
inventory-shaped. Read it when you are deciding whether a seam exists before you
work around one that does not.

## The rule every facility here follows

A seam is widened when its default output stays identical and its contract stays
the caller's. A change to the shape an application author writes against is
refused.

That is the whole policy, and it has two consequences worth stating plainly:

**Unused is free.** A project using none of a facility generates byte-identical
Go and writes byte-identical bytes. Every item below was measured against that,
not merely intended to satisfy it.

**The module decides what is needed; you decide where it comes from.** tinybind
computes which assets a page requires, which head tags merge, and which client
runtime a response needs. It never chooses a URL, serves a file, sets a header,
or decides a route.

## Facilities at a glance

| Facility | For | Status |
| --- | --- | --- |
| Generation symbols and emitter overrides | naming what generated code calls | shipped |
| Router type independence | naming your own router type | shipped |
| Generated-source exclusion | keeping your generated files out of discovery | shipped |
| Server action resolution | addressing a handler from your own route table | shipped |
| Native form submit | a server action reached with no browser runtime | **shipped 2026-08-12** |
| Script block reporting | reading a component's own script without parsing the template | **shipped 2026-08-12** |
| Client handlers (`on-click="…"`) | a template naming a function that block produced | **shipped 2026-08-12** |
| Component parameters as JSON | a script block reading an argument with its type | **shipped 2026-08-12** |
| Render context for a synchronous external | a per-request value rendered inline | **shipped 2026-07-31** |
| Caller head contributions | a head tag decided per response | **shipped 2026-07-31** |
| `noscript` in the head | telling a scriptless client something | **shipped 2026-07-31** |
| Head contribution provenance | naming the component behind a contribution | shipped |
| Static asset extraction | component style and script as served files | shipped |
| Fragment capability introspection | deciding what runtime a response needs | shipped |
| Live boundary rendering | a region re-rendered per delivery from a source | shipped |
| Live reconnection mode | resuming a subscription on the page's own route | specified, not built |
| Registered builtin elements (`<csrf-token/>`) | markup your framework implements | specified, not built |
| Component assets | a library shipping a component plus its `.js` | specified, not built |
| Render-time script contribution | selecting a script per response by name | specified, not built |

The rest of this document covers the three 2026-07-31 rows in depth, then states
what the unbuilt ones would give you and why they are not built yet. The four
2026-08-12 rows are one feature set with its own guide:
[httpbind_client_behavior.md](httpbind_client_behavior.md), which is where to
start if your framework owns a browser runtime.

## Per-request values in markup

> [!NOTE]
> **A CSRF token is no longer one of these.** It was this section's worked
> example until it became native: every unsafe form now carries a hidden field
> emitted for you, and the token arrives as `htmlbind.WithCSRFToken(token)` on
> the render call rather than through the context. What follows still applies to
> every *other* per-request value.

### The problem

A component never receives an `http.Request`, and a `Fragment` is immutable and
reusable across requests. So a value that belongs to the request — a request id,
a nonce, a cookie test — has nowhere obvious to live.

Passing it through the parameter struct works and is what most frameworks do
first. It has two costs. Every page that needs the value repeats the plumbing
from handler to `Params` to template. And the value becomes an ordinary template
variable, which means a rule like *never put this in a URL, an attribute, or a
log line* can only be asked for, never enforced.

### What shipped: a synchronous external that takes the context

An `external async` function's Go implementation may declare a leading
`context.Context`, and an `external live` one must. As of 2026-07-31 a plain
`external` may too.

Nothing changes in the template. The declaration says what it always said:

```
external RequestID(): string
external LocaleBanner(): html
```

The choice belongs to whoever writes the Go, function by function:

```go
// Takes the render context, because the id belongs to the request.
func RequestID(ctx context.Context) string {
	return traceFrom(ctx)
}

// Takes none, and is called exactly as it was before.
func SiteName() string { return "example" }
```

Generation reads your package's Go sources and passes the context to the
functions that accept one. The detection is syntactic — it runs before the
package compiles, so an unparsable file is skipped rather than failing
generation, and a call shape that then does not match is an ordinary Go compile
error at the generated call site.

If you drive generation through the generator command, this is automatic. If you
call the compiler yourself, it is `GenerateOptions.ContextExternals`, the same
map you already pass for async externals:

```go
out, err := htmlbind.Generate("page.tb.html", source, htmlbind.GenerateOptions{
	ContextExternals: map[string]bool{"RequestID": true},
})
```

**A loader that can fail declares a trailing error.** The same scan reports it,
and the template declaration is unchanged either way:

```go
func LoadRecord(id string) (Record, error) { ... }
```

Such a function may only be the whole value of a `{val}` binding, which is the
one position whose lowering can carry a failure out. Return one of the status
helpers and a page answers for itself:

```go
return Record{}, httpbind.NotFound(httpbind.Problem{Code: "absent"})
return Record{}, httpbind.Redirect("/sign-in")
```

`WriteError` turns those into a 404 and a 303 with a `Location`. A component's
own leading bindings run while the chain is assembled, before the shell writes,
so the status is still free.

Driving generation through the generator command fills this too. Calling the
compiler yourself, it is `GenerateOptions.ErrorExternals`, beside the context
map:

```go
out, err := htmlbind.Generate("page.tb.html", source, htmlbind.GenerateOptions{
	ContextExternals: map[string]bool{"RequestID": true},
	ErrorExternals:   map[string]bool{"LoadRecord": true},
})
```

Leaving it empty is not a silent downgrade to something that works: the template
binds a one-result call against a two-result Go function, and the generated file
does not compile. `templates/sqlbind` takes the same field for SQL statements.

**Reading what generation wants to tell you.** `routetree.GenerateTree` returns
`Result.Deprecations`, one entry per route with a path and a message. Nothing is
logged or printed — you own the output, so you own how a warning reaches whoever
ran the build. The typed `Load` rung is reported there today.

**Return a fragment, not a string, when the value is markup.** An external
declared `: html` renders as a subtree rather than as escaped text or trusted
raw bytes — so a framework can return a whole element instead of a bare value,
and it goes through the ordinary context checks:

```
<header>
  {LocaleBanner()}
  <h1>{title}</h1>
</header>
```

```go
func LocaleBanner(ctx context.Context) htmlbind.Fragment {
	return banners.Locale(banners.LocaleParams{Tag: localeFrom(ctx)})
}
```

That half already worked before this round; only the context was missing.

#### Where the context comes from

It is the render's own context — the `ctx` argument of an async entry, or what
`WithContext` supplied to a synchronous one — read at the position the call
occupies. Inside an await or live boundary subtree that is the boundary's
context, so work started by a live delivery is bounded by that delivery rather
than by the original render.

A render that supplied no context still has one, so unlike a registered builtin
element's provider, a context-taking external can never fail for want of it.

#### What it costs in generated code

Nothing, unless you use it. A plan instruction holding such a call takes the
context as a leading closure parameter and is named for it:

```go
// Without a context-taking external — unchanged from before this existed:
planPageOps.Text(func(p PageParams) string { return SiteName() }),

// With one:
planPageOps.TextCtx(func(ctx context.Context, p PageParams) string { return RequestID(ctx) }),
```

The forms are `TextCtx`, `RawCtx`, `AttrCtx`, `BoolAttrCtx`, `IfCtx`, `SlotCtx`,
`ComponentCtx`, and the package-level `ForCtx`. Selection is per expression, so a
template mixing both kinds emits both kinds.

#### The one position that refuses it

Awaiting a value the caller started — `{await v = handles[Which()].count}` —
lowers its unset check to a `Require` instruction, which runs before anything is
written precisely so a caller who left a value unset still gets an error status.
A check that could call out is a different thing, so a context-taking external
there is a generation error naming the position.

#### What this is not

It is not the registered builtin element seam described later, and it does not
replace it. An external hands the value to the template, which may then
interpolate it anywhere; a builtin element renders it and never puts it in
template scope. The first is cheap and general, the second is checkable. A
framework that wants both is asking for the right thing.

## The document head

Component head declarations are merged into the single root head before the first
body byte, deduplicated by tag, with the declaring component recorded for each.
None of that changed. What changed is who else may contribute, and what a
contribution may be.

### What shipped: contributions from the render call

A component declares what it can know at generation time. A caller knows things a
template cannot: a title taken from the record the page just loaded, a marker
emitted only while some cookie is absent.

```go
err := htmlbind.RenderChain(w, chain, page,
	htmlbind.WithHead(
		htmlbind.HeadTitle(order.Customer),
		htmlbind.HeadMeta(
			htmlbind.HeadAttr{Name: "name", Value: "description"},
			htmlbind.HeadAttr{Name: "content", Value: order.Summary},
		),
	),
)
```

Five node kinds are available — `HeadTitle`, `HeadMeta`, `HeadLink`,
`HeadScript`, `HeadNoScript` — and they are values, not markup. A caller cannot
introduce an element by supplying a string, and every value is escaped for its
position.

Four properties are worth pinning down:

**They arrive strictly before the head pass.** That is the same ordering
component contributions already satisfy, so nothing about streaming changes and
no body byte is buffered.

**They merge as the innermost contributor.** Component contributions come first,
so a caller's tag may depend on one. A tag a component already declared is not
written twice.

**A malformed node fails before the first byte**, so the response can still carry
an error status.

**`HeadScript` requires a `src`.** An asset is a reference to something served.
No path through this package writes inline script, which is what lets a policy
keep `script-src 'self'` with no nonce.

This is a channel for the caller, not a way into the byte stream. Nothing
supplied here reaches template scope, and a component cannot read it. An author's
document shell stays an author's document.

#### On a response with no shell

A fragment response has no head to merge into. Rather than discover that in a
browser, ask:

```go
tags, err := htmlbind.RenderHeadNodes(nodes)
```

You get the same ready-to-write tags without a render, so you can put them in
your navigation payload, reject the response, or drop them deliberately.

### What shipped: `noscript` in the head

A browser with scripting disabled is not a crawler and no `User-Agent` says so,
so a page that wants to hand such a client to a scriptless path has one
conforming place to say it: a `noscript` refresh in the head.

Head contributions previously accepted `link`, `meta`, `style`, `script`, and
`title`. `noscript` is now accepted in both the authored set and the caller set:

```html
<head>
  <noscript><meta http-equiv="refresh" content="0; url=/_handoff"></noscript>
</head>
```

```go
htmlbind.WithHead(htmlbind.HeadNoScript(htmlbind.HeadMeta(
	htmlbind.HeadAttr{Name: "http-equiv", Value: "refresh"},
	htmlbind.HeadAttr{Name: "content", Value: "0; url=/_handoff"},
)))
```

Its children are `link`, `style`, and `meta` only — anything else there is body
content. It is the only contributed element with element children; every other
one accepts static text alone.

The authored form is unconditional, which is right for a page that always wants
it and wrong for a handoff that should stop once its cookie is set. That
conditional case is exactly what the caller channel above is for.

### Attribution

`Head` and `HeadSources` are parallel lists on both `Fragment` and `Wrapper`:
index *i* of either describes the same tag, and a source reads
`ComponentName (file:line:col)`. When you reject a contribution you cannot
deliver, name the component instead of printing the markup.

## Assets

Component `<style>` is scoped and hoisted; component `<script>` is extracted to a
content-hashed file and referenced from the merged head. Both are written under
`PublicDir` and referenced under `PublicURLBase`, and both are deterministic:
identical input regenerates identical names and bytes.

You serve the directory. tinybind ships no static file server, so one route for
the whole generated asset directory is the manual step.

This covers a component declared in a template file of the generation unit being
compiled. It does not cover a component a *library* supplies — see below.

## What is specified but not built

These are designed, agreed, and unimplemented. They are listed with what each
would give you so you can decide whether to wait or work around.

### Registered builtin elements

Markup your framework implements, callable by name from any template in the
generation unit, with no import and no per-page declaration:

```html
<csrf-token />
<pw-noscript-handoff />
```

**These are not components**, and the distinction is load-bearing rather than
pedantic. A `component` is PascalCase, declared in a template file, and reached
across files through an `external` declaration that restates its contract. A
builtin element is kebab-case, registered on the generate command, ambient across
the whole generation unit, and lowered into the plan steps of whatever component
contains it rather than becoming a component of its own. If you have seen this
called *framework-provided components* elsewhere, it is the same seam under a
name that reads more naturally from the framework side.

The hyphen is what makes the space available: `rule:template-name-casing` gives
kebab-case to real HTML elements, and a hyphen is the HTML custom-element marker,
so a bare `csrf-token` sits in the custom-element space and can never collide
with a standard element.

Registration is a whitelist passed to the generate command — a name-to-Go-symbol
map, static, needing no reflection and no init ordering — so a project that
registers none generates what it generates today. An element with no per-request
value folds entirely into static bytes and costs nothing at render time; one with
a value calls a context-taking provider function at its plan step.

The whitelist has a second entry kind: a passthrough name or glob pattern, so an
application can list the Web Components it uses — `sl-*` for a whole library —
and have them emitted verbatim. Without it, closing the hyphenated space to
registered names would ban Web Components outright.

Three properties are only available this way, and are the reason it is not sugar
over an external:

- **The value never enters template scope.** The element renders the token; an
  external hands it to the template.
- **Placement is checkable.** A head-only contribution written in the body
  becomes a generation error rather than a page that half works.
- **The dependency is declared.** An element that reads a cookie makes the whole
  response vary on it. A declared vary axis is what lets you build a `Vary`
  header for something no template mentions, and what lets an output cache refuse
  to store what it cannot key.

### Component assets

This row does mean *components* — a date picker, a chart, a sortable table: a
PascalCase declaration in a template file, with a script beside it. It is a
different actor from the row above, where the framework registers an element it
implements in Go. The seam has to serve both, because a builtin element can need
a script exactly as a component can, but the case that has no answer at all today
is the library one.

A component library is a template plus a script plus some Go. Two of the three
have a home. A library owns no route, no scaffold, and no shell, so it cannot
reference its own file the way a framework references its runtime. It also has to
be reachable across packages at all, which is still an open question about what an
`external` declaration may name.

What this would add:

- an **embedded asset table** — bytes, digest, media type, emitted as generated
  Go, so nothing reads a filesystem at runtime and a TinyGo or wasm target works;
- a **statically known required-asset set**, folded through a chain the way
  `HasAwaitBlock` already is, so a document can carry every script a later
  delivery might need and nothing is fetched mid-swap;
- a **caller-supplied URL function**, generalizing the pattern where only you
  know the mount path, the cache policy, and whether you serve from memory or
  from disk.

With deduplication by digest, order independent of registration order, and no
inline delivery anywhere.

The middle item is the one that matters most and is hardest. A live delivery or a
fragment swap can insert a component whose script was not in the first render. If
the required set is only discoverable while rendering, you have to load scripts at
swap time — which is client design this module deliberately does not own.

### Render-time script contribution

Selecting a registered script by name for one response. The caller-head channel
above covers the external-URL case in the meantime: `HeadScript` with a `src` is
a per-response script tag today. What is missing is the registry that would let
you name a contribution the generator already hashed and wrote.

## Constraints the whole surface preserves

- **Escaping is unchanged.** Reading the request changes nothing about how a
  value is written. A context-taking external's result is escaped for its
  position exactly as any other expression is.
- **No inline script, anywhere.** Neither extraction, nor a contribution, nor a
  caller-supplied node ever emits an inline script block.
- **No cloaking.** Reading the request decides delivery, never content. The mode
  header never reaches template scope, and no template can branch on it.
- **No reflection, no filesystem read at runtime, no init-order dependency.**
  TinyGo and wasm targets stay viable.
- **No `net/http` in `htmlbind`.** The rendering side has no opinion about
  transport, and gained none from any facility here.

## Reporting a gap

Three rounds of downstream requests are recorded in this repository's
`.knowledge` catalog: generation seams, live integration, and the component seams
this document's shipped items came from. Each round was checked against the
released source before it was accepted, and each recorded what was refused and
why alongside what was taken.

The most useful report names the feature that hit the seam, not only the seam.
Two of the four asks in the last round turned out to be designs this catalog
already held; what moved them was that a framework arrived at them from three
features in one week.

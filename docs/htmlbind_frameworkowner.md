# htmlbind for Framework Owners

This guide is for people building a web framework **on top of** tinybind, not for
people building an application with one. It covers the parts of `htmlbind` an
application author never touches: the boundary wire protocol, how to decide what
client runtime a response needs, and what is still yours to implement.

For the authoring language, component syntax, and ordinary rendering, read
[htmlbind.md](htmlbind.md) first. Nothing here repeats it.

## What the module owns and what you own

`htmlbind` deliberately stops at the response body. It has no `net/http`
dependency, writes no headers, serves no files, and makes no routing decision.

| Concern | Owner |
| --- | --- |
| Render plans, escaping, slots, chain composition | module |
| Await boundary identifiers, placeholder markup, completion payloads | module |
| Head contribution merging and deduplication | module |
| Putting request-scoped values into the context | you |
| Response status, content type, encoding, flushing policy | you |
| Navigation, history, and any SPA behavior | you |
| Framing of every completion, and the client script that applies it | you |
| When to open a live connection, how to re-establish it, and what to do when it cannot be | you |
| Bounding how long a live response stays open, and closing it | you |
| Making a builtin element's provider a read of session state rather than a mint | you |
| Creating, storing, and destroying the CSRF token, and verifying it | you |
| Putting a redraw's head contributions in the document shell | you |

That last row is broader than it may look. `htmlbind.Content` carries a boundary
id and rendered HTML, and nothing else — deliberately, so that a framework can
put it in a streamed document, a JSON payload, or anything else it invents. The
module writes no `<script>` on any path and injects nothing into the merged head,
so what a completion looks like on the wire is a decision you make once and pair
with the runtime you ship.

The last three rows are newer, and each is a place where the module built the
mechanism and deliberately stopped short of the policy. They are collected in
[what you have to do yourself](#what-you-have-to-do-yourself) below, because each
one is a thing that silently does nothing until you wire it.

## Deciding whether a response needs a client runtime

A response only needs the script that applies settled boundaries if something in
it can open one. Ask the values you already hold:

```go
page := pages.Home(pages.HomeParams{})

if page.HasAwaitBlock() {
	// this response will stream boundaries
}
```

`HasAwaitBlock` is available on both `Fragment` and `Wrapper`, and there is a
chain form that unions the members:

```go
document := pages.BindDocument(pages.DocumentParams{Title: "Home"})
layout := pages.BindLayout(pages.LayoutParams{})
page := pages.Home(pages.HomeParams{})

if htmlbind.HasAwaitBlock([]htmlbind.Wrapper{document, layout}, page) {
	// one decision for the whole chain, so one runtime tag
}
```

Three properties of the flag decide how far you can lean on it.

**It is transitive.** Generation walks the component call graph, so a component
that merely calls an async one reports `true` without declaring `await` itself.

**It renders nothing.** The flag is a constant on the generated plan, copied onto
the bound value. Reading it starts no goroutine and consumes no sequence, so you
can ask before choosing an entry point.

**It does not see fragments you passed in.** A `Fragment` handed to a component
through its parameter struct is not counted, because the binder cannot look
inside a caller's parameter struct without reflection. That fragment is in your
hand, so union the flag across the values you built:

```go
sidebar := pages.Sidebar(pages.SidebarParams{})
home := pages.Home(pages.HomeParams{Sidebar: sidebar})

needsRuntime := sidebar.HasAwaitBlock() || home.HasAwaitBlock()
```

The chain helper already does this for the ordinary document–layout–page shape.

## The boundary wire protocol

A progressive render writes each pending boundary as a placeholder holding its
fallback:

```html
<tb-boundary id="tb-1" style="display:contents">…fallback…</tb-boundary>
```

That element and its id are the module's, but not its name: `WithBoundaryPrefix`
renames both, so `pw` yields `<pw-boundary id="pw-1">`. Pass the prefix the
generator wrote the instance attributes with, or one document carries two naming
systems.

Everything after the placeholder is yours: a settled boundary arrives as a
`Content` holding the rendered fragment and the id of the placeholder it
replaces, and `Content.WriteTo` writes that fragment alone.

So what follows is a recommendation the module does not enforce — but it is the
shape the module was designed around, and the marker rule below is load-bearing.
Append each completion as an inert template followed by a marker:

```html
<template data-tb-boundary="tb-1">…resolved…</template><tb-apply for="tb-1"></tb-apply>
```

which on the Go side is:

```go
func writeCompletion(w io.Writer, content htmlbind.Content) error {
	if _, err := io.WriteString(w, `<template data-tb-boundary="`+content.BoundaryID+`">`); err != nil {
		return err
	}
	if _, err := content.WriteTo(w); err != nil {
		return err
	}
	_, err := io.WriteString(w, `</template><tb-apply for="`+content.BoundaryID+`"></tb-apply>`)
	return err
}
```

The contract a conforming client script must honor:

- **Trigger on the marker, never on the template.** This is the one rule that is
  not a style preference; [Why the marker exists](#why-the-marker-exists) gives
  the failure it prevents.
- Read the boundary id from the marker's `for` attribute.
- Replace the element whose `id` matches with the template's content.
- Remove both the marker and the template afterward.
- Apply each boundary at most once. Completions arrive in settle order, not
  document order.
- Tolerate a missing template or placeholder by doing nothing. A truncated
  response must leave the committed fallback visible.

Depart from this shape when the transport differs — a navigation response
carrying completions as JSON has no parser to trigger a marker — but keep the
rule that nothing acts on a fragment before the bytes that carry it are complete.

### Why the marker exists

An HTML parser inserts an element when it reads its **start** tag. A runtime that
reacted to the template appearing could therefore read a template whose content
had not arrived yet, replace the placeholder with nothing, and remove the
template — losing the fallback along with the result.

This is not hypothetical. It was observed once the template's start tag landed in
its own network chunk. It is invisible in development, because a small completion
arrives in one chunk and parses in one task; it appears only once a proxy, a TLS
record boundary, or a compressing encoder splits the bytes.

Because `<tb-apply>` comes after `</template>` in the byte stream, the template
is guaranteed complete by the time the marker exists, however the bytes were
chunked.

**The rule is about the trigger source, not the API.** A `MutationObserver`
watching for the marker is conforming — the marker cannot appear before its
template is complete. A `MutationObserver` watching for the template is not.
A custom element's `connectedCallback` is recommended because it runs during
parsing, making the swap as prompt as an inline script; an observer is correct
but one microtask later.

### A conforming client script

This is the reference runtime for the shape above:

```js
customElements.define("tb-apply", class extends HTMLElement {
	connectedCallback() {
		const id = this.getAttribute("for");
		this.remove();
		const template = document.querySelector(`template[data-tb-boundary="${id}"]`);
		if (!template) return;
		const placeholder = document.getElementById(id);
		if (placeholder) placeholder.replaceWith(template.content);
		template.remove();
	}
});
```

Put it in the bundle your pages already load rather than inline in a completion.
Then no completion carries a script of its own, and a page can run under a policy
that forbids inline script, with no nonce and no `unsafe-inline`.

### Getting the script onto the page

That part is yours too. The module used to prepend this runtime to the merged
head itself; it no longer does, so the decision and the injection now sit in the
same place. `HasAwaitBlock` is how you make the decision, and a `script` tag your
document shell emits — as a head contribution on the shell component, or as
literal markup in its template — is how you act on it.

A response that never loads the script is not broken, only unimproved: every
placeholder keeps the fallback it committed, which is also what a client without
JavaScript sees.

## Initial load

Nothing special is required. The shell writes the merged head and your runtime
tag, the initial pass commits the document with every fallback in place, and
completions stream after:

```go
for content, err := range htmlbind.RenderChainAsync(ctx, w, wrappers, page) {
	if err != nil {
		log.Printf("boundary failed: %v", err)
		break
	}
	if err := writeCompletion(w, content); err != nil {
		break
	}
	htmlbind.Flush(w)
}
```

There is no entry point that hides this loop, and that is deliberate: how many
boundaries a render produces is not knowable up front, least of all for a chain
assembled per request, so a streaming handler has to be written against the
sequence anyway.

Once the initial pass flushes, the status code is committed. A later failure is
for logging, not for rewriting the response.

## Live boundaries

A boundary that binds a `live` source replaces the same region every time its
source yields. Two things about the wire protocol change, and one thing you have
to decide.

The framing changes first. A completion on the initial response is markup an
HTML parser is consuming, which is why the template-and-marker shape exists at
all. A delivery after the document is complete has no parser reading it, so the
marker rule has nothing to trigger and the framing buys nothing. Send a record
instead:

```go
_, err := w.Write(content.AppendJSON(nil))
```

which produces `{"id":"tb-1","html":"…"}` with the fragment escaped for a script
context as well as a JSON one. Appending is what lets you build a framed record
without a second buffer — `content.AppendJSON([]byte("data: "))` for an event
stream, for instance. The framing around the record is still yours, because it
has to match the client that reads it.

The identifiers change second. Boundary ids name a position in the render tree,
not an allocation order: a boundary nested inside another one is `tb-1-1`, and a
live boundary's subtree hands out the same ids on every delivery rather than
minting new ones. Your client therefore replaces the same element repeatedly, and
a long-lived subscription does not accumulate placeholders nothing will ever
fill. The runtime cancels a superseded delivery's nested work before those ids
are handed out again, so a slow nested boundary cannot settle into the
replacement's placeholder.

What you decide is what a live connection does when it is re-established. The
same page executed again produces the same ids, so an id your client does not
already hold means the structure itself changed — a panel added to a dashboard
someone has been watching, say. Reconciling that correctly means placing a new
boundary in a document your client did not render, which is the navigation
problem rather than the reconnect one; doing it approximately puts the panel in
the wrong place. Stop the connection and tell the user to reload. A plain
`alert()` is a defensible first implementation, because the case is rare and
being wrong on screen is worse than being blunt.

Two behaviours are worth knowing before you write that client.

A boundary on `RenderAsync` takes one delivery and unsubscribes, so an initial
load with live regions still finishes and still shows real content rather than a
loading state. Nothing about the initial response tells your client that more is
coming, though — `htmlbind.HasLiveBlock` does, over the same chain and before
rendering starts:

```go
if htmlbind.HasLiveBlock(wrappers, page) {
	// this screen will keep changing; the client should open a live connection
}
```

Ask it, and a screen that will never change again costs no speculative request.
It is a subset of `HasAwaitBlock`, which stays the question of whether the
response needs the boundary runtime at all.

And a quiet source cannot hold a response open. `WithAsyncTimeout` bounds how
long a boundary may show nothing on the entries that must answer, and running out
leaves the committed fallback rather than rendering `recover`, because a source
with nothing to say yet has not failed. `RenderChainLive` applies no such bound.

## When a boundary without recover fails

When the bindings of an `await` block that declared no `recover` clause fail, the
sequence yields a `*htmlbind.UnrecoveredError` and ends. It carries the
`BoundaryID` of the committed placeholder and the original Go error.

The template has nowhere to put that failure, so the failure leaves the boundary
and arrives here instead. What is on screen is a "Loading…" placeholder, and
nothing is coming to replace it. **Replace the whole document and show an error.**
A template that wants one part of the page to fail visibly writes a `recover`
clause; for a block that did not, returning the single fact that the page failed
beats patching up a screen whose author never considered what is missing from it,
and letting someone go on using it. All the more so because the sequence ends
here: any other outstanding boundary keeps its fallback forever too.

```go
for content, err := range htmlbind.RenderChainAsync(ctx, w, wrappers, page) {
	if err != nil {
		var unrecovered *htmlbind.UnrecoveredError
		if errors.As(err, &unrecovered) {
			log.Printf("boundary %s failed: %v", unrecovered.BoundaryID, unrecovered.Err)
		} else {
			log.Printf("render failed: %v", err)
		}
		writeFailureScreen(w) // the initial pass is committed: replace, do not rewrite
		htmlbind.Flush(w)
		break
	}
	if err := writeCompletion(w, content); err != nil {
		break
	}
	htmlbind.Flush(w)
}
```

What `writeFailureScreen` writes is a shape you choose, like the completion
framing. One marker is enough:

```html
<tb-failed></tb-failed>
```

```js
customElements.define("tb-failed", class extends HTMLElement {
	connectedCallback() {
		this.remove();
		document.body.replaceChildren(failureScreen());
	}
});
```

The status code was committed when the initial pass flushed, so this replaces the
screen rather than rewriting the response. A truncated response carries no such
marker either, so nothing happens there and the committed fallbacks stay — the
existing rule.

Do not build the error screen's text from the server's error.
`UnrecoveredError.Err`, like what `WithErrorReporter` receives, is the raw Go
error, and putting it on the page leaks the server's insides. To show a code or a
message, put only a `PublicError` projection on the marker's attributes — and note
that the reporter is called concurrently from each boundary's goroutine, so
aggregating several failures needs a lock of your own.

### The synchronous entries

`Render` and `RenderChain` return the same `*UnrecoveredError`. They write no
fallback in its place, so no finished-looking document holding a loading state
that will never resolve comes out of them.

What you get for that is a response that has committed nothing yet. Render into a
buffer and you can drop the buffer and answer with an error status; write straight
to an `http.ResponseWriter` and the bytes before the failing boundary are already
out. Buffer any render you might want to turn into an error response — a
navigation response, or a page you do not stream.

## SPA-style navigation

This is the least finished area. Read this section as a description of the
available pieces rather than of a supported feature.

**What works today.** A navigation response is an ordinary render of a chain that
omits the document shell, because only the shell emits the head. So the body
comes out on its own with no extra API:

```go
// no document shell in the wrapper list, so no <html>, <head>, or <body>
err := htmlbind.RenderChain(w, []htmlbind.Wrapper{layout}, page)
```

And the head that render *would* have merged is available without rendering:

```go
tags := htmlbind.MergeHead([]htmlbind.Wrapper{document, layout}, page)
```

`Fragment.Head()` and `Wrapper.Head()` give one member's own contributions,
already scoped and ready to write.

**What you have to build.** Everything about applying that to a live document.
The browser will not replace the previous page's `title`, `meta`, `link`, and
`script` for you, and the document also holds tags your application added by
hand, which a navigation must not evict. That means you need a way to tell your
tags from everyone else's, and a diff rather than a clear-and-rebuild, so a
stylesheet present before and after is neither refetched nor flashed.

**What is still in design.** Marking merged head tags with an ownership attribute
and a content-derived id, so a client can diff by identity instead of by
serialized markup; a navigation response shape carrying the head change; and
document-lifetime unique boundary identifiers. Today boundary ids are numbered
per render (`tb-1`, `tb-2`, …), which is fine for a full page load into an empty
document but will collide once a navigation inserts boundaries into a document
that still holds earlier ones. If you are building navigation now, allocate your
own ids on top rather than assuming these are unique over a document's lifetime.

Also note that a completion applied during navigation cannot use the marker
mechanism at all: no parser runs over a navigation response body, so there is
nothing to connect. Apply by boundary id instead, after you have installed the
new content.

## Head merging

Contributions merge outermost first, and later duplicates drop, so two
components declaring the same stylesheet emit one tag. Identity is the exact
merged string.

One limitation to know about: `Bind` copies only the plan's own contributions, so
a `Fragment` passed into a component through its parameter struct does not
contribute its head to the merged document head. Components called statically
inside a template are fine — the compiler folds their contributions into the
calling plan. The gap is specific to a fragment supplied at runtime through a
parameter, which is exactly the cross-file composition case. Until it is fixed,
a framework composing slots that way should merge those fragments' `Head()`
values itself.

### Contributing from the render call

A component declares what generation can know. For a tag you decide per response
— a title from the record you just loaded, a marker emitted only while a cookie
is absent — supply it at the call:

```go
err := htmlbind.RenderChain(w, chain, page,
	htmlbind.WithHead(
		htmlbind.HeadTitle(order.Customer),
		htmlbind.HeadNoScript(htmlbind.HeadMeta(
			htmlbind.HeadAttr{Name: "http-equiv", Value: "refresh"},
			htmlbind.HeadAttr{Name: "content", Value: "0; url=/_handoff"},
		)),
	),
)
```

The nodes are values rather than markup, so nothing you pass can become an
element, and every value is escaped for its position. They merge after every
component contribution, through the same deduplication, and they are in hand
before the head pass — so streaming is unaffected and no body byte is buffered.
A malformed node fails the render before the first byte.

`HeadScript` requires a `src`: no path through this package writes inline script.

On a response with no document shell there is no head to merge into.
`htmlbind.RenderHeadNodes(nodes)` gives you the same ready-to-write tags without
a render, so you can carry them in a navigation payload or refuse the response
rather than discover the loss in a browser.

The full inventory of what is available to a framework, and what is specified but
not yet built, is [framework facilities](httpbind_framework_facilities.md).

## Generator integration

A framework builds its own generate command rather than shipping tinybind's. The
call registry lets you name your own wrappers so discovery works against your
API instead of tinybind's:

```go
calls := generator.NewCallRegistry()
if err := calls.Register(generator.ConfigBindCall(
	generator.Function("example.com/framework", "RegisterConfig"),
	generator.GenericType("config", 0),
	generator.Argument("prefix", 1),
)); err != nil {
	return err
}
options, err := calls.Options(generator.DefaultOptions())
```

Generation runs one package directory per invocation — there is no recursive or
multi-directory mode — so any identity your framework derives at generation time
must not depend on a per-run counter. Two directories are two processes.

`Options` also carries the template file patterns, the SQL API shape, and the
feature switches that let a framework turn off generation phases it does not use.

Component styles and inline scripts are extracted into content-hashed files under
`PublicURLBase`, and the head carries reference tags rather than inline blocks, so
a policy that forbids inline style works. `Result.Assets` is what your build
writes. `htmlbind.MergeAssets(wrappers, leaf)` is the same set as identities, read
from the bound value before rendering starts, for deciding what a document shell
must already carry.

### Builtin elements

A framework can register markup an author writes by name:

```go
options.BuiltinElements = []htmlbind.BuiltinElement{{
	Name:   "csrf-token",
	Markup: `<input type="hidden" name="{{.FieldName}}" value="{{.Token}}">`,
	Vary:   []string{"Cookie"},
	Provider: &htmlbind.ElementProvider{
		Package: "example.com/framework",
		Name:    "TokenFor",
		Result:  "CSRF",
	},
}}
```

An author then writes `<csrf-token />` in any template of the generation unit,
with no prefix, no import, and no per-file declaration.

The reason this is a seam of its own rather than sugar over an external call is
what does *not* happen: **the value never enters template scope.** No name is
bound to it, so it cannot be interpolated somewhere else, put in a query string,
or logged. An external returning the same token hands it to the template, and
everything a template may do with a value becomes possible. Both seams exist and
neither replaces the other.

Generation rewrites the element. The fixed part of the markup folds into the
surrounding static run, so it costs what typing it would; what remains is one
plan step calling `TokenFor(ctx)` and writing each hole. A definition with no
provider and no expression attribute reduces entirely to static bytes and adds no
step at all.

Each hole is escaped for its position, and a hole may only sit in element text or
a quoted attribute value. A token containing a quote cannot break out of the
attribute it sits in — which is the property that would be lost if a framework
built the same markup as a trusted string.

Three things follow from a declared provider, and all three are derived rather
than declared, so a registration cannot disagree with itself:

- **It is per-request.** A component reaching one cannot be `@cache`d — a stored
  body would serve one visitor's token to the next, which is a security failure
  rather than a staleness bug. The exclusion follows the call graph.
- **It needs a context.** Rendering with none fails naming the element, rather
  than rendering the absence of a value.
- **The provider may fail.** During the initial pass that is before the response
  commits, so you can still choose an error status.

`Vary` is the one thing you must declare, because only your implementation knows
what its provider reads. It reaches the bound value as `Fragment.Vary()` and
`htmlbind.MergeVary(wrappers, leaf)` — a response depending on a cookie says so
nowhere else, and neither a `Vary` header nor a cache key can be built from a
dependency nothing reports.

The provider signature is `func(context.Context) (V, error)`. Nothing here checks
it: as with a context-taking external, you read your own Go sources and the Go
compiler rejects a mismatch. `Result` names `V` because a hole closure has to be
written down and Go infers a call's type arguments but never a function literal's
parameter types.

### The hyphenated element space is closed

A hyphen is HTML's own custom-element marker, so registering builtins means
deciding what happens to every *other* hyphenated element. The answer is that the
space is a whitelist: anything in it is declared, and anything else is a
generation error naming the file, line, and column.

That removes the case a prefix was supposed to solve. `<csrf-toekn />` today is
markup a browser ignores and nothing reports; with the space closed it is a
compile error suggesting `<csrf-token>`.

Your application's Web Components go in the same whitelist, by name or by a
prefix glob:

```go
options.PassthroughElements = []htmlbind.PassthroughElement{
	{Name: "sl-*"},        // a whole component library, declared once
	{Name: "my-widget"},
}
```

A passthrough element is emitted verbatim and produces no plan step. A builtin
name always wins over a glob that happens to cover it; the reverse never happens.
Hyphenated names inside `<svg>` and `<math>` are standard foreign-namespace
elements and stay outside the whitelist entirely.

**This is the one behavior change for an existing project.** A project already
writing Web Components adds passthrough entries once, and the diagnostic names
every element it has to declare. A project writing none is unaffected and
regenerates byte for byte.

Two things from the design are not built. The **opaque shape** — a provider
returning a trusted value or a fragment, for output whose *structure* varies
rather than only its values — is deferred, because its cost is that the trust
assertion moves into your code and generation can no longer verify the emitted
structure. And a builtin element in a **`<head>` declaration** is still refused
by the head validator, so `Placement: PlaceHead` today means "refuse this in the
body" and not yet "accept it in the head".

### Reading a component's signature

If you generate code around templates rather than replacing tinybind's, you need
component parameters in **Go** terms. Reimplementing that mapping would put you
one release behind the compiler, so it is exported:

```go
sigs, err := htmlbind.Signatures("page.tb.html", source)
page, ok := htmlbind.Lookup(sigs, "Page")
// page.Parameters[0] == {Name: "orders", GoType: "htmlbind.Pending[[]Order]", Async: true}
```

It runs the same analysis `Generate` does, so a module that would not compile
fails here with the same diagnostic instead of yielding a partial answer. That is
what the filesystem router in
[httpbind_discovered_router.md](httpbind_discovered_router.md) reads to decide
what a generated handler must decode.

### Resolving a server action

A template can name a Go handler instead of a URL:

```html
<button server-action="Rename" data-target="#name">rename</button>
```

The compiler cannot lower that on its own. A URL depends on where the handler is
mounted, and the module knows nothing about routing, so resolution takes two
passes with you in the middle:

```go
refs, err := htmlbind.ActionRefs("page.tb.html", source)
// refs[0] == {Component: "Page", Handler: "Rename", Element: "button", Pos: ...}

out, err := htmlbind.Generate("page.tb.html", source, htmlbind.GenerateOptions{
	Package:          "id_",
	ServerActions:    map[string]string{"Rename": "/_action/00369cf962b6/Rename"},
	ServerActionAttr: "hx-post",   // optional; defaults to data-tb-action
})
```

`ActionRefs` reports what a module references, with a position for each so your
diagnostic can quote the template. You resolve those names against whatever
package the template belongs to, and hand the answers back through
`ServerActions`. A reference you leave unresolved is a compile error rather than
a silently dead element.

`ServerActionResolver` answers what the map does not hold, for a name you would
rather resolve on demand than enumerate:

```go
	ServerActionResolver: func(name string) (string, bool) { ... },
```

The map wins over the resolver, so adding one cannot retarget a name you already
supplied, and the unresolved-name diagnostic names both sources once it is set.

The lowering is deliberately thin. `server-action` becomes one attribute carrying
the URL, and every other attribute on that element survives unread — which is
what leaves `data-target`, `hx-swap`, or anything else to mean whatever your
client runtime decides. `ServerActionAttr` is there so a generated action can
drive a library you already use instead of one of ours.

`GenerateOptions.ContextExternals` works the same way and is the precedent worth
noticing: both are template facts only the caller can settle, resolved by reading
the Go package between passes.

The discovered router does all of this for you. What it derives — the hash, the
endpoint path, which handlers are exposed — is described in
[httpbind_discovered_router.md](httpbind_discovered_router.md).

## What you have to do yourself

Everything below is a mechanism the module ships and a policy it does not. Each
one works without you, and each does something slightly wrong until you wire it.
They are gathered here because a seam that silently degrades is worse than one
that fails, and none of these fails.

### Bound how long a live response stays open

The module never closes a healthy live response on its own. It has no opinion on
how long a subscription may live, how many one session may hold, or how long a
boundary may sit idle — those are budgets only your deployment knows.

What it gives you is the record that says a close was healthy:

```go
if time.Since(opened) > lifetime {
	return stream.Retry(0)          // or Retry(d) to spread the return yourself
}
```

A client reading `retry` reconnects promptly and **spends no attempt**, where a
close it reads as a fault backs off exponentially. So an unbounded response is not
merely long-lived: it also never re-checks authorization, never rolls onto a new
deploy, and never rebalances, because those are the things a bounded lifetime buys.

The one case the module does handle is its own: a cancelled request context —
a shutdown, a rolling deploy, a deadline you set — closes `retry` rather than
`done`. Without that, a deploy would tell every open screen its sources had
finished, and they would sit frozen until somebody reloaded.

`retryMs` is yours to fill and only you can. A client's backoff reacts to failure;
you are the only party that knows you are shedding load or mid-deploy, and so the
only one who can spread the return **before** anything fails.

### Make a provider a read, not a mint

**A provider must return the same value for the same session.** Calling it once,
twice, or never has to yield one answer. The usual shape is that your middleware
puts the value in the context and the provider takes it out — a map lookup, not a
round trip.

This is a requirement rather than a performance note, and the reason is the second
channel. A CSRF token reaches the browser twice: in a response header for script
to read, and in a hidden input so a form still works without script. **A header
carries one value.** Two forms on a page holding two different tokens is a bug
nobody observes until one of them is submitted, and then it is a failed submission
with no diagnostic attached.

So the module holds up its half: **a provider is called at most once per render,
and every occurrence shares the result.** The memo is keyed by the *provider*, not
by the element, so a hidden input and a meta tag backed by one function cannot
disagree either.

The scope is one render. A redraw and a live delivery call again, which is correct
— each is a separate response carrying its own header. A failure is never
memoized, because it ends the render and there is nothing left to share it with.

A provider that mints a fresh value per call breaks this, and nothing can detect
it: the module cannot tell a stable read from an unstable one, and the symptom
surfaces in a form submission rather than in a render.

### Own the CSRF token's lifecycle

Every unsafe form this module renders carries a hidden field, and the runtime
puts the same value in a header on every request it issues. **An author writes
nothing**, which is the point: a security control you have to remember to write
is one you will forget, and the omission renders a working page that fails only
on submission.

What is yours is the token itself:

```go
htmlbind.WithCSRFToken(csrf.FromContext(ctx))   // in your render entry, once
options.ScriptTagFor(token)                     // so the runtime sends it too
options.VerifyCSRF(r, csrf.FromSession(r))      // in your middleware
```

It is a render option rather than something read from the context because **this
module cannot read it from a context**: the key belongs to whoever owns the
session, and there is nothing here to look up. Passing it once inside your own
render entry means no handler changes.

Create it at login or session creation, store one per session, destroy it at
logout and at session regeneration. **Do not rotate per request.** A second tab's
form would carry a token the first tab's submission had already replaced, and you
would buy nothing for it: one value per session is also what lets the header and
the hidden field agree, and a header carries one value.

A render with no session behind it — a mail body, a static export, a golden test
— says `WithoutCSRFToken()`. That is explicit because the alternative, treating
an absent token as "none wanted", turns a forgotten option into a form that
submits and is rejected with nothing pointing at the cause.

**Origin and Fetch Metadata validation stay yours**, and are worth having. This
module never inspects where a request came from — that is a check on an inbound
request before any render, so it belongs to middleware, and Go's own
`http.CrossOriginProtection` is what you wrap handlers with. Run both: the two
defenses fail for unrelated reasons. A proxy that rewrites `Origin`, or a
credentialed CORS misconfiguration, removes the whole origin defense at once and
with no failing request to notice; the token does not share that failure. And a
sibling subdomain is `same-site` to both SameSite and `Sec-Fetch-Site`, so a rule
that only rejects `cross-site` lets it through.

If you have settled on origin checks alone, `CSRFMode: CSRFOff` turns the field,
the header, and the per-request marking off — which is what gives you back the
one thing the token costs:

### Split a cached list from an uncached form

**A component rendering an unsafe form cannot be `@cache`d.** A stored body would
serve one session's token to whoever asked next, which is a security failure
rather than a staleness bug, so it is a generation error and it follows the call
graph.

The composition this pushes you toward is the right one anyway: cache the list
that came from the database, leave the form uncached, and make them two
components.

```
@cache(ttl: "1m") component List(rows: string[])   ← cacheable
component Form()                                   ← carries the token
export component Page(rows: string[])              ← composes both
```

### Put a redraw's head in the document shell

A redraw swaps markup into a page the endpoint never rendered, so it cannot merge
into a head it owns. The response carries `X-Tinybind-Head` and the runtime
installs what is missing before swapping — but that is the repair, not the plan.
The plan is that the tags are already there:

```go
shell := layout.LayoutParams{Head: registry.RequiredHead()}
```

Do this and the header installs nothing on every redraw, because everything it
names is present. Skip it and every first redraw of a component pays a stylesheet
fetch mid-swap, which is a slower swap rather than a broken one — which is exactly
why it is easy to leave undone for a long time without noticing.

### Decide what a fragment response owes

`Fragment.Head()` and `Fragment.Assets()` report what a value needs, including
what a component handed in through a slot needs. A response with no document
shell has nowhere to put either. The module does not decide what that means:
dropping them silently and refusing the response are both defensible, and only
you know which your framework promises.

## Routing

Routing is not an `htmlbind` concern. The module writes a response body and stops
there, so neither router lives in it. The one that reads your registrations is
described in [httpbind_frameworkowner.md](httpbind_frameworkowner.md), and the one
that generates them from a directory of templates in
[httpbind_discovered_router.md](httpbind_discovered_router.md).

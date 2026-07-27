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

That last row is broader than it may look. `htmlbind.Content` carries a boundary
id and rendered HTML, and nothing else — deliberately, so that a framework can
put it in a streamed document, a JSON payload, or anything else it invents. The
module writes no `<script>` on any path and injects nothing into the merged head,
so what a completion looks like on the wire is a decision you make once and pair
with the runtime you ship.

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

That element and its id are the module's. Everything after it is yours: a
settled boundary arrives as a `Content` holding the rendered fragment and the id
of the placeholder it replaces, and `Content.WriteTo` writes that fragment alone.

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

Note that component styles and scripts are merged into the document head as
inline markup today. There is no extraction to files under a public directory, so
there is nothing for you to serve yet and no asset URL to configure. If your
framework needs external stylesheets for caching or for a policy that forbids
inline style, that is on you for now.

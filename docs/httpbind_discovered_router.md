# The Discovered Router

This guide is for people building a web framework **on top of** tinybind. It
covers the discovered router: how a directory becomes a route, how a mutation
reaches Go, and which parts stay yours.

For how this router relates to the ordinary registered one, and for what
generation can see in each, read
[httpbind_frameworkowner.md](httpbind_frameworkowner.md) first. For the rendering
side — boundary protocol, client runtime, head merging — read
[htmlbind_frameworkowner.md](htmlbind_frameworkowner.md). Nothing here repeats
either.

Every framework built on this module eventually writes the same loop: walk a
directory of templates, derive a URL from each path, register a handler. The loop
is easy. The constraints it has to respect are not, and two of them stay
invisible until the Go toolchain refuses to build a package you never touched.

`routetree` is that loop plus those constraints. It discovers, it generates, and
it stops. It holds no opinion about page metadata, sitemaps, or `robots.txt`;
those stay yours, and the route table below is what you build them from.

## What a page can be

A route is any directory under the root holding `page.tb.html`. That file alone
is enough: the generated handler decodes the URL, renders the component, and
writes the response, while the template's own `external` calls fetch whatever
data it needs.

Add `page.go` and Go runs between the request and the render. How much depends on
the signature:

| Files | Shape | What you get |
| --- | --- | --- |
| `page.tb.html` alone | template only | the whole handler is generated; the template's own `external` calls fetch the data |
| `+ page.go` with `func Load(id string, page int) (User, error)` | typed | the generated handler decodes, calls `Load`, and renders its results |
| `+ page.go` with `func Load(w http.ResponseWriter, r *http.Request)` | handler | only the registration is generated; you own the whole response |

Because the signature selects the shape, a `Load` matching neither is a
generation error that names what it has and the two contracts it could have had.

`Load` is an odd name for the entry point of a page, and `Page` was the first
choice. It does not survive contact with the compiler. The template compiler
already emits `func Page(params PageParams) htmlbind.Fragment` into that same
package, so a second `Page` beside it is a Go redeclaration. The file is still
`page.go` and the component is still `Page`; only the entry point moved aside.

One rule covers the inputs at both rungs. The leading parameters are the route's
dynamic segments, in route order, and everything after them is a query parameter
keyed by its own name. Without `page.go` that rule reads the component's
parameter list; with it, the `Load` parameter list. Moving a page up the ladder
therefore never changes how its inputs are spelled. Only scalars are accepted,
because a URL carries no object.

A query parameter may be optional, spelled as the pointer the template's own
optional marker already produces:

```text
export component Page(topic: string, page: int?): html { ... }
```

`page` arrives as `*int`: nil when the key is absent or its value is empty, and a
pointer to the parsed number otherwise. That is the only way to tell `?page=0`
from no page at all, and an unparsable `?page=x` still fails before rendering. A
path segment cannot be optional — a single segment is always present when the
route matches, and a catch-all binds an empty remainder as a string.

One thing does change. At the typed rung the component's parameter list becomes
`Load`'s return list, and generation checks it: a mismatch in count, order, or
type fails, naming both lists.

## Segment notation, and why it is not `[id]`

A trailing underscore marks a dynamic segment; two mark a catch-all.

```
pages/users/id_/page.tb.html      → GET /users/{id}
pages/files/rest__/page.tb.html   → GET /files/{rest...}
```

If you have used a file-based router before, you expect brackets, and the first
question is why these directories are not `users/[id]/`. The answer is not taste.
A route directory is also a Go package, and the toolchain rejects an illegal
import path element while matching package patterns — before it evaluates build
constraints. So a single `pages/users/[id]/page.go` does not break that package.
It breaks `go build ./...` for the whole module. `{id}`, `$id`, `@id`, `:id`,
`=id`, `(group)`, `-id`, and `~id` fail the same way, and discovery refuses them
up front with a message that explains why.

Exclusions follow the same authority. Discovery skips what the Go toolchain
already skips: a leading `_` or `.`, and `testdata`. A private-folder convention
comes free with that.

The root page is the other place where the obvious spelling is wrong. It
registers as `GET /{$}`, not `GET /`, because a bare `/` is a prefix pattern in
the standard library — it would answer every unmatched path instead of letting it
404.

## Layouts

`layout.tb.html` in any ancestor directory wraps every page below it, outermost
first. Two rules constrain what such a layout may be.

It must declare `children: html`. That is the shape the template compiler emits a
`BindLayout` wrapper for, so without it there is no binder and the generated call
would not compile. Discovery reports the missing declaration rather than leaving
it to the Go compiler.

It may read only the dynamic segments at or above its own directory. A layout at
`pages/users/` cannot read `id` from `/users/{id}`, because a wrapper that
depends on a deeper segment cannot be reused when that segment changes — which is
the property that makes an ancestor layout worth reusing at all.

## What lands where

```
pages/                       ← Config.Root, default "pages"
├── layout.tb.html
├── layout_gen.go            ← compiled Layout component
├── page.tb.html
├── page_gen.go              ← compiled Page component
├── route_gen.go             ← RouteParams + DecodeRoute for "/"
├── routes_gen.go            ← Register, NewServeMux, Routes, Actions
└── users/id_/
    ├── page.tb.html
    ├── page.go              ← optional func Load, plus any server actions
    ├── page_gen.go
    └── route_gen.go
```

Each template names its own generated file, so `page.tb.html` and
`layout.tb.html` in one directory do not fight over one output.

Registration lives in the root package rather than in the route packages. The
natural design puts a composer beside each page, and it does not work: the leaf
would import the root for its ancestor layout while the root imports the leaf for
its page. Go calls that a cycle. Composition therefore lives in the registry,
every generated import points down the tree, and no upward edge exists anywhere.

That constraint reaches the handler rung. A hand-written `Load` cannot call a
composer above it, so a handler wanting the layout chain builds one with
`htmlbind.RenderChain`. For a rung whose whole point is owning the response, that
is the right side of the trade.

## Server actions

A page is a `GET`, but a website is not. Something has to receive a form or a
button press, and a mutation target is not written as a URL here. A template
names an exported Go handler, and generation supplies the address:

```html
<button server-action="Rename" data-target="#name">rename</button>
```

```go
func Rename(w http.ResponseWriter, r *http.Request) { /* owns the whole response */ }
```

The attribute lowers to one that carries the endpoint, and every other attribute
survives untouched:

```html
<button data-tb-action="/_action/00369cf962b6/Rename" data-target="#name">rename</button>
```

That is the whole contract. `server-action` resolves a name to a URL and writes
it down; it models no client protocol at all, which is what leaves `data-target`
— or `hx-target`, or anything else — to mean whatever your runtime decides.
Point `Emitter.ActionAttr` at `hx-post` and a generated action drives HTMX with
no glue code.

What this buys over a hand-written `action="/users/42/rename"` is the compiler. A
URL is a string nothing checks against the handler it targets; a name is a symbol
that has to resolve. Rename the Go function and generation fails at the template
that referenced it.

The handler is an ordinary `http.HandlerFunc`, so it is testable with `httptest`
and needs no registration to exercise. Nothing is generated around it: what it
writes is the response, verbatim.

It reads its input the way any other handler does:

```go
func Rename(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[RenameRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	// ...
}
```

`Bind` dispatches through a binder generated for the package that declares the
request type, so the route package has to be one the generator was run over. The
loop for that is under [Running it](#running-it).

### Naming a handler the tree does not hold

A template outside the route tree — a classic-side page a framework compiles
itself — names a handler discovery never walks to. Supply the address and it
lowers like any other:

```go
files, err := routetree.Generate(routetree.GenerateOptions{
	Config: cfg,
	ActionResolver: func(name string) (string, bool) {
		url, ok := myRouteTable[name]
		return url, ok
	},
})
```

A handler exported by the template's own route package always wins, so adding a
resolver cannot silently retarget an action that already has an endpoint. A name
neither source answers is still a generation error, and it names both sources it
tried.

### What is reachable

Every exported handler-shaped function in a route package gets an endpoint,
whether or not a template mentions it. That sounds broad until you notice what a
route package is: a package imported by nothing but the generated registry, so an
exported symbol there is that route's surface rather than a general API.

Lower-case the function to keep it private. That is the opt-out, and it needs no
declaration at all, because generated code in another package cannot reach an
unexported symbol.

`Load` is excluded: it is the page's own entry point and merely shares the
signature at the handler rung.

The generated `Actions` table lists every endpoint, which is what makes that
surface inspectable rather than implicit. The path hides structure but grants
nothing — it is not a capability token — so each handler still authenticates and
authorizes its own caller.

### The address

`/_action/<hash>/<HandlerName>`. The hash is the first 12 hex characters of a
digest over the declaring directory and the handler name. There is no build salt,
so regenerating an unchanged project reproduces it, and a page held open across a
deploy still posts somewhere the server recognizes. The readable name rides along
so a network trace names the Go function that ran.

The declaring directory rather than the serving route path is what goes into the
digest, and that detail matters for layouts. A layout is compiled once but renders
under every page below it, so hashing a route path would give one handler a
different address per page and defeat the determinism the hash exists for.

`Emitter.ActionPrefix` moves the whole space. The default is safe for free because
discovery ignores directories beginning with an underscore, so no route tree can
ever produce `/_action`. A configured prefix has no such protection, so generation
rejects one that any discovered route occupies rather than letting it surface as a
`ServeMux` panic.

## Running it

```go
files, err := routetree.Generate(routetree.GenerateOptions{
	Config: routetree.Config{
		Root:       "pages",
		ImportBase: "example.com/app/pages",
	},
	RootPackage: "pages",
})
if err != nil {
	return err
}
return routetree.Write(files)
```

`ImportBase` has to be named because it cannot be derived: a directory does not
reveal its position inside a module. `Generate` itself writes nothing — it returns
the files and `Write` is a convenience — which is what leaves post-processing and
output redirection in your hands.

If any page or server action calls `httpbind.Bind`, generate the binders too. A
binder is generated per package from the `Bind` call sites inside it, so the tree
reports its packages and you loop over them:

```go
tree, err := routetree.Discover(cfg)   // or reuse the one you already discovered
// ... write the files from Generate first
for _, pkg := range tree.Packages() {
	_, err := generator.Generate(pkg.Dir, "", "tinybind_gen.go")
	if err != nil && !errors.Is(err, generator.ErrNothingToGenerate) {
		return err
	}
}
```

Order matters: analysis type-checks each package, so the tree's own generated
files have to be on disk first. Most route packages have nothing to bind — a
template-only page declares no request type at all — and that is the
`ErrNothingToGenerate` above rather than an empty generated file.

None of this puts a page into an OpenAPI document. The only registrations in the
tree are in the generated registry, and discovery skips what tinybind generated —
by header, so your own output name is skipped too. An HTML page is not a published
API contract, and an action endpoint is an implementation detail of one page.

An application then touches one symbol:

```go
mux := pages.NewServeMux()        // or pages.Register(existingMux)
```

`Register` takes `htmlbind.Option` values and passes them to every render, so a
per-request cache, timeout, or error hook is configured once. It installs the page
routes and the action endpoints together.

## The route table

The registry also emits what the filesystem knows, and deliberately nothing more:

```go
var Routes = []RouteInfo{
	{Pattern: "GET /{$}", Path: "/", Dir: "", Params: nil},
	{Pattern: "GET /users/{id}", Path: "/users/{id}", Dir: "users/id_", Params: []string{"id"}},
}

var Actions = []ActionInfo{
	{Pattern: "POST /_action/00369cf962b6/Rename", Path: "/_action/00369cf962b6/Rename",
		Dir: "users/id_", Handler: "Rename", Hash: "00369cf962b6"},
}
```

`Routes` is the seam for a sitemap or a route inspector. Patterns, methods, and
which segments are dynamic all come from the tree. The concrete values a dynamic
segment expands into are application data, so they stay yours.

Note what neither table is: an OpenAPI source. Page routes never enter an OpenAPI
document, because OpenAPI describes a published API contract and an HTML page is
not one; action endpoints stay out for the same reason, being implementation
details of one page. If your framework wants either documented, it adds them from
these tables through its own artifacts.

## Customizing the output

Most frameworks need only the first of three levels.

**Repoint the symbols.** Generated code calls the packages named in `Symbols`, so
aiming it at your own runtime needs no template at all:

```go
e := routetree.NewEmitter()
e.Symbols.RuntimeImport = "example.com/framework/render"
e.Symbols.RuntimeAlias = "render"
e.Symbols.ErrorImport = "example.com/framework/web"
e.Symbols.ErrorAlias = "web"
e.Symbols.BadRequest = "Invalid"
e.Symbols.Problem = "Fault"
```

The generated declarations rename the same way, through `ParamsType`,
`DecodeFunc`, `RenderFunc`, `RegisterFunc`, `MuxFunc`, `TableVar`, and
`ActionTableVar`. `ActionPrefix` and `ActionAttr` move the action URL space and
the attribute it is written into.

The router is its own pair of symbols, separate from the request package:

```go
e.Symbols.MuxImport = "example.com/framework/web"
e.Symbols.MuxAlias = "web"
e.Symbols.MuxType = "web.Router"          // written verbatim, so an interface needs no pointer
e.Symbols.MuxConstructor = "web.NewRouter" // empty omits the constructor function
```

They are separate because one alias supplying both the router and `Request` would
drag the request package along with the router, and a framework wanting only its
own router would be left replacing the whole registry. Generated code registers
through `HandleFunc` alone, so a one-method interface satisfies `MuxType`.

**Replace one block.** The built-in templates are compositions of named blocks,
so you can change one without owning the rest:

| Block | What it writes |
| --- | --- |
| `imports` | the import statement of any generated file |
| `convert` | one scalar read out of a raw string |
| `error` | every error value a decoder produces |
| `render` | the render call itself, in the registry and the composer |
| `handler` | one route's handler body in the registry |

```go
err := e.Parse("error", `web.Invalid(web.Fault{Code: {{ .Code | quote }}})`)
```

`render` is the one to reach for when your entry point takes more than a writer.
A `Symbols` change repoints a package but never a signature, so a framework whose
response entry needs the request — for bot-mode selection, compression, a document
shell, an error page — changes the call rather than its name:

```go
err := e.Parse(routetree.TemplateRender,
	`web.WriteHTML({{ .Writer }}, {{ .Request }}, {{ .Chain }}, {{ .Leaf }})`)
```

Everything in scope arrives by name — `Writer`, `Request`, `Chain`, `Leaf`,
`Options` — and the block writes an expression of type `error`, so what a failure
does stays with the caller. `Chain` is the layout chain, or `nil` for a page with
no ancestor layout; `Wrappers` is the same value left empty in that case, for an
override that would rather branch than pass `nil`.

`Request` is empty where none is in scope, which is the case in the composer:
its contract is a writer, because a handler rendering into a buffer to choose its
status has no response to hand over. Two settings change that when you want it:

```go
e.RenderWriterType = "http.ResponseWriter" // default io.Writer
e.RenderRequestParam = "r"                 // default: no request parameter
```

The composer imports what its own signature names, so a writer type qualified by
`io`, by the request package, or by your runtime needs no further configuration.
The same limit applies to the block itself: it may only name packages the file
already imports — your runtime and error packages, the request package, and `io`.
Pointing `Symbols.RuntimeImport` at the package holding your entry covers the
usual case, where that entry and the option type ship together.

This is the same argument the `error` block already made: the entry that writes a
failure took `(w, r)`, and now the entry that writes the page can too.

**Replace a whole file.** `TemplateDecoder`, `TemplateComposer`, and
`TemplateRegistry` each render one file end to end:

```go
err := e.Parse(routetree.TemplateRegistry, myRegistryTemplate)
```

Every template receives a model whose fields are exported and documented on
`DecoderModel`, `ComposerModel`, and `RegistryModel`. `quote` and `dict` come as
helpers; `dict` exists because a Go template cannot otherwise pass two values
into a nested block. `Clone` gives you an independent emitter when one configured
base is specialized per run.

A failed `Parse` leaves the emitter untouched, so a broken template never costs
you the working set.

To generate code around templates rather than replacing tinybind's, you need
component parameters in Go terms, and the server action references a template
makes; `htmlbind.Signatures` and `htmlbind.ActionRefs` are documented in
[htmlbind_frameworkowner.md](htmlbind_frameworkowner.md).

## What is not there yet

- CSRF is not wired. These are `POST` endpoints reachable with ambient
  credentials, so wrap them yourself until it is.
- The script-free mode is designed but unimplemented. Today a `<form
  server-action>` lowers like any other element and needs a runtime to intercept
  it; posting to the page itself with a `303` back is the second phase, and it is
  what a form submitted with JavaScript disabled will need.
- `document.tb.html` is discovered but not yet applied; the document shell is
  still yours.
- Route groups that contribute no URL segment have no notation, because the
  parenthesis spelling other frameworks use is an illegal import path character.
- A catch-all binds as a string; richer typing is undecided.

The last two are one collision seen twice: where a Go rule and a routing
convention disagree, the Go rule wins.

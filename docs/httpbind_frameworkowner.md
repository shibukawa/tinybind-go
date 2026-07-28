# httpbind for Framework Owners

This guide is for people building a web framework **on top of** tinybind. It
covers routing: how a route becomes a registered handler, what generation can see
in each case, and which parts stay yours.

For the authoring language and ordinary application use, read
[httpbind.md](httpbind.md) first. For the rendering side — boundary protocol,
client runtime, head merging — read
[htmlbind_frameworkowner.md](htmlbind_frameworkowner.md). Nothing here repeats
either.

## Two routers: one general, one specialized

tinybind ships two ways to get a handler onto a `ServeMux`. They are not two
styles of the same feature, and they are not a split between APIs and pages.

The **registered router** is the ordinary Go way, and it has no ceiling. You
write the registration, your handler writes whatever it likes — JSON, HTML, a
file, a stream — and generation reads the registration to learn what that handler
binds and returns:

```go
mux.HandleFunc("POST /users/{id}", updateUser)
```

Nothing about it is API-specific. An API is simply the case where the generated
OpenAPI pays for itself; serving HTML from a registered route is ordinary and
supported, and for years it was the only way to do it here.

The **discovered router** trades that generality for one shape. A person
navigates to a URL and gets a page rendered from a template and its layouts, so
the traffic is `GET` and the input is whatever the URL carries. You create a
directory and generation writes the registration:

```
pages/users/id_/page.tb.html    → GET /users/{id}
```

Inside that shape you stop writing the walk-and-register loop. Outside it, you
are back on the registered router, which is not a fallback so much as the floor
everything else stands on.

That difference in scope sets the direction each one runs in. A general router is
defined by your Go code, so your Go source is the truth and generation reads it.
A page router is defined by its pages, so the filesystem is the truth and
generation writes the Go.

| | Registered router | Discovered router |
| --- | --- | --- |
| Scope | anything `net/http` can serve | a `GET` page rendered from a template |
| Methods | any | `GET` pages |
| Response | whatever the handler writes | the rendered page, unless you take the handler rung |
| A handler reads from | path, query, header, cookie, body, multipart | path and query |
| A route is defined by | a registration call in your Go source | a directory holding `page.tb.html` |
| Who writes the handler | you, always | generated, unless you opt out per route |
| Generation produces | request and response bindings, OpenAPI | components, decoders, and a `ServeMux` |
| Who calls `mux.HandleFunc` | you | the generated `Register` |
| OpenAPI | yes | never, by design |
| Unit of one run | one package directory | one route tree |
| Your configuration hook | call patterns on `Options` | templates and symbols on `Emitter` |
| Generation fails when | a pattern is not a compile-time string | a directory name is not a legal import path element |

Two of those rows are the same fact seen twice. A `GET` has no body, so path and
query are the whole input surface a page can have — which is why the discovered
router needs none of the tag vocabulary `Bind` offers, and why its inputs are
ordinary Go parameters rather than a struct.

Read the last row as the practical difference instead. The registered router can
be defeated by an expression it cannot evaluate; the discovered router can be
defeated by a directory name Go will not accept. Neither failure is silent.

A website is not purely `GET`, though. A form has to post somewhere, and the
discovered router registers `GET` and nothing else today, so that post is an
ordinary registered route on the same mux:

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /users/{id}", updateUser)   // registered: the form target
pages.Register(mux)                               // discovered: the pages
```

`Register` takes a mux you already own rather than insisting on making one, and
what follows from that is plain `ServeMux` behavior. The two registrations above
coexist because `GET /users/{id}` and `POST /users/{id}` are distinct patterns.
Registration order does not matter. The generated `/{$}` root leaves a
hand-registered `/api/` subtree alone, an unmatched path still answers 404, and a
`POST` to a page that only has a `GET` still answers 405.

One collision is real. Registering the same method and path twice panics, because
that is what `ServeMux` does with a duplicate pattern. A hand-written route
shadowing a generated one is therefore a startup crash rather than a silent
override — the right failure, but it also means adding a page can break a server
that already registered that exact pattern by hand.

Even so, the form target is a gap rather than a design. The intended shape is a
server function in the sense React 19 uses the term: a form names an exported Go
handler instead of a URL, and generation emits the endpoint, the `action`
attribute, and the CSRF field that connect them. Until that ships, write the
target by hand.

## The registered router

Generation reads route registrations to learn three things: the pattern, the
handler, and the types that handler binds and writes. From those it produces the
binding code and the OpenAPI document. It never produces the registration itself.

### Teaching it your API

A framework rarely exposes `http.ServeMux` and `httpbind.Bind` directly. The call
registry exists so discovery matches your wrappers instead:

```go
calls := generator.NewCallRegistry()
if err := calls.Register(generator.RequestBindCall(
	generator.Function("example.com/framework", "Parse"),
	generator.GenericType("request", 0),
)); err != nil {
	return err
}
options, err := calls.Options(generator.DefaultOptions())
```

`Options` carries the route side too. `ServeMuxes` names the types whose `Handle`
and `HandleFunc` methods count as registration, `RouteMethods` covers a mux with
differently named methods, and `RouteFunctions` covers package-level registrars.
Each is an authoritative set: configuring one replaces the built-in identities
rather than extending them, so `DefaultOptions` is what you build on when you
want the standard ones too.

Identity is resolved through `go/types`, not by name. A method called
`HandleFunc` on a type you did not configure is not a registration, which is what
keeps an unrelated API from being mistaken for routing.

### What it cannot see

Discovery is static and package-local, and it says so rather than guessing:

```go
mux.HandleFunc("GET " + prefix, handler)   // not a compile-time string
for _, r := range routes {                  // pattern unknown per iteration
	mux.HandleFunc(r.Pattern, r.Handler)
}
otherPackage.RegisterRoutes(mux)            // handler leaf is elsewhere
```

Each of these produces a diagnostic naming the file, line, and reason, and the
affected route is omitted from OpenAPI rather than documented incorrectly. Run
the generator with `-check` in CI to make those diagnostics fail the build.

The package-local rule shapes framework design more than it first appears. One
invocation never follows a handler into another package, so a registration and
the handler it points at normally belong together. Two directories are two
processes, which also means any identity your framework derives at generation
time must not depend on a per-run counter.

## The discovered router

Every framework built on this module eventually writes the same loop: walk a
directory of templates, derive a URL from each path, register a handler. The loop
is easy. The constraints it has to respect are not, and two of them stay
invisible until the Go toolchain refuses to build a package you never touched.

`routetree` is that loop plus those constraints. It discovers, it generates, and
it stops. It holds no opinion about page metadata, sitemaps, or `robots.txt`;
those stay yours, and the route table below is what you build them from.

### What a page can be

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

One thing does change. At the typed rung the component's parameter list becomes
`Load`'s return list, and generation checks it: a mismatch in count, order, or
type fails, naming both lists.

### Segment notation, and why it is not `[id]`

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

### Layouts

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

### What lands where

```
pages/                       ← Config.Root, default "pages"
├── layout.tb.html
├── layout_gen.go            ← compiled Layout component
├── page.tb.html
├── page_gen.go              ← compiled Page component
├── route_gen.go             ← RouteParams + DecodeRoute for "/"
├── routes_gen.go            ← Register, NewServeMux, Routes, RouteInfo
└── users/id_/
    ├── page.tb.html
    ├── page.go              ← optional func Load
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

### Running it

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

An application then touches one symbol:

```go
mux := pages.NewServeMux()        // or pages.Register(existingMux)
```

`Register` takes `htmlbind.Option` values and passes them to every render, so a
per-request cache, timeout, or error hook is configured once.

### The route table

The registry also emits what the filesystem knows, and deliberately nothing more:

```go
var Routes = []RouteInfo{
	{Pattern: "GET /{$}", Path: "/", Dir: "", Params: nil},
	{Pattern: "GET /users/{id}", Path: "/users/{id}", Dir: "users/id_", Params: []string{"id"}},
}
```

That is the seam for a sitemap or a route inspector. Patterns, methods, and which
segments are dynamic all come from the tree. The concrete values a dynamic
segment expands into are application data, so they stay yours.

Note what the table is not: an OpenAPI source. Page routes never enter an OpenAPI
document, because OpenAPI describes a published API contract and an HTML page is
not one. If your framework wants them documented, it adds them from this table
through its own artifacts.

### Customizing the output

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
`DecodeFunc`, `RenderFunc`, `RegisterFunc`, `MuxFunc`, and `TableVar`.

**Replace one block.** The built-in templates are compositions of named blocks,
so you can change one without owning the rest:

| Block | What it writes |
| --- | --- |
| `imports` | the import statement of any generated file |
| `convert` | one scalar read out of a raw string |
| `error` | every error value a decoder produces |
| `handler` | one route's handler body in the registry |

```go
err := e.Parse("error", `web.Invalid(web.Fault{Code: {{ .Code | quote }}})`)
```

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
component parameters in Go terms; `htmlbind.Signatures` is documented in
[htmlbind_frameworkowner.md](htmlbind_frameworkowner.md).

## What is not there yet

- The discovered router registers `GET` page routes only. Anything outside that
  shape — another method, a response that is not a rendered page — stays a
  registered route, which is where it belongs; a form target is one only until
  server functions ship.
- `document.tb.html` is discovered but not yet applied; the document shell is
  still yours.
- Route groups that contribute no URL segment have no notation, because the
  parenthesis spelling other frameworks use is an illegal import path character.
- A catch-all binds as a string; richer typing is undecided.
- An optional query parameter has no spelling, because a Go parameter is always
  present.
- The registered router still ignores dynamic, looped, and cross-package
  registration rather than diagnosing every case.

Three of the first five are the same collision: where a Go rule and a routing
convention disagree, the Go rule wins.

# httpbind for Framework Owners

This guide is for people building a web framework **on top of** tinybind. It
covers routing: how a route becomes a registered handler, what generation can see
in each case, and which parts stay yours.

For the authoring language and ordinary application use, read
[httpbind.md](httpbind.md) first. The discovered router has its own guide in
[httpbind_discovered_router.md](httpbind_discovered_router.md), which covers page
shapes, segment notation, layouts, server actions, and the generated output. For
the rendering side — boundary protocol, client runtime, head merging — read
[htmlbind_frameworkowner.md](htmlbind_frameworkowner.md). Nothing here repeats
any of them.

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
| Scope | anything `net/http` can serve | a `GET` page rendered from a template, plus its server actions |
| Methods | any | `GET` for pages, `POST` for actions |
| Response | whatever the handler writes | the rendered page, unless you take the handler rung |
| A handler reads from | path, query, header, cookie, body, multipart | path and query for a page; anything for an action |
| A route is defined by | a registration call in your Go source | a directory holding `page.tb.html` |
| Who writes the handler | you, always | generated for a page, yours for an action |
| Generation produces | request and response bindings, OpenAPI | components, decoders, and a `ServeMux` |
| Who calls `mux.HandleFunc` | you | the generated `Register` |
| OpenAPI | yes | never, by design |
| Unit of one run | one package directory | one route tree |
| Your configuration hook | call patterns on `Options` | templates and symbols on `Emitter` |
| Generation fails when | a pattern is not a compile-time string | a directory name is not a legal import path element |

Two of those rows are the same fact seen twice. A `GET` has no body, so path and
query are the whole input surface a *page* can have — which is why a page needs
none of the tag vocabulary `Bind` offers, and why its inputs are ordinary Go
parameters rather than a struct. A server action is the other half of that shape:
it does read a body, and it is an ordinary handler you write, precisely because no
fixed typed return could cover what a mutation needs to answer with.

Read the last row as the practical difference instead. The registered router can
be defeated by an expression it cannot evaluate; the discovered router can be
defeated by a directory name Go will not accept. Neither failure is silent.

The two routers share a mux without negotiating:

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /api/users/{id}", updateUser)   // registered: your JSON API
pages.Register(mux)                                   // discovered: pages and actions
```

`Register` takes a mux you already own rather than insisting on making one, and
what follows from that is plain `ServeMux` behavior. Registration order does not
matter. The generated `/{$}` root leaves a hand-registered `/api/` subtree alone,
an unmatched path still answers 404, and a `POST` to a page that only has a `GET`
still answers 405.

One collision is real. Registering the same method and path twice panics, because
that is what `ServeMux` does with a duplicate pattern. A hand-written route
shadowing a generated one is therefore a startup crash rather than a silent
override — the right failure, but it also means adding a page can break a server
that already registered that exact pattern by hand.

A page usually registers `GET` alone, so its own path stays free for whatever
`POST` you want there. The exception is a page whose template declares a `<form
server-action="…">`: a form with no `action` submits to the page's own URL, so
that page registers `POST` on its path too and dispatches on a hidden field. If
you were already handling `POST` at that address by hand, adding the form is the
startup crash above. A `server-action` on a bare button changes nothing here,
because a button has no native submit to serve and registers no `POST`.

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

### When the type is an argument, not a type parameter

`GenericType` reads a role from a type parameter. Some wrappers do not put the
type there. A cache lookup is the clearest case: it is generic over the *result*
it caches, and the thing that needs generated code is the key beside it.

```go
// func Memo[T any](ctx context.Context, key cachekeybind.CacheKey, fetch func(context.Context) (T, error)) (T, error)
calls.Register(generator.CacheKeyCall(
	generator.Function("example.com/framework", "Memo"),
	generator.ArgumentType("key", 1),
))
```

`ArgumentType` takes the zero-based *value* argument index and reads the static
type of what is passed there. The generated method lands on that type, so the
generator has to run on the package that declares it — a key type from another
package is reported rather than generated, the same rule the item and entity
codecs follow.

See the [cachekeybind guide](cachekeybind.md) for what the marked struct looks
like and what the emitted key contains.

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

## What is not there yet

- The registered router still ignores dynamic, looped, and cross-package
  registration rather than diagnosing every case.
- The discovered router registers `GET` page routes and `POST` server action
  endpoints. Anything outside those shapes — another method, a response that is
  not a rendered page — stays a registered route, which is where it belongs.
- `server-action` works only inside the route tree. The template half is
  mode-agnostic, so the gap is that nothing resolves a handler to a URL for a
  flat-mode template; the natural fix reads the pattern from the registration
  discovery described above.

The discovered router has its own list, in
[httpbind_discovered_router.md](httpbind_discovered_router.md).

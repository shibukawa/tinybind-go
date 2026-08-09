# The fasthttp Backend for Framework Owners

This guide is for people building a web framework **on top of** tinybind who
want their users to be able to build against fasthttp. It covers what the
transform needs from you, where your own tag boundary falls, and which of your
helpers have to exist twice.

Read [httpbind_fasthttp.md](httpbind_fasthttp.md) first for what an application
author sees; nothing here repeats it. For routing in general read
[httpbind_frameworkowner.md](httpbind_frameworkowner.md).

## The transform only knows what you declare

Generation derives fasthttp handlers by rewriting the authored net/http ones.
Two transport parameters collapse into one context, and every call that took the
writer or the request drops those arguments. That works for `httpbind.Bind` and
its siblings because the generator already knows their shapes.

It knows nothing about yours. A handler calling `framework.Render(w, r, page)`
looks exactly like a handler calling an untraceable third-party logger, and the
transform refuses both for the same reason: it cannot tell which arguments
disappear. Every such handler in your users' code is then a build error they
cannot fix without you.

So register your calls, and say where the transport sits:

```go
calls := generator.NewCallRegistry()
err := calls.Register(
	// func Render(w http.ResponseWriter, r *http.Request, page Page) error
	generator.ResponseWriteCall(
		generator.Function("example.com/fw", "Render"),
		generator.ArgumentType("response", 2),
		generator.WriterArgument(0),
		generator.RequestArgument(1),
	),
)
```

`WriterArgument` and `RequestArgument` are the new half. The other roles say
where a semantic value is *read from*; these say which arguments carry nothing
semantic and exist only because the net/http shape passes both halves
separately. A single-value transport drops exactly those.

A call that takes the transport and names no model needs a pattern too, or it
refuses every handler that makes one. That is what `TransportCall` is for:

```go
generator.TransportCall(
	generator.Function("example.com/fw", "Abort"),
	generator.WriterArgument(0),
	generator.RequestArgument(1),
)
```

The module's own `WriteError` is registered this way. Without it, every handler
that reported an error would have been refused — which is how the need was
found.

## Your helpers, in two packages

Declaring the slots tells the transform which arguments to drop. It does not
produce the function they will be dropped from. `framework.Render` still has to
exist over `*fasthttp.RequestCtx`, and that is yours to write.

Ship it under the same names in a second package, and register the pair:

```go
transform := generator.DefaultTransformOptions()
transform.ImportRewrites["example.com/fw/render"] = "example.com/fw/render/fast"
```

The generated file imports the second package **under the first one's local
name**. `render.Page(ctx, p)` is what the rewritten body says, and it says the
same thing it said before; only the import line moved. Nothing about the
mapping is built in beyond the module's own runtime pair, and nothing stops you
adding several.

Follow the accessor convention while you are writing the fasthttp half: take the
transport value first, and keep the parameter names the net/http version uses.
The module's own runtime does this, and the payoff is that a generated binder
body is the same text on both transports — only its signature line moves. A
helper that reorders its parameters costs you a rewrite rule that a helper
mirroring the original does not need.

## Where your tag boundary goes

An application tags its handler files. You have the harder version of the same
problem, because most of your package has nothing to do with the transport and
would resent being tagged.

Split by package and the transport-free majority pays for it: every
configuration function, every option struct, every log helper needs an alias in
both halves. Keep one import path and tag inside it, and they cost nothing.
That is the opposite arrangement from tinybind's own — `httpbind` and
`fasthttpbind` are separate packages — and it is the same reasoning reaching a
different answer, because there almost the whole surface is transport-shaped and
here almost none of it is.

The move that keeps the tagged layer thin is to **tag the type, not its users**:

```go
//go:build !fasthttp
type Handler func(w http.ResponseWriter, r *http.Request)

//go:build fasthttp
type Handler func(ctx *fasthttp.RequestCtx)
```

```go
// untagged: one copy, because the signature text does not change
func (a *App) GET(path string, h Handler) { ... }
```

A function whose signature names only your own types keeps the same signature
under both tags, even when those types are defined differently. Routing tables,
option structs and registration APIs stay single-copy that way. Only code that
reaches *into* the request needs two versions.

The boundary propagates, though. An untagged function may call a tagged one only
while the callee's signature is identical under both tags; the moment it is not,
the caller is tagged too. This is the same closure the transform computes over
application code, applied to yours by hand. If tagging starts spreading past
your request-handling layer, a signature that could have been transport-free is
not — and making it so is cheaper than maintaining the second copy.

## The error model is one type, deliberately

`Problem`, `FieldError` and `HTTPError` live in a shared leaf and are aliased by
both runtimes. They are the same types, not two matching definitions, so an
error your framework builds on one surface still matches when the other inspects
it. Do not redeclare them; alias them if you re-export.

The same applies to anything you carry across the seam. Two definitions that
agree today are two chances to disagree later, and the failure is silent: an
`errors.As` that stops matching, a `Problem` that no longer unwraps.

You will probably want a richer application-facing problem value than
`Problem` — a status, a title, fields, a cause — and naming yours `Problem` too
puts two different types with one name in front of your users. Put yours in a
leaf both your runtimes alias, which is the move made here for `FieldError`, and
the name means one thing whichever backend a user built against.

The update surface is split the same way and is worth reading as the worked
example: `htmlupdate` and `fasthttpupdate` redeclare only `Options` and
`Response`, because those need methods, and alias everything else from one core.
The payoff is concrete — a `Registry`, a `Reloadable` and an `Update` are one
type across both backends, so the code building them needs no build tag at all.
Each shell converts with `updatecore.Options(o)`, so a field added on one side
and not the other stops compiling rather than drifting.

## Choosing the router

`RouterTarget` names the package, qualifier, type, registration function and
catch-all spelling that generated route registration uses:

```go
transform.Router = generator.RouterTarget{
	Import:         "example.com/fw/mux",
	Qualifier:      "mux",
	Type:           "mux.Router",       // written verbatim, so an interface needs no pointer
	RegisterFunc:   "Wire",
	CatchAllSuffix: ":*",
}
```

The default is tinygodriver's fasthttprouter fork. If your framework owns
routing, point this at your own type and generation installs on it instead.

Leave `CatchAllSuffix` empty when your router has no catch-all and generation
will reject such a route by name rather than invent a spelling for it.

## What you inherit from the refusal contract

There is no adapter. A handler the transform cannot rewrite stops the build, and
the diagnostic names the occurrence and a remedy. Two consequences land on you.

Your users will read remedies that mention *your* framework by name — "passes r
to fw.Render, whose transport arguments are undeclared" — and the fix is a call
pattern only you can register. Registering them all is the difference between a
backend your users can adopt and one they cannot.

The other consequence is quieter. Every framework helper taking the transport is
a potential refusal, so the surface you expose in that shape is the surface you
have to maintain twice and declare once. A helper you can give a transport-free
signature — `func Render(ctx context.Context, page Page) (Body, error)` rather
than one taking the writer — costs nothing on either side and refuses nothing.
Application authors get the same advice, but it is worth more to you: your
choice multiplies across every application built on you.

## Checking your work

Point the report at a package written against your framework and read what comes
back:

```bash
tinybind-gen generate -dir ./example -backend fasthttp -transport-report
```

Nothing reported means an application using your framework the ordinary way can
build on fasthttp. Anything reported is a call pattern you have not registered,
a helper you have not ported, or a signature that should not have taken the
transport in the first place — and the message says which.

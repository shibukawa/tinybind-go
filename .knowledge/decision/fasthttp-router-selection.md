---
id: decision:fasthttp-router-selection
type: decision
title: The Router Is The Application's Choice, And The Fork Constrains It
---
Generated route registration names a router the application supplies, defaulting to fasthttp/router, whose patterns already match the discovered ones — but whose handler type is upstream fasthttp's and therefore not the fork's.

```yaml
status: implemented with a blocked default, 2026-08-08
requested: user, 2026-08-08, naming github.com/fasthttp/router
why_registration_is_generated_at_all: fasthttp has no router, so nothing installs a route unless something emits the call
what_transfers_untouched:
  pattern_syntax: fasthttp/router spells a named parameter {name}, exactly as the net/http ServeMux patterns discovery already reads, so a route carries over verbatim
  path_values: the router stores matches with SetUserValue, which is where api:fasthttpbind-bind PathValue reads, so no adapter sits between them
  significance: the two were designed apart and agree anyway; only one segment shape needed a rule
what_moves:
  catch_all: "{rest...} becomes {rest:*}"
  refusal: a router target declaring no catch-all spelling rejects such a route rather than guessing one
the_blocker:
  fact: github.com/fasthttp/router requires github.com/valyala/fasthttp, and system:tinygodriver-fasthttp is a vendored copy rather than an alias layer
  therefore: a handler taking the fork's RequestCtx is not a fasthttp.RequestHandler, and the router will not accept one
  evidence: 'the compiler, not inference: cannot use handler (value of type func(ctx *tinygodriver/fasthttp.RequestCtx)) as valyala/fasthttp.RequestHandler value in argument to r.GET'
  predicted_by: decision:fasthttpbind-tinygo-not-first-class recorded this cost and judged the ecosystem it breaks one the design was not reaching for; that premise no longer holds
resolutions:
  vendor_the_router:
    shape: fork fasthttp/router into tinygodriver beside the fasthttp fork
    size: 6 files and about 2500 lines, plus a radix subpackage and savsgio/gotils, against 49 files for fasthttp itself
    keeps: the TinyGo compile requirement of requirement:fasthttpbind-tinygo, and the existing vendoring machinery
    costs: a second tree to re-vendor on each upstream release
  depend_on_upstream:
    shape: fasthttpbind imports valyala/fasthttp instead of the fork
    gains: the router and the rest of the ecosystem, by type identity rather than by porting
    costs: TinyGo no longer compiles, which decision:fasthttpbind-tinygo-not-first-class still requires even after dropping every other TinyGo obligation
  status: undecided; the generated registration is written and correct under either, since only the import path differs
what_was_built_meanwhile:
  configurable: RouterTarget names the import, qualifier, type, registration function and catch-all spelling
  default: fasthttp/router, as asked
  reason_it_is_configuration: this module should not decide which third-party package an application depends on, the same reason the import rewrites are configuration
  untested_gap: the emitted registration is checked as text and not compiled, because the router cannot join a build here until the blocker above is resolved
related:
  - decision:fasthttpbind-tinygo-not-first-class
  - decision:backend-build-tag-mode
  - rule:transform-rewrite-table
  - flow:fasthttp-generation
  - system:tinygodriver-fasthttp
```

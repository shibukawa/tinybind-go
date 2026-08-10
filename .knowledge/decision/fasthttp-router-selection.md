---
id: decision:fasthttp-router-selection
type: decision
title: The Router Is The Application's Choice, And The Fork Constrains It
---
Generated route registration names a router the application supplies, defaulting to fasthttp/router, whose patterns already match the discovered ones — but whose handler type is upstream fasthttp's and therefore not the fork's.

```yaml
status: implemented and compiling, 2026-08-08
requested: user, 2026-08-08, naming github.com/fasthttp/router
why_registration_is_generated_at_all: fasthttp has no router, so nothing installs a route unless something emits the call
what_transfers_untouched:
  pattern_syntax: fasthttp/router spells a named parameter {name}, exactly as the net/http ServeMux patterns discovery already reads, so a route carries over verbatim
  path_values: the router stores matches with SetUserValue, which is where api:fasthttpbind-bind PathValue reads, so no adapter sits between them
  significance: the two were designed apart and agree anyway; only one segment shape needed a rule
what_moves:
  catch_all: "{rest...} becomes {rest:*}"
  refusal: a router target declaring no catch-all spelling rejects such a route rather than guessing one
  catch_all_value_agrees_too_2026_08_10:
    reported: downstream framework survey 2026-08-10, as a report on the driver rather than as an ask on this module
    doc_comment_is_wrong: fasthttprouter's package documentation shows a catch-all carrying a leading slash, with /files/LICENSE giving filepath="/LICENSE"
    behaviour: verified against the local checkout; /files/ gives "", /files/LICENSE gives "LICENSE", /files/templates/article.html gives "templates/article.html"
    net_http_agrees: the same values, with no leading slash, which is what makes the rewrite above a spelling change and nothing more
    provenance: the comment is inherited from upstream httprouter, where it was accurate; the reporter wrote a test from the comment and it failed against both implementations
    consequence_here: none for generated code, and one fewer place a rewritten route could differ; the fix belongs in the driver's doc.go
    fixed_2026_08_10: the driver's fasthttprouter/doc.go now states the values it produces and says upstream documents otherwise, so the next reader writes a test from the behaviour rather than from the inheritance
page_tree_registration_2026_08_10:
  what: requirement:routetree-transport-selection emits a fasthttp page tree, and it registers on an interface rather than on this router
  emitted: 'interface{ HandleFunc(string, func(*fasthttp.RequestCtx)) }'
  patterns: Go 1.22 spellings reach it unchanged, '{$}' included; no catch-all rewrite runs on this path
  reading: the two registration paths differ on purpose. The transform path targets a named router and rewrites the catch-all; the page tree names no router and defaults to writing what it discovered
  spelling_available_2026_08_10: Symbols.CatchAllSuffix and Symbols.RootPattern let a page tree target a router reading another syntax, defaulting to net/http's markers so an unset field changes nothing
  root_marker_matters_here_too: this decision recorded the catch-all as the only segment that moves, which held for the transform path because it registers discovered handlers; a page tree also writes '{$}' for the root, and a router reading another syntax takes that for a parameter named '$'
  fasthttprouter_is_not_a_page_tree_router:
    found: 2026-08-10, while settling the two targets
    fact: it declares GET, POST and Handle rather than a HandleFunc taking "GET /path", so it cannot be a page tree's MuxType directly
    consequence: a page tree reaches it through an adapter, as the downstream framework's own pwfast.ServeMux is; the spelling fields are what let that adapter be handed a pattern its router can parse
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
  resolved_2026_08_08:
    taken: vendor_the_router; tinygodriver v1.2.1 carries fasthttprouter beside its fasthttp fork, built against the fork's RequestCtx
    default_now: github.com/shibukawa/tinygodriver/fasthttprouter
    unchanged_by_it: the API and the pattern syntax are upstream's, so an application on upstream fasthttp points Import at github.com/fasthttp/router and nothing else in the target moves
    verified: the generated registration now joins the -tags fasthttp build of the fixture and compiles, which the text-only check could not establish
    module_note: go mod tidy does not look inside testdata, so the router is named in a generator test file to keep it in go.mod; it is a test dependency of this module, and an application generating code records it in its own
what_was_built_meanwhile:
  configurable: RouterTarget names the import, qualifier, type, registration function and catch-all spelling
  default: fasthttp/router, as asked
  reason_it_is_configuration: this module should not decide which third-party package an application depends on, the same reason the import rewrites are configuration
  now_compiled: the emitted registration joins the fixture's tagged build alongside the handlers and binders
related:
  - decision:fasthttpbind-tinygo-not-first-class
  - decision:backend-build-tag-mode
  - rule:transform-rewrite-table
  - flow:fasthttp-generation
  - system:tinygodriver-fasthttp
```

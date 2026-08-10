---
id: requirement:routetree-transport-selection
type: requirement
title: routetree Transport Selection
---
routetree emits net/http in three places and offers no option to emit the other transport, so a fasthttp build of a project carrying a page tree gets no routes and no decoders at all.

```yaml
status: implemented 2026-08-10
source: downstream framework survey 2026-08-10, against v0.5.1, by diffing every exported surface rather than from memory
reporter_ranking: the single largest thing between this module and a runnable second backend; it reached the report only once the downstream side was built far enough to meet it
verified_here_2026_08_10: by reading the templates and the page-func check, and by putting a decoder of the emitted shape through the transform
three_outputs_are_net_http_only:
  decoder:
    emits: "func Decode(r *http.Request), then 'query := r.URL.Query()' and 'r.PathValue(key)'"
    reads: off the request value itself, as methods rather than as calls through Symbols
    where: routetree/templates/decoder.go.tmpl
  registration:
    emits: 'mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {...})'
    where: routetree/templates/registry.go.tmpl, the handler block
  accepted_page_function:
    accepts: "func(http.ResponseWriter, *http.Request) as one of the two legal shapes, being rung 3 of decision:route-handler-shape"
    where: routetree/pagefunc.go isHandlerSignature, which resolves the literal import path net/http from the file
consequence: a page tree contributes nothing to a fasthttp build; not a degraded route set but an empty one
what_the_existing_seam_already_covers:
  symbols: HTTPImport and HTTPAlias repoint the package a generated file names, and the handler body already calls WriteError through ErrorAlias
  limit: the alias reaches the spelling and not the shape. The handler template writes 'func(w {{HTTPAlias}}.ResponseWriter, r *{{HTTPAlias}}.Request)', which under any alias is still two values naming two types that only net/http has
  measured: routetree names fasthttp nowhere
the_transform_cannot_close_this:
  reporter_position: generated files are outputs rather than transform inputs, emitted per backend, so the emitter choosing its transport is the whole fix; no rewrite of generated output is asked for
  confirmed_independently_2026_08_10: a decoder of the emitted shape put through AnalyzeTransform is refused, kind unknown_selector, 'reads r.URL, which no rewrite covers'
  why_it_stays_refused: rule:transform-rewrite-table lists r.URL among its deliberately absent selectors and r.PathValue is not in the seed, so admitting the decoder would mean growing the enumerated table for code this module writes itself
  reading: the emitter is the right seam not only because generated output is not source, but because the alternative spends the rewrite table's own policy on a shape the emitter controls
decoder_is_a_substitution_table_not_a_design_question:
  accessors_already_agree: httpbind.PathValue(r, key) and fasthttpbind.PathValue(ctx, key) carry the same name and take the transport first; the query reads pair the same way
  therefore: routing the decoder through Symbols as calls rather than as methods is exactly the fix decision:fasthttpbind-generator-backend-selection already names, and the receiving half is built
  path_values_note: fasthttp has no transport PathValue of its own, so the seam must also let the router publish path parameters; api:fasthttpbind-bind reads router-set user values, which is where the fork's router writes them
registration_needs_one_decision_from_this_module:
  question: what the emitted Router interface is on the fasthttp side
  reporter_shape: 'HandleFunc(pattern string, handler func(*fasthttp.RequestCtx))'
  reporter_claim: their pwfast.ServeMux satisfies it today, translating Go 1.22 patterns onto the vendored trie router, including the '{$}' this emitter writes for the root of every page tree
  precedent: requirement:router-type-independence already made MuxType a written-verbatim string, so a framework names its own interface without this module choosing one for it
  tension_to_settle:
    fact: decision:fasthttp-router-selection defaults the transform path's registration to tinygodriver/fasthttprouter, rewriting a catch-all to '{rest:*}'
    against: this ask registers a page tree on a framework router that reads Go 1.22 pattern syntax unchanged
    therefore: one run may want two router targets, and the emitters must agree on which one a page tree uses or be told apart
    settled_2026_08_10:
      answer: told apart, with the page tree able to spell either
      page_tree: names no router and defaults to Go 1.22 spellings, which is what the framework that raised this reads; as_built pattern_spelling is how it targets another
      transform_path: keeps its named router and its refusal, unchanged
      found_while_settling: fasthttprouter cannot be a page tree's MuxType directly, because it declares GET, POST and Handle rather than a HandleFunc taking "GET /path"; a page tree reaches it through the same kind of adapter the reporter wrote, and the spelling fields are what let that adapter receive a pattern its router can parse
as_built_2026_08_10:
  entry: NewFastHTTPEmitter(transportImport), which sets the emitted symbols and the recognized rung 3 signature together
  why_one_call: the two must name the same transport, and a build that emitted one while admitting the other's handler shape fails in generated source rather than in configuration
  symbols_added: RequestType, HandlerParams, Writer, Request and RequestIsContext, plus the accessor selectors PathValue, QueryValues and QueryLookup
  derived_not_declared:
    TransportArgs: the leading arguments of a call taking both halves; one value where Writer and Request are the same identifier, two where they differ
    ContextOf: takes the identifier rather than reading Request, because the composer's request parameter is named by the caller
  handler_shape:
    type: HandlerShape, naming the transport import and the parameter types, resolved syntactically from the file's own import because the check runs before the package compiles
    reaches: rung 3 pages and server functions alike, which share one signature; DiscoverActionsWith and AnalyzeWith carry it
    diagnostic: a near-miss names the configured shape rather than the net/http one, so a correct fasthttp handler no longer reports as a malformed typed page
  compatibility:
    empty_transport_fields: fall back to the net/http pair built from HTTPAlias, so an emitter configured before these fields existed emits what it always did
    registry_output_unchanged: the net/http registry and composer are byte-identical; only the decoder moved
  decoder_output_changed:
    was: 'r.URL.Query() and r.PathValue(key), as methods on the request value'
    now: 'httpbind.Queries(r), httpbind.QueryLookup(query, key) and httpbind.PathValue(r, key)'
    supersedes: decision:fasthttpbind-generator-backend-selection standalone_value, which claimed httpbind output would stay byte-identical
    why_the_claim_was_given_up: 'fasthttp has no PathValue on the transport at all, so the fasthttp decoder must go through the runtime; routing net/http through it too is what makes the two decoder bodies the same statements rather than two shapes to keep in sync'
    cost: 'one call layer, each a delegation the compiler inlines; Queries skips the parse entirely when there is no query string'
    churn: committed fixtures regenerate; the change is visible in a diff and semantically identical
  router_interface_as_emitted: 'interface{ HandleFunc(string, func(*fasthttp.RequestCtx)) }, written verbatim through MuxType with an empty MuxConstructor, so this module names no third-party router'
  pattern_spelling:
    fields: Symbols.CatchAllSuffix and Symbols.RootPattern, both defaulting to net/http's own markers, so an unset field leaves every pattern as discovered
    reaches: only these two segment shapes; every router this seam has met spells a named parameter {name}, which is why a route otherwise carries over verbatim
    why_the_root_counts_too: '{$} is Go 1.22 exact-match; a router reading another syntax takes it for a parameter named "$" and installs the root as a one-segment wildcard'
    failure_it_prevents: not a rejection but a misreading, so without the seam the route is silently registered somewhere else
    no_unsupported_spelling:
      diverges_from: the transform's RouterTarget, where an empty CatchAllSuffix refuses a catch-all route
      why: there every field is caller-declared and unset means unknown; here the defaults are a working router, so an unset field cannot be told from a router that reads Go 1.22 and needs no rewrite
      consequence: a router with no catch-all at all is not modelled, because a refusal keyed on a field that is also the default would fire on the common case
    route_table: Pattern carries the router's spelling and Path the declared address, because a router's catch-all spelling is not a URL and a sitemap needs the URL
  verified:
    fixture: internal/fasthttppagesfixture, a whole tree generated for the second transport and committed, mirroring internal/pagesfixture
    covers: layout chain render, an optional query left nil, a rejected query, a rung 3 route registered unwrapped, a discovered server function, and the typed entry point receiving the request value as its context
    negative: no generated file in the tree names net/http
related:
  - decision:fasthttpbind-generator-backend-selection
  - decision:fasthttp-router-selection
  - requirement:router-type-independence
  - requirement:generated-route-registration
  - requirement:transport-port-surface
  - decision:route-handler-shape
  - rule:transform-rewrite-table
```

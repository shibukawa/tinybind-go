---
id: requirement:transport-port-surface
type: requirement
title: Transport Port Surface
---
The declarations a second transport backend must reproduce, inventoried across the module so the port is scoped by measurement rather than by estimate.

```yaml
status: proposed 2026-08-08
surveyed: 2026-08-08 against v0.4.3, by scanning every package for net/http in exported declarations and in emitted code
source: downstream framework survey, raised 2026-08-08
needs_nothing:
  packages:
    - htmlbind and htmlbind/delta, whose render entry points all take io.Writer
    - jsonbind, an append-to-bytes API
    - sqlbind, configbind, dynamobind, firestorebind, minitoml, cliparser
  significance: htmlbind is the heaviest dependency a downstream framework has on this module, and it is already transport-neutral, so the port is smaller than the dependency list suggests
runtime_work:
  root_package:
    count: 26 exported declarations taking a request or a response writer
    request_side: Bind, ReadBody, Queries, QueryValue, PathValue, HeaderValue, CookieValue, IsJSONRequest, IsFormRequest, IsMultipartRequest, ParseMultipartMap, ReadJSONObject, ParseFormMap, RegisterBind
    response_side: Write, WriteStatus, WriteError, WriteJSON, WriteJSONBytes, RegisterWrite
    stream: WriteStream and NegotiateStreamFormat, per decision:stream-callback-shape; the held NewStream entry was removed 2026-08-10
    api_docs: OpenAPIJSON and SwaggerUI
  htmlupdate:
    count: 14 exported declarations; the package is the partial-update, redraw, and live surface end to end
    surface: WantsUpdate, Negotiate, WriteUpdate, WriteUpdateStatus, WriteNavigate, WriteFailure, Render, RenderStream, RenderStreamAsync, RenderLiveStream, Redraw, OpenStream, OpenLiveStream, CSRFToken, VerifyCSRF, RuntimeHandler, and Mount
    superseded_by: done_2026_08_10_update_surface and done_2026_08_10_writing_half below; the whole surface is carried, and OpenStream and OpenLiveStream no longer exist under those names
    note: a root-package port does not carry this, and a downstream unified update runtime stops at the boundary without it
    already_recorded: requirement:fasthttpbind-parity-scope defers this, and the recordWriter http.Flusher field is why
analysis_work:
  parser:
    finding: the net/http package path is one constant, and the handler and registration shapes are recognized around it
    addition: an adapter for the other backend's router and registration surface
    new_capability: decision:transport-source-transform needs per-identifier occurrence analysis of w and r, which no pass performs today; analyzeBody collects recognized calls and walks past everything else
codegen_work:
  routetree:
    finding: page-tree emission is already parameterized through Symbols carrying HTTPImport, HTTPAlias, MuxImport, MuxAlias, MuxType, MuxConstructor, the error symbols, and the runtime import, with DefaultSymbols targeting net/http
    consequence: swapping the emitted package and router is configuration rather than a rewrite
    remaining: the decoder template still spells r.PathValue and r.URL.Query as methods, and the model names a writer and a request as separate symbols
    corrected_2026_08_10:
      what_the_consequence_line_overstated: the alias reaches the spelling and not the shape, so swapping it is configuration only for the package name; the emitted handler is two values and the decoder reads methods whatever the alias says
      third_output_not_counted_above: routetree/pagefunc.go accepts func(http.ResponseWriter, *http.Request) as the rung 3 page function, resolving the literal import path
      owned_by: requirement:routetree-transport-selection, which is now the largest open item in this port
  generator:
    finding: the import table carries net/http as an entry, and writer emission formats a two-parameter signature directly into the output
    work: the same parameterization routetree already has, plus the AST printing decision:transport-source-transform requires
sequencing:
  settled_2026_08_08:
    - packaging and isolation, by decision:backend-build-tag-mode
    - the adapter, which is not implemented
    - transitivity, which the missing adapter forces to be a call-graph closure
    - naming, which needs no prefix because the alias import carries it
  done_2026_08_08:
    what: the shared transport-free leaf, as internal/bindcore
    holds: Problem, FieldError, HTTPError and the error constructors, aliased from the root; File; the check helpers; the multipart limit, MediaType, FileFromHeader, the rest-map builders, and AppendFileJSON
    verified: the full suite passes and the package's dependency graph contains neither net/http nor database/sql
    finding: most root helpers named as candidates turned out to be one-line jsonbind or strconv delegations, so each surface declares its own rather than routing through a third package
    behaviour_note: HTTPError.Error no longer calls net/http.StatusText; bindcore.StatusText covers the produced codes and the ordinary ones beside them and returns "" otherwise, as the standard library does. Every constructor sets Title, so this is reached only by a hand-built HTTPError
  done_2026_08_08_runtime:
    what: the fasthttp runtime surface, as package fasthttpbind
    holds: Bind, Write, WriteStatus, WriteError, WriteJSON, WriteJSONBytes, the binder and writer registries, and the request accessors Queries, QueryLookup, QueryValue, PathValue, HeaderValue, CookieValue, Is*Request, ReadJSONObject, ParseFormMap, ParseMultipartMap and ReadBody
    names: identical to the net/http surface, so a generated binder differs only in its signature line and the substituted identifier
    path_values: PathValue reads a router-set user value, because fasthttp has no routing of its own
    shared: the error model, problem-document derivation, multipart limits and FileFromHeader all come from bindcore, so error bytes match by construction rather than by two implementations agreeing
    verified: parity tests compare bind results, JSON field access, form parsing, WriteJSONBytes and four WriteError cases against the net/http runtime; a mutation check confirms the pooled-body copy test fails when the copy is removed
    not_yet: the stream entry, which waits on decision:stream-callback-shape
  done_2026_08_10_update_surface:
    what: the read-only half of htmlupdate, as internal/updatecore behind two shells; see decision:update-core-shared-leaf
    holds: WantsUpdate, Negotiate, Redraw, WriteUpdate, WriteUpdateStatus, WriteNavigate, Sequence, CSRFToken, VerifyCSRF, Headers, RedrawHeaders, StreamHeaders, LiveHeaders, FailureResponse, the query decoders, the manifest codec and the runtime asset
    sending_half_included: Response.WriteTo, Response.NotModified and ApplyTo, without which an entry computing an answer could not send it
    added: Redirect, as a registered pair, because WantsUpdate exists to create the branch that takes one
    still_here_at_the_time: Render, RenderStream, RenderStreamAsync, RenderLiveStream, OpenStream, OpenLiveStream and the recordWriter; done_2026_08_10_writing_half closes this
    corrects_the_count_below: 'the htmlupdate row says a root-package port does not carry this; it is now carried, and what is left is the six writing entries rather than fourteen'
    known_gap:
      what: 'a package-level var of type Options'
      why: a var is a declaration and not a function, so the transform does not rewrite it and the build tag excludes the file it lives in
      remedy_today: declare it in a tagged file pair, or build it inside the handler
      remedy_available: extend the transform to rewrite the imports of an authored file that only names rewritten packages, which is a change to its model rather than to this table
  done_2026_08_10_writing_half:
    what: the streamed and live renders, closing the port
    shape: the callback entry of decision:stream-callback-shape, extended to htmlupdate; OpenStream and OpenLiveStream are gone and WriteStream and WriteLiveStream replace them on both transports
    record_writer: takes an io.Writer and duck-types Flush and Flush error, so the http.Flusher field that made it transport-bound is gone
    plan_then_run: everything read from a request is captured before the response opens, which is what lets one loop run from a fasthttp body stream writer
    render_reclassified: Options.Render never flushed and needed no inversion; see requirement:fasthttpbind-parity-scope
    residual: fasthttp has no per-request cancellation, so a live stream ends on a failed write rather than on a disconnect
  blocked_on_release:
    fact: the fasthttp fork is unreleased; it is 9 commits ahead of origin/main on the local tinygodriver checkout and absent from every tag through v1.1.12
    workaround: a gitignored go.work supplies it for local development
    gate: fasthttpbind cannot be committed with a resolvable dependency until tinygodriver publishes the package
  then: transport slots on generator call patterns, without which rule:transform-rewrite-table cannot know which arguments to drop
  then: the AST path in the binder and writer emitter, which builds strings today
  then: the parser adapter and the occurrence analysis, useful on their own
  then: root package and htmlupdate, the runtime surfaces
  last: generator parameterization, following the routetree pattern that already exists
arity_note: the survey names handler arity as the first question; decision:transport-source-transform answers it for handlers, leaving only the emitted signature, which is naming
related:
  - decision:transport-source-transform
  - decision:backend-build-tag-mode
  - requirement:fasthttpbind-parity-scope
  - requirement:router-type-independence
  - system:tinybind
runtime_port_closed_2026_08_10:
  claimed_by: downstream framework survey 2026-08-10 against v0.5.1, by comparing exported sets rather than by reading release notes
  reproduced_here_2026_08_10: both diffs re-taken against the worktree and both agree
  htmlupdate_against_fasthttpupdate: identical entry for entry; the one difference is the return type of RuntimeHandler, http.Handler against fasthttp.RequestHandler, which is the transport saying its own name
  httpbind_against_fasthttpbind: 71 exported functions against 64
  the_seven: AssembleOpenAPI, OpenAPIJSON, RegisterOpenAPIFragment, RegisterOpenAPIFragmentString, ResetOpenAPIFragments, SetOpenAPIInfo and SwaggerUI
  reading: nothing else is missing in either direction, so the runtime half of this port is finished and only the OpenAPI surface is unpaired
openapi_is_the_whole_remaining_diff_2026_08_10:
  five_of_seven_need_no_port: SetOpenAPIInfo, AssembleOpenAPI, RegisterOpenAPIFragment, RegisterOpenAPIFragmentString and ResetOpenAPIFragments take no transport, so a second backend calls them as they stand
  two_take_a_transport: api:openapi-json and SwaggerUI
  in_use_downstream: only OpenAPIJSON; served on the second transport by reassembling per request
  ask: api:openapi-assembly cached_read_is_unexported
closed_2026_08_10:
  page_tree: requirement:routetree-transport-selection, implemented the same day it was raised; a page tree now emits for either transport and a committed fixture serves through a fasthttp router
  openapi: api:openapi-assembly cached_read_is_unexported, answered by OpenAPIDocument
  reading: every ask this survey carried is answered, and what is left is the generator wiring that would select the routetree backend from a command line rather than from a caller building the emitter
patterns_closed_2026_08_10:
  was_listed_as_left: a page tree registered Go 1.22 spellings unchanged, so a router reading another syntax needed the rewrite the transform path already carried
  now: Symbols.CatchAllSuffix and Symbols.RootPattern, defaulting to net/http's own markers so an unset field changes nothing
  why_both: 'a router that does not read Go 1.22 patterns does not fail on these two, it misreads them: {rest...} becomes a parameter named "rest..." and {$} one named "$", so the route installs somewhere else silently'
cli_was_not_a_gap_2026_08_10:
  claimed_here_earlier: '-backend fasthttp does not reach routetree'
  checked: the generator's generate command is per-package and its GenerateRequest carries no route tree; RoutesName names the transform backend's registration of discovered handlers, not a page tree
  correction: there is no page-tree CLI for net/http either, so this is an absent feature rather than a hole in the port. A page tree is generated by a caller passing an emitter to routetree.Generate, which is what a framework does on both transports
  reading: the earlier line read a symmetry that was never there, and would have sent someone looking for a fasthttp-shaped bug in a command that has never generated a page tree
```

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
    stream: NewStream and NegotiateStreamFormat, which decision:stream-callback-shape reshapes into api:write-stream
    api_docs: OpenAPIJSON and SwaggerUI
  htmlupdate:
    count: 14 exported declarations; the package is the partial-update, redraw, and live surface end to end
    surface: WantsUpdate, Negotiate, WriteUpdate, WriteUpdateStatus, WriteNavigate, WriteFailure, Render, RenderStream, RenderStreamAsync, RenderLiveStream, Redraw, OpenStream, OpenLiveStream, CSRFToken, VerifyCSRF, RuntimeHandler, and Mount
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
```

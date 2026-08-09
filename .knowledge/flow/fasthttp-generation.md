---
id: flow:fasthttp-generation
type: flow
title: fasthttp Backend Generation
---
Generation reads the same net/http source flow:code-generation already parses, then admits, rewrites, and emits a tagged fasthttp copy of every handler and helper.

```yaml
flow:
  trigger: a generate run with the fasthttp backend selected, over a package whose net/http source is unchanged
  steps:
    - id: reuse-discovery
      action: run the existing discovery unchanged; the fasthttp backend adds no new way to declare a route or a model
      refs:
        - flow:code-generation
        - concept:handler-discovery
        - concept:route-discovery
    - id: seed-admission
      action: take every discovered handler as an admission seed
      refs:
        - rule:transform-eligibility
        - concept:handler-forms
    - id: close-call-graph
      action: add every same-package function that an admitted function passes a transport value to, and iterate
      refs:
        - rule:transform-eligibility
    - id: classify-occurrences
      action: classify each occurrence of a transport value as an admitted form or a refusal, and propagate refusals to callers until the set is stable
      refs:
        - rule:transform-eligibility
    - id: report-refusals
      action: on any refusal, emit every refusal with its position and chain, and stop before writing files
      refs:
        - requirement:transform-diagnostics
    - id: rewrite
      action: collapse signatures, substitute the context identifier, drop transport arguments from recognized calls, and apply enumerated selector rewrites
      refs:
        - rule:transform-rewrite-table
    - id: emit-handlers
      action: print the rewritten declarations into a fasthttp-tagged file, deriving imports from the rewritten body
      refs:
        - decision:backend-build-tag-mode
        - decision:transport-source-transform
    - id: emit-binders
      action: emit the fasthttp binder and writer for every discovered model, registering only those in the tagged init
      refs:
        - api:fasthttpbind-bind
        - api:fasthttpbind-write
        - rule:fasthttpbind-requestctx-lifetime
    - id: emit-registration
      action: emit the tagged route registration and the per-route body limit table and header hook
      refs:
        - rule:fasthttpbind-body-limit-mapping
        - requirement:generated-route-registration
    - id: emit-openapi
      action: emit the same OpenAPI fragment as the net/http run, since it derives from the field plan and not from the transport
      refs:
        - concept:openapi-generation
  wired_2026_08_08:
    selection: Options.Transform, nil by default, so a run that does not ask for a backend is byte-identical to one predating the feature
    phase: transportArtifacts, between the binder and configbind phases, sharing the one type check
    output: one package-wide file, tinybind_transport_gen.go, because the transform closes over the call graph and a handler's helper may be authored elsewhere
    refusal: stops the run and writes nothing, since decision:backend-build-tag-mode leaves no adapter and a partial emit would serve fewer routes silently
    report_only: TransformOptions.ReportOnly returns the refusals as parser diagnostics, writes nothing and exits zero, riding the same rail as --check
    cli: "-backend fasthttp, -transport-name, -transport-report"
    stamp: the transport file joins Paths and goPaths, so it is stamped and cache-checked with the rest; the stamp comment sits above the build constraint and the file still compiles
  binders_wired_2026_08_08:
    mechanism: the emitter takes a transportTarget naming the binder and writer parameter lists, the writer's transport variable, the runtime import path, and the build constraint
    bodies_unchanged: the fasthttp binder parameter keeps the name r, because every accessor the fasthttp runtime declares takes its transport value first under the same name, so only the signature line and the import move
    writer: the one place both halves were named; there w and r collapse onto r and the "_ = r" discard disappears with them
    alias: the runtime is imported as httpbind whichever package it is, so no emitted call changes
    file_split: selecting a backend tags the existing binder file !fasthttp and adds tinybind_fasthttp_gen.go; the transport-free half is duplicated into both rather than split out, because only one ever compiles
    why_the_split_is_needed: a generated init registering a net/http binder is reachable by construction, which is exactly the reachability rule:transport-dead-code-elimination warns about
    verified: one authored package generates two builds and each compiles on its own, tagged and untagged, in one test
  not_yet_wired:
    - the route registration and the per-route body limit hook of rule:fasthttpbind-body-limit-mapping
  outputs:
    net_http: unchanged; the authored files and their existing generated companions
    fasthttp: one tagged file set per package, compiled only under the fasthttp tag
  invariants:
    - a run with the backend unselected produces byte-identical output to a run predating this feature
    - the two backends produce the same status, headers, and body bytes for the same request, per requirement:fasthttpbind-parity-scope
    - no generated file imports both transports
prerequisites:
  ast_printing: the binder and writer emitter builds strings today; rewriting user statements needs an AST path, which templates and artifacts already use through format.Node
  call_pattern_slots: recognized calls must declare which arguments are the writer and the request, which no pattern records today
  stream_shape: decision:stream-callback-shape must land first, so no stream value is held across statements for the rewriter to track
related:
  - decision:backend-build-tag-mode
  - requirement:transport-port-surface
  - requirement:fasthttpbind-product-goals
```

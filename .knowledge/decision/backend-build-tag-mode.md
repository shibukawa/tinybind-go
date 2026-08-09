---
id: decision:backend-build-tag-mode
type: decision
title: Split Runtime Packages, Aliased Import, Tagged Application Files
---
Ship each transport as its own runtime package under the same declaration names, have generation import the fasthttp one under the httpbind alias, and tag only the application files, because the two surfaces never needed to be one package and the authored and generated handlers do need to be kept apart.

```yaml
status: accepted 2026-08-08
decided_by: user, 2026-08-08; no adapter, and the packaging arrived at over the same session
no_adapter: settled and unchanged; a handler the transform refuses is a generation error, per rule:transform-eligibility
arrangement:
  runtime_net_http: the root package, unchanged and untagged
  runtime_fasthttp: a sibling package declaring the same names over a RequestCtx
  shared_leaf: the transport-free declarations both surfaces need, so neither owns them and a model struct naming File pulls in no transport
  application_authored: "//go:build !fasthttp"
  application_generated: "//go:build fasthttp"
import_rewrite:
  mechanism: the generated file imports the fasthttp package under the httpbind alias
  example: 'import httpbind "github.com/shibukawa/tinybind-go/fast"'
  effect: call selectors in a rewritten body are untouched, so the transform's work on a recognized call reduces to dropping the transport arguments
  gain: the two import paths cost nothing at the call site, which was the only argument the single-package model had left
file_granularity_2026_08_08:
  found: while implementing the rewriter
  fact: a build tag excludes a whole file, so an authored file holding a transport handler beside a type, const or var declaration cannot be tagged !fasthttp without taking those with it, and both tags need them
  consequence: transport handlers belong in files containing nothing else, which is a real constraint on application layout rather than a style preference
  enforced: the rewriter reports every authored file mixing the two, naming the declarations that would be lost
  same_rule_upstream: decision:framework-tag-boundary reaches this from the other direction, keeping a framework's tagged layer thin
why_tags_are_still_needed:
  reason: the authored handler and its generated counterpart are the same function name in the same application package, so one of them must be excluded or Go reports a redeclaration
  scope: application files only; no library file lives behind a tag
  consequence: go test, go vet, and gopls cover both runtime surfaces in one untagged run, which a tag-switched single package would not allow
naming_needs_no_prefix:
  question_raised: whether the fasthttp declarations should carry a prefix such as FHBind, to keep completion clean
  answer: no; an authored file imports the net/http package and is compiled under !fasthttp, so the fasthttp declarations are neither imported nor visible to completion there
  therefore: the fasthttp surface reuses Bind, Write, WriteStatus, WriteError, and the stream entry
shared_leaf_contents:
  transport_free_today: check_helpers.go and file.go
  nearly_transport_free: errors.go, whose only net/http use is status constants and StatusText, both trivially inlined
  why_it_matters: Problem, FieldError, and the error constructors must be one type across both surfaces, or an error crossing between them stops matching
  doc_note: doc.go names net/http in prose only and imports nothing
given_up:
  incremental_migration: adoption is all-or-nothing per build; a service cannot move route by route while running
  mixed_process: one binary serves one transport
  third_party_middleware: anything net/http-shaped is unusable in a fasthttp build, since nothing wraps it
forces_a_transitive_transform:
  reasoning: with no fallback, one refused helper makes the application unbuildable rather than slower
  specified_by: rule:transform-eligibility
wiring: generation emits the tagged route registration; the application supplies a small tagged server bootstrap per transport, which is the only application file expected in two copies
related:
  - decision:transport-source-transform
  - rule:transform-eligibility
  - rule:transform-rewrite-table
  - requirement:transform-diagnostics
  - flow:fasthttp-generation
  - decision:runtime-package-boundaries
```

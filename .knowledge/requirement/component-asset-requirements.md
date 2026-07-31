---
id: requirement:component-asset-requirements
type: requirement
title: Component Asset Requirements
---
Let a component declare the static assets it requires, report the required set of a chain before rendering, and let the caller decide where each asset is served.

```yaml
priority: should
source:
  - downstream framework component seam report 2026-07-31
  - requirement:static-asset-extraction
  - requirement:builtin-element-registration
review_gate: proposed
problem:
  library_shape: a component library is a template plus a script plus some Go; the template and the Go have a home and the script does not
  no_route: a library owns no route, no scaffold, and no document shell, so it cannot reference its own file the way a framework references its runtime through a caller-supplied URL function
  upgrade_reach: a capability added after a project was scaffolded reaches no existing project, because the shell is a file the author owns
half_already_closed:
  shipped: 2026-07-31, after the reported v0.2.8; requirement:static-asset-extraction now extracts a component's inline script into a content-hashed file and emits its reference tag, exactly as it already did for style
  remaining: that path serves a component declared in a template file of the generation unit being compiled; a registered or cross-package component still has no way in, and the written file is a filesystem artifact
  reading: the reported asymmetry between style and script is closed for the authoring case and open for the library case
declaration:
  owner: data:builtin-element-definition for a registered component, and the component's own head declaration for a template one
  content: the asset bytes, a media type, and nothing about where it is served
  identity: a content digest, so two libraries shipping a same-named file are two assets and a changed file is a changed URL
  division: the module decides what is required and what its identity is; the caller decides where it is served
embedded_table:
  form: generated Go holding the bytes, the digest, and the media type of every asset in the generation unit
  reason: nothing reads a filesystem at runtime, which is what requirement:tinygo-wasm targets need, where there may be no filesystem at all
  relation: requirement:static-asset-extraction writes files; this is the same asset set expressed as generated source, and the open question that requirement already carries
  choice: whether a generation unit emits files, the table, or both is a generator option, not a per-asset property
static_required_set:
  shape: a constant on the decision:generated-render-plan value, foldable through a chain exactly as requirement:fragment-capability-introspection folds the await flag
  accessor: on Fragment and on Wrapper, beside Head, HeadSources, HasAwaitBlock, and HasLiveBlock
  aggregate: a package-level union over the MergeHead argument pair, following the shape that requirement already established
  timing: readable before rendering starts, because it is bound to the value rather than produced by walking
  conservatism: a member below a conditional slot that never renders still counts, which is the direction requirement:fragment-capability-introspection already chose
  why_it_matters_most: a requirement:live-boundary-rendering delivery or a requirement:component-delta-rendering fragment swap can insert a component whose script was not in the first render; a statically known set lets the document carry every script a later delivery might need, so nothing is fetched mid-swap and no client-side loading design enters the module
  not_derivable_from_head: a head contribution is a ready-to-write tag string, so a caller reading Head gets markup rather than an asset identity, and a fragment produced while rendering contributes no head at all
url_function:
  shape: a caller-supplied function from asset identity to reference URL, generalizing the existing scaffolded runtime-script-URL pattern
  reason: only the caller knows the mount path, the cache policy, and whether it serves from memory or from disk
  head_contribution: the module builds the reference tag from the returned URL and merges it through requirement:head-merging like any other contribution
  absent: with no function supplied, a generation-time asset keeps the requirement:static-asset-extraction PublicURLBase reference it has today
delivery_rules:
  dedup: three components requiring one asset produce one tag; identity is the digest
  order: deterministic and independent of registration order, because `--check` in CI compares bytes
  never_inline: an asset is always a reference to a served file, so a policy may keep script-src to self with no nonce
  escaping: unchanged; a reference tag is static markup under rule:template-context-safety
fragment_path:
  rule: a response with no document shell reports the required assets it cannot deliver, rather than dropping them
  reason: the caller already rejects a fragment response carrying head contributions instead of discarding them, and an asset requirement deserves the same treatment
  form: the same accessor, so a caller decides before writing rather than discovering it in a browser
  precedent: requirement:head-contribution-provenance made the head case nameable for exactly this caller
non_goals:
  - bundling, minification, or any JavaScript toolchain
  - the module choosing URLs, serving files, or setting cache headers
  - inline delivery of any asset
constraints:
  - a project declaring no asset regenerates byte-identical Go
  - no reflection, no filesystem read at runtime, and no init-order dependency, per decision:reflection-free and requirement:tinygo-wasm
  - an asset never enters a requirement:component-output-cache body; it is a head reference, not component output
acceptance:
  - a registered component library ships a script the caller never names, and a page using that component links it
  - three components requiring one asset emit one tag
  - the required set of a chain is readable before rendering, and a fragment swap inserting a new component needs no mid-swap fetch
  - two libraries shipping a same-named file produce two assets with different URLs
  - a changed asset changes its URL, so an immutable cache header stays honest
  - regenerating with a different registration order produces identical bytes
  - a fragment response requiring an asset it cannot deliver reports it instead of rendering silently
  - a TinyGo target resolves every asset from generated source with no filesystem access
related:
  - requirement:framework-script-contribution
  - requirement:scoped-component-style
  - requirement:cross-template-components
  - decision:library-component-seams
open_questions:
  - whether a component may declare the custom element name its script defines, so generation catches a collision between two libraries; the reporter would use it and does not need it in a first shape
  - whether a cross-package template component reaches this seam at all, which depends on the requirement:template-file-scope question of what an external declaration may name
  - whether the embedded table and the written files are mutually exclusive per generation unit
  - how an asset required by a registered component orders against requirement:framework-script-contribution groups in decision:framework-script-delivery
```

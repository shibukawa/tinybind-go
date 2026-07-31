---
id: requirement:render-value-provider
type: requirement
title: Render Value Provider
---
Supply per-request values to a builtin element through a context-taking framework function resolved at generation time, so components still never receive an http.Request.

```yaml
priority: should
source:
  - requirement:builtin-element-lowering
  - user design discussion 2026-07-27
review_gate: proposed
problem:
  boundary: requirement:html-component-api forbids http.ResponseWriter and http.Request parameters on a component, and a fragment is immutable and reusable across requests
  need: a CSRF token, a request ID, or a nonce is per request, so it cannot live in the fragment or in generated static bytes
signature:
  form: func(ctx context.Context) (V, error)
  V: a named struct type, or a single value type when the definition declares one hole
  identity: data:builtin-element-definition Provider, resolved by rule:go-types-symbol-identity
  purity: the provider reads request-scoped state from the context; it must not write the response
context_channel:
  source: the request context the caller already supplies
  async_entry: the ctx argument of the api:render-html-chain async entry
  sync_entry: the existing context functional option on the sync entry
  reason: per-request resources belong to the caller, and a package-level default would be shared by every server in the process
  framework_role: the framework middleware puts the token or trace value into the context; tinybind only carries the context through
validation:
  timing: composition validation runs before any byte is written, so a missing context still allows a different response status
  rule: a composition containing a needs_context builtin element rendered without a context option fails with an actionable message naming the element
  static_part: provider symbol resolution and signature checking happen at generation time, not per request
execution:
  when: during the initial pass of requirement:chain-render-pipeline, at the plan step position
  before_commit: the initial pass ends with the flush that commits status and headers, so a provider error can still produce an error response
  after_commit: a provider invoked from a settled await boundary or a delta rerender cannot change the status; it routes through data:async-render-error like any other member failure
  head_pass: a provider never runs during the head pass, because requirement:head-merging contributions stay static markup
caching:
  exclusion: provider output never enters a requirement:component-output-cache key or cached body, per requirement:builtin-element-lowering per_request handling
  reason: a cached response carrying another session token is a policy:html-update-csrf-protection failure, not only a staleness bug
constraints:
  - no reflection; the generated step calls the provider and reads its result fields through typed access
  - the generated file imports the framework provider package only where a provider is used, per rule:usage-directed-generation
  - htmlbind gains no net/http dependency, so decision:runtime-package-boundaries is unchanged
  - a provider result is treated as untrusted data and escaped by position, unless the definition uses the opaque shape
acceptance:
  - a handler passing a request context renders the token without the component naming a request type
  - rendering the same fragment to a buffer with no context fails before writing, naming the offending element
  - a provider error during the initial pass produces an error response rather than a half-written document
  - a component containing a provider-backed element still compiles with no net/http identifier in generated output
call_site_wiring:
  question: whether a provider is instead supplied as an api:render-html-chain option at each render call
  default: no; the provider is a generation-time symbol and only the context flows through the render call
  rejected_form:
    shape: an option carrying a map from element name to implementation
    reason: this is the requirement:cross-template-components runtime_map shape, already rejected; names become runtime strings and a missing entry fails at render instead of at generation
  variant_if_needed:
    shape: one generated typed struct of providers passed as a single option, such as a Builtins value per generation unit
    gain: call-site substitution without name lookup, useful for a test double or a per-server implementation
    cost: every call site must pass it, and omitting it becomes a render-time failure rather than a compile error
    status: not selected; recorded because call-site wiring is a reasonable want
open_questions:
  - whether the typed provider-struct variant ships alongside the linked-symbol default
  - whether a provider result is memoized per render, per response, or invoked per occurrence
  - whether a provider may take typed element parameters in addition to the context
```

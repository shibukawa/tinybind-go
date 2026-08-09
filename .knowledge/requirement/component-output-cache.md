---
id: requirement:component-output-cache
type: requirement
title: Component Output Cache
---
Reuse explicitly cache-enabled component output for equivalent typed inputs until its TTL expires.

```yaml
source: concept:html-render-runtime-extensions
declaration: decision:cache-component-declaration
identity: decision:cache-key-derivation
store: api:cache-store
value: validated rendered HTML bytes for one component invocation
behavior:
  hit: write cached bytes through the caller's current response stream
  miss: render into an isolated buffer, publish only after the whole subtree renders, then reuse until expiry
  expiry: expired entries behave as misses
  no_store: without a supplied store the component renders normally, so caching is a deployment choice rather than a template rewrite
streaming:
  restriction: a cached component owns no decision:async-boundary-syntax boundary, so its output is one contiguous byte range
  buffering: only the cached subtree is buffered; the rest of the document keeps streaming
partial_update:
  boundary: independent from requirement:partial-update-boundaries; cache and client update flags may be enabled separately
  reuse: cached content validator may satisfy requirement:component-delta-rendering without rerendering
safety:
  - never cache errors or partial output
  - preserve rule:template-context-safety at insertion; cached bytes were context-checked when produced
  - cache only output whose complete dependency set is represented by the key
  - per-user variation is covered by the decision:cache-scope-declaration prefix; locale and any other axis the scope value does not cover must be an explicit parameter or the component must not be cached
  - a request property a builtin element declares makes the component ineligible rather than mis-keyed
concurrency:
  misses: concurrent misses each render; the runtime does not coalesce, so a store that wants single-flight implements it
  reason: coalescing inside the runtime would tie one request's cancellation to another request's render
compatibility: requirement:html-rendering-compatibility
acceptance:
  - identical inputs within TTL can avoid component logic and rendering
  - changed input, changed generated plan, or expired TTL cannot reuse the old value
  - a render with no store behaves exactly like the same template without the annotation
structured_output:
  layering: this cache reuses execution keyed on inputs; requirement:structured-render-output reuses transfer keyed on the template, so the two are complementary rather than alternatives
  collision: the stored value is assembled bytes, so a cached subtree on a structured path either loses the split or the store holds the structured form instead
  raised: downstream framework partial transfer report 2026-08-08, which prefers the structured form and says it is cheap to design for and awkward to retrofit
  forced_by_the_unit_set: requirement:structured-render-output makes an @cache component a unit, and a hit does not execute, so a store holding bytes alone loses the split for exactly the components declared most reusable
opaque_unit:
  decided: 2026-08-08 by the owner; a cached component's output is one opaque unit and is never decomposed internally
  general_argument: the annotation already declares this output reusable as a unit, so separating its statics from its variables contradicts its own declaration
  the_case_that_makes_it_obvious: a rendered markdown body arrives from an external function as one trusted string, so it has no template structure to separate, and recording spans inside it would find exactly one
  application_too: a client installs the unit whole, with no slot mapping to carry
  where_it_still_participates: it is a requirement:boundary-decomposed-render hole in its parent, so the parent carries a placeholder and the parent's own render computes the decomposition around it
  api_consequence:
    decided: api:cache-store keeps its byte slice; no typed entry, no span offsets, no static sequence address
    withdraws: an interface change proposed earlier in the same round on the assumption that a cached component would decompose internally
    reasoning_that_failed: a store holding bytes alone was said to lose the decomposition, and there is no decomposition inside to lose
  head_excluded: head contributions live on Plan.Head and merge before the first body byte; decision:cache-component-declaration forbids html parameters, so a cached component's head is fully static and nothing about it needs storing
  omitting_an_unchanged_cached_fragment:
    mechanism: the existing validator path; the parent's boundary list names the hole and the client returns the validator it holds, so an unchanged cached fragment is a hole with no fragment
    no_new_identity: requirement:component-delta-rendering already owns this comparison, so nothing about the cache has to reach the client
    never_send_the_cache_key: decision:cache-key-derivation frames every declared parameter in plaintext and hashes nothing in the runtime, so a cache key carries parameter values and must never reach a browser
    recorded_because: sending the key looks like a free way to let a client hold a cached fragment, and it would publish whatever the parameters are
open_questions:
  - stale-while-revalidate and explicit invalidation
  - caching a fully settled boundary set, which requirement:suspense-html-streaming currently excludes
  - what a rich entry costs the document path in practice, which is the one number that should be measured before the interface changes
```

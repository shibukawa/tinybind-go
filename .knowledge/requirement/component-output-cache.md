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
  - request, authorization, locale, and header-derived variation must be explicit parameters or the component must not be cached
concurrency:
  misses: concurrent misses each render; the runtime does not coalesce, so a store that wants single-flight implements it
  reason: coalescing inside the runtime would tie one request's cancellation to another request's render
compatibility: requirement:html-rendering-compatibility
acceptance:
  - identical inputs within TTL can avoid component logic and rendering
  - changed input, changed generated plan, or expired TTL cannot reuse the old value
  - a render with no store behaves exactly like the same template without the annotation
open_questions:
  - stale-while-revalidate and explicit invalidation
  - caching a fully settled boundary set, which requirement:suspense-html-streaming currently excludes
```

---
id: api:cache-store
type: api
title: Cache Store
---
Pluggable storage the caller supplies per render, holding rendered component output for requirement:component-output-cache.

```yaml
source:
  - requirement:component-output-cache
  - user composition request 2026-07-26
owner: shared HTML runtime, alongside api:render-html-chain
conceptual_signature:
  interface: |
    Get(ctx, key) ([]byte, bool)
    Set(ctx, key, value, ttl)
  unchanged_2026_08_08:
    decided: the byte slice stays; requirement:component-output-cache opaque_unit is the reason
    considered_and_withdrawn: a typed entry carrying slot span offsets and a static sequence address, proposed while a cached component was expected to decompose internally
    why_it_is_not_needed: a cached component's output is one opaque unit, so there is nothing inside it to report; it is a hole in its parent, and the parent's own render computes the decomposition that places it
    what_the_cache_supplies: the child's bytes, which is what it supplies today
    validator_not_stored: a hit yields the bytes, so a validator is computable from them; storing one would also read wrong under a build that changed a Go function body without changing a plan, since decision:cache-key-derivation keys on the plan fingerprint and not on the build identity
    reading: the interface survived the round because the component it serves declares its output reusable as a unit, and decomposing that would contradict the declaration
supply:
  path: a render option passed to api:render-html-chain, not package state
  reason: the store is a caller resource with a caller lifetime, and a package-level default would make two servers in one process share entries
  absent: cached components render normally; no key is computed and nothing is stored
context:
  argument: leading, so a shared network-backed store can honor request cancellation and deadlines
  sync_render: the render option may carry a context; otherwise the runtime supplies a background context
set_signature:
  no_error: a store write failure must not fail a response that already rendered correctly
  reporting: an implementation logs or counts its own failures
get_contract:
  miss: return false; an expired entry is a miss
  bytes: returned output is written unmodified, so a store must not mutate a returned slice
implementations:
  bundled: in-process store with TTL expiry and a maximum entry count
  external: a Redis or memcached adapter is caller code; the runtime declares no dependency
concurrency: a store is used from several goroutines per request under requirement:chain-render-pipeline and must be safe for concurrent use
key: decision:cache-key-derivation
constraints:
  - the runtime never enumerates or invalidates entries; expiry is the only removal path it relies on
  - a store may coalesce, evict, or ignore writes; correctness of the page cannot depend on a hit
```

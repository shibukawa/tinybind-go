---
id: rule:component-capability-combinations
type: rule
title: Component Capability Combinations
---
Validate interactions among slot, cache, async boundary, and client-update capabilities before Go emission.

```yaml
source: decision:component-capability-lowering
rules:
  async_pending:
    requirement: every pending effect is dominated by requirement:suspense-html-streaming before reaching an exported synchronous render boundary
    invalid: async external value escapes its decision:async-boundary-syntax owner
    signature: only the owning await boundary component is exported with decision:async-component-signature; a pending-only component is never exported
  slot_and_async:
    requirement: a slot owner is compiled without assuming the async effect of its continuation
    classification: requirement:chain-render-pipeline computes the effective mode from the assembled chain, not from the slot owner alone
    invalid: a generated slot contract that admits only sync or only async continuations
  slot_and_cache:
    requirement: slot content identity or canonical rendered bytes participates in the cache key
    otherwise: reject caching because parent parameters alone do not determine output
  cache_and_async_pending:
    requirement: cache only a completely resolved deterministic region
    initial_policy: reject unresolved async or streamed fallback output as a cache value
  cache_and_async_boundary:
    status: requires an explicit policy for settled-primary caching versus complete streamed interaction replay
    initial_policy: diagnostic until that policy is selected
  update_and_slot:
    requirement: boundary continuation captures child identity and immutable inputs; replacement preserves or regenerates descendant manifest state
  update_and_async:
    requirement: pending work is consumed by an await boundary inside the updated subtree and uses the boundary revision
    stale_completion: discard success and recover updates instead of updating a newer revision
  update_and_cache:
    behavior: input validator may reuse cached HTML; returned content validator still updates data:component-update-manifest
acceptance:
  - compiler derives the same plan independent of source declaration order
  - unsupported combinations fail with capability names and source locations
open_questions:
  - final cache and suspense interaction policy
  - whether slot output uses a content digest, explicit slot key, or both in cache identity
```

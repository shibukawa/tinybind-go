---
id: data:component-render-capabilities
type: data
title: Component Render Capabilities
---
Compile-time feature set used to select and compose generated HTML rendering logic.

```yaml
source: concept:html-render-runtime-extensions
dimensions:
  route_role:
    ordinary: reusable component without file-route behavior
    page: leaf renderer in data:html-render-route-plan
    layout: slot wrapper discovered by requirement:layout-chain-discovery
  composition:
    leaf: no html slot
    slot: declares requirement:html-slot-syntax insertion points accepting typed html continuations; async-agnostic per requirement:chain-render-pipeline
  reuse:
    normal: render for each invocation
    cached: requirement:component-output-cache enabled
  async_effect:
    sync: no unhandled async dependency
    pending: directly or transitively reads requirement:async-external-functions
    async_boundary: consumes pending effects through a decision:async-boundary-syntax await clause and takes decision:async-component-signature
  client_update:
    static: no independent update instance
    boundary: requirement:partial-update-boundaries enabled
derivation:
  explicit: cache flag, update flag, declared slot, route file role, and decision:async-boundary-syntax
  local_inferred: direct async external references
  propagated: pending async effect through component call graph until an await boundary consumes it
  chain: effective execution mode from requirement:chain-render-pipeline; a slot continuation never propagates into its owner
metadata:
  - source declaration and call-site positions for diagnostics
  - stable component version for cache and update validators
  - selected lowering handlers from decision:component-capability-lowering
principle: dimensions compose; avoid one exclusive component kind for every feature combination
```

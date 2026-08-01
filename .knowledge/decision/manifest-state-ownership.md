---
id: decision:manifest-state-ownership
type: decision
title: Update Manifest State Ownership
---
Keep update state on the client and carry it per request, instead of holding rendered-document state on the server.

```yaml
source:
  - data:component-update-manifest
  - user reload-design discussion 2026-07-26
review_gate: proposed architecture requires user approval
options:
  client_held:
    state: the browser holds every instance validator and sends the relevant subset
    cost: request size grows with boundary count
  server_session:
    state: the server stores the last rendered manifest per document instance
    cost: session affinity or a shared store, eviction correctness, multi-tab and back-forward divergence, restart loses state
  hybrid_signed:
    state: client holds validators; capability data travels as a signed continuation the server can verify without storage
decision:
  chosen: client_held validators plus hybrid_signed boundary continuations
  no_server_store: the first milestone adds no server-side per-document state
rationale:
  - a Go HTTP framework must scale horizontally without session affinity
  - restart, deploy, and multi-tab keep working, because a stale hint degrades to a complete document per rule:delta-consistency-model
  - back and forward hold their own manifest in history state naturally
  - eviction and memory bounds stop being correctness problems
costs:
  request_size: a list page with many boundaries sends many validators
  key_management: continuation signing and rule:update-validator-computation keying need rotation, and rotation forces full renders
  no_server_diff_memory: the server cannot compute a delta without client hints
size_mitigations:
  - authoring guidance to place boundaries at meaningful regions rather than per list row
  - truncated validators at the length rule:update-validator-computation allows
  - omit validators for boundaries the client knows cannot change, such as layout frames excluding search parameters
  - a configured request-size cap; over the cap the client sends only frame-level validators and accepts a larger delta
transport_shape:
  preferred: navigation mode stays a GET on the page URL with a compact manifest header, keeping read semantics, caching, and existing authorization middleware unchanged
  fallback: exceeding the header cap degrades to fewer hints rather than switching method
  rejected_default: POST for navigation, because a read becomes uncacheable and unlike an ordinary page request
  boundary_mode: stays a protected POST to the generated update endpoint, per policy:html-update-csrf-protection
revisit_if:
  - real pages routinely exceed the header cap, making server-side manifest storage cheaper than degraded deltas
  - a use case needs server-authoritative document state for reasons beyond delta computation
open_questions:
  - manifest wire encoding and compression inside a header value
  - concrete header size cap and its interaction with proxy limits
  - continuation signing key provider API and rotation signaling
```

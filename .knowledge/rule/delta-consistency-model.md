---
id: rule:delta-consistency-model
type: rule
title: Delta Consistency Model
---
State exactly how much consistency a partially updated document guarantees, so divergence is a documented boundary rather than a bug.

```yaml
source:
  - requirement:component-delta-rendering
  - user consistency analysis 2026-07-26
guarantees:
  per_response: all boundaries covered by one delta response come from one server render and are mutually consistent
  per_boundary: each instance advances monotonically through its revision
  convergence: applying every accepted response in order reaches the state a complete document render would produce for those boundaries
not_guaranteed:
  document_atomicity: after independent api:client-component-update redraws, the document may mix renders from different times
  cross_boundary_invariants: a boundary whose correctness depends on a sibling's current state can display a stale pairing
  authoring_consequence: a value that must agree across regions belongs in one boundary or in an ancestor boundary
fencing:
  navigation: every navigation carries a sequence; a response from a superseded navigation is discarded unapplied
  boundary: a response whose base revision is not the latest accepted state is discarded or answered with authoritative replacement
  async: a requirement:suspense-html-streaming completion for a superseded revision is dropped, per rule:component-capability-combinations
  interaction: a navigation invalidates pending boundary updates for boundaries it replaces
document_validator:
  scope: valid only for a document produced by one complete render
  invalidation: any applied boundary update clears it, so a later navigation cannot claim document-level equality
staleness:
  causes: interrupted requirement:streaming-delta-response stream, failed mid-apply, expired continuation, rotated validator key, changed render version
  effect: the manifest is marked stale, hints are dropped, and the next request is a complete document
  never_error: a stale or missing manifest is always answerable; the server recomputes a full or larger delta
side_effects:
  requirement: boundary rerendering is repeatable and free of exactly-once effects, because a response may be discarded after the server rendered it
  consequence: mutations belong in ordinary handlers, not in a rerender path
acceptance:
  - out-of-order responses cannot restore older boundary state
  - a discarded response leaves no partial manifest entry
  - a stale manifest degrades to a complete document rather than to an error
  - an update whose server work was wasted causes no duplicate side effect
open_questions:
  - whether a document-wide generation counter is worth carrying for debuggability
  - rebase policy for a stale base revision instead of outright rejection
  - multi-tab behavior when tabs share cookies but hold independent manifests
```

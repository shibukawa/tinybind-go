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
fencing_as_built:
  status: 2026-08-08; neither mechanism above exists on the wire, so the per_boundary guarantee is stated and not carried
  measured: the delta body carries an instance id and a frame validator, and htmlbind/delta holds no revision at all, so nothing a client receives expresses order
  frame_is_not_a_fence:
    answers: whether two renders produced the same markup
    cannot_answer: which of two renders is newer, because a digest is unordered
    worse: a digest returns to an earlier value, so a region cycling 5, 6, 5 produces one digest at two different times and a late response carrying the first is indistinguishable from current state
  what_was_built_instead:
    technique: remove the race at its source rather than detect it afterwards
    where: rule:live-boundary-delivery navigation_ordering, where the client aborts its live request before applying navigation operations, so no delivery for the outgoing page surfaces at all
    recorded_reason: that rule considered a document generation counter for this race and rejected it as unnecessary
    verdict: a legitimate substitute wherever the client owns both sides of the race, which on this protocol is every case reached so far
  races_nobody_checked:
    redraw_versus_redraw: two redraws of one instance in flight — a search box firing per keystroke — returning out of order, leaving the region showing the earlier query's result under the later query's input; both are 200, both name the same instance, and both carry a valid ETag, so nothing detects it
    redraw_versus_navigation: a redraw in flight when a navigation applies, writing the outgoing page's markup into the incoming page; this is the fencing.interaction clause above, designed and unbuilt
    why_it_surfaced_now: requirement:partial-update-boundaries makes a reloadable component a boundary, so a redraw became a third writer to a region the navigation delta and a live delivery already write
  smallest_fix: state which technique covers which race in requirement:update-wire-contract client_obligations, and have the client abort a pending redraw, rather than adding a revision to the wire
  revision_deferred: it stays the answer for a race the client cannot abort, and for the debuggability the open question below already asks about
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

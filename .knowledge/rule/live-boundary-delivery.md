---
id: rule:live-boundary-delivery
type: rule
title: Live Delivery Ordering And Coalescing
---
Decide which deliveries of a live boundary reach the screen and in what order, so a fast source, a slow client, and a reconnect cannot leave a region showing older content than it already showed.

```yaml
source:
  - requirement:live-boundary-rendering
  - requirement:live-boundary-resume
review_gate: proposed
status: not implemented; blocked on requirement:component-delta-rendering, which supplies the response form and the validators every part of this depends on
revision:
  monotonic: a boundary's revision strictly increases across every accepted delivery on every live-mode response that serves it
  advances_always: an unchanged delivery advances the revision even though it carries no operation, so the client's resume state stays current
  base_check: the client applies a delivery only when its base revision matches the boundary's current applied state
  discard: a delivery whose base revision is older is dropped, which is what makes an overlapping older response harmless rather than corrupting
  reuse: this is the requirement:boundary-parameter-updates revision rule applied to server-initiated renders
navigation_ordering:
  contract: a client performing a decision:response-mode-header navigation must abort its live-mode request before applying navigation operations
  mechanism: aborting the fetch surfaces no further chunks to the client, so no delivery for the outgoing page can be applied after the incoming one, whatever is still in flight on the network
  no_epoch_needed: a document generation counter was considered and is unnecessary, because the race is removed at its source rather than detected afterwards
  full_navigation: a real navigation aborts the request by itself, so only same-document navigation needs the explicit abort
  server_side: the server observes the abort as ordinary request cancellation, which affects when resources are released and never whether the client applied something stale
  content_rule: rule:live-boundary-content covers what a delivery may safely replace once ordering is settled
ordering:
  per_boundary: deliveries for one boundary are applied in revision order
  across_boundaries: no ordering is defined between two boundaries, because they are independent subscriptions that happen to share one response
  no_transaction: two live boundaries never update atomically; a screen that needs a consistent pair must render them inside one boundary
coalescing:
  latest_wins: when a source produces faster than the boundary can render or the client can apply, intermediate values are dropped and the newest is rendered
  legal_because: decision:live-boundary-syntax deliveries are snapshots, so the newest value is sufficient by construction
  mechanism: the pull sequence of decision:live-external-signature blocks the source in its own yield, so intermediates are never produced rather than produced and discarded
  not_configurable: there is no delivery queue to size, because backpressure is the sequence itself
  consequence: a source whose values are individually meaningful, such as one event that must be seen, is the wrong shape for a live boundary and belongs in a source that yields the accumulated state instead
suppression:
  unchanged: a rendered delivery whose content validator matches the boundary's current one sends no HTML operation
  structural: a delivery that only appends sends insert operations, per requirement:component-delta-rendering
  nested: an unchanged nested boundary inside a changed live boundary may be omitted independently when the protocol can preserve its DOM, which is the requirement:partial-update-boundaries rule unchanged
  accessibility: granularity also decides what a screen reader announces, per rule:live-boundary-content, so falling back to ancestor replacement re-announces a whole log rather than its new item
  canonicalization: validators exclude transport markers and request-unique values, so an identical render compares equal across separate executions
restart:
  after_patch: a subscription restarted by api:client-component-update discards in-flight deliveries from the previous source
  after_resume: a restarted source's first delivery is always sent in the first milestone, per requirement:live-boundary-resume stateless_v1, so a resume repaints its live region once even when nothing changed
  identity: a restart keeps the rule:component-instance-identity instance and the revision sequence, so the monotonic rule holds across it even though the server held no state
failure_delivery:
  ordering: a failure delivery occupies a revision like any other, so a later success cannot be overtaken by it
  recover_then_primary: rendering recover and later replacing it with primary content is two ordinary deliveries, not a special transition
cancellation:
  no_output: expected cancellation and a superseded subscription produce no operation, which is the decision:async-boundary-syntax rule unchanged
  closed: a subscription the server ends is reported as closed, so the client stops expecting deliveries rather than treating the content as live
identifier_reuse:
  rule: a delivery replaces the same placeholder every time, and boundaries nested in its subtree keep their ids across deliveries
  requires: the superseded delivery's nested work is cancelled, per requirement:live-boundary-rendering identifiers, since a reused id is otherwise a live target for stale content
  reconnect: an id the client does not hold means the structure changed, which requirement:live-boundary-resume answers with a stop and a reload rather than an insertion
constraints:
  - a boundary's applied content is always some render the server produced, never a merge of two
  - no delivery is applied twice, because the base revision no longer matches after the first
  - a client cannot advance a revision on its own; only a server delivery does
```

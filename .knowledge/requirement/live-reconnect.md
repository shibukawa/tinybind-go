---
id: requirement:live-reconnect
type: requirement
title: Live Source Reconnect
---
Resume a dropped live delivery stream by re-requesting the same page in a live render mode.

```yaml
source:
  - user live reconnect note 2026-08-01
review_gate: proposed protocol surface requires user approval
application: decision:dom-application-strategy; a delivery replaces the region today, which is why a live region forbids form controls
existing_feature:
  what: an external live declaration returns a sequence, bound in an await block, and the boundary re-renders for every value it yields
  transport: one chunked response written by the live render entry points
  identity: boundary ids are allocated by position, so rendering the same chain again reproduces the ids already on the client's screen
problem: a chunked stream ends on any network fault, proxy timeout, or sleep and resume, leaving live regions frozen with no signal
status: delivered 2026-08-03; the mode it was designed to travel in now exists, per requirement:live-mode-token-contract as_built
what_shipped_2026_08_01:
  client: detection, backoff, give-up, and the normal-end distinction, all as client_policy_shipped below
  server: a delivery stream that stays open, reached by delegating the live entry to the streaming navigation entry
  token: none; the client sends the navigation token for both the first connection and every reconnect
  effect: the feature works and the mode does not exist, which decision:update-composition-seams found when a downstream built against the published table
  settled_by: requirement:live-mode-token-contract
client_policy_shipped:
  detection: a stream that ends without its terminator
  backoff: linear, bounded by a configured attempt count
  give_up: reload the page, so a server that never comes back does not leave a frozen screen
  normal_end: a terminated stream stops without reconnecting, because the server finished on purpose
shared_mechanism:
  with: requirement:streaming-delta-response, whose truncation rule is delivered
  detail: a stream ending without its terminator is already treated as unknown state, which is exactly the signal a reconnect needs
  remaining: a live producer to reconnect to
solution:
  trigger: the client observes the stream end without the page navigating away
  request: the same URL in a live mode of requirement:render-mode-negotiation, sharing the header namespace and the endpoint namespace
  server: re-execute the chain and resume delivering to the boundaries the client already holds
why_resumption_is_trivial:
  rule: every live delivery carries the whole state of its region rather than an increment
  consequence: a missed delivery costs nothing, so reconnect needs no cursor, no event log, no replay, and no equivalent of Last-Event-ID
  contrast: a stream of increments would have to be replayed from a checkpoint, which is what this design already avoids
  identity_reuse: positional boundary ids are the same property the automatic boundaries of requirement:layout-reuse-boundaries rely on
first_delivery:
  behavior: reconnect paints the current value immediately, because the source yields current state rather than waiting for the next change
  effect: a reconnected region is correct as soon as it arrives, without a visible gap
client_policy:
  - distinguish a stream that ended normally, because every source finished, from one that dropped
  - back off between attempts so a server restart does not attract a reconnect storm
  - stop reconnecting once the page navigates away, since the new page has its own boundaries
  - fall back to a complete page load after a configured number of failures
interaction:
  navigation: a navigation supersedes reconnection, per rule:delta-consistency-model
  redraw: independent; a redraw addresses one instance and does not touch subscriptions
  version_mismatch: complete document, like every other unrecognized condition
acceptance:
  - a dropped stream resumes without a full page load and without replaying missed values
  - a reconnected region shows current state rather than a loading placeholder
  - a client whose page navigated away issues no further reconnects
  - repeated failures degrade to an ordinary page load rather than retrying forever
answered_2026_08_03:
  mode_spelling: 'live;v=N', its own mode rather than a navigation held open
  body: the delta record stream, which already carried both a delta and a delivery
  validators: the opening delta carries them and a delivery does not, so a reconnect skips unchanged non-live boundaries without a delivery ever needing one
  backoff: exponential with jitter on a fault, prompt with jitter on a healthy close, honouring the server's retryMs hint; defaults are client policy through the live entry's options
open_questions:
  - server-side cost control when many clients reconnect at once, which is requirement:live-boundary-lifecycle
```

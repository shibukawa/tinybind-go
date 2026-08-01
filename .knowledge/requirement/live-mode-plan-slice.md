---
id: requirement:live-mode-plan-slice
type: requirement
title: Live Mode Plan Slice
---
Execute only the plan operations a live binding's arguments depend on when a page runs in live mode, so a subscription costs its live regions rather than the whole page it sits on.

```yaml
priority: should
source:
  - downstream framework live integration report 2026-07-31
  - decision:live-transport-boundary
  - requirement:live-boundary-resume
review_gate: proposed
status: not implemented; named as a later optimization in decision:live-transport-boundary and as an open question for the first milestone
problem:
  mechanism: decision:live-transport-boundary execution_is_the_reconstruction runs the route handler, its layouts, and the page again in live mode and discards the body, so every await boundary and every non-live component does its work for output nobody transfers
  cost_shape: the cost is proportional to the page, not to its live regions; a dashboard with one gauge and six database-backed panels pays for seven
occurrence:
  - the first subscription of every screen
  - each lifetime rotation, at a default of roughly one per client per ten minutes
  - each reconnect after a drop
  - every deploy, where requirement:live-boundary-resume turns every open screen over at once
not_workable_around_downstream:
  finding: this is the one item of the round nobody below the module can absorb, which is why it is the ranking one
  surface: what the caller holds is a parameter-bound Fragment chain; which ops execute is inside the plan, and the public API offers no partial execution
  extend_lifetime: trades away the authorization re-check and the rollover the bound exists for, per requirement:live-boundary-lifecycle
  jitter: disperses the load and leaves the total unchanged
  idle_close: removes unwatched tabs and adds one execution when they return
  reading: each available dial relocates the cost rather than reducing it
proposal:
  shape: a generated plan variant executing the slice of ops that feeds a live binding's arguments, selected at generation time
  feasible_because: generation already knows statically which values flow into a live binding's arguments, and decision:generated-render-plan already carries per-plan ops, so this is a plan variant rather than a new mechanism
  entry: the live render entry selects the variant; the document entry is untouched
hard_constraint:
  identity: requirement:live-boundary-resume addresses placeholders already on screen by positional id, so a sliced render must allocate the same ids the document render allocated
  consequence: ids are assigned from the full render tree rather than from the ops the slice happens to run; a slice that renumbers breaks the resume contract itself, not only this optimization
  test: the same chain rendered document-mode and slice-mode produces the same id set for its live boundaries
must_stay_in_the_slice:
  nested_await: an await clause inside a live primary subtree re-runs per delivery, per requirement:live-boundary-rendering, so it is part of the live region rather than of the discarded body
  plan_check: a plan Check can reject the render before anything is written, so removing it would let a live-mode request succeed where the document request failed
  authorization: anything whose effect the request's own authorization depends on, since decision:live-transport-boundary makes the page's own check the security control
acceptance:
  - a reconnect to a page with one live boundary and several settle-once ones executes the live boundary's dependencies and not the others
  - boundary ids from a sliced live-mode render address the same placeholders the document render created
  - a page whose whole render feeds a live binding executes exactly as it does today
  - a live-mode request that would have failed its checks in document mode still fails
  - the cost of one reconnect is expressible in ops executed, not only in page executions
open_questions:
  - whether the slice is computed per plan at generation time or derived at bind time from the chain actually assembled
  - whether a caller can observe which ops were skipped, given requirement:live-boundary-lifecycle wants the reconnect cost measurable
  - whether a side effect in a skipped op is a diagnostic at generation time or an accepted authoring hazard
```

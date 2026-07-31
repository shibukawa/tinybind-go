---
id: decision:live-integration-seams
type: decision
title: Live Integration Requests From The Downstream Framework
---
Accept all three live-integration findings from the framework that built a client against the shipped live runtime, and sequence them by what each unblocks rather than by how much each costs its reporter.

```yaml
source:
  - downstream framework live integration report 2026-07-31
  - decision:framework-integration-seams
review_gate: proposed
round:
  when: 2026-07-31, against the shipped live runtime rather than against the plan
  reporter: the downstream framework, which has a working client and paid the workaround cost for two of the three
  precedent: decision:framework-integration-seams, the 2026-07-30 round on generation seams, whose principle applies unchanged
verification:
  method: each finding checked against the runtime source before it was accepted
  liveness: confirmed; the await and live ops write identical placeholder markup, the delivery record is one shape for both, and the chain-level liveness flag is the only classification that exists
  lock: confirmed; the live clause's shared delivery state holds its mutex across the render callback, which reports the failure, while the await path reports from the boundary goroutine with no lock
  slice: confirmed; the live render entry runs the composed chain in full and the caller is told to pass a discarding writer, so the whole page executes for the deliveries alone
accepted:
  - what: requirement:live-mode-plan-slice
    value: highest, because it is the only cost in the round that no caller can absorb; every dial available downstream relocates it
    cost: highest, and the only item with design risk, since a slice that renumbers boundaries breaks requirement:live-boundary-resume itself
  - what: requirement:live-boundary-liveness-signal
    value: high; it is a fact the runtime already knows and does not state, and withholding it costs the caller two permanent DOM nodes per boundary, a stale-range branch, and a full retransfer per rotation
    cost: low; an added placeholder attribute and an added record field, both additive and byte-identical for a template with no live binding
  - what: requirement:live-error-report-off-lock
    value: lowest of the three in reach, and it is the only defect rather than a missing capability
    cost: lowest by a wide margin; a call site moves out of a locked region, with no signature change
    status: implemented 2026-07-31
sequencing:
  reporter_proposed: slice, then liveness, then lock, ranked by what each costs the reporter
  chosen: lock, then liveness, then slice
  why_lock_first: it is nearly free, it needs no design round, and it removes a stall that only appears while a source is already failing, which is when a stalled region is least acceptable; ranking it last by reach would leave a known defect open behind two larger pieces of work
  why_liveness_second: it is additive, it is forward-compatible with the data:component-update-manifest live marker requirement:component-delta-rendering already plans, and it turns a permanent per-boundary cost into a per-live-boundary one
  why_slice_last_in_order_and_first_in_priority: it is the ranking item by value and the agreement with the reporter is on that point; it belongs with the generated plan work rather than in front of two changes that block on nothing, and its identity constraint deserves its own design round
  not_a_disagreement: the reporter ranked issues by value and this ranks execution by readiness; both put the slice at the top of what matters
severity:
  reading: none of the three can put wrong content on screen, leak anything, or break the resume contract, so none is an incident
  correctness: only the lock item is a defect at all; the other two are a missing capability and a cost
  worst_case_lock: the reporter takes no context, so a reporter that blocks indefinitely leaks the clause's goroutines and sources rather than only delaying them, and cancellation does not free them; the response itself still ends, because the consumer returns on its own context
  worst_case_slice: a rolling deploy, where the cost is a load spike of the shape requirement:live-boundary-lifecycle already anticipates; the slice moves the threshold rather than changing the shape
  consequence: the lock item was taken on cost rather than on urgency
principle:
  applies: the decision:framework-integration-seams rule, widen a seam whose default output stays identical and whose contract stays the caller's
  fits: all three are additive or internal; none changes a shape an author's own template or Go is written against
  new_reading: a cost the caller cannot relocate outranks a cost the caller has already absorbed, even when the absorbed one is cheaper to fix
related:
  - requirement:live-boundary-rendering
  - requirement:live-boundary-resume
  - requirement:live-boundary-lifecycle
  - decision:live-transport-boundary
```

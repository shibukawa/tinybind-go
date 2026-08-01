---
id: requirement:live-error-report-off-lock
type: requirement
title: Report Live Failures Off The Boundary Lock
---
Call the caller's error reporter after releasing a live boundary's delivery lock, so a reporter that blocks cannot stall the subscription it is reporting on.

```yaml
priority: should
source:
  - downstream framework live integration report 2026-07-31
  - requirement:live-boundary-rendering
review_gate: accepted 2026-07-31, implemented the same day
status: shipped
where:
  live_path: the clause's shared delivery state held its mutex across the render callback, and that callback reported the failure before rendering the recover subtree
  sync_entry_too: the synchronous entry runs the same pump and the same shared state, so it held the lock across the report as well; the earlier reading that it was unaffected was wrong
  await_path: already correct; the reporter is called from the boundary goroutine with no lock held, so the two paths differed only in where the call sat
occurrence:
  triggers: a live source yielding an error, and a recovered panic in a source, which travels the same path
  amplifier: a reporter that blocks, such as a full pipe or a synchronous exporter
  bad_shape: a failing source produces failure deliveries and log pressure at the same moment, so the region stops updating exactly when it is failing
blast_radius: the lock is per clause, so a blocked reporter freezes that boundary's deliveries and no others; every binding of that clause is serialized behind it
why_the_lock_exists:
  rule: rule:live-boundary-delivery serializes deliveries so two bindings moving at once cannot put an older render on screen after a newer one, and so a consumer that is not reading blocks the sources instead of queueing behind them
  scope: that reason covers the render and the emit, which touch shared state; the reporter call touches none of it
workaround_rejected_downstream:
  shape: an asynchronous reporter, a buffered channel with drop, roughly forty lines plus tests, a drop counter, and a queue depth to choose
  why_rejected: it drops the log lines that explain a failure while the failure is happening, puts a log queue in the response path of a logging policy that deliberately has none, and reorders against every other log
fix:
  chosen: the render callback returns the failure to report instead of reporting in place, and the binding goroutine hands it to the reporter after the delivery state releases its lock
  rejected: reporting from the binding goroutine before entering the delivery state, because the decision to report depends on the cancellation check and the stopped check that live inside the locked region, so moving it earlier would change which failures are reported rather than only when
  size: one added result struct, one changed callback shape, and no exported signature change
  ordering_effect: reports from two bindings of one clause may interleave, which is already true of the await path and of two live boundaries
  residual: a blocking reporter still delays the next pull of the binding that reported, because that goroutine is the one waiting; what it no longer does is hold the other bindings of the clause
as_built:
  where: htmlbind, the live op's delivery result and the pump's per-binding wrapper
  covers: the yielded-error path, the recovered-panic path, and the synchronous entry, since all three run through the same pump
  test: a clause with one failing binding and one healthy one; the healthy binding delivers while the reporter is still inside its call, which times out against the previous code
acceptance:
  - a reporter that blocks delays no delivery of the other bindings of the boundary that reported
  - the reported error value and its normalization to data:async-render-error are unchanged
  - the recover subtree still renders inside the locked region, so delivery ordering is unchanged
  - both the yielded-error path and the recovered-panic path report off the lock
  - a failure is reported exactly once, as before
```

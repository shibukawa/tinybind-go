---
id: flow:signal-dispatch
type: flow
title: Signal Dispatch Flow
---
One server-authored signal from a live source to a registered client callback, and the lifecycle names that reach the same table from the other side.

```yaml
flow:
  trigger: a decision:live-external-signature source yields a data:signal in the error position of its iter.Seq2, while a decision:response-mode-header live-mode response is open
  precondition: the page registered its names during load, per requirement:client-signal-dispatch registration_before_dispatch
  steps:
    - id: emit
      action: the source constructs the signal, which rejects a reserved name and encodes its payload through the generated jsonbind codec resolved at the call site, then yields it as the error
    - id: block
      action: the source stays blocked in its own yield, which is the backpressure that keeps the signal from being dropped or buffered
    - id: classify
      action: the live pump checks cancellation, then classifies the error as a signal through the reflect-free unwrap walk of decision:signal-in-the-error-slot
    - id: bypass
      action: skip the failure path entirely, so no recover subtree renders, no UnrecoveredError is built, no data:async-render-error is normalized, and the render error hook is not called
    - id: forward
      output: the signal is yielded on the response iter.Seq2 in the error slot, with a zero Content, and the sequence continues
    - id: resume
      action: the source's yield returns true and it keeps producing; the boundary's content and revision are untouched
    - id: frame
      action: the ranging caller classifies the error, recognizes the signal, and writes it as a data:signal record in its own framing, per decision:client-runtime-ownership
    - id: read
      action: the client reads the record from the live stream it is already draining for delta records
    - id: lookup
      action: the client looks the name up in its registration table; a byte-for-byte match or nothing
    - id: dispatch
      output: the registered callback is called once with the parsed payload, with no eval, no new Function, and no global name resolution, so the page's script-src is not exercised
    - id: continue
      action: the client returns to reading records; nothing about the boundary state it holds has changed
  failure:
    unknown_name: the client ignores the record and keeps reading, because a server ahead of a client is ordinary and a stopped screen is worse than a missed instruction
    reserved_name_emitted: the emit fails server-side, so it never reaches the wire and cannot impersonate a lifecycle name
    malformed_record: dropped by the client, which is safe because a signal carries no revision and skipping one desynchronizes nothing
    handler_throws: caught and reported by the client; the apply loop continues, per requirement:runtime-lifecycle-signals handler_isolation
    unclassified_by_the_caller: an unmigrated loop returns on the signal, the response ends with no rule:stream-termination-marker record, the client reads truncation and reconnects; see decision:signal-in-the-error-slot migration
    boundary_gone: the pump discards the signal, as it discards a delivery, and the source's yield reports false so it stops
    disconnect: the request context ends and nothing in flight is written; the signal is not held and not replayed, per requirement:live-signal-emission best_effort
    aborted_by_navigation: the client aborted the request before applying navigation operations, so the signal is never dispatched even if its bytes arrived
    emitted_on_the_sync_entry: discarded; htmlbind.Render returns one error and no client is listening
    emitted_before_the_first_delivery: forwarded immediately on the live entry, because a signal waits on no binding; the document entry does not carry signals in the first milestone
lifecycle_flow:
  trigger: the client applies something, per requirement:runtime-lifecycle-signals
  steps:
    - id: apply
      action: the client applies the completion, the delivery, or the navigation operations, or reads the rule:stream-termination-marker terminal record
    - id: synthesize
      action: the client builds the reserved name and whatever the moment carries; nothing crosses the wire and no server code runs
    - id: dispatch
      output: the same table lookup and the same call as above, so an application handler sees one path
  ordering: after the application and never before, so a handler reads the DOM the signal describes
```

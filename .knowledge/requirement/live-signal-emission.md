---
id: requirement:live-signal-emission
type: requirement
title: Live Signal Emission
---
Forward a signal yielded by a live source to the ranging caller without rendering, failing, or ending anything, and never drop one to make room for a newer value.

```yaml
priority: should
source:
  - concept:signal-channel
  - decision:signal-in-the-error-slot
  - user design discussion 2026-08-10
review_gate: proposed
status: shipped 2026-08-11, the first milestone of concept:signal-channel, end to end from a source's yield to a signal record on the live stream
as_built:
  htmlbind: htmlbind/signal.go holds the type, the sealed interface, the reflect-free walk, and the name check; the classification is in liveOp's render callback and in execBlocking
  carried_through: htmlbind/delta DeltaRecord gained a Signal field and its loop classifies, and internal/updatecore writes the record kind; fasthttpupdate came free because both backends share updatecore
  wire: docs/httpbind_update_wire_contract.md gained a Signals section and a seventh client obligation
  tinygo: the htmlbind suite passes under TinyGo, which is the check that the detection linked no reflect
  tests: 16 in htmlbind/signal_test.go over the classification, the entries, the fault path, and the walk; 4 in htmlupdate/signal_test.go over the wire record and the terminator
  change_surface_was_smaller_than_surveyed:
    predicted: six changed call sites including a forward path on deliveryResult
    actual: five, because the render callback closes over the emit function startStream hands it, so it forwards directly and deliveryResult is untouched
found_while_building:
  classification_must_precede_the_subtree_call:
    what: the render callback opened its subtree before examining the delivery error, and that call supersedes the previous delivery and cancels the nested boundaries it opened
    consequence_if_missed: a signal would have cancelled the work behind the content on screen, leaving a nested placeholder holding a fallback nothing would replace
    fix: classify first, and open the subtree only for a delivery or a failure
    covered_by: the nested-boundary test, which fails on the naive placement
  a_fault_must_not_stay_a_signal:
    what: AsSignal walks the wrap chain, and UnrecoveredError wraps what failed, so an invalid signal routed through the failure path arrived at the caller still recognizable as a signal
    consequence_if_missed: the caller classifies it, skips it, and keeps ranging a subscription that already ended
    fix: the fault travels as a description of itself rather than as the malformed value
    general_rule: nothing this runtime wraps in a failure may remain classifiable as a signal
  signal_is_never_a_delivery:
    what: keep must be true on every entry, not the entry's own keepOpen
    why: the rule that stops the document entry after one render counts deliveries; returning keepOpen there let a signal consume that one shot and strand the fallback on a page that had content to show
classification:
  where: the live boundary pump, at the point a non-nil delivery error is examined today, in htmlbind liveOp
  order: cancellation first, signal second, failure last; the cancellation check must stay first, because a source unblocking on a dead context must still produce no output
  before_normalization: a signal is classified ahead of the async error projection, so it never becomes a data:async-render-error
  must_be_here: decision:signal-in-the-error-slot classification_cannot_be_downstream verifies that no caller-side placement works, so this is the module's to implement
module_change_surface:
  surveyed: 2026-08-11, against the shipped runtime
  new_code:
    - the decision:signal-type-embedding struct, its sealed accessor, its constructors, and the Unwrap walk, beside the findPublicError walk it copies
  changed_call_sites:
    - liveOp.Exec render callback: classify before the recover-or-unrecovered branch, and forward instead of rendering
    - liveOp.execBlocking: classify and keep the subscription open, without marking the boundary as having delivered, so a signal before the first value still leaves the fallback rather than suppressing it
    - deliveryResult: a forward path distinct from both failure, which ends the subscription, and report, which reaches the error reporter
    - boundaryResult: a third kind beside a settled content and a failure
    - asyncCoordinator.startStream: a signal emit beside the Content emit, since the run closure today has only the latter
    - the sequence drivers in streamChain and CollectChainAsync: yield and continue where a failure yields and stops
  needs_no_change:
    generated_code: the generated LiveBinding closure passes whatever the source yielded straight to deliver, and a signal is an error, so it flows verbatim; this is what makes concept:signal-channel no_template_surface true in practice rather than only in principle
    liveState_deliver: a non-nil error already bypasses the wait-for-every-binding check and falls through to the render callback, which is exactly the mixed_clause behaviour below, so it falls out of the existing code rather than being added
    ordering: the boundary lock is already held across the render callback, so a forwarded signal is serialized against that clause's deliveries and the within-a-source ordering guarantee comes for free
  reading: one new type and six call sites, none of which changes a signature an application's own Go or template is written against
not_a_failure:
  no_recover: the recover subtree does not render, and the boundary's current content stays exactly as it was
  no_unrecovered: a clause that declared no recover subtree does not produce an UnrecoveredError, which is the one place the omitted-recover rule of decision:async-boundary-syntax does not apply
  no_error_hook: the render error hook is not called, because nothing failed; a signal is not an observation of a fault
  subscription_lives: the source keeps being ranged, and every other binding of the clause is untouched
no_render:
  produces: no boundary render, no buffer, no context-safety pass, and no data:async-boundary-content
  cost: a signal is forwarded and the source is resumed, so it does not queue behind a render the way a delivery does
  nested: no nested boundary is opened, superseded, or cancelled, so the identifier reuse rule of requirement:live-boundary-rendering is not engaged
no_revision:
  rule: a signal advances no boundary revision and carries no base revision
  reason: rule:live-boundary-delivery revision exists to order applications to a region, and a signal applies to none
  consequence: the client's resume state is unchanged by a signal, so a signal neither helps nor harms a reconnect
not_coalesced:
  rule: a signal is never superseded by a later delivery or a later signal
  resolves: the rule:live-boundary-delivery coalescing note that a value which must be seen is the wrong shape for a live boundary; it is still the wrong shape for a delivery, and this is the shape it belongs in instead
  legal_because: latest-wins is justified for a snapshot, which is sufficient by construction; a signal is not a snapshot of anything, so nothing later makes it redundant
  mechanism: the pull sequence blocks the source in its own yield until the signal has been forwarded, so a signal is neither buffered nor dropped, and the backpressure that coalesces fast deliveries slows a fast emitter instead
  no_queue: unchanged from rule:live-boundary-delivery; there is still nothing to size
ordering:
  within_a_source: a signal is forwarded in the order its source yielded it relative to that source's own deliveries, so highlight what just arrived is expressible
  across_bindings: undefined, unchanged from rule:live-boundary-delivery ordering
  across_boundaries: undefined, unchanged
  with_the_terminal_record: every signal precedes the terminal record of rule:stream-termination-marker, because that record is still the last thing written
  against_lifecycle: a requirement:runtime-lifecycle-signals name fires when the client observed something, so it is ordered against applications rather than against this stream; the two producers share a table and not a clock
best_effort:
  not_queued: a signal produced while no response is open is not held
  not_replayed: a reconnect delivers nothing that happened during the outage, which is the requirement:live-boundary-resume missed_deliveries rule applied unchanged
  not_acknowledged: the server learns nothing about whether a client dispatched one
  consequence: an instruction that must be seen exactly once does not belong here, and concept:signal-channel states it as a non-goal rather than leaving it to be discovered
  why_acceptable: the alternative is a per-subscription backlog and a cursor, which decision:live-boundary-syntax and requirement:live-boundary-resume refused for deliveries for the same reason
cancellation:
  gone_boundary: a signal yielded after the boundary is gone is discarded, exactly as a delivery is
  disconnect: the request context ends, the pump stops, and nothing in flight is written
  no_output: expected cancellation produces no signal record, which is the decision:async-boundary-syntax rule unchanged
entries:
  live: forwards signals; this is the only entry whose consumer is reading records
  document: does not forward in the first milestone, because the caller is writing into an HTML parser and an inert framing has not been specified
  sync: discards, because htmlbind.Render returns one error and no client is listening; a signal returned there would be indistinguishable from a failure to a caller that does not classify
  mixed_clause: a live binding may emit while a settle-once binding of the same clause is still pending; the signal is forwarded immediately rather than held until the first render, since it depends on no binding
reserved_names:
  rule: emitting a name under the data:signal reserved prefix fails at emit
  reason: the prefix holds the requirement:runtime-lifecycle-signals set and the existing control records, and a client trusts those names because only its own runtime produces them
compatibility:
  bytes: a project whose sources emit nothing streams byte-identically, per requirement:html-rendering-compatibility
  callers: the caller-seam change of decision:signal-in-the-error-slot migration is the one break, and it lands only on a project that adopts the feature
acceptance:
  - a source that yields a signal mid-stream keeps delivering values afterwards, and the boundary's content is unchanged by the signal
  - a signal from a clause with no recover subtree does not end the subscription
  - a source emitting faster than the response drains blocks in its own yield, and nothing is dropped
  - a signal yielded between two deliveries reaches the caller between them
  - a delivery that follows a signal advances the revision by one, as though the signal had not happened
  - the sync entry renders the first value and returns nil when the source emitted a signal before it
  - a source emitting a reserved name fails at emit rather than reaching the wire
  - a source that emits nothing produces byte-identical output to the shipped runtime
open_questions:
  - whether an unbounded emitter needs a per-response rate or byte bound, and whether that bound belongs here or with requirement:live-boundary-lifecycle
  - whether the document entry gains an inert framing later, given a toast on first load is the obvious case it does not serve
  - whether a signal emitted before the first delivery on the document entry should be held until the live-mode request arrives, which would need the queue this requirement refuses
```

---
id: requirement:runtime-lifecycle-signals
type: requirement
title: Runtime Lifecycle Signals
---
Specify a reserved set of signal names a conforming client dispatches when it observes a render arriving, so an application registers one table and stops writing DOM observers to learn what the runtime already knew.

```yaml
priority: should
source:
  - concept:signal-channel
  - user design discussion 2026-08-10
  - requirement:live-boundary-liveness-signal downstream_cost
review_gate: proposed
status: reference specification, names settled 2026-08-11; no module code, because the producer is the client
zero_server_side:
  claim: implementing this set adds no Go
  already_done: the one server-side piece, rejecting a reserved name at construction, shipped with requirement:live-signal-emission; a handler trusts a reserved name because application data has no route to it, and only Go can close that route
  already_on_the_wire: every fact a name below carries is already emitted — the rule:stream-termination-marker reasons, the boundary ids, the instance ids, and the frame validators
  what_would_break_it: specifying a payload the wire does not carry; see delivery_applied below, whose revision was dropped for exactly that reason
  module_deliverable: a section in requirement:update-wire-contract, not code
the_producer_is_the_client:
  rule: every name here is dispatched by the client runtime, locally, and none of them crosses the wire
  why: each describes an arrival, and the client is the party that observes one; it opens the live request, applies the completion, and applies the delivery
  why_not_the_server: a server record saying the update arrived is a claim about something the server cannot see, and it would be lost by exactly the truncation rule:stream-termination-marker exists to detect
  live_open_is_the_near_miss: the server does know it accepted a subscription, but the client knows it earlier and cannot lose the notice, so the client stays the producer here too
  consequence: this requirement adds no module code and no bytes; it is a specification plus client work
ownership:
  module_specifies: the reserved names, when each fires, what each carries, and the ordering rule below
  caller_produces: the dispatch, because decision:client-runtime-ownership puts the browser script with the caller
  precedent: the same split requirement:client-signal-dispatch already draws for the server-authored half, so this adds names rather than a mechanism
  where_written: requirement:update-wire-contract, as a client obligation; it is behavior a second implementation cannot infer, which is the shape that document's mostly_assembly.exception names
it_is_a_suffix_vocabulary:
  settled: 2026-08-11
  what: the set below names lifecycle moments as suffixes; the dispatched name is a namespace prefix plus one of them
  why: the module reserves tb. and produces nothing under it, while the runtime that actually dispatches belongs to a framework with a namespace of its own; publishing the suffixes lets both layers name one moment the same way
  module_form: `tb.<suffix>`, reserved and enforced, and in practice never emitted by anything
  framework_form: `<its prefix>.<suffix>`, for example `pw.boundary_settled`, which is what a client written on this specification actually dispatches
  reused_verbatim: a framework takes the suffixes as they are rather than renaming them, so one moment reads the same across two catalogs and a reader moving between them learns it once
reference_set:
  document_committed:
    fires: the rule:stream-termination-marker document marker was read
    carries: the reason, one of final, live_pending, or failed
    covers_all_three: the marker's three document-side reasons map onto this one name, so a handler distinguishes a finished static page, one about to go live, and one that ended on an unrecovered failure, without a name per outcome
    refined: 2026-08-11; the earlier draft fired on final and live_pending alone and left failed with nowhere to arrive
  document_truncated:
    fires: parsing finished with no terminal marker at all, which that rule already tells a client to detect
    carries: nothing; the useful part is that it happened, and recovery stays the caller's policy
    distinct_from_failed: a marker saying failed is a response that ended and said so; no marker is a response that was cut, and only the second means the bytes are untrustworthy
  boundary_settled:
    fires: after an await boundary's completion is in the DOM
    carries: the boundary id, which addresses an await marker rather than an instance
  live_opened:
    fires: after the live-mode response began yielding
    carries: whether this was a first subscribe or a reconnect, which requirement:live-boundary-resume makes the same request for and a handler may want to distinguish
  live_closed:
    fires: on any of the endings rule:stream-termination-marker names, including a stream that ended with no record
    carries: the terminal reason and the server's retry hint when one was sent, so a handler sees done, retry, and truncation apart without reimplementing the backoff rule
  delivery_applied:
    fires: after a live delivery's operations are in the DOM
    carries: the instance id and its frame validator
    revision_dropped: 2026-08-11; no revision exists on the wire, so specifying one would have made this the single name that needs server work, and rule:live-boundary-delivery has not shipped its revision rule
    why_the_validator_is_enough: a handler wants to know which region changed, and ordering is the apply layer's problem rather than the handler's
  navigation_applied:
    fires: after a decision:response-mode-header navigation delta is applied
    carries: the URL now displayed, which is what an analytics or scroll-restoration handler needs
  directive_received:
    fires: on a navigate or reload directive from data:component-delta-response
    carries: which one, and the target for a navigate
    weakest_of_the_eight: the page is about to go away, so a handler has little time to act; kept because logging one is cheap and its absence is invisible
naming:
  under_a_reserved_prefix: every suffix here is dispatched under a namespace an application may register a handler for and may never emit into
  why_reserved: a handler trusts delivery_applied because only its own runtime produces it; an application able to emit that name could make a screen believe a render landed that never did
layered_reservation:
  shape: each layer reserves its own prefix and enforces it in its own constructor
  module: tb., checked in htmlbind.NewSignal and rejected at construction
  framework: its own, for example pw., checked in the wrapper it exports over that constructor
  application: any name outside the prefixes above it
  why_the_module_does_not_hold_the_framework_prefix:
    reason: a constructor is called at a yield site inside a source and is not render-scoped, so it can reach no render option and no configured prefix; threading one there would mean passing configuration into every source
    consequence: a framework owning a namespace also owns the constructor that guards it, which is one wrapper function and no module surface
  client_side_is_indifferent: dispatch is a byte-for-byte table lookup, so a second reserved namespace needs no client change at all; a prefix constrains who may emit, never how a name is resolved
after_not_before:
  rule: a lifecycle signal fires after the client applied the thing it describes, never before
  reason: the whole use is a handler that reads or decorates what just arrived, and firing first would hand it the previous DOM
  exception: document_truncated, which describes an absence and has nothing to fire after
  against_a_server_signal: a client dispatches in record order, so a signal a source emitted before a delivery fires before the delivery_applied for it; that ordering holds only while dispatch stays synchronous with reading, which is therefore part of the contract
handler_isolation:
  rule: a handler that throws does not break the apply loop; the client catches, reports through its own diagnostics, and continues
  reason: a signal is a notification, and an application bug in a toast handler must not stop deliveries from landing
  no_veto: a handler cannot cancel, defer, or alter what it was told about, which concept:signal-channel states as a non-goal
what_it_replaces:
  today: a caller learns a boundary settled by observing the DOM or by patching the runtime, and requirement:live-boundary-liveness-signal downstream_cost measures one framework paying 43 of 341 client lines and two permanent comment nodes per boundary for a related question
  reading: the runtime knows these facts at the moment they happen and drops them, which is the same shape as that finding rather than a new class of problem
  not_a_substitute: this reports arrival and does not classify a boundary as live, which requirement:live-boundary-liveness-signal still owns and still needs
relation_to_dom_events:
  not_a_replacement: a caller free to dispatch a CustomEvent instead is not doing anything wrong
  why_specify_the_table_anyway: an application registering one table gets server-authored and lifecycle names through one path, which is the concept:signal-channel why_one_table argument; a caller dispatching DOM events as well is additive
acceptance:
  - an application registers one table and receives both a server-authored signal and a lifecycle name through it
  - a handler for delivery_applied reads the boundary's new content from the DOM without any observer of its own
  - a throwing handler does not stop the next delivery from being applied
  - an application attempting to emit a reserved name fails at emit, per requirement:live-signal-emission reserved_names
  - a client written from the wire contract alone dispatches the reserved set at the specified moments
open_questions:
  - whether delivery_applied may be subscribed per instance rather than per name, since a busy dashboard fires it hundreds of times a minute and a handler filtering afterwards pays for every one
  - how a client is conformance-tested against a set with no wire form, given requirement:update-wire-contract harness refuses a JavaScript entry surface; registering a handler through the ordinary API is arguably the feature rather than an exposed internal, and that reading has to be stated rather than assumed
  - whether an application may opt out of lifecycle dispatch entirely to save the per-application call, or the cost is small enough to ignore
```

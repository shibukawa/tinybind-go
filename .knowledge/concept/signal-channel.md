---
id: concept:signal-channel
type: concept
title: Signal Channel
---
Give the page one table of named callbacks, registered at load, that both the server and the client runtime dispatch into, so directing a screen never means transferring code.

```yaml
evidence:
  source: user design discussion
  received: 2026-08-10
  refined: 2026-08-10, from an application-event channel to a two-producer signal channel with a reserved lifecycle set
review_gate: proposed requirements require user approval
baseline:
  - concept:live-boundary-updates
  - decision:live-external-signature
  - rule:live-boundary-delivery
  - decision:client-runtime-ownership
problem:
  only_state: a decision:live-external-signature source yields snapshots, so the only sentence it can utter is "this region now shows X"; a one-shot instruction that renders nothing has no slot
  the_catalog_already_says_so: rule:live-boundary-delivery coalescing names the gap outright, that a source whose values are individually meaningful, such as one event that must be seen, is the wrong shape for a live boundary and belongs elsewhere, and no elsewhere exists
  nothing_reports_arrival: a caller wanting to know that a boundary settled, that a live response opened, or that a delivery landed has to observe the DOM or patch the runtime; requirement:live-boundary-liveness-signal downstream_cost is a measured instance of exactly that being paid
  workaround_today: the application opens its own socket beside the module's live response, which is the second endpoint per page decision:response-mode-header removed for deliveries, reintroduced one layer up
  script_is_not_the_answer: instructing a client by writing script into the response is what decision:client-runtime-ownership forbids in its constraints, and what a strict script-src costs a nonce or unsafe-eval to allow
ask:
  one_table: names registered once at load, per requirement:client-signal-dispatch; the handler does not care where a signal came from
  dispatched_not_executed: a name is a lookup key in that table and never code, which is what keeps the page's script-src a fixed allowlist
  out_of_band: a signal renders nothing, replaces nothing, and advances no revision
  does_not_disturb: it is not a failure, so no recover subtree renders and no boundary ends
  keeps_flowing: the source's sequence and the response's sequence both continue after it
  named_and_typed: a server-authored signal carries a name and a JSON payload, per data:signal
two_producers:
  server_authored:
    who: application Go, from inside a live source
    how: decision:signal-in-the-error-slot, the second slot of the iter.Seq2 the source already fills
    for: instructions the client could not have derived, because only the server knows them
    milestone: first, per milestones below
  runtime_lifecycle:
    who: the client runtime, locally
    how: requirement:runtime-lifecycle-signals, a reserved name set with no wire form
    for: arrivals the client is the one to observe, such as a settled boundary or an opened live response
    milestone: second
  why_one_table: an application handler cares what happened, not which side noticed; keeping two registries would make every handler pick a side and would put the reserved names somewhere an author could shadow them
  why_the_prefix_matters: data:signal reserved_prefix stops the two sets colliding, which is load-bearing once the runtime set exists rather than merely defensive
milestones:
  first:
    what: server-authored signals from a live source, sent manually through the error slot
    why_first: it is the whole feature in one seam, needs no client runtime work beyond dispatch, and settles the carrier question every later use depends on
    concepts: decision:signal-in-the-error-slot, decision:signal-type-embedding, data:signal, requirement:live-signal-emission, requirement:client-signal-dispatch, rule:signal-payload-trust
    status: shipped 2026-08-11, server side; the client half is the caller's and is specified rather than written here
  second:
    what: requirement:runtime-lifecycle-signals, the reserved set the client synthesizes
    why_after: it is specification and client work with no module code, and it reuses the table the first milestone establishes
  not_scheduled: signals from anywhere but a live source, which requires a second carrier and is deliberately left open
motivating_uses:
  server_authored:
    toast: a notification the server decides to show, which no region on screen represents
    attention: scroll to, focus, or highlight a region a delivery just changed
    invalidate: tell the client a cached thing it holds is stale
    redraw_elsewhere: ask for a requirement:component-redraw-endpoint reload of a region this subscription does not own
    ambient: play a sound, flash the tab title, start an animation
  runtime_lifecycle:
    first_paint: the document render committed, so a handler may start work that needed the whole page
    live_open: the live response opened or re-opened, and whether it was a first subscribe or a reconnect
    boundary_settled: an await boundary's content arrived and is in the DOM
    delivery_applied: a live boundary re-rendered, so a handler may re-run whatever decorates that region
  none_of_these_are_state: each is a thing that happened once, which is why a snapshot cannot express it
vocabulary:
  signal: a named, dispatched instruction or notification addressed to client code rather than to a region
  emit: yielding a server-authored signal from a live source, which the runtime forwards instead of rendering
  dispatch: the client's lookup of the name in its table, and the call that follows
  registration_table: the caller-owned name-to-callback map, `registerEvent(name, fn)` on the downstream framework's client object
  signal_name: the lookup key, not a selector and not code, per data:signal
  reserved_name: a lifecycle name under the module's prefix, which an application may register a handler for and may never emit
no_template_surface:
  decided: the template neither declares, names, nor observes a signal
  consequence: no grammar change, no annotation, no generator change, and no new op form; the first milestone is the Go source signature and the runtime pump
  compatibility: requirement:html-rendering-compatibility holds trivially, because a project that emits none renders and streams exactly as before
  cost: the generator cannot report a signal capability the way decision:component-capability-lowering reports a live one, so a caller decides on the live flag instead; any page with a live boundary may emit
scope:
  - server-authored signals from live sources, per decision:signal-in-the-error-slot scope_is_live_only
  - one runtime classification at the delivery pump, before the failure decision
  - one record kind on the live-mode wire, and the client obligation that reads it
  - a reserved lifecycle name set the client synthesizes and the module specifies
  - best-effort delivery, with no queue, no cursor, and no replay
non_goals:
  - a state channel; anything a region displays is a delivery, and a signal that carries display state is the wrong shape
  - at-least-once or exactly-once delivery, per requirement:live-signal-emission best_effort
  - a client-to-server channel, which stays api:client-component-update and requirement:template-server-functions
  - shipping code or markup to the client; the payload is data a registered callback reads, per rule:signal-payload-trust
  - replacing concept:streaming, which is typed JSON event delivery on an API handler's own response rather than a signal beside an HTML render
  - a general pub/sub bus; the module forwards what one subscription's source yields and owns no fan-out topology, which decision:live-external-signature already settled for deliveries
  - a veto or cancellation hook; a handler observes and never blocks what it was told about
naming:
  chosen: Signal, 2026-08-10
  why_not_event: concept:streaming owns event for typed JSON delivery on an API response and data:provenance-event owns it for config loading; decision:live-external-signature applied the same test to Stream and renamed rather than overloaded
  residual: requirement:live-boundary-liveness-signal uses the word for a placeholder marker, which is a title collision and not a surface one, since that marker is an attribute and never reaches this table
  client_side_spelling: the downstream framework's `registerEvent` keeps its own name; the module does not specify the caller's API, per decision:client-runtime-ownership
carrier: decision:signal-in-the-error-slot
go_shape: decision:signal-type-embedding
emission: requirement:live-signal-emission
lifecycle: requirement:runtime-lifecycle-signals
record: data:signal
client: requirement:client-signal-dispatch
trust: rule:signal-payload-trust
runtime_flow: flow:signal-dispatch
milestone: additive to the shipped live runtime; it needs no part of the requirement:component-delta-rendering stack, because a signal is not an operation on a boundary
open_questions:
  - whether a signal will later be emittable from outside a live source, and what carries it there, given the error slot pays only where a source yields repeatedly
```

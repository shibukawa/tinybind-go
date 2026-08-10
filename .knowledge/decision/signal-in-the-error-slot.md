---
id: decision:signal-in-the-error-slot
type: decision
title: A Server-Authored Signal Is An Error Value
---
Carry a server-authored signal in the error slot both sequences already have, the way fs.SkipDir carries a control signal, so no signature changes and no intermediate code has to be taught a third value.

```yaml
source:
  - concept:signal-channel
  - decision:live-external-signature
  - user design discussion 2026-08-10
review_gate: proposed
status: shipped 2026-08-11; the classification, the caller seam, and the helper are in place, and the in-repo callers are migrated
statement: a server-authored signal is a Go value implementing error; the runtime classifies it before it reaches any failure path, and the classification is the only thing that separates it from a fault
covers: the first milestone only; requirement:runtime-lifecycle-signals reaches the same table with no wire form and no error value, so this decision does not bind it
precedent:
  cited: fs.SkipDir and filepath.SkipDir, an error value that means neither failure nor success but keep going differently
  what_transfers: the error slot is the one channel every layer already forwards, so a control signal placed in it needs no parameter, no second return, and no wrapper type
  what_does_not: WalkDir absorbs SkipDir and never returns it to its own caller, where here it is forwarded to the ranging caller, per caller_seam below
scope_is_live_only:
  decided: 2026-08-10, narrowed by the user from an earlier live-and-async reading
  live: decision:live-external-signature yields iter.Seq2[T, error] many times, so a signal is one more yield among many
  async_rejected: requirement:async-external-functions returns (T, error) once, and a settle-once call that emits one signal and then cannot emit another is not the channel the ask describes
  sync_rejected: a synchronous external returns Result with no error slot at all, so there is nothing to ride; requirement:render-context-externals would be the only opening and it is a different mechanism
  reading: the error slot pays exactly where the source already yields repeatedly, which is one place
why_not_a_second_channel:
  emit_callback:
    shape: pass func(name string, payload) into the source
    why_not: it changes the decision:live-external-signature Go signature, which is the one shape the catalog promised covers a ticker, a fan-out subscription, and a service that already returns a channel; an adapter around an existing service would have to grow a parameter it has nothing to fill
  discriminated_delivery:
    shape: yield iter.Seq2[Delivery[T], error] where Delivery is a value-or-signal union
    why_not: every source, every adapter, and every test constructs the union on the value path too, so the cost lands on the common case to serve the rare one
  side_channel_on_the_context:
    shape: htmlbind.EmitSignal(ctx, ...) from inside the source
    why_not: it leaves the pull sequence's ordering behind, so a signal and the delivery it belongs with race; the error slot is ordered with the values by construction
    still_open: this is the shape a later non-live producer would need, which concept:signal-channel leaves as its open question
detection_without_reflect:
  constraint: htmlbind links no reflect on purpose, stated in htmlbind/async.go findPublicError, which is errors.As for a fixed interface written out by hand for exactly this reason
  consequence: errors.As is unavailable, because it reflects on the target
  shape: the same hand-written walk over Unwrap() error and Unwrap() []error, looking for one fixed interface a signal implements
  what_implements_it: decision:signal-type-embedding, an application type embedding the module's Signal struct, which promotes the unexported accessor the interface names
  sealed: an unexported method name is qualified by its defining package, so nothing outside can claim to be a signal
  joined: the []error case is already handled there, so errors.Join of two signals in one yield is expressible and both are forwarded
  errors_is_convenience: an exported sentinel the signal's Unwrap chain includes, so errors.Is(err, htmlbind.ErrSignal) works for a caller that only wants the classification; additive, and never the way the payload is read
classification_cannot_be_downstream:
  finding: verified 2026-08-11 against the shipped runtime; a caller cannot classify a signal that arrived as a delivery error, at any price, so the branch must be inside htmlbind
  recover_declared: htmlbind renders the recover subtree from the normalized error and emits it as ordinary Content, then returns keep, so the caller receives a settled boundary and never sees the value at all
  recover_omitted: the caller does receive it, as an UnrecoveredError, but liveState.deliver has already set stopped and cancelled the boundary context by then; recognizing it downstream and continuing revives nothing, because every binding of the clause is already unwinding
  no_wrapping_seam: the generated LiveBinding closure ranges the source and calls the deliver the pump handed it, with no interposition point between the two, and the render options carry a reporter rather than a delivery interceptor
  therefore: the pull-side classification is the only placement, which is what makes this a module change rather than a convention a framework can adopt on its own
  alternative_considered: emit the classification into the generated LiveBinding closure instead of the pump, keeping the runtime untouched
  why_not: the closure holds only deliver and has no path to the response sequence, so it would need a runtime call anyway, and the check would be duplicated into every generated binding rather than written once
caller_seam:
  decided: 2026-08-10 by the user; the signal rides the response sequence's error slot too, and the ranging caller classifies it
  shape: |
    for content, err := range seq {
        if err != nil {
            if signal, ok := htmlbind.AsSignal(err); ok { writeSignalRecord(w, signal); continue }
            return err
        }
        writeBoundary(w, content)
    }
  rejected_alternative: absorbing the signal at the pump and surfacing it as a field on data:async-boundary-content, which would have kept every existing loop working untouched and matched what WalkDir does with SkipDir
  why_the_error_slot_won: one classification rule holds end to end, so a reader of the source and a reader of the route handler learn the same thing once; a record field would have made the source-side and caller-side shapes differ for one value
  inverts: decision:async-component-signature execution.error, which says yield zero Content with the error and the sequence ends; that now holds for a failure and not for a signal
migration:
  breaks: any loop written as `if err != nil { return err }` against the live entries, which is every one shipped today
  sites: htmlupdate and fasthttpupdate route code, the reference loop in the htmlbind guides, docs/httpbind_update_wire_contract.md, and any downstream client written against them
  failure_mode_if_missed: the loop returns on the first signal, so the live response ends with no rule:stream-termination-marker record, the client reads that as truncation, and it reconnects
  severity: no wrong content and nothing leaked, because an unclassified signal is never applied; the cost is a reconnect that re-executes the page, per decision:live-transport-boundary execution_is_the_reconstruction
  worst_case: a source emitting steadily against an unmigrated caller becomes a reconnect loop, one full page execution per signal, which is why this is a migration item rather than a soft deprecation
  bounded_by: a project that emits nothing never yields one, so an unmigrated caller that adopts nothing is unaffected
  detection: the classification helper is exported by the same version that can emit, so a caller compiling against the new module has the call site available before any source can produce one
  done_in_repo: htmlbind/delta and internal/updatecore were migrated with the runtime, so both shipped backends classify; fasthttpupdate needed no edit of its own because it shares updatecore
  still_owed: any downstream client ranging a live entry directly, which is the case this section exists to warn
  helper_shipped: AsSignal, returning the value and a bool, plus ErrSignal for errors.Is when only the classification is wanted
  one_hazard_confirmed: AsSignal walks the wrap chain, so anything wrapping a signal is classified as one; the runtime therefore never wraps a signal in a failure, per requirement:live-signal-emission found_while_building, and an application joining a signal onto a real failure gets the signal reading
error_text:
  rule: the Error() string identifies the value as a signal and names it, and carries no payload
  reason: an unclassified signal reaches a log or an error page, and a payload there is the leak rule:signal-payload-trust exists to prevent
constraints:
  - classification happens before normalizeAsyncError, so a signal never becomes a data:async-render-error and never reaches a recover clause
  - the module exports the classification helper; a caller must never string-match the error text to recognize one
  - a signal is immutable once yielded, because data:signal encodes its payload at construction
related:
  - requirement:live-signal-emission
  - data:signal
  - concept:signal-channel
open_questions:
  - whether the helper is AsSignal returning (Signal, bool) or a Signal method on a returned interface, given neither may link reflect
  - whether a non-live entry that receives a signal from a mixed clause should discard it silently or report it through the render error hook
```

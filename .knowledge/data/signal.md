---
id: data:signal
type: data
title: Signal
---
One named instruction and its JSON payload: the Go value a live source yields, the record the live-mode response carries, and the name a client dispatches on.

```yaml
source:
  - concept:signal-channel
  - decision:signal-in-the-error-slot
review_gate: proposed
status: shipped 2026-08-11
as_built:
  name_grammar: must start with an ASCII letter, then letters, digits, dot, underscore, or hyphen, at most 64 bytes
  reserved_prefix: `tb.`, matching the tb- convention the boundary ids and the action attributes already use
  enforced_at: construction, and the pump turns a faulted signal into an ordinary failure so it is loud at the source
  wire_record: `{"r":"signal","name":...,"data":...}` on the ndjson live stream, written by internal/updatecore
  encoding_deviation: the payload appends itself through an interface rather than through a generic codec lookup; decision:signal-type-embedding as_built carries why
go_shape:
  authored_by: the application, per decision:signal-type-embedding; the module exports one struct and an application signal type embeds it
  implements: error, promoted from the embed, so it travels the slot decision:signal-in-the-error-slot chose with no boilerplate
  recognized_by: the unexported accessor the embed promotes, asserted through the Unwrap walk
  name: the dispatch key the client looks up, held unexported and set at construction
  payload: JSON bytes, encoded once at construction, held unexported
  initialized_marker: unexported; an embed that was never constructed is rejected at emit rather than forwarded unnamed
  error_text: names the value as a signal and carries the name; never the payload
  immutable: nothing mutates after construction, which mirrors the live source rule that a yielded value must not be changed afterwards
payload_encoding:
  constructor: generic over the payload type, so the codec is resolved at the emit call site while the concrete type is still known
  codec: the jsonbind generated encoder, looked up through the typeMarker[T] registry that exists precisely so dispatch needs no reflect
  encoded_when: at construction, not at write time, so the runtime seam holds bytes rather than an any it would have to reflect on to encode
  no_encoding_json: the runtime keeps the property concept:standalone-json-codec and api:encode-json already hold for every other typed payload in this module
  escaping: encoded for a script context as well as a JSON one, the rule htmlbind.Content.AppendJSON already states, so a caller may frame the record inside an inline data block as safely as in a response body
  raw_escape_hatch: a pre-encoded JSON byte slice, for a payload whose shape the application already has bytes for; the module validates nothing about it, which rule:signal-payload-trust records as the caller's
  absent_payload: legal; a name with no data is an instruction with no arguments
name_grammar:
  charset: bounded and stated, so a name is a lookup key and never a selector, a path, or an expression
  length: capped, because it is dispatched by a table lookup and an unbounded key is a way to spend memory on a miss
  case: a name is compared byte for byte, with no normalization, so the client's registration and the server's emission are the same string or they do not match
reserved_prefix:
  rule: one prefix belongs to the module; an application may register a handler for a reserved name and may never emit one
  holds: the existing control records, navigate and reload from data:component-delta-response directives and the terminal states of rule:stream-termination-marker, plus the whole requirement:runtime-lifecycle-signals set
  load_bearing: once the lifecycle set exists the two producers of concept:signal-channel share one namespace, so the prefix is what keeps an application from shadowing an arrival it did not observe
  enforced: at emit, so a source naming a reserved signal fails rather than reaching a client that would trust it
  layered:
    settled: 2026-08-11
    pattern: every layer that produces signals of its own reserves a prefix and guards it in the constructor it exports; the module holds tb., a framework holds its own such as pw., and an application uses what is left
    suffixes_are_shared: requirement:runtime-lifecycle-signals publishes lifecycle moments as suffixes, so `tb.boundary_settled` and `pw.boundary_settled` name one moment at two layers rather than two facts
    module_does_not_hold_the_others: a constructor is called at a yield site and is not render-scoped, so it can reach no configured prefix; a layer that owns a namespace owns the wrapper that guards it, which costs one function and no module surface
    dispatch_is_indifferent: a client resolves a name by byte-for-byte lookup, so a further namespace needs no client change; a prefix constrains who may emit and never how a name resolves
wire_record:
  applies_to: server-authored signals only; a requirement:runtime-lifecycle-signals name has no wire form, because the client is the one that produced it
  where: one more record kind in the decision:response-mode-header live-mode stream, beside the delta records and the terminal record
  fields: the record kind, the signal name, and the payload
  carries_not:
    instance_id: a signal addresses no boundary
    revision: it advances none, per requirement:live-signal-emission no_revision
    base_revision: nothing is applied to a region, so there is nothing to fence against
  size_bound: a stated maximum per record, so one source cannot fill a response; the aggregate bound belongs with requirement:live-boundary-lifecycle
  framing: the caller's, per decision:client-runtime-ownership, exactly as the delta records already are
other_modes:
  navigation: expressible as a data:component-delta-response directive, which already carries navigate and reload; an application signal is the open, author-named member of that set
  action_response: requirement:action-response-update carries a region list, and a signal beside it is what lets a server action say show a toast without rendering one
  document: the document response is bytes an HTML parser is consuming, so a signal there needs an inert element rather than a record; requirement:live-signal-emission entries leaves it out of the first milestone
  first_milestone: the live mode only, because that is the one mode whose consumer is already reading records
constraints:
  - the name is data, never code, and never resolved against anything but the client's own table
  - the payload is opaque to the module past encoding it; no field is inspected, rewritten, or projected
  - a record carrying an unparseable payload is a module defect, because the payload was encoded by a generated codec rather than assembled by hand
related:
  - requirement:client-signal-dispatch
  - rule:signal-payload-trust
open_questions:
  - whether the payload is carried as an embedded JSON value or as a JSON string the client parses again, given the second is simpler to frame and costs a second parse
  - whether an application name is required to be declared somewhere the server can check, so a typo fails at emit rather than silently at dispatch
```

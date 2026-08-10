---
id: requirement:client-signal-dispatch
type: requirement
title: Client Signal Dispatch
---
Dispatch a signal by looking its name up in a table the page registered before the stream opened, so the server directs the client without a byte of transferred code.

```yaml
priority: should
source:
  - concept:signal-channel
  - decision:client-runtime-ownership
  - user design discussion 2026-08-10
review_gate: proposed
status: specified 2026-08-11; the module half and the contract are shipped, and the client half is the caller's to write against them
as_built:
  contract: docs/httpbind_update_wire_contract.md gained a Signals section stating the record, the name grammar, the reserved prefix, and the table of client actions, plus a seventh entry in the normative client obligations
  obligation_added: resolve a signal name against your own registration table and nothing else, called out separately because a dynamic fallback for an unregistered name reaches the same code execution by another route
  not_written_here: the table, the registration API, and any handler; the downstream framework's `registerEvent` keeps its own name
ownership:
  module_owns: the data:signal record shape, the name grammar, the reserved prefix, the ordering guarantee, and the obligations below
  caller_owns: the table, the registration API, and what a callback does
  downstream_api: `registerEvent(name, fn)` on the framework's client object, named in the design discussion; the module does not specify it, per decision:client-runtime-ownership caller_owns
  precedent: the split is the one already drawn for the delta protocol, so this adds a section rather than a mechanism
one_table_two_producers:
  rule: server-authored signals from data:signal and reserved names from requirement:runtime-lifecycle-signals dispatch through the same registry
  reason: a handler cares what happened rather than which side noticed, per concept:signal-channel why_one_table
  separation: the reserved prefix is what keeps the two sets apart, and it is enforced at emit rather than at dispatch, so a client trusts a reserved name without checking anything
registration_before_dispatch:
  rule: names are registered while the page loads, before any live-mode request is issued
  reason: a table filled after the stream opened drops what arrived first, and the module holds nothing to replay it from, per requirement:live-signal-emission best_effort
  consequence: registration is a property of the page's own script, not of the subscription
no_code_transfer:
  rule: a client resolves a name only against its own table, and never with eval, new Function, import, a global property lookup by name, or an attribute handler it writes
  what_it_preserves: script-src stays a fixed allowlist with no unsafe-eval and no unsafe-inline, which is the property decision:client-runtime-ownership already guarantees for a completion chunk and rule:stream-termination-marker for the terminal marker
  why_it_is_the_whole_point: the flexibility comes from the payload varying, not from the instruction varying; the set of things the client can be told to do is fixed at build time and is exactly what its table holds
  module_side: the module writes no script into any response, unchanged
unknown_name:
  rule: ignore it, optionally report it through the caller's own diagnostics; never fall back to any dynamic resolution
  why_ignore: a deploy that adds a name to the server ahead of the client is ordinary, and a screen that stops working over an instruction it does not understand is worse than one that misses it
  why_never_dynamic: a fallback that resolves a name against anything but the table is the eval this design exists to avoid, reached by another route
payload_is_data:
  obligation: a callback treats the payload as untrusted data
  concretely: not assigned to innerHTML, not passed to a DOM sink that parses markup, not used to build a selector or a URL without the caller's own escaping
  why_stated: the module's escaping makes the record safe to transfer and to embed; what a callback does with a parsed value afterwards is outside every guarantee the module makes, which is the rule:signal-payload-trust boundary seen from the client side
apply_discipline:
  at_most_once: a signal is dispatched once, matching the apply-at-most-once obligation requirement:update-wire-contract already states
  aborted_request: a signal whose request the client aborted is never dispatched, even when its bytes arrived, which is the supersession discipline of requirement:update-wire-contract applied to this record
  navigation: the live request is aborted before navigation operations are applied, so a signal for the outgoing page cannot fire against the incoming one; this is the live_handoff_sequence obligation unchanged
  failure_path: a malformed record is dropped and the client keeps reading, because a signal carries no revision and skipping one cannot desynchronize anything
  handler_throws: caught and reported, never allowed to stop the apply loop, per requirement:runtime-lifecycle-signals handler_isolation; the same rule holds for a server-authored name
wire_contract:
  belongs_in: docs/httpbind_update_wire_contract.md, as a section beside the delta records and the terminal record
  states: the record kind, the name grammar and reserved prefix, registration-before-dispatch, the no-code-transfer rule, unknown-name handling, and the apply discipline above
  plus: the requirement:runtime-lifecycle-signals set and its firing moments, which is behavior rather than bytes and has nowhere else to live
  harness: the existing conformance suite over observable wire behavior, per requirement:update-wire-contract harness
capability:
  granularity: none of its own; any composition with a live boundary may emit, so requirement:fragment-capability-introspection HasLiveBlock is the flag a caller already reads to decide whether to load the client half
  reason: concept:signal-channel no_template_surface leaves the generator nothing finer to report
acceptance:
  - a page with a strict script-src carrying neither unsafe-eval nor unsafe-inline dispatches every signal without a violation
  - a name with no registration is ignored and the stream continues
  - two signals arriving in one response fire in the order the server wrote them
  - a signal delivered on a request the client aborted is not dispatched
  - a client written from the wire contract alone dispatches correctly without reading the reference runtime
open_questions:
  - whether a registration may be replaced or removed after the page loads, and what an in-flight dispatch does when it is
  - whether the contract should require a client to expose which names it registered, so a mismatch is diagnosable in development
```

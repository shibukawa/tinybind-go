---
id: rule:stream-termination-marker
type: rule
title: Explicit Stream Termination
---
End every stream the module produces with an explicit record naming whether more work exists, because no transport signal distinguishes a finished stream from a truncated one.

```yaml
source:
  - requirement:suspense-html-streaming
  - decision:live-transport-boundary
  - user termination discussion 2026-07-30
review_gate: proposed
status: partly implemented; the requirement:component-delta-rendering blocker cleared when that shipped
what_shipped: the delta stream's terminal record, and the client rule that a stream ending without it is truncated
what_did_not: the marker on a document response naming whether a live connection is expected at all, so a page with no live boundary cannot be told not to open one
consequence: requirement:live-mode-token-contract carries the missing half
principle: completion is never inferred from transport; it is always an explicit record in the byte stream
why_transport_cannot_say:
  document:
    fact: a chunked HTML response that ends without its terminating chunk is treated as end of file by the parser, which renders what arrived and fires DOMContentLoaded and load with no error surfaced to the page
    no_size_check: requirement:suspense-html-streaming omits Content-Length, so encodedBodySize from PerformanceNavigationTiming has nothing to be compared against
    consequence: a truncated document and a complete one are indistinguishable to page script
  live:
    fact: a fetch reader rejects on a truncated body and resolves done on a clean close, so truncation is detectable here
    still_ambiguous: a clean close can come from an intermediary as well as from the source ending, so done alone does not mean the work finished
  generalization: this is the decision:client-runtime-ownership commit-marker reasoning one level up; there, byte boundaries could not be trusted within a stream, and here the end of the stream cannot be trusted either
document_mode:
  written_by: the caller, when the decision:async-component-signature sequence exits, after every completion it wrote
  form: one inert element, the last bytes of the response, carrying no script
  states:
    final: this render produced everything it will ever produce; the client must not issue a live-mode request
    live_pending: the document part is complete and decision:live-transport-boundary live boundaries remain, so a live-mode request is expected
    failed: the sequence ended on an unrecovered failure, per decision:async-boundary-syntax omitted_recover; nothing more is coming and the committed fallback is not going to be replaced by this response
  derivation:
    upfront: whether live boundaries exist is known before rendering, from the requirement:fragment-capability-introspection live flag
    at_exit: whether the sequence ended normally or on a failure is known when the loop exits, which is where the marker is written
live_mode:
  written_by: the live-mode response when its sequence exits
  form: a terminal control record, as data:live-boundary-subscription describes
  states:
    closed_done: every source ended terminally; the client stops and does not reconnect
    closed_retry: the response hit a requirement:live-boundary-lifecycle bound, or the server is shutting down; the client is expected to reconnect
  why_both: requirement:live-boundary-lifecycle deliberately closes healthy responses at their maximum lifetime, so a bare close cannot mean stop
client_initiated:
  mechanism: the client aborts its own live-mode request, which needs no record because the client already knows it ended the stream
  when: same-document navigation, a hidden tab the caller chooses to release, and any client-side idle bound
  ordering: this is what rule:live-boundary-delivery navigation_ordering depends on
  three_sources: a live stream therefore ends from the source, from a server bound, or from the client, and only the first two need a record
client_decision:
  document_marker_final: stop; issue no live-mode request
  document_marker_failed: stop; the screen is known incomplete and recovery is the caller's policy
  document_marker_live_pending: issue the live-mode request
  document_marker_absent_and_parsing_done: the document was truncated, detected as no marker while document.readyState is no longer loading
  truncation_recovery:
    action: refresh through the navigation mode first, then enter live mode
    not_live_alone: a truncated document may have stranded an await boundary's fallback with no completion, and may have lost body content entirely, neither of which a live-mode request repairs
    cost: a navigation-mode refresh replaces the body without re-fetching a document, so the conservative choice is also the cheap one
  live_closed_done: stop
  live_closed_retry: reconnect promptly with jitter, because a lifetime rollover is not a failure and exponential backoff would stall a healthy screen
  live_stream_ended_without_a_record: treat as truncation and reconnect, which is safe because a reconnect re-executes and re-delivers
  unknown_marker_state: treat as live_pending and issue exactly one live-mode request, so an older client meeting a newer server neither loops nor sits on a dead screen
why_suppression_matters:
  cost: a live-mode request re-executes the route, its layouts, and its page, per decision:live-transport-boundary execution_is_the_reconstruction
  consequence: a client that cannot tell final from live_pending pays a full page execution per screen that never had a live boundary, so the attribute is a cost control rather than tidiness
  scale: that cost lands once per document for a static page, and once per reconnect for a live one
stall:
  not_detectable: no API reports that a stream is open but idle; only elapsed time since the last applied record can suggest it
  out_of_scope: the module defines no stall timeout, because a live source is allowed to be quiet for as long as its data is quiet
  client_option: a caller may impose its own idle bound and reconnect, which is indistinguishable from any other reconnect to the server
backoff_by_reason:
  concern: treating every ending as one kind of failure applies exponential backoff to the normal case, which is the requirement:live-boundary-lifecycle lifetime rollover
  rule: the terminal record already distinguishes the cases, so the client's retry policy keys on it rather than on the fact that the stream ended
  rollover: prompt reconnect with jitter
  truncation_or_error: exponential backoff with jitter and a cap
  done: no reconnect
  server_hint:
    resolved: yes, worth carrying, decided 2026-07-31
    why: a client's backoff can only react to failure, while the server is the only party that knows it is overloaded or mid-deploy and can spread load before anything fails
    shape: an optional delay on the retry record, the Retry-After idea applied to this protocol
  status: deferred with the rest of the transport layer
constraints:
  - the marker is inert and carries no script, so the no-nonce guarantee of decision:client-runtime-ownership holds for it too
  - the marker is the last thing written; nothing may follow it in either mode
  - a client must not infer completion from load, DOMContentLoaded, readyState, or a resolved fetch reader alone
  - a client that sees a marker twice treats the first as authoritative and ignores the rest
open_questions:
  - the element name, the record name, and the spelling of each state
  - whether the document marker also carries the number of boundaries the render committed, so a client can report which completions it never received
  - whether a failed document marker should name the failed boundary, given the recover-omitted case already reached the caller server-side
```

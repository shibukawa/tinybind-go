---
id: decision:live-transport-boundary
type: decision
title: Live Reconnect Mode On The Page Route
---
Carry live deliveries on the page's own route in a decision:response-mode-header live mode that skips the body transfer entirely and chunks deliveries for as long as the subscription lives.

```yaml
source:
  - concept:live-boundary-updates
  - decision:response-mode-header
  - decision:client-runtime-ownership
  - user reconnect discussion 2026-07-30
review_gate: proposed
status: implemented 2026-08-03; the live mode this decision names now has its own spelling
document_mode_shipped: a live boundary settles in place through the blocking op, so the document response commits first content and finishes, which is chosen.document_mode and first_delivery_inline built
live_mode_shipped: the deliveries connection travels on 'live;v=N', and only that mode keeps subscriptions open
independently_confirmed: a downstream reached rejected_endless_document_response on its own, from the proxy timeout and sleep-resume argument, and offered its implementation; see decision:update-composition-seams
problem:
  terminating_by_design: requirement:suspense-html-streaming ends its sequence when every request-owned boundary settles, and the response completes; a live boundary has no settle, so keeping it on that sequence means a document response that never finishes
  reconnect_has_no_home: a full document response cannot be resumed; re-requesting the page as a document re-renders and retransfers regions the user asked to leave alone
  set_changes: requirement:component-delta-rendering can insert and remove boundaries after the document is live, so the set of live regions outlives any one response
chosen:
  document_mode: commits fallback or first content for every live boundary and ends, exactly as an async response does today
  handoff: that response's rule:stream-termination-marker marker tells the client whether a live-mode request is expected at all, so a page with no live boundary costs no speculative execution
  live_mode: the same URL and the same generated route, requested again with the live mode header
  body_skipped: the route executes normally and writes no body output; only live deliveries are transferred
  framing: chunked transfer of one data:component-delta-response per delivery, scoped to the boundary that produced it
  duration: the response stays open for as long as the subscription lives, bounded by requirement:live-boundary-lifecycle
  request: data:live-boundary-subscription
  multiplexed: one live-mode response carries every live boundary of that page, so a dashboard with several live regions costs one connection
execution_is_the_reconstruction:
  mechanism: the route handler, its layouts, and its page run again for the same URL and the same credentials, so every live binding's arguments are evaluated with the values they had before
  supersedes: the continuation token, the server-side input reconstruction, and the per-boundary capability the earlier plan needed
  identity_matches: rule:component-instance-identity is deterministic for the same logical instance across executions, so the boundary IDs line up with the ones on screen without the client sending anything to prove it
  why_this_is_better: the reconstruction path is the render path, so there is no second way to compute a boundary's inputs that can disagree with the first
  cost: the page's own work runs again on every reconnect, and its await boundaries would run for nothing
  optimization_later: requirement:live-mode-plan-slice, which generation can compute because it knows statically which values feed a live binding's arguments; decision:generated-render-plan already carries per-plan ops, so this is a plan variant rather than a new mechanism
  idempotence: decision:response-mode-header states the handler-reuse constraint this depends on
first_delivery_inline:
  behavior: the document-mode render may treat a live boundary's first delivery as an await completion, committing real content instead of fallback when it arrives within the ordinary boundary deadline
  bound: it never extends the response past when an await boundary would have settled; a source with nothing yet leaves the fallback committed
  gain: the first paint shows content rather than a loading state, and a client with no JavaScript sees one real render instead of a permanent placeholder
  reuse: this is the existing flow:suspense-html-render path with no protocol addition, because the delivery becomes data:async-boundary-content like any other completion
rejected_endless_document_response:
  shape: keep the boundary on the document response's own sequence and hold that response open for the life of the screen
  why:
    - the document never reaches load completion, so the browser's loading state, back-forward cache eligibility, and any middleware that observes response completion stay wrong for as long as the screen is open
    - a dropped connection has no resume; the only recovery is a full document request, which discards the selective part
    - a boundary inserted by a later navigation delta cannot join a stream that started with the document
    - the connection is pinned for the session, which HTTP/1.1 keep-alive reuse and per-origin connection limits punish
  kept: the byte-stream completion form, which first_delivery_inline still uses during the document render
rejected_separate_endpoint:
  shape: a caller-authored live channel route beside every page, addressed by boundary handle and continuation
  why:
    - it doubles the route surface and lets the fragment path drift from the document path
    - it needs a capability token precisely because it has no page execution behind it, which is machinery the mode header makes unnecessary
    - the caller would own an endpoint whose correctness depends on generated identity rules, which is the wrong side of the decision:client-runtime-ownership line
  superseded: 2026-07-30 by decision:response-mode-header
delivery_form:
  chosen: data:component-delta-response, the navigation and boundary-update form, rather than the data:async-boundary-content id-and-html pair
  reason: a live re-render of a list is naturally an insert or a move, so the structural operations requirement:component-delta-rendering already defines are what a growing chat or feed needs
  unchanged_suppression: a delivery whose boundary output matches the boundary's current content validator sends operations for nothing and only advances the revision, which keeps a periodic re-render cheap on the wire
  not_parsed: the client reads a fetch stream rather than feeding a parser, so the decision:client-runtime-ownership commit-marker rule does not apply; chunked encoding is transfer framing, not markup
  record_not_markup:
    decided: past the initial document a delivery is a JSON record rather than HTML plus framing markup, approved 2026-07-31
    reason: the template-and-marker form exists to survive an HTML parser consuming bytes; with no parser reading, that framing buys nothing and a record is the ordinary shape
    shipped_today: htmlbind Content.AppendJSON writes the id and the rendered fragment as one object, escaped for a script context as well as a JSON one, so a caller streaming completions into a live document does not hand-roll it
    still_the_caller_s: the framing around the record — newline-delimited, an event stream, a length prefix — stays the caller's, because it has to match the client that reads it; decision:client-runtime-ownership is unchanged
    forward: data:component-delta-response is the same idea with operations and a revision instead of one fragment, so this is the record that grows into it rather than a form it replaces
  termination: the response ends with the rule:stream-termination-marker terminal record, which distinguishes a source that finished from a lifetime bound the client is expected to reconnect after; a resolved fetch reader alone cannot say which happened
ownership:
  module_owns:
    - the endless decision:async-component-signature sequence the live mode ranges
    - mapping one delivery to a scoped data:component-delta-response with a new revision
    - boundary identity, revision monotonicity, and the resume contract of requirement:live-boundary-resume
    - the live mode itself, because it is generated route behavior rather than caller code
    - the guarantee that no delivery carries script, so CSP needs no nonce
  caller_owns:
    - the browser script that issues the live-mode request, applies operations, and reconnects
    - reconnect timing, backoff, and whether a hidden tab keeps the request open
    - whether the mode is used at all, decided from requirement:fragment-capability-introspection
  changed: the endpoint is no longer caller-owned, because it is the page's own route
protocol_mode:
  relation: a third mode beside streaming completion and navigation delta
  driver: shares the navigation-delta driver, since operations are applied to live DOM with no parser running
  shared: boundary identity vocabulary and the operation form
security:
  - the live mode is an ordinary authenticated request to the page's own route, so authorization is the page's own check rather than a token's
  - policy:html-update-csrf-protection applies, and the custom mode header itself blocks the simple cross-origin requests it targets
  - nothing grants access a document request for the same URL would not have granted
  - decision:response-mode-header requires Vary on the mode header, without which a cache could serve a delivery stream where a document was expected
open_questions:
  - whether the module ships a reference live client as an opt-in requirement:framework-script-contribution, as decision:client-runtime-ownership plans for the streaming one
  - the media type of a chunked delivery stream, and whether it reuses the navigation delta media type
  - whether requirement:live-mode-plan-slice is in the first milestone or the full page render is accepted per reconnect; decision:live-integration-seams ranks it first by value and last in execution order
  - whether the protocol version for this mode is separate from the one data:html-client-bootstrap already carries
```

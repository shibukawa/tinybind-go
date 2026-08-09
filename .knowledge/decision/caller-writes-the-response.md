---
id: decision:caller-writes-the-response
type: decision
title: Caller Writes The Response
---
Stop writing any header field or status in htmlupdate and hand the caller a computed header set instead, because a library that decides half a response leaves a wrong header traceable to two places at once.

```yaml
source:
  - owner decision 2026-08-09, on the round after the manifest-entry defect
review_gate: approved 2026-08-09
position:
  module_writes: bytes
  module_computes_and_hands_over: the Vary axes, the content type, the echoed served mode, the live marker, and the entity tag
  caller_writes: all of it, plus the cache policy, the conditional answer, and the final status
  reason_given: splitting the responsibility makes the place a wrong header came from findable, which a shared write path does not
what_the_module_still_knows:
  vary: which request headers this module reads, per mode; a caller cannot derive it without reading this module's negotiation
  content_type: which body shape the negotiated mode produces
  echo: the served mode, which requirement:render-mode-negotiation needs so a proxy-substituted body stays detectable
  live_marker: whether the composition owns a live boundary, which is a property of the wrappers and leaf
  etag: a digest of bytes only this module has
  reading: everything it computes is something the caller cannot; everything it gave up is something the caller decides better
what_the_module_gave_up:
  cache_control: requirement:redraw-cache-policy had already made it an override; this removes the default it was overriding
  the_304: requirement:redraw-cache-policy shipped the comparison and the answer together; the comparison stays as Response.NotModified and the answer leaves
  failure_response: requirement:update-error-hook's OnFailure wrote the response; it now observes and the failure travels on the returned Response
  status: every buffered entry returns one rather than sending it
shape:
  buffered: Redraw, Sequence, WriteUpdate, WriteUpdateStatus, WriteNavigate return htmlupdate.Response{Status, Header, Body, Failure}
  streamed: Render, RenderStream, RenderStreamAsync, RenderLiveStream keep writing the body; the caller sets headers first from Headers, StreamHeaders, or LiveHeaders
  why_two_shapes: a redraw digests what it rendered, so its headers cannot exist before its body; a stream commits before its first record, so its headers cannot exist after
  apply: htmlupdate.ApplyTo copies a header set onto a ResponseWriter, adding rather than replacing so a caller's own values survive
  send: Response.WriteTo writes headers, status, and body, so the common case is one call
three_header_accessors_not_one:
  headers: for Render, where a live request resolves to the document that entry actually serves
  stream_headers: for RenderStream and RenderStreamAsync, where a live request is served as a navigation and terminated, so the echo says navigation
  live_headers: for RenderLiveStream, the one entry that holds subscriptions open, so live stays live
  why_not_one_function: the accessor cannot see which entry it is about to be paired with, and a response claiming a mode it did not serve is exactly the proxy-substitution failure the echo exists to catch
redraw_headers_is_the_odd_one:
  what: Options.RedrawHeaders returns only the Vary axes and is applied before the branch, not after
  why: a page and the redraws of the components on it share one URL, so the axes have to be on the response whichever way the request turns out
  consequence_if_skipped: a cache that learned only the page answers a redraw from it, and one that learned one component's redraw answers another component with it
cost:
  admitted: Vary is a correctness control rather than a preference, and a caller that applies nothing can be handed a delta body by a shared cache for a page request
  mitigation: every entry computes it and both ApplyTo and WriteTo write it, so omitting it is a decision rather than an oversight
  not_mitigated: nothing forces the call, which is the price of the split and is stated in the guide rather than hidden
precedent:
  htmlbind: sets no status, writes no header, and chooses no encoding, which is why it composes
  update_error_hook_drew_the_line: 'requirement:update-error-hook contrast_with_htmlbind said writing a response is not the same as deciding what a failure looks like, and kept the writing'
  this_moves_the_line_further: htmlupdate owns endpoints but not responses; owning an endpoint turns out to mean producing a body and its facts, not sending them
downstream_report_2026_08_09:
  accepted: the reporter took the whole of what the split hands over — cache policy for all four shapes, the conditional request, the refusal body, and the vary axes — and reports no argument against the split
  they_corrected_the_sequence_echo_on_the_answer: possible only because the header is theirs to write now, which is the split paying for itself on the round that introduced the defect
  failure_response_carries_no_vary:
    found: while checking their vary placement, not reported by them
    what: 'FailureResponse sets Content-Type alone, so a redraw refusal has no Vary of its own'
    why_it_bites: '404 is heuristically cacheable, so a stored redraw 404 with no Vary and no cache policy can be served to a document request at the same URL'
    who_it_bites: a caller applying vary only on the success branch; a caller following the guide's before-the-branch placement is covered by accident
    fixed: 2026-08-09; Options.failure adds the axes for the negotiated mode, so a refusal varies on what it read
    still_true_of_the_exported_one: FailureResponse has no Options and no request, so it carries none and says so; a caller raising its own refusal adds them
    test: TestARefusalCarriesTheVaryAxesAnAnswerWouldHave
  vary_placement_they_chose_differently:
    guide_said: apply RedrawHeaders before the branch, so a page response declares the redraw axes too
    they_do: declare the render and build headers before the branch, and leave kind and instance to the redraw response itself
    their_argument: a cache matches on each stored response's own Vary, a page response varying on the render header cannot be selected for a request naming a mode, and every redraw response already carries kind and instance
    verdict: correct, per RFC 9111 selecting-header-field matching; the reason the guide gave for the placement does not hold
    the_placement_still_earns_its_place: for the reason above — a refusal carries no vary at all — which is a defect to fix rather than a reason to keep the advice
    lesson: the advice was right for a reason that was wrong, and it took someone disagreeing with the reason to find the actual one
  etag_is_the_one_header_a_caller_cannot_take:
    what: it digests the body this package assembles, so producing it caller-side means rendering the component a second time
    status: the correct exception to the rule
    ask: state it in the guide beside the rule, which is a doc gap rather than a defect
    done: 2026-08-09, in docs/httpbind_update_surface.md beside the Vary note and in the wire contract's redraw response
docs:
  - docs/httpbind_update_surface.md 'What this package writes, and what you write'
  - docs/httpbind_update_wire_contract.md redraw response
  - docs/httpbind_render_modes.md static sequence cache table
related:
  - requirement:redraw-cache-policy
  - requirement:update-error-hook
  - requirement:render-mode-negotiation
  - decision:client-runtime-ownership
  - decision:caller-owned-wire-versioning
```

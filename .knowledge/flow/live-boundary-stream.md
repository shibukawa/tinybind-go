---
id: flow:live-boundary-stream
type: flow
title: Live Boundary Stream Flow
---
Continuous server-paced update flow for one live boundary, from document commit through delivery to reconnect on the same route.

```yaml
flow:
  trigger: a document containing a decision:live-boundary-syntax live boundary commits, and its client re-requests the same URL in decision:response-mode-header live mode
  steps:
    - id: commit
      action: document-mode response commits fallback, or the first delivery under decision:live-transport-boundary first_delivery_inline, and ends
    - id: publish
      output: data:component-update-manifest entry marking the instance live, so the client knows the live mode is worth requesting
    - id: decide
      action: client reads the document response's rule:stream-termination-marker marker and issues a live-mode request only when it says live work remains
    - id: request
      output: data:live-boundary-subscription, which is the page's own URL plus the live mode header and no per-boundary state
    - id: authorize
      action: the route authenticates and authorizes as it does for a document request, and enforces policy:html-update-csrf-protection
    - id: execute
      action: run the route handler, layouts, and page for the same URL, evaluating live clause headers and discarding body output instead of transferring it
    - id: subscribe
      action: start each decision:live-external-signature source under the request context
    - id: deliver
      action: pull one delivery and re-render only that boundary subtree with the delivery bound
    - id: compare
      action: consult requirement:component-output-cache, then compare descendant validators per rule:live-boundary-delivery
    - id: respond
      output: data:component-delta-response scoped to the subtree, chunked, with the new revision, or revision only when nothing changed
    - id: apply
      action: client applies operations when the base revision matches and stores the next revision
    - id: loop
      action: return to deliver for as long as the source yields and the response lives
    - id: bound
      action: requirement:live-boundary-lifecycle closes the response at its maximum lifetime or on idle, writing the rule:stream-termination-marker retry record
    - id: terminate
      action: a response whose sources all ended writes the done record instead, so the client stops rather than reconnecting
    - id: resume
      action: client reissues the same live-mode request, per requirement:live-boundary-resume, which re-executes the page and transfers only live deliveries again
  failure:
    source_failure_delivery: render the recover subtree from data:async-render-error and continue; a later value restores primary content
    source_ended: leave the last rendered content, report the subscription closed, and stop pulling
    render_error: keep current content, advance nothing, and report through the render error hook
    handler_error_before_delivery: answer with the ordinary status, because nothing is committed yet
    redirect_error: emit a navigate control record instead of a 3xx the fetch would follow into a body
    incompatible_render_version: emit a reload control record instead of applying operations
    disconnect: cancel every subscription on the response and leave no source goroutine running
    stale_response_delivery: client discards it because the base revision no longer matches
    truncated_document: no terminal marker once parsing has finished, which the client answers with a navigation-mode refresh before entering live mode
    truncated_live_response: the stream ends with no terminal record, which the client treats as a reconnect
    missing_mode_header: the route answers with the ordinary full document, which is the decision:response-mode-header invariant
```

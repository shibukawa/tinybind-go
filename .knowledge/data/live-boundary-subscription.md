---
id: data:live-boundary-subscription
type: data
title: Live Mode Request
---
The request that opens or re-establishes live delivery: the page's own URL and credentials, plus the decision:response-mode-header live mode token.

```yaml
source:
  - requirement:live-boundary-resume
  - decision:live-transport-boundary
  - decision:response-mode-header
status: not implemented; blocked on requirement:component-delta-rendering, which supplies the response form and the validators every part of this depends on
request:
  method_and_url: the page's own, unchanged, because the route executes as itself
  mode_header: the live mode token
  credentials: ambient, exactly as for the document request
  csrf_header: the policy:html-update-csrf-protection token, as for any credentialed non-document request
  body: none in the first milestone
carries_no_state:
  omitted: boundary handles, continuations, revisions, content validators, and component arguments
  why: requirement:live-boundary-resume reconstructs by executing the route, and rule:component-instance-identity makes the resulting IDs match what is on screen
  consequence: the request for a fresh subscribe and the request for a reconnect are byte-identical apart from ordinary headers
deferred_state:
  shape: one compact header carrying per-instance revision and content validator, plus an optional subscribed-instance list
  buys: suppressing an unchanged first delivery, and narrowing a multi-live page to the boundaries the client still shows
  constraint: it cannot travel in the URL or the query string, because the page must execute as its own URL; a header bounded in size, or a request body, is the only place for it
response:
  framing: chunked transfer, one record per delivery, for as long as the subscription lives
  per_delivery: data:component-delta-response scoped to the boundary that produced it, carrying its new revision
  no_body_transfer: the page's own body output is executed and discarded, so no document markup appears in the stream
  control_records:
    reload: generated version incompatible with what the client holds
    navigate: the page returned a requirement:redirect-error
    closed_done: every source ended terminally; the client stops and does not reconnect
    closed_retry: a requirement:live-boundary-lifecycle bound or a shutdown ended a healthy response; the client is expected to reconnect
  termination: one of the closed records is always the last thing written, per rule:stream-termination-marker, because a clean transport close cannot distinguish the two cases
  headers: Vary on the mode header, and no-store, because a delivery stream is never shareable
safety:
  - ordinary authentication, authorization, origin and CSRF policy, and request limits, all of them the page's own
  - no capability token exists to forge, because nothing is addressed that the page did not itself render
  - a custom mode header cannot be set by a simple cross-origin form or link, which is the class of request policy:html-update-csrf-protection targets
  - decision:response-mode-header requires Vary, without which a cache could store a delivery stream under the page URL
constraints:
  - the mode header never reaches template scope, so no live boundary can render differently because it is a reconnect
  - the request is idempotent in the sense decision:response-mode-header requires of any mode request
  - an absent mode header yields the ordinary full document, so this record describes an opt-in and never a default
```

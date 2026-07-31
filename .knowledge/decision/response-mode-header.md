---
id: decision:response-mode-header
type: decision
title: Response Mode Request Header
---
Select what one generated route returns with a request header constant, so a full document, an SPA partial with its head list, and a live reconnect stream are three modes of one route and one handler rather than three endpoints.

```yaml
source:
  - concept:live-boundary-updates
  - requirement:client-managed-head
  - user response mode decision 2026-07-30
review_gate: proposed
status: not implemented; blocked on requirement:component-delta-rendering, which supplies the response form and the validators every part of this depends on
statement: one URL, one generated route, one handler; a request header names the mode and the route decides what it writes
modes:
  document:
    header: absent
    writes: complete HTML through the decision:html-document-shell shell
    invariant: this is what any request without the header gets, so a bookmark, a crawler, a cross-site link, and a JavaScript-disabled browser always receive a document
  navigation:
    header: an SPA partial token
    writes: body-region operations plus the requirement:client-managed-head merged head tag list
    chain: the shell is omitted from the chain, which decision:html-document-shell already supports by rendering the body interior alone
    response: data:component-delta-response, which requirement:client-managed-head is already extending to carry the head list
  live_reconnect:
    header: a live token
    writes: no body transfer at all; only decision:live-transport-boundary deliveries, chunked, for as long as the subscription lives
    request: data:live-boundary-subscription
  extensible: a mode is a registered token, so a later mode is an addition rather than a new route
why_a_header:
  same_path: the route, its authentication, its parameter binding, and its handler are the ones already generated and already tested; a mode changes what is written, not what is computed
  no_endpoint_duplication: an SPA fragment endpoint and a live endpoint beside every page would double the route surface and let the two drift
  no_url_change: the URL stays the page's own URL, which matters because the page's render depends on the path and search parameters and must not see a mode token among them
  csrf_synergy: a simple cross-origin form or link cannot set a custom request header, so fragment and live modes are unreachable by exactly the requests policy:html-update-csrf-protection worries about
  supersedes: the decision:live-transport-boundary earlier plan of a caller-authored live endpoint, and the requirement:live-boundary-resume continuation-token reconstruction it required
rejected_accept_negotiation:
  shape: select the mode with Accept and a per-mode media type
  why:
    - the modes differ in what is rendered, not only in how it is encoded, so a media type understates the choice
    - browsers send Accept values the application does not control, and intermediaries rewrite them
    - rule:stream-content-negotiation already owns Accept for the concept:streaming API surface, so overloading it here would put two unrelated decisions on one axis
  kept: nothing; the mode header is a separate axis from the response media type, which each mode still sets normally
caching:
  vary_required: every mode-capable response must send Vary on the mode header
  why: without it a shared cache or CDN can store a body-only fragment under the page URL and serve it to a browser expecting a document, which is a cache-poisoning class bug rather than a cosmetic one
  enforcement: the generated route writes the Vary header itself rather than trusting middleware, because a route that supports modes always needs it
  private: a personalized mode response follows the existing policy:html-update-csrf-protection rule against sharing token-bearing HTML through public caches
  live_mode: a live reconnect response is never cacheable and says so
handler_contract:
  idempotence: a mode request re-executes the route handler, so a page handler must be safe to run again for the same URL
  scope: this is what HTTP already requires of a GET, so it constrains the same handlers that were already constrained; it is stated because re-execution is now a normal event rather than a user action
  side_effect_free_render: requirement:partial-update-boundaries already requires boundary rerendering be repeatable; this extends the expectation to the route handler in mode requests
  redirect: a handler returning a requirement:redirect-error in a fragment or live mode must produce a navigate instruction the client applies, not a 3xx a fetch would follow opaquely into a document body
  error: an error before anything is written keeps its ordinary status, because a mode response has not committed either
observability: the mode belongs in request logs and metrics labels, because one route's latency and error rate now mix a document render with a fragment render and a long-lived stream
constraints:
  - an unknown mode token is answered as the document mode rather than rejected, so an older client stays functional against a newer server
  - the mode header never reaches template scope; a template cannot branch on it
  - a mode changes what is transferred, never authorization or validation
open_questions:
  - the header name and the token spelling for each mode
  - whether the mode is read by generated route code or by a wrapper the caller installs
  - whether a generated route may opt out of modes entirely, and what it answers when asked for one
  - how a mode interacts with requirement:component-output-cache keys, given the same inputs render different transfers
```

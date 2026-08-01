---
id: api:client-navigate
type: api
title: Client Navigation API
---
Drive a navigation-delta page change from browser code and observe update lifecycle events.

```yaml
source: requirement:client-navigation
shipped:
  surface: update, navigate, redraw, apply, subscribe
  events: start, applied, superseded, fellBack, and redrawn, carrying outcomes and never component arguments
  subscriber_safety: a failing subscriber cannot break the update it is watching
  absent: reload and prefetch
surface: the same namespaced runtime module as api:client-component-update; no global symbols
conceptual_signature:
  navigate: navigate(url, options?) -> Promise<NavigateResult>
  reload: reload(options?) -> Promise<NavigateResult>
  events: subscribe(handler) -> unsubscribe
arguments:
  url: same-origin URL or path; a cross-origin value performs an ordinary browser navigation instead
  options:
    history: push, replace, or none
    scroll: top, preserve, or anchor
    signal: caller abort signal
result:
  applied: operations applied and next manifest installed
  fell_back: the runtime performed a full browser navigation, so the caller must not retry
  superseded: a newer navigation replaced this one
events:
  kinds: navigation start, response committed, operations applied, boundary updated, fell back, error
  purpose: progress indicators, analytics, and third-party widget reinitialization after requirement:delta-head-sync
  payload: URL, mode, instance IDs affected, and outcome; never component arguments or validators
behavior:
  - resolve after DOM operations, requirement:delta-head-sync gating, scroll, and focus handling complete
  - supersede and abort an in-flight navigation, per rule:delta-consistency-model
  - cancel pending api:client-component-update requests for replaced boundaries
security:
  - same-origin only; a URL derived from page content or a message is never navigated to as a delta
  - send the requirement:render-mode-negotiation mode header and, when required, the policy:html-update-csrf-protection header
  - never place manifest state or tokens in the URL
errors:
  invalid_url: reject without a request
  unavailable_runtime: no API surface exists, so authors must feature-detect rather than assume
constraints:
  - the API drives navigation state; callers do not rewrite runtime marker attributes, per decision:update-manifest-transport
  - link and form interception uses this same path, so behavior cannot diverge between manual and intercepted navigation
open_questions:
  - JavaScript namespace and generated typing strategy, shared with api:client-component-update
  - whether prefetch joins this surface
  - event granularity that does not lock protocol internals into public API
```

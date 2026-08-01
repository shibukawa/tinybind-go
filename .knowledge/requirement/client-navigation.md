---
id: requirement:client-navigation
type: requirement
title: Client Navigation
---
Turn same-origin page navigation into a navigation-delta request while preserving the observable behavior of a full browser navigation.

```yaml
source:
  - requirement:component-delta-rendering
  - user SPA-navigation discussion 2026-07-26
review_gate: proposed client behavior requires user approval
mode: requirement:render-mode-negotiation navigation mode
api: api:client-navigate
status: delivered
interception:
  eligible: same-origin GET link activation and GET form submission inside a document carrying the update runtime
  form_url: the fields become the query, so a search form refines the page it is on and replaces the URL rather than stacking a history entry per submit
  non_get: left to the browser, which is what keeps post-redirect-get working and why rule:form-state-reconciliation rarely has to clear a form
  excluded: modified click, non-default mouse button, target attribute, download attribute, cross-origin URL, and non-GET form
  opt_out: explicit author attribute disables interception for one element or subtree
  hash_only: a same-document fragment change is left to the browser
history:
  new_navigation: push the target URL after the response commits, not before, so a failed delta leaves history untouched
  same_url_update: replace instead of push
  back_forward: popstate issues a navigation-delta request with restore intent
  state: history entry stores scroll position and manifest identity, never component arguments
scroll:
  new_navigation: reset to top after operations apply
  restore: reapply the saved position after operations apply and after requirement:delta-head-sync stylesheet readiness
  anchor: honor a URL fragment after the same point
  inner_containers: only a rule:preserved-client-subtree region keeps its own scroll offset
focus_and_accessibility:
  - preserve focus when the focused element lives in a retained or preserved subtree
  - otherwise reset focus to a documented landmark, because a silently lost focus ring breaks keyboard and screen-reader users
  - announce navigation completion, since no browser-level page load event fires
  - update the document title through requirement:delta-head-sync, so history and assistive technology see the new page
concurrency:
  latest_wins: a new navigation supersedes an in-flight one and aborts it
  fence: responses carry a navigation sequence, per rule:delta-consistency-model
  redraws: pending api:client-component-update requests for the leaving document are cancelled
fallback:
  triggers: network failure, timeout, non-delta response, protocol mismatch, error status, or in-band navigate directive
  action: perform the ordinary browser navigation to the same URL, so the user action is never lost
  idempotence: fallback happens at most once per user action
no_javascript:
  - links and forms keep their native behavior
  - the runtime enhances only after it loads, so early clicks navigate normally
acceptance:
  - clicking a link updates only changed boundaries and leaves layout DOM in place
  - back and forward restore both content and scroll position
  - a delta failure still takes the user to the target page
  - rapid link clicks converge on the last target
  - keyboard focus and screen-reader context never disappear silently
open_questions:
  - prefetch and speculative navigation policy
  - View Transitions integration and whether it is opt-in
  - navigation progress indicator ownership
  - non-GET form submission and post-redirect-get handling
  - browser back-forward cache interaction with runtime state
```

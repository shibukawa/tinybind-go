---
id: requirement:action-response-update
type: requirement
title: Action Response Update
---
Let a mutating endpoint return the regions its action changed, so one round trip both performs the action and refreshes the page.

```yaml
source:
  - user action-response proposal 2026-08-01
review_gate: proposed protocol surface requires user approval
motivation: acting and then fetching the new markup is two round trips for one user gesture, and the second one re-derives what the handler already knew
model:
  server: after the mutation succeeds, the handler names the regions to rewrite and writes them
  client: the application makes its own fetch and hands the response to the runtime
  body: the same shape a redraw returns, so one client applies both
negotiation:
  signal: the requirement:render-mode-negotiation header in an action mode
  absent: the handler's ordinary response, which is JSON for an API endpoint and a redirect for a form handler
  effect: one endpoint serves a non-browser client, a page without the runtime, and an update-capable browser
  branch_point: a single predicate in the handler, so the two paths cannot drift apart
regions:
  selection: the handler chooses them in Go, naming a target id and a bound component
  target: the rendered root element must carry the same id, or the region becomes unaddressable after the first update
  count: many regions in one response, because one action commonly changes several
  navigate_instead: a directive replacing the region list, for an action that changed where the user belongs
status:
  rule: the response carries the handler's real status
  client: applies the regions whatever the status says
  reason: a rejected submission returns 4xx and the regions it carries are the validation errors, which is exactly what must be shown
  contrast: a redraw client falls back on a non-2xx, because there the status means the render failed
trust:
  new_surface: none
  reason: the handler authorizes the action and chooses the components in Go, so unlike requirement:component-redraw-endpoint no parameter arrives from the caller
  csrf: policy:html-update-csrf-protection applies, because the request mutates and carries ambient credentials
  cache: never cacheable
client_state:
  manifest: an action carries none, so it must leave the navigation validators alone
  invalidation: a rewritten region drops its stored validator, or a later navigation could call that boundary unchanged and leave the action's markup in place
addressing:
  navigation: an automatic boundary, by its framework attribute
  action_and_redraw: a region, by the id its author wrote
  resolution: the client tries the framework attribute and falls back to the element id, since both namespaces are server-controlled
acceptance:
  - a request without the header receives the endpoint's ordinary response
  - one request performs the action and rewrites the regions it changed
  - a 4xx action still rewrites the region carrying its errors
  - an action never restates the manifest, and never leaves a stale validator behind
  - an ordinary JSON response is never mistaken for an update
open_questions:
  - whether a form submission helper belongs in the runtime, or stays the application's fetch
  - whether an action may also request a navigation-style diff instead of naming regions
```

---
id: api:client-component-update
type: api
title: Client Component Redraw API
---
Redraw one registered component instance from the browser, naming it by its DOM id and supplying its inputs.

```yaml
source: requirement:component-redraw-endpoint
surface: the same namespaced runtime module as api:client-navigate; no global symbols
conceptual_signature: redraw(elementId, params, options?) -> Promise<RedrawResult>
arguments:
  elementId: the decision:author-declared-boundary-id the author wrote at the call site
  params: every declared parameter of the component, since nothing is reconstructed server side
  options:
    debounce: optional duration or scheduling policy
behavior:
  - resolve the element, refusing without a network request when no such id exists
  - issue the requirement:component-redraw-endpoint GET with the parameters as query values
  - supersede and abort an older in-flight redraw for the same id
  - replace the element with the returned root element, which arrives carrying the same id
  - resolve after the swap, reporting applied, superseded, or fell back
security:
  - the endpoint is public input, per rule:redraw-input-trust; the browser API is not an authorization boundary
  - target only the generated same-origin endpoint; an attacker-controlled URL is never fetched
  - apply policy:html-update-csrf-protection when the deployment requires it for credentialed reads
errors:
  unknown_id: reject locally
  unknown_kind: the component changed since the page loaded, so fall back to a complete page load
  invalid_params: rejected by the generated typed decoder on the server
constraints:
  - the API replaces the element; callers do not hand-edit runtime attributes, per decision:update-manifest-transport
  - a redraw is repeatable and free of side effects, because it may be retried or superseded
  - exact JavaScript namespace and generated typing strategy remain open
```

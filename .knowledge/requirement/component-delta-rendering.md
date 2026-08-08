---
id: requirement:component-delta-rendering
type: requirement
title: Component Delta Rendering
---
Transmit only changed partial-update boundaries after page navigation or direct boundary execution.

```yaml
source: concept:html-render-runtime-extensions
flow: flow:html-partial-update
mode_selection: requirement:render-mode-negotiation
request:
  navigation: target path, new search parameters, and prior data:component-update-manifest validators
execution_modes:
  navigation:
    - authenticate and validate the request normally
    - execute the generated route, runtime layouts, and page with current request state
    - build the next document update-boundary graph and validators
common_execution:
  - use requirement:component-output-cache when eligible; otherwise render boundary output before comparison
disappearance:
  case: the browser holds a boundary this render did not produce, which a shorter chain causes
  problem: the hints carry ids and validators but no structure, so the server cannot say where it was
  behavior: replace the outermost boundary, which removes it along with everything else that moved
  otherwise: the stale region survives whenever the boundary above it rendered identical markup
comparison:
  match: rule:component-instance-identity
  validators: rule:update-validator-computation
  unchanged: same content validator; omit boundary HTML
  changed: return replacement template and new validator
  structural: return insert, remove, or move; retain unchanged descendants or safely replace an ancestor when a granular operation is unavailable
response: data:component-delta-response
delivery: requirement:streaming-delta-response
assets: requirement:delta-head-sync
consistency: rule:delta-consistency-model
dom_state: rule:preserved-client-subtree
security:
  - client validators are optimization hints and cannot bypass authorization, validation, or page execution
  - server-derived current request values are the source of truth
  - do not reflect raw request data into operation scripts
failure:
  incompatible_render_version: full render or explicit reload response
  invalid_manifest: ignore hints and compute a safe full or larger delta
  render_error: no partially valid manifest publication
compatibility: requirement:html-rendering-compatibility
acceptance:
  - changing search parameters can re-execute the page without sending unchanged boundary HTML
  - changing an allowed data parameter can rerender only its declared boundary subtree
  - changed, inserted, removed, and reordered instances converge to the server render
  - next manifest represents the DOM state after all operations apply
open_questions:
  - request and response media types, deferred to requirement:render-mode-negotiation
  - validator hash length and wire compression, deferred to rule:update-validator-computation
  - maximum manifest size and compact encoding, deferred to decision:manifest-state-ownership
resolved:
  retain_holes:
    decided: 2026-08-08, they ship; requirement:boundary-decomposed-render makes the hole the ordinary shape rather than an optimization over ancestor replacement
    what_changed: the response decomposes at every boundary by default, so a parent fragment always carries placeholders and a hole with no fragment sent is a retain
    ancestor_replacement: stays the fallback for a server that cannot express holes, which the data:component-delta-response clause already allows
```

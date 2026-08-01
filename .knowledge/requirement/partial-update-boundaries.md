---
id: requirement:partial-update-boundaries
type: requirement
title: Explicit Partial Update Boundaries
---
Mark components whose rendered DOM and validators may participate in client-side partial updates.

```yaml
source: concept:html-render-runtime-extensions
declaration:
  activation:
    explicit: update flag on an ordinary component
    automatic: requirement:layout-reuse-boundaries for generated route layouts
  exported_only:
    rule: only an exported component can be a boundary
    reason: becoming a boundary publishes an identity into the DOM and the protocol, so a file-private implementation detail must not be addressable from outside
    cost: a component must be exported to become updatable, which is worth revisiting at requirement:client-update-rollout m3 if it forces unwanted public surface
  cache_relation: independent from requirement:component-output-cache
rendering:
  - emit stable boundary markers using rule:component-instance-identity
  - add one entry to data:component-update-manifest
  - carry identity and validators per decision:update-manifest-transport, which requires one root element per update boundary
  - preserve normal complete HTML for initial navigation and non-update clients
parameters:
  source: generated page execution from current path, search parameters, request state, and typed parent inputs
  client_state: raw arguments need not be exposed or resubmitted
direct_redraw:
  capability: requirement:component-redraw-endpoint, for an explicitly registered component only
  identity: decision:author-declared-boundary-id
  inputs: supplied by the caller and therefore untrusted, per rule:redraw-input-trust
nested_boundaries:
  - track parent identity and stable child ordering
  - unchanged nested boundaries may be omitted independently when the protocol can preserve their DOM
  - otherwise retain them through data:component-delta-response holes, or replace the nearest safe changed ancestor
client_state: rule:preserved-client-subtree
consistency: rule:delta-consistency-model
constraints:
  - repeated component calls require stable explicit keys through rule:component-instance-identity
  - browser runtime cannot instantiate undeclared components or select arbitrary server arguments
  - boundary rerendering is side-effect-free and safe to repeat
acceptance:
  - update flag is opt-in and leaves ordinary component output compatible
  - generated route layouts participate automatically without making their parameters client-mutable
  - browser can locate every returned operation target without inspecting component arguments
open_questions:
  - update flag syntax and default boundary element or marker form
  - whether cache-enabled components become update-enabled by shorthand
  - nested boundary preservation versus ancestor replacement in the first milestone
```

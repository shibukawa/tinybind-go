---
id: data:component-update-manifest
type: data
title: Component Update Manifest
---
Compact client-held state for comparing updateable component instances between renders.

```yaml
source: concept:html-render-runtime-extensions
document:
  route_id: stable generated route or page identity
  render_version: generated-code and update-protocol version
  document_validator: optional whole-render validator
instances:
  - instance_id: rule:component-instance-identity
    parent_id: optional enclosing update boundary
    component_id: stable generated declaration identity
    revision: monotonic boundary state version
    input_validator: opaque digest of component version and canonical typed inputs
    content_validator: opaque digest of canonical rendered boundary HTML
    position: optional structural ordering token
    redraw: decision:author-declared-boundary-id value, present only for a registered component
ownership: decision:manifest-state-ownership keeps this state on the client
transport:
  shape: decision:update-manifest-transport splits element attributes from one inert document payload
  update_request: client returns prior render version and instance validators
  streamed_updates: requirement:streaming-delta-response merges per-record entries instead of replacing a whole manifest
validators: rule:update-validator-computation
consistency: rule:delta-consistency-model
privacy:
  - omit raw component arguments by default
  - validators may be keyed or opaque when plain hashes expose sensitive low-entropy values
  - server reconstructs arguments from the new request and generated render plan
  - re-execution supplies current authentication and authorization; no stored capability substitutes for them
canonicalization:
  - exclude compression and request-unique transport markers from content hashing
  - include component generated-code version in input validation
```

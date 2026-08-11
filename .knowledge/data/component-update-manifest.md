---
id: data:component-update-manifest
type: data
title: Component Update Manifest
---
Compact client-held state for comparing updateable component instances between renders.

```yaml
source: concept:html-render-runtime-extensions
status: the document and instances blocks are the designed shape; as_built is what ships, and it is a strict subset
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
as_built:
  verified: 2026-08-11 against v0.5.5
  carried: instance_id, parent_id, and two validators; nothing else reaches a client
  entry_grammar: instance_id:frame[:children[:parent]], comma separated, in internal/updatecore EncodeManifest and DecodeManifest
  json_form: DeltaInstance carries id, frame, children, parent, matching the header
  streamed_form: ManifestEntry carries frame, children, parent beside the instance id its record already names
  boundary_attribute: the rendered element carries the instance id alone, so markup names no declaration either
  content_validator_split: shipped as two digests, frame for the boundary's own bytes and children for the nested ids, so a reordered list does not replace its parent
  server_only: component_id and input_validator exist on delta.Instance and are never serialized; component_id seeds the frame digest and leaves no trace a client can read
  not_shipped: route_id, render_version, document_validator, revision, position, redraw
  consequence: a client cannot learn which declaration a live instance belongs to, which is what blocks requirement:scoped-script-declaration for a component with more than one instance
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

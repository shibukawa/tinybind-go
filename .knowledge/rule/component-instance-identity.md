---
id: rule:component-instance-identity
type: rule
title: Stable Component Instance Identity
---
Identify the same updateable component invocation across two executions without trusting browser-provided arguments.

```yaml
source: concept:html-render-runtime-extensions
scope:
  applies_to: automatic requirement:layout-reuse-boundaries chain members, whose identity the author never names
  not_applies_to: an explicit boundary, which uses decision:author-declared-boundary-id and needs no derivation
  reason: derivation exists so re-execution reproduces the same identity; an author-written id already is stable
identity_parts:
  - route and runtime layout-chain identity
  - generated component call-site identity
list_keys: decision:list-item-key, which now owns repeated-markup identity; a chain member never repeats, so no item key appears here
properties:
  - deterministic for the same logical instance across search parameter changes
  - unique within one rendered document
  - independent from generated transport marker IDs
  - opaque and safe in HTML attributes and protocol fields
validation:
  - duplicate instance IDs are rendering errors before delta publication
  - browser IDs and validators are comparison hints, never authority for component arguments or access control
structural_change:
  missing_old_id: insertion or nearest-ancestor replacement
  missing_new_id: removal or nearest-ancestor replacement
  changed_parent_or_order: move operation or nearest-ancestor replacement
```

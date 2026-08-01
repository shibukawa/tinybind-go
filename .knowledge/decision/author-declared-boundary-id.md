---
id: decision:author-declared-boundary-id
type: decision
title: Author Declared Boundary ID
---
Address an explicitly reloadable component by an ordinary DOM id the author writes, instead of an identity the framework derives.

```yaml
source:
  - requirement:partial-update-boundaries
  - user identity proposal 2026-08-01
review_gate: proposed authoring convention requires user approval
decision:
  explicit_boundary: the author supplies a DOM id, and the framework writes it on the component's root element
  automatic_boundary: requirement:layout-reuse-boundaries chain members keep a generated positional identity, which the author never names
rationale:
  intent: reloading a region is a deliberate act, so naming it should be too; a derived identity hides that decision
  existing_mechanism: getElementById already solves lookup, uniqueness is already an HTML rule, and existing tooling already flags duplicates
  api_symmetry: the browser API takes the same id the author reads in the markup
declaration_site:
  rule: the id is an argument at the call site, not markup inside the component
  reason: an id written inside a component body would repeat with every instance
  form: '<UserCard id="card-1" user={u} />', with the framework emitting id on the root element
  repeats: the author composes a unique id from the item key, which is what an author writes anyway
simplifies:
  removed:
    - rule:component-instance-identity derivation for explicit boundaries, including stable-key validation for repeated call sites
    - the requirement that re-execution reproduce a derived identity
    - a framework-owned identity attribute for explicit boundaries, which now use the native id attribute
  retained:
    - decision:update-manifest-transport single root element rule, because the id still needs an element
    - a generated positional identity for automatic chain boundaries
uniqueness:
  owner: the author
  static_check: not possible when the id is composed from data, so this stays a runtime and authoring concern
  duplicate: an ordinary HTML error; the runtime addresses the first match and does not attempt recovery
consequences:
  - an explicit boundary is nameable from outside, which reinforces the exported-only rule in requirement:partial-update-boundaries
  - structural insertion of a not-yet-existing id is not addressable, so adding a region stays a navigation-delta concern
```

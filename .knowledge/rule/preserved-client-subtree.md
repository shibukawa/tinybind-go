---
id: rule:preserved-client-subtree
type: rule
title: Preserved Client Subtree
---
Never silently destroy browser-owned state when a delta operation replaces DOM the server considers changed.

```yaml
source:
  - requirement:partial-update-boundaries
  - user DOM-state analysis 2026-07-26
at_risk_state:
  unrecoverable: file input selection, in-progress IME composition, media playback position, canvas and WebGL contexts, custom element internals, third-party widget state
  recoverable_only_by_policy: focus and text selection, uncontrolled input value, inner scroll offset, open details or dialog, CSS transition progress
application: decision:dom-application-strategy, which decides how much of a region is touched at all
preference_order:
  - omit the boundary because its content validator matched
  - replace the changed boundary alone
  - replace an ancestor using retain holes so unchanged descendant DOM is moved rather than recreated
  - replace an ancestor wholesale, accepting the state loss
status: delivered; the runtime moves a keyed region into the replacement before installing it
explicit_preservation:
  marker: author-declared preserved region inside an update boundary, keyed by an attribute value
  behavior: the runtime moves the existing node into the new markup instead of accepting the server's version for that region
  constraint: a preserved region is identified by a stable key
  unmatched_key: a key with no counterpart in the replacement is a new region, so the server's version stands rather than being replaced by an unrelated node
focus:
  - restore focus and selection when the focused element is inside a retained or preserved region
  - restore by stable instance and field identity when its boundary was replaced
  - reset to a documented landmark rather than leaving focus on the document body silently, per requirement:client-navigation
composition:
  rule: defer applying an operation that would replace the element with an active IME composition until composition ends
  reason: replacing it drops partially typed text in scripts where that text is most expensive to retype
forms: rule:form-state-reconciliation, which transfers control state by value instead of protecting the node, because a control whose options the server rewrote cannot be preserved as a subtree
acceptance:
  - a replaced ancestor keeps a video playing inside a preserved region
  - keyboard focus survives an update to an unrelated sibling boundary
  - typing with an IME is not truncated by a background update
  - a file input selection is never discarded by an update the user did not initiate
open_questions:
  - preserved-region declaration syntax and whether it implies an update boundary
  - whether retain holes are in the first milestone or deferred to ancestor replacement
  - scroll restoration for containers not explicitly preserved
```

---
id: decision:list-item-key
type: decision
title: List Item Key
---
Give a loop body an author-supplied key drawn from the data, so repeated markup can be matched by identity rather than by position when a region is patched.

```yaml
source:
  - decision:dom-application-strategy
  - user list key question 2026-08-01
review_gate: proposed authoring convention requires user approval
layer:
  boundary_level: solved already; a repeated reloadable component carries a decision:author-declared-boundary-id composed from the item identity
  inside_a_boundary: unsolved; a for body has no id and no boundary, only repeated markup, and this is where a key applies
  not_the_dom_id:
    reason: a DOM id must be unique across the document and is public API through getElementById, fragment links, and aria references
    key_scope: unique among its siblings in one loop, which is all matching needs
when_it_pays:
  today: nothing, because whole replacement recreates every row regardless
  from_morphing_onward: matching decides which existing row corresponds to which new value set
  failure_without_it:
    correctness: positional matching still converges on the right markup
    damage: inserting one row at the top shifts every row, so focus, an open menu, a checked box, and a playing video each move one row across, and transitions replay
    cost: work becomes proportional to the list rather than to the change
    prior_art: Idiomorph added id-set matching precisely to correct keyless mispairing
decide_early_because:
  surface: a key is authoring surface rather than protocol, so retrofitting means revisiting every template that renders a list
  silent_degradation: an unkeyed loop does not fail, it misplaces state, which is harder to notice than an error
  asymmetry: the same reason the single root element rule was adopted before it was needed
cannot_be_derived:
  content_hash: loses identity exactly when the content changes, which is when identity matters most
  position: is the thing being replaced
  inference: an identity field cannot be guessed from a record without being wrong often
  conclusion: the key comes from the data and the author names it
form:
  placement: on the root element of the loop body, following policy:frontend-convention-alignment
  example: '<li key={item.ID}>'
  single_root_consequence: a keyed loop body must have exactly one root element, because a match target has to be a node; the same constraint as an update boundary, for the same reason
  type: a scalar the generated code can compare, not a record
uniqueness:
  scope: siblings produced by one loop in one render
  detection: duplicates are data-dependent, so they surface when a patch is applied rather than at generation time
  behavior_on_duplicate: fall back to replacing the list, since matching cannot be trusted
supersedes:
  what: the stable item key clause in rule:component-instance-identity
  why: that clause covered repeated update boundaries, which explicit ids now identify and automatic chain members never produce
id_or_key:
  rule: a row that will be reloaded individually carries a decision:author-declared-boundary-id; a row that only needs matching carries a key
  no_double_writing: where an id is present it also serves as the match key, since it is already unique and stable
  reading: the choice states intent, an id meaning the row is independently addressable and a key meaning it is only matched during a patch
open_questions:
  - whether an unkeyed loop inside an updatable region is a diagnostic, a warning, or silently positional
  - whether the key participates in the static-dynamic value encoding or only in application
```

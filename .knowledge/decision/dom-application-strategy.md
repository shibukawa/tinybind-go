---
id: decision:dom-application-strategy
type: decision
title: DOM Application Strategy
---
Stage how returned markup reaches the DOM, ending at a static-dynamic split the compiled render plan already makes possible.

```yaml
source:
  - rule:preserved-client-subtree
  - user incremental-update question 2026-08-01
review_gate: proposed architecture requires user approval
scope:
  question: how a fragment the server produced is installed, not how the server decided to produce it
  distinct_from: decision:boundary-update-execution, which is server-side execution scope
problem:
  current: whole-element replacement destroys everything the browser owns inside the region
  acceptable_for: a complete navigation, where the page changes anyway
  unacceptable_for: a search-parameter update or a live delivery in a region holding a form, focus, media, or a third-party widget
options:
  replace_with_islands:
    how: whole replacement, plus author-declared regions the runtime moves instead of recreating
    cost: an author marker on every region worth keeping, and forms are the common case
    strength: cheap, predictable, no heuristics
  morphing:
    how: walk old and new trees and mutate in place, keeping nodes whose tag and key match
    prior_art: morphdom, Idiomorph, and the same choice in Turbo and LiveView
    strength: untouched nodes keep identity, so form values, focus, media, and listeners survive with no author markup
    cost: heuristic pairing goes wrong without decision:list-item-key, and form controls need special handling because the value attribute and the value property diverge
  static_dynamic_split:
    how: the client holds the compiled static skeleton and the server sends only the values of the dynamic holes
    enabled_by: templates are already compiled to an instruction list, so which parts are static is known at generation time
    skeleton_distribution: the component kind hash is a content address, so a skeleton is immutable, permanently cacheable, and invalidated by a deploy
    strength: application is exact rather than heuristic; a hole whose value did not change emits nothing and its surroundings are never touched
    payload: only changed values travel, which matters most in a loop sharing one skeleton
    cost: the largest of the three, effectively a second rendering backend emitting values instead of bytes
    hard_parts:
      - a loop is a list of value sets sharing one skeleton and needs its own encoding
      - a conditional changes structure rather than values
      - genuine structural change still needs decision:list-item-key plus insert, remove, and move
staging:
  1: whole replacement, plus rule:preserved-client-subtree islands and automatic focus, selection, and scroll restoration; delivered
  2: morphing on the replacement path, which removes the author marker for the common form case
  2.5: boundary-decomposed transfer — requirement:boundary-decomposed-render, which returns a fragment per boundary plus the tree composing them, with a placeholder where each nested boundary sits
  3: static-dynamic split, which removes the heuristics entirely and shrinks payloads
  where_2_5_sits:
    added: 2026-08-08, by the owner scoping the partial-transfer round
    changes: what is transferred, not how it is applied; each fragment is still installed by parsing, so stage 1's application cost is unchanged inside a boundary
    reaches: retained descendants, which this concept's islands and stage 3 both wanted, and a statically-known boundary that is never transferred again under one build
    does_not_reach: the no-reparse property, which is stage 3's whole point and the half the downstream ranked first
  stage_three_server_half:
    named: requirement:structured-render-output, asked for by the caller that would consume it, 2026-08-08
    why_it_was_missing: this concept designed how a fragment is installed and named no module output to install from, so a reader concluded the split was undesigned rather than half-designed
    kinds: rule:dynamic-slot-kinds, which is what keeps the escaping decisions on this side of the line
    settled_2026_08_08: a unit is a component, a skeleton identity is a content address, and a unit is not an update boundary
    loop_encoding_answered: this concept's open question about loop skeletons and their value sets resolves to one skeleton per row component and one value set per item, provided the row is written as a component call
    first_delivery_answered: this concept's open question on how a skeleton is first delivered resolves to fetched by content address, per requirement:structured-render-output skeleton_delivery; a skeleton has a different lifetime and cache policy from the request that names it, and the assembled first delivery is what keeps the fetch off the critical path
    first_build_answered: a client assembles the skeleton's string, inserts its own markers, and parses once, rather than constructing nodes; a post-parse address cannot be expressed by the server because the browser's parser decides the tree
  composability: each stage replaces the previous application mechanism; islands survive all three, because a third-party widget the server does not own cannot be patched by any of them
form_controls: rule:form-state-reconciliation applies at every stage, because no application strategy can protect a control whose surrounding option set legitimately changed
live_region_consequence:
  today: a live region forbids form controls, because every delivery replaces the subtree
  after_split: the constraint relaxes to forbidding a control whose value is itself a live hole, since an unchanged hole emits nothing
  value: a far more natural rule than banning the elements outright
open_questions:
  - whether morphing is worth shipping at all, or whether stage 1 should hold until the split is ready
  - encoding for loop skeletons and their value sets
  - how a skeleton is first delivered: inline with the initial render, or fetched by content address
  - whether the split applies to navigation deltas, redraws, and live deliveries alike, or only where regions update repeatedly
```

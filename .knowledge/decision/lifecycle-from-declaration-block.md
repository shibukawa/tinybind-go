---
id: decision:lifecycle-from-declaration-block
type: decision
title: A Component's Own Script Is A Declaration Block
---
Give a component a top-level script block beside its head block, the way a single-file component does, and let that position carry the lifetime, because a block a component declares is its own and a head contribution is the document's.

```yaml
source:
  - concept:scope-lifecycle
  - user direction 2026-08-11, that a script outside head is what a single-file component author already expects
  - downstream responsibility survey 2026-08-11
review_gate: proposed
supersedes: an earlier draft of this decision, 2026-08-11, which put the lifetime in an authoring attribute because it found no honest position to read; the user asked for the block, and the block is a better position than the one that draft rejected
statement: a script block declared at the top of a component, as a sibling of its head block, is that component's own script and is bound to its instances; a script inside the head block is a document head contribution and lives as long as the document
the_finding_that_still_stands:
  what: extraction reads a component's head contribution alone, verified against templates/htmlbind/assets.go at v0.5.3, and raw-text reading is gated on being inside a head contribution in templates/htmlbind/html.go
  therefore: there is no script outside head today, so nothing existing changes meaning and no project is reinterpreted
  what_changed: the earlier draft read the downstream survey's body versus head axis, found body markup carried nothing, and concluded position could not carry the lifetime; the axis was wrong rather than the idea
  the_right_axis: declaration block versus head contribution, which is neither of the two the survey named and is the one a single-file component author already reads
why_the_block_is_the_honest_position:
  it_is_already_the_shape: a component is written as a head block followed by markup, so a script block beside the head block adds a third sibling to a list of two rather than a new kind of place
  head_means_document: a script in the head block is a tag the component contributes to the merged head, and requirement:client-managed-head never retires what it installed in a way that releases a script's registrations; document lifetime is what that position already means
  block_means_component: a block the component declares belongs to the component, and there is nothing else it could belong to
position_alone_is_ambiguous:
  corrected: 2026-08-11, while implementing; an earlier reading of this decision said no attribute was needed because the position carried the lifetime
  what_it_missed: testdata/templates/htmlbind/contexts declares '<script>{RawJavaScript(javascript)}</script>' at the top of a component body, a shipped and tested feature where the script is markup carrying an insertion
  therefore: the top of a component body is equally the shape of the component's own script and of a script it renders, and no position rule separates them
  what_it_would_have_cost: reading a top-level script as a block would have turned that insertion into literal text, silently, in every project using the feature
  resolution: the block carries a bare component attribute, and the position still describes it because a block is only accepted at the top of the component; the marker says which of the two a top-level script is, and the position says where the one kind may go
  precedent_unchanged: decision:script-load-mode-authoring already put a source-only marker in a bare attribute and lowered it away, which is exactly this shape
  not_body_markup: a script inside the rendered markup keeps its current meaning, which is markup; requirement:component-script-block rejects only a marked block found there
convention:
  matches: the single-file component layout of Vue and Svelte, where template, script, and style are sibling blocks of one declaration
  policy: policy:frontend-convention-alignment prefers React, and React offers nothing to prefer here because it has no template-block authoring surface at all
  already_diverged: that policy records fill blocks and inline style as known divergences taken for the same reason, that markup rather than JSX is the authoring surface; this is the third case of the same divergence and needs no new exception
  reading: the divergence clause already covers it, so this is convention-following rather than convention-breaking
parser_change_accepted:
  needed: the raw-text gate generalizes from inside a head contribution to inside a declaration block, so a brace in JavaScript stops being template syntax
  shape: the same treatment head already gives style and script, applied one level out
  cost: real but contained; it is one flag's meaning widened, not a new parsing mode
  precedent: a component already rejects more than one head element, so one script block per component is the same check on a second block
what_it_does_not_decide:
  the_export: setup, its argument, and the teardown convention stay the caller's, per decision:client-runtime-ownership
  the_call: the module calls nothing and imports nothing; requirement:scoped-script-declaration publishes the owner and the caller's runtime acts
  the_language: whether the block is JavaScript or TypeScript is requirement:authored-language-transform, and it is a separate axis from the lifetime
rejected_alternatives:
  lifecycle_attribute_on_a_head_script:
    shape: a bare marker on a script inside the head block, which an earlier draft chose
    why_not: it puts a component's own script in the document's head block and then annotates it back out, so the position says one thing and the attribute says another
    note: the marker survives, on the top-level block rather than in the head, per position_alone_is_ambiguous
    kept_from_it: the module-mode reasoning, which requirement:component-script-block carries, and the finding about what extraction reads
  body_markup_script:
    shape: a script written among the rendered elements
    why_not: it needs a lowering rule for what the tag becomes, a placement rule inside the subtree requirement:component-delta-rendering replaces, and a parser change larger than the block's; and it expresses nothing the block does not
  export_convention_alone:
    shape: no position rule; import every extracted script and call setup when one is exported
    why_not: it makes an export name a contract the module cannot check without reading the JavaScript, and it imports every script to discover which participate
    reconsider_when: requirement:authored-language-transform puts a real parser in the pipeline, which would make the export set readable; noted there rather than assumed here
constraints:
  - a component declaring no script block behaves exactly as today, so requirement:html-rendering-compatibility holds for every existing project
  - the block is extracted to a file like a head script, so rule:template-context-safety and the content-hashed naming of requirement:static-asset-extraction are unchanged
  - the module names no global and writes no script into any response, unchanged
related:
  - requirement:component-script-block
  - requirement:scoped-script-declaration
  - concept:scope-lifecycle
open_questions:
  - whether a style block should also become a top-level sibling, since decision:component-style-delivery puts it in head today and the single-file layout would put it beside the script
  - whether a wrapper or a page-level component declares the block the same way, given its identity is derived rather than author-declared
```

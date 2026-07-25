---
id: requirement:html-slot-syntax
type: requirement
title: HTML Slot Syntax
---
Mark component content insertion points with a reserved slot element instead of a separate layout declaration kind.

```yaml
source:
  - requirement:html-template-v1
  - user syntax decision 2026-07-25
review_gate: approved 2026-07-25
model: an ordinary decision:template-declaration-kinds component becomes slot-capable by placing a reserved element in its body
declaration:
  unnamed: slot element with no name attribute; binds the reserved children parameter of type html
  named: slot element with a static name attribute; binds an html parameter of the same name
  default_content: children of the slot element render when the bound html argument is absent
  self_closing: a slot with no default content is written self-closing
  optional: slots are optional by default
  required: a required attribute on the slot element makes the fill mandatory at generation time
  absent: render default content when declared, otherwise emit nothing
  deletion: an absent slot leaves no element, no wrapper, and no marker in the output
required_rules:
  purpose: catch a forgotten fill on a component whose markup is meaningless empty, instead of emitting an empty shell
  conflict: required together with default content is a generation error; the default already answers absence
  conditional: required constrains the argument, not the rendering; a required slot inside an if branch may still render zero times
  diagnostic: report the slot declaration position and the offending call position
  alternative_rejected: declaring the slot parameter in the signature with an optional type; it would force authors to write every slot twice
  layout: a layout may mark its slot required, but data:html-render-route-plan still owns the exactly-one-unnamed-slot shape rule
fill:
  named: template element with a static slot attribute among the component call children
  unnamed: remaining bare children of the call; no wrapper element
  static_names: fill and slot names are static; expressions are forbidden
  not_vue2: the shape is chosen on its own merits; Vue 2 compatibility is not a goal
value_access:
  decision: a slot parameter is never readable in the expression language; the slot element is its only use site
  reason:
    - the argument stays a continuation, so nothing forces requirement:html-template-v1 intermediate DOM
    - one use site keeps the at-most-one-rendering guarantee structural instead of analytic
    - html never enters attribute, url, script, or style contexts as a value
    - requirement:chain-render-pipeline laziness needs no rule about when a held value executes
  not_expressible:
    - testing whether a caller supplied a slot
    - forwarding a received slot to another component
    - wrapping markup that should disappear together with an absent slot; declare it as default content instead
conditional:
  allowed: a slot may sit inside an if branch, so a component can legitimately drop its children
  condition_inputs: the branch condition reads ordinary typed parameters only, never slot presence
  multiple_sites: repeated declaration sites are valid when mutually exclusive if branches make at most one reachable per execution
  zero_renderings: an unrendered slot invokes no continuation and produces no output; this is not an error
  laziness: a slot argument is a continuation, never pre-rendered before its insertion point is reached
reserved:
  slot_element: slot is reserved as an element name and never emitted
  slot_attribute: slot is not reserved as an attribute name; ordinary elements keep passing it through to external Web Components
  template_element: template is a fill block only when it carries a slot attribute and is a direct child of a component call
  disambiguation: every other template element, including one directly under a component call without a slot attribute, is ordinary emitted HTML
  lost_case: authoring a literal template element carrying a slot attribute directly under a component call; wrap it in an element or place it inside a named fill
  authoring: this framework does not author Web Components, so no escape for a literal slot element is needed
layout:
  shape: exactly one unnamed slot declaration site
  keyword: no distinct layout declaration keyword; requirement:layout-chain-discovery selects ordinary components by file role
  validation: data:html-render-route-plan rejects a layout with zero declaration sites, multiple reachable sites, or named slots
constraints:
  - slot appears only in child-node position; attribute, url, script, and style contexts reject it
  - at most one rendering per name per execution path, matching requirement:nested-layout-composition duplicate-slot validation
  - slot inside a for body is a generation error because repetition breaks that guarantee
  - the slot element itself never reaches the output; only the bound content or its default does
  - a slot argument is typed html and follows rule:template-context-safety at its insertion context
  - slot content classification stays with requirement:chain-render-pipeline; the owner never fixes it
milestone:
  v1: unnamed slot, default content, and the required marker through requirement:template-v1-scope
  post_v1: named slots alongside requirement:nested-layout-composition
acceptance:
  - a component with one unnamed slot is usable as both an ordinary wrapper and a route layout
  - a call supplying no content renders the declared default content
  - a slot with no default content and no argument leaves nothing in the output
  - a call omitting a required fill fails at generation time with both positions
  - a slot marked required while declaring default content fails at generation time
  - a slot parameter used in any expression fails at generation time
  - named and unnamed slots compose in one component without ordering rules on the fill side
  - the same slot placed in both branches of an if compiles; the same slot inside a for does not
  - a layout that conditionally drops its slot never executes the child chain member
  - a template element without a slot attribute is always emitted verbatim, including directly under a component call
  - a template element under an ordinary element is emitted verbatim, slot attribute included
  - a fill naming an undeclared slot fails at generation time with both positions
update_interaction:
  no_anchor: an absent slot leaves nothing to target, so a later partial update cannot fill that position in place
  presence_change: when a conditional slot appears or disappears, requirement:layout-reuse-boundaries replaces the nearest enclosing boundary instead of patching the slot position
  rationale: markup is never inserted at a vanished slot position after the fact
```

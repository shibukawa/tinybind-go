---
id: requirement:scoped-component-style
type: requirement
title: Scoped Component Style
---
Scope a component-local style block to that component's own elements while hoisting it into the merged document head.

```yaml
source:
  - requirement:head-merging
  - user style decision 2026-07-25
alignment: policy:frontend-convention-alignment selects the CSS Modules model over the marker-attribute model used by Svelte and Vue
model:
  declaration: a style block inside a component head declaration
  scope_key: stable identifier derived from the component identity and its generated version
  mechanism: rename the class names the style block declares, then rewrite matching class attributes in the same component markup
  no_marker_attribute: no per-element scope attribute is emitted, so output stays close to hand-written HTML
  pass_through: a class the style block does not declare is left untouched, so external framework classes keep working
  delivery: decision:component-style-delivery extracts the rewritten block into a generated stylesheet, one contribution per component and never per instance
element_selectors:
  problem: a bare element selector carries no name to rename, so CSS Modules leaks it globally
  decision: reject it with a generation error naming the selector and suggesting a class
  reason: silent global leakage is the one CSS Modules behavior worth diverging from, and the diagnostic keeps the model honest
  allowed: an element selector qualified by a declared class, such as a descendant of a scoped class
boundaries:
  own_markup: class attributes in this component are rewritten, including inside if and for bodies
  slot_content: written in the caller file, so it carries the caller renamed classes and the receiver cannot restyle it
  child_component: a child renames its own classes in its own file
  root_document: decision:html-document-shell markup is outside any component scope
parent_styling_slot_content:
  decision: no slotted-content selector; the React answer is to pass a class through a declared parameter
  rejected: a slotted or deep selector from Vue and Svelte
local_names:
  principle: rename an identifier whose definition and every reference stay inside CSS; leave names shared with the outside world global
  precedent: CSS Modules, and the same choice in Svelte and Vue single-file component scoped styles
  renamed:
    keyframes: at-rule name plus every animation and animation-name reference in the same style block
  reference_rewrite:
    properties: [animation, animation-name]
    shorthand: the animation shorthand is parsed to locate the name token among durations, easings, counts, and keywords
    unresolved_reference: naming a keyframes definition this component does not declare is a generation error
  global:
    font_family: font-face family names are a shared resource
    custom_properties: cascade inheritance is the point of a custom property, so scoping would break theming
    media_and_supports: condition at-rules carry no name; their inner selectors scope normally
escape:
  form: the CSS Modules global selector function
  applies_to: selectors and keyframes names alike
  rejected: the Svelte global name prefix, per policy:frontend-convention-alignment
  effect: a global rule is emitted unrewritten and stays the author's responsibility
constraints:
  - selector and local-name rewriting happen at generation time; no runtime CSS parsing or string evaluation
  - a rewritten class attribute value stays static, satisfying the requirement:html-template-v1 dynamic-name ban
  - a class name reaching an element through an expression cannot be rewritten and is a generation error
  - a style block still follows rule:template-context-safety style-context rules
  - declaration values are rewritten only for the listed local-name references, never otherwise
  - one component emits its style contribution once regardless of instance count
  - trusted_css from requirement:explicit-output-control is inserted unscoped and stays the author's responsibility
acceptance:
  - two components declaring the same class name do not affect each other
  - two components declaring the same keyframes name animate independently
  - a custom property set by an ancestor still reaches a scoped component
  - an undeclared class such as an external framework utility passes through unchanged
  - a bare element selector fails generation with an actionable message
  - repeated instances of one component emit one style block
  - content passed into a slot is not restyled by the receiving component
  - removing a component from a composition removes its style contribution
open_questions:
  - whether counter-style, layer, and container names join the renamed set
  - hashed class name length and whether it stays stable across unrelated edits in the same file
  - whether a class list built from a conditional expression gets a dedicated typed helper instead of the generation error
```

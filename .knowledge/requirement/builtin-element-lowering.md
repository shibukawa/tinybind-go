---
id: requirement:builtin-element-lowering
type: requirement
title: Builtin Element Lowering
---
Rewrite a registered builtin element at generation time into render plan steps, folding fixed markup into static bytes and leaving only per-request holes to resolve at render time.

```yaml
priority: should
source:
  - requirement:builtin-element-registration
  - user design discussion 2026-07-27
review_gate: proposed
model:
  timing_split: the element is recognized and rewritten at generation time; only declared per-request values are produced while rendering
  target: decision:generated-render-plan instructions on the enclosing component plan, not a separate component plan
  inlining: a builtin element declaring no phase-dependent capability adds no plan indirection and no component boundary
shapes:
  markup:
    input: the data:builtin-element-definition markup template
    lowering: parse the markup at generation time, coalesce its fixed bytes into the surrounding static run, and emit one typed expression step per hole
    escaping: each hole takes the escaping of its own parsed position, so an attribute hole gets attribute escaping and a text hole gets text escaping
    result: output cannot inject markup even if a provider returns hostile bytes
    preferred: true
  opaque:
    input: a provider returning a requirement:explicit-output-control trusted value or an htmlbind fragment
    lowering: one plan step inserting that value in the declared insertion context
    use_for: output whose structure, not only its values, varies
    cost: the trust assertion moves into framework code and the generator can no longer verify the emitted structure
constant_folding:
  rule: a definition with no provider and no expression parameter reduces entirely to static bytes
  effect: such an element costs nothing at render time
context_checking:
  rule: the element position must match the definition insertion context under rule:template-context-safety
  local_only: the check uses the insertion context at the element position; decision:builtin-element-syntax rules out ancestor-element constraints because a component boundary hides the ancestor
  parameters: an attribute expression on the element is checked against the declared parameter type exactly as on an ordinary element
  children: a declared children parameter is a lazy html value, following requirement:html-slot-syntax laziness
capability_effects:
  per_request:
    cache: a per-request builtin element inside a requirement:component-output-cache region is a generation error, per policy:html-update-csrf-protection token-outside-cache rule
    layout_reuse: the same exclusion applies to requirement:layout-reuse-boundaries reusable frames
    delta: a requirement:component-delta-rendering rerender reinvokes the provider, so a boundary refresh carries a current value
  needs_context: propagates to the enclosing component in data:component-render-capabilities, so requirement:render-value-provider validation can run on the whole composition
  needs_bootstrap: forces requirement:html-runtime-bootstrap selection for the document
  scripts: declared script names become requirement:framework-script-contribution head contributions on this component plan, merged by requirement:head-merging
propagation:
  through_calls: capabilities propagate up the component call graph like other decision:component-capability-lowering effects
  through_slots: a slot continuation carries its own capabilities on its bound component value, per requirement:cross-template-components
constraints:
  - no runtime template parsing; the markup template is parsed once at generation time
  - no reflection; a provider result field is read through generated typed access, keeping decision:reflection-free
  - a builtin element never emits document tags and never becomes a decision:html-document-shell replacement
  - lowering preserves source positions, so a diagnostic points at the element in the template file, not at generated markup
acceptance:
  - csrf-token inside a form emits the hidden input with an attribute-escaped per-request token
  - the same page rendered twice produces two different token values and identical surrounding bytes
  - a token containing quote characters cannot break out of the value attribute
  - csrf-token inside a cached component fails generation with an actionable message
  - a parameterless static builtin element adds no render-time step
  - removing the element from a template removes its capability effects and its script contribution
open_questions:
  - whether a provider may fail the render, or must return a usable value, given the head pass runs before the first body byte
  - whether one provider call is shared when the same builtin element appears several times in one response
  - whether an opaque shape is worth shipping in the first milestone
```

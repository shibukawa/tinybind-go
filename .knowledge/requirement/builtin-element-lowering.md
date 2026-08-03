---
id: requirement:builtin-element-lowering
type: requirement
title: Builtin Element Lowering
---
Rewrite a registered builtin element at generation time into render plan steps, folding fixed markup into static bytes and leaving only per-request holes to resolve at render time.

```yaml
priority: should
status: markup shape delivered 2026-08-03; opaque shape deferred, see as_built
as_built:
  markup_shape: parsed once at registration into static runs and holes, folded into the enclosing plan's own static bytes, with one step left for the per-request part
  constant_folding: a definition with no provider and no expression attribute reduces entirely to static bytes and adds no plan step
  no_provider_with_params: lowers to ordinary Static and Text steps reading the call site's own expression, so it needs no provider machinery
  escaping: element text and a quoted attribute value take the same escaping in this module, and a hole anywhere else is a registration error rather than a widened rule
  one_call_per_occurrence: the whole element is one step, so a provider runs once for the occurrence rather than once per hole; the open question of memoizing across occurrences is answered conservatively, since a framework that wants one value per response can memoize on its own context
  capability_effects:
    cache: a per-request element inside a cached component is a generation error, followed over the call graph
    needs_context: checked at the step rather than statically, and reported naming the element
  not_built:
    opaque_shape: deferred; the trust assertion would move into framework code and the generator could no longer verify the emitted structure, which is why the verifiable shape went first
    layout_reuse_exclusion: requirement:layout-reuse-boundaries is not built, so there is no reusable frame to exclude from yet
    script_contributions: requirement:framework-script-contribution is not built
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

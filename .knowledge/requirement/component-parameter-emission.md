---
id: requirement:component-parameter-emission
type: requirement
title: Component Parameter Emission
---
Emit a JSON object of named component parameters onto the component's root element, so a script block reads a value with its type rather than as attribute text.

```yaml
priority: should
status: implemented 2026-08-12
as_built:
  where: templates/htmlbind/parameter.go, written onto the same root element the declaration marker rides, default attribute data-tb-props
  option: GenerateOptions.ComponentParameters, a name list per component; an absent or empty entry emits nothing
  type_rule: the compiler's own jsonSerializable, so a record and an array are emitted and html and a pending value are refused, which is the rule whose justification transfers
  encoder: the existing jsonEncodeCall path that already backs JsonForScript, so no second encoding scheme appeared
  absence: htmlbind.JSONMember appends a member only when the generated code reaches it, so an absent optional leaves no key rather than writing null
  escaping: the closure escapes, because the attribute op writes its value verbatim
  four_diagnostics: an unknown component, a component declaring no script block, an undeclared parameter, and a parameter with no JSON form
  script_block_required: not asked for, and added because nothing else would consume the object and because the single-root invariant it rides exists for a component declaring a block
  tests: templates/htmlbind/parameter_test.go over emission, the untouched default, and each diagnostic; htmlbind/values_test.go over the member assembly
source:
  - downstream framework change request 2026-08-11, ask 4
  - decision:client-handler-seams
  - requirement:scoped-script-declaration
review_gate: proposed
problem:
  a_handler_needs_a_value: the component was called with it, and the script block cannot reach it
  why_not_today: the block is read verbatim and extracted to a content-hashed file shared by every instance and every render, which is exactly what makes it cacheable; there is nothing per-render to interpolate
  not_changing_that: the file stays shared and hashed, per requirement:component-script-block extraction
  what_a_rendered_attribute_already_covers: most of it, and it is the documented first answer; what it cannot do is keep a type, because a dataset read is text whatever the Go value was
emits:
  where: the component's single root element, beside the declaration marker requirement:scoped-script-declaration already writes there
  what: a JSON object holding only the parameters the compile option names
  escaping: ordinary attribute escaping
  when: only where the named set is non-empty
  root_invariant_already_exists: a component declaring a script block must render exactly one root element, which requirement:scoped-script-declaration asset_field.single_root already imposes for the marker
input:
  from: the caller, per component declaration, as the set of parameter names to emit
  how_the_caller_computes_it: by reading which parameters the block's setup destructured, so the author's own code is the declaration of what crosses and there is no second list to keep in step
  unverifiable_here: the module cannot check that reading, per decision:client-handler-seams the_cost_of_the_arrangement
type_rule:
  chosen: jsonSerializable, the rule templates/htmlbind/compiler.go already applies to a JsonForScript argument
  accepts: string, bool, int, float, decimal, enum, an array of an accepted type, and a record whose fields are all accepted, recursively and with cycles handled
  rejects: a pending value, and html, which falls through the same switch
  encoder_already_exists: the record types reachable from an emitted set are collected the way collectJSONRecords already collects them for JsonForScript
  not_the_redraw_rule: the ask named requirement:component-redraw-endpoint, whose refusal of a record and a slice is justified by a query string carrying values deterministically; JSON is not a query string, so that justification does not transfer and the rule is over-restrictive here
  an_error_not_an_omission: naming a parameter the rule refuses fails generation, for the reason the redraw diagnostics already give, that the author asked for it by naming it in code that uses it
absence:
  rule: an absent optional omits its key
  not_null: it would leave JavaScript two absences to test where one will do
  matches: the attribute context, which omits the whole attribute when a value is absent, through the presence bool the generated Attr op already carries
what_this_is_unlike_the_declaration_marker:
  marker: static text, one compile-time constant, costing nothing per render, which is why requirement:scoped-script-declaration made it static
  this: rendered content, encoded per instance per render
  cache: the values are part of what a component's inputs already key, so requirement:component-output-cache and requirement:layout-reuse-boundaries stay correct; stated because the marker's cost argument does not carry over
  delta_gain: a component whose markup is identical and whose parameters differ now compares as changed, which is correct and was not previously representable
disclosure:
  fact: an emitted parameter is in the DOM, so it is client-readable and client-editable
  precedent: decision:action-lowering-profile round_trip_state says the same of framework state, that DOM-exposed state must be signed or treated as untrusted input
  who_decides: the author, by naming the parameter, which is why the set is opt-in per component rather than every parameter
  the_sharp_edge: the caller derives the set from a setup destructuring, so the disclosure boundary is decided by whether a name was destructured in JavaScript; that reads fine until a server-authoritative value is pulled out for display
  documentation: this must be stated where the feature is documented, not inferred from the type rule
compatibility:
  bytes: a component whose named set is empty, or that declares no block, emits nothing and regenerates byte for byte
  precedent: the same shape as Scope being written onto a generated asset only when set, per requirement:scoped-script-declaration as_built.emitted_by
acceptance:
  - a component naming a parameter set emits one attribute on its root element holding those keys and no others
  - an absent optional omits its key rather than emitting null
  - a parameter of a record or array type emits as JSON rather than failing
  - a parameter of an html type, or a pending one, fails generation naming the parameter
  - a component naming no parameter, or declaring no script block, regenerates byte for byte
  - the attribute coexists with the declaration marker on the same root element
related:
  - requirement:scoped-script-declaration
  - requirement:component-script-block
  - requirement:script-block-reporting
  - requirement:component-redraw-endpoint
  - requirement:component-output-cache
```

---
id: requirement:template-client-handlers
type: requirement
title: Template Client Handlers
---
Let a template name a function the component's script block produced, by reserving an on-prefixed hyphenated attribute inside such a component and lowering it to one marker attribute per element.

```yaml
priority: should
status: implemented 2026-08-12
as_built:
  reservation: templates/htmlbind/handler.go, gated on the component declaring a script block, so the namespace stays ordinary everywhere else
  matching: the literal prefix then one or more ASCII lowercase letters to the end, the same tail rule:event-attribute-context spells; a second hyphen such as on-my-event is not matched and keeps its custom-element reading
  lowering: one attribute per element, default data-tb-on, written where the declaration marker is and skipping the authored attributes in the same pass
  option: GenerateOptions.ClientHandlers, a ClientHandlerSet per component with Resolved names and an Unresolved name-to-reason map
  unchecked_is_a_state: a component absent from the map is accepted and lowered, which is what lets requirement:script-block-reporting run before the caller has anything to answer with
  emission_and_analysis_agree: emission reads the same script-block condition analysis did, through the scope identity the declaration marker already sets
  tests: templates/htmlbind/handler_test.go over the marker, the free namespace outside a block, an unknown name, the caller's reason reaching the diagnostic, an unchecked component, four bad values, and the second-hyphen case
source:
  - downstream framework change request 2026-08-11, ask 2
  - decision:client-handler-seams
  - requirement:component-script-block
review_gate: proposed
model:
  authored: '<button on-click="increment" data-id={row.ID}>+1</button>'
  a_handler_is_not_an_action: a server action is a destination, one per element, whose trigger follows from the element kind; a client handler is a listener, several per element, and most useful events are not the activation event
  division: the same one decision:server-action-lowering already has; the module reserves the attribute and lowers it, and the caller owns everything happening in a browser
  resolution: the handler set comes back as a compile option, on the surface GenerateOptions.ServerActions occupies, so nothing here reads JavaScript
spelling:
  chosen: an on-prefixed hyphenated attribute, on- then the event name
  free_because: rule:event-attribute-context matching.not_matched already excludes a hyphenated on- name from the handler roster, so onclick keeps meaning inline JavaScript and that rule needs no other change
  not_onclick: a bare identifier is a valid expression statement and RawJavaScript in an on-attribute is a shipped feature, so reinterpreting onclick would be the silent reading decision:lifecycle-from-declaration-block position_alone_is_ambiguous already recorded as a failure
  amends_that_rule: the same clause assigns the namespace to custom elements, so reserving part of it is a reversal of a rule approved 2026-08-06 and is written back into it rather than only consumed here
  prior_art: the spelling is Polymer's declarative event binding, which is an argument for familiarity and a note that a template ported from it changes meaning inside a script-block component
reservation_scope:
  chosen: inside a component declaring a script block, and nowhere else
  outside_one: an on-prefixed hyphenated attribute stays an ordinary attribute, emitted unread, exactly as today
  narrowed_from: an ask that also made the attribute an error on an element inside no component declaring a block
  why_narrowed: that rule takes the namespace from every template declaring no block, and buys one diagnostic for it; rule:event-attribute-context assigned that namespace deliberately and nothing in this feature needs it back
  inside_one_and_unresolved: a generation error, per diagnostics below
value:
  form: a static string naming one handler
  computed_rejected: a generation error with the wording analyzeServerAction uses, because a computed name can be neither resolved nor checked
  several_on_one_element: allowed; an element may carry on-click and on-blur alike
lowering:
  emitted: one prefixed attribute per element, listing each event and the handler it binds
  authored_attribute: never emitted, exactly as server-action is never emitted
  grammar: comma between entries and colon within one, as in 'click:increment,blur:validate'
  grammar_is_new_here: the caller's argument that this needs no second parse rule rests on parseScopeCatalog, which is the caller's own symbol and exists nowhere in this module; the spelling is accepted on its own merits
  why_lowered_rather_than_left: CSS has no attribute-name prefix match, so finding an on-anything means walking every element on every mount and every swap, where one marker is a single indexed query; leaving it would also claim the namespace rule:event-attribute-context assigns elsewhere
  emit_shape: static text, the same single write templates/htmlbind/emit.go emitServerAction performs
diagnostics:
  unknown_name: a generation error naming the template position, which is the half the module holds
  unresolved_marker: the caller supplies an explicit unresolved entry rather than omitting the name, because an omission is indistinguishable from a map that was never populated; mandatory rather than offered
  position_is_ours_and_the_reason_is_theirs: the module reports where, the compile option says why
  no_block_no_handlers: an on-attribute inside a component declaring a script block whose reported handler set is empty is the unknown-name case, not a separate one
the_unverifiable_dependency:
  what: the handler set is produced by a JavaScript reading this module cannot check
  consequence: a wrong reading emits a wrong binding and the only diagnostic available is that the map said so
  accepted: decision:client-handler-seams the_cost_of_the_arrangement records the trade the caller stated
compatibility:
  bytes: a project writing no on-prefixed hyphenated attribute inside a script-block component emits identical output
  reinterpretation: confined to a component declaring a script block, which requirement:component-script-block shipped 2026-08-11, so the affected surface is one release old
  tinygo: generation-time only, so requirement:tinygo-wasm targets are untouched
acceptance:
  - an element inside a component declaring a script block carries one lowered attribute naming its events and handlers, and no authored on-attribute
  - two on-attributes on one element produce one lowered attribute holding both entries
  - an on-attribute outside a component declaring a script block is emitted unchanged, as an ordinary attribute
  - onclick with an expression still requires trusted_javascript, per rule:event-attribute-context
  - a handler name the compile option reports unresolved fails generation at the attribute position
  - a computed on-attribute value fails generation
  - every other attribute on the element survives to the output unread
related:
  - rule:event-attribute-context
  - requirement:script-block-reporting
  - decision:server-action-lowering
  - decision:client-runtime-ownership
  - requirement:scoped-script-declaration
```

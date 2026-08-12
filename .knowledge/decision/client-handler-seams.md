---
id: decision:client-handler-seams
type: decision
title: Client Handler Seam Requests From The Downstream Framework
---
Accept the sixth downstream round: three asks widen a seam and the fourth changes default emitted output, which no previous round did and which is accepted anyway because that output is wrong.

```yaml
source:
  - downstream framework change request 2026-08-11, against v0.5.7
  - decision:framework-integration-seams
  - decision:update-composition-seams
review_gate: proposed
round:
  when: 2026-08-11, the sixth round from this reporter
  previous: generation seams 2026-07-30, live integration 2026-07-31, component and asset seams 2026-07-31, runtime ownership 2026-08-01, composition seams 2026-08-02
  shape: four asks and one item offered for discussion rather than requested
  reporter_position: ask 1 is a correctness fix closing an acceptance condition it currently fails; asks 2 to 4 are one feature whose seam is ask 3
  reporter_catalog: the report names nine of its own entries holding the reasoning behind the asks, spelled action-invocation-runtime, scriptless-action-forms, action-entry-point-selection, component-script-event-binding, event-binding-attribute-spelling, component-handler-namespace, script-block-parsing-ownership, template-behavior-attributes, and server-action-authoring; all are recorded in that project's catalog rather than this one, so they are written unqualified here
the_division_offered:
  reporter_owns: the script block's contents, the browser runtime, the setup signature, the endpoint prefix, CSRF, and the page-tree wiring
  module_owns: the template grammar, attribute reservation, emitted markup, and every diagnostic needing a source position
  the_load_bearing_line: the module reads no JavaScript; where a decision needs to know what is inside a block, the reporter parses it and passes the result as a compile option
  verdict: sound, and it is the arrangement decision:lifecycle-from-declaration-block rejected_alternatives.export_convention_alone deferred to a parser rather than refused; this round supplies the parser from outside
verification:
  method: every claim checked against the v0.5.7 source and against this catalog
  form_is_a_get_form: confirmed, and stronger than reported. templates/htmlbind/emit.go emitServerAction writes the URL attribute alone, and templates/htmlbind/csrf.go unsafeForm requires a static unsafe method, so an absent method yields no token either. The reporter derived this from the rule and could not observe it; the code agrees with the rule.
  the_get_form_also_discards_page_state: measured 2026-08-12; the submit replaces the page's own query rather than appending to it, so the user lands on a URL the application never produced and the mutation is skipped as well. Neither side had this.
  hyphenated_namespace_free: confirmed; rule:event-attribute-context matching.not_matched excludes it and TestHyphenatedOnNameIsNotAHandler fixes it
  script_free_half_never_built: confirmed; no ScriptFree symbol exists in the tree, and decision:script-free-render-mode downstream_dependency already records the design as unimplemented
  page_pattern_is_get_only: confirmed; routetree.Route.Pattern returns 'GET ' plus the path, and the only POST registration in the tree is routetree.Action.Pattern
  unknown_fields_harmless: confirmed; nothing in the binder rejects a form field the input type does not declare, so generated hidden fields reaching the direct entry point cost nothing
four_corrections_to_the_reporter_s_reading:
  an_author_can_do_more_than_reported:
    reported: an application cannot work around the GET form at all
    actual: templates/htmlbind/action.go permits method="post" on a form carrying server-action and makes every other value an error, so writing it yields a POST form with the CSRF field today
    still_missing: the selector field and the page-pattern POST route, so the handler still never runs
    why_it_matters: the conclusion holds and the gap is narrower than the ask assumes, which changes what the fix costs
  the_action_attribute_is_not_needed:
    finding: a form with no action submits to the document URL, and a POST keeps that URL's query rather than replacing it
    consequence: the script-free lowering needs no concrete request path, so decision:server-action-lowering form_action_url is deleted rather than paid for
    scale: that channel was the most expensive part of the script-free design, a render option read through renderer state at the typed rung of decision:route-handler-shape
    status: measured in a browser 2026-08-12, after being drafted from the specification alone; requirement:native-action-form-submit records the probe and its numbers
    one_sub_claim_was_wrong: the draft also said an absent action is more correct than action="" because the empty string resolves against a base element. Measured false; the specification defines an empty action as the document URL rather than as a URL to resolve, so both spellings reach the same target on a base-carrying page. Removed rather than repaired, because the module emits neither.
    worth_recording: the finding that survived was drafted from the same reading as the one that did not, so the probe is what separated them and the reporter's own standard, deriving rather than observing, is what both sides had been working to
  parse_scope_catalog_is_not_ours:
    reported: the comma-and-colon value grammar needs no second parse rule because parseScopeCatalog already reads it
    actual: no such symbol exists anywhere in this module, in Go, in documentation, or in shipped script; it is the reporter's own
    consequence: the grammar is new here either way, and this module emits no comma-and-colon catalog today
    verdict: the spelling is still accepted, on its own merits rather than on a shared-grammar argument that does not hold
  the_redraw_type_rule_is_the_wrong_one:
    reported: reuse the rule refusing a record, a slice, and html, which requirement:component-redraw-endpoint already has
    why_not: that rule's stated reason is that a query string must carry the value deterministically, and an attribute holding JSON is not a query string
    already_present: templates/htmlbind/compiler.go jsonSerializable, which backs JsonForScript, accepts records and arrays recursively, rejects a pending value, and already falls through to false for html
    verdict: use jsonSerializable; it is the rule whose justification transfers, it has a generated encoder path, and it is strictly more capable than the one asked for
accepted:
  - what: requirement:native-action-form-submit
    ask: 1
    value: highest; it is the only item where shipped markup is wrong rather than absent
    cost: the page-pattern POST registration and the selector dispatcher, and nothing else once the request-path channel is deleted
    changes_two_decisions: decision:server-action-lowering emits both sets from one compile, and decision:script-free-render-mode stops selecting between them
    status: implemented 2026-08-12
    two_costs_found_while_building: a page declaring a form now owns its own POST address, which collided with a composition property the framework-owner guide documents; and such a page needs a CSRF token on every render, because the emitted form is unsafe by construction. Both are recorded in that requirement, and the first is what narrowed registration to form-referenced handlers.
  - what: requirement:script-block-reporting
    ask: 3
    value: high; it is the seam the other two consume and the cheapest item in the round
    cost: a third reader beside ActionRefs and Signatures, running the analysis Generate already runs
    status: implemented 2026-08-12, in the order this decision's sequencing_correction gives
  - what: requirement:template-client-handlers
    ask: 2
    value: high
    narrowed: the reservation applies inside a component declaring a script block and nowhere else, against an ask that also made the attribute an error everywhere outside one
    cost: one reserved attribute, one lowering, and an amendment to rule:event-attribute-context
    status: implemented 2026-08-12
  - what: requirement:component-parameter-emission
    ask: 4
    value: medium; reading a rendered attribute covers most of it and this buys the type back
    cost: one attribute, using the serialization rule and the encoder that already ship
    type_rule_replaced: see the fourth correction above
    status: implemented 2026-08-12
    one_rule_added_while_building: the component must declare a script block, because nothing else would consume the object and the single-root invariant the attribute rides exists for such a component
integrated_path:
  added: routetree.GenerateOptions.ScriptResolver, reporting a template's blocks and taking back the two answers
  why: the reporter compiles its own templates directly, but the tree generator is where the feature is used, and without this the whole round would have been reachable only from a direct htmlbind compile
  cost: one extra parse per template carrying a block, paid only by a tree configuring a resolver, because the blocks have to be reported before the compile that consumes the answers
end_to_end:
  where: internal/pagesfixture, whose about page declares a script block, an on-prefixed handler and an emitted parameter set, and whose users page declares a native form
  what_only_this_reaches: that the generated Go compiles and that the emitted closure produces a value a browser can read; a source-level assertion reaches neither
  confirmed: a native submit reaching its handler with the path value and answering 303; the handler marker rendering while the authored attribute does not; the parameter object rendering as JSON with an absent optional omitting its key rather than writing null
  the_resolver_is_a_stub_there: it returns fixed answers rather than parsing JavaScript, which is what the fixture is for; the parser is the caller's and stays outside this module
sequencing_correction:
  reported: among asks 2 to 4, ask 3 lands first because it is the seam
  actual: the half of ask 3 reporting referenced handler names cannot exist until the attribute of ask 2 is parsed and recorded
  order: report the block text, reserve and record the attribute, report the references, lower with the resolved map, with ask 4 independent throughout
  effect_if_unnoticed: one avoidable round trip, because a build against ask 3 as specified would find half of it unimplementable
entry_point_discussion:
  item: the reporter's fifth item, offered rather than asked, on a scripted action's inability to read a path parameter the script-free channel reads
  their_first_option: the runtime posts to the page URL and names the handler in a header
  verdict: sound, and not free from here; a page pattern accepts no POST today, and that registration is exactly what ask 1 introduces, so this option is sequenced behind ask 1 rather than independent of it
  once_ask_1_ships: a form needs no header at all, because it already carries the selector as a hidden field and its absent action already targets the page URL; the header is needed only for an element with no form
  their_third_option_is_the_honest_one: if the runtime posts to the page URL then the emitted URL is an address nobody fetches, which is the smell the reporter named; carrying the selector instead makes both channels one string
  why_it_cannot_be_the_default: decision:action-lowering-profile rests on a URL lowering driving an existing client library, and hx-post needs a URL; the selector channel is a profile setting there instead
  their_second_option_is_unnecessary_rather_than_limited: requirement:template-server-functions entry_points.form.path_parameters already emits no hidden fields for path parameters because the URL carries them, and an absent-action POST keeps that true
  unnamed_by_either_side: the two entry points also disagree about what a response means, 303 on the page pattern and verbatim on the direct one; emitting both sets makes that asymmetry permanent rather than per-mode, and a runtime posting to the page URL receives the redirect rather than a delta, which requirement:action-response-update has to answer
principle:
  applies: the decision:framework-integration-seams rule, widen a seam whose default output stays identical and whose contract stays the caller's
  asks_3_and_4_fit: pure addition, and byte-identical output where unused
  ask_2_widens_a_different_contract: the template grammar rather than the shape an author's Go is written against; it is the first ask in six rounds to change what an author may write in a template
  ask_1_does_not_fit: it changes default emitted bytes, which no previous round did
  accepted_anyway: because the current bytes are wrong, so correctness outranks the principle here rather than the principle being wrong
  stated_because: a round that quietly bends its own test leaves the next round unable to tell what the test is
what_the_module_still_does_not_do:
  no_javascript: unchanged; asks 2 and 4 take a resolved map on the surface GenerateOptions.ServerActions already occupies
  no_opinion_about_setup: unchanged, per decision:client-runtime-ownership and requirement:scoped-script-declaration what_the_module_does_not_do
  no_trigger_model: the event in emitted markup follows from the element kind; a template naming an arbitrary event to fire an action was not asked for and is refused for the reason the reporter gives, that it would reopen the split ask 1 closes
  no_wire_change: no header, no manifest field, and no record kind in the whole round
the_cost_of_the_arrangement:
  what: generation for asks 2 and 4 depends on a JavaScript reading performed outside the module, which the module cannot check
  consequence: a wrong reading emits a wrong handler set or wrong parameters, and the only diagnostic available is that the map said so
  accepted: the reporter states the trade outright, that its parser will be wrong about real code at first and is cheaper to fix on its own cadence
  mitigation_required_not_offered: the unresolved marker the reporter offers to supply is mandatory, because an omission is indistinguishable from a map that was never populated
related:
  - decision:server-action-lowering
  - decision:script-free-render-mode
  - decision:action-lowering-profile
  - rule:event-attribute-context
  - requirement:component-script-block
```

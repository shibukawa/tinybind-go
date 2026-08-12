---
id: decision:server-action-lowering
type: decision
title: Server Action Attribute Lowering
---
Lower the reserved server-action attribute to a URL and nothing else, leaving every other attribute on the element untouched.

```yaml
source:
  - requirement:template-server-functions
  - user attribute-spelling decision 2026-07-29
  - user framework-arrangement discussion 2026-07-29
review_gate: proposed
spelling:
  chosen: server-action
  closes: the attribute spelling left open by requirement:template-server-functions
  scope: reserved on any element, not on form alone
  emitted: never; each lowering below replaces it
rule:
  one_job: resolve the named Go handler and write its URL into the element
  pass_through: every other attribute survives to the output unread
  gain: the compiler models no client protocol, so client behavior is authored in the framework's own attribute vocabulary
  profile: decision:action-lowering-profile chooses the names written
lowerings:
  selected_by: nothing, as of 2026-08-12; both sets are emitted from one compile, per both_sets_emitted below
  was: decision:script-free-render-mode chose one of the two, and a build carried only that one
  scripted:
    applies_to: every element, a form included
    emits: one URL-carrying attribute, defaulting to data-tb-action
    url: the direct entry point of requirement:template-server-functions
    static_url: that URL carries no path parameter, so it is a compile-time constant and needs no render-time channel
    form: intercepted by the runtime, so no action, method, hidden selector, or hidden token is written
    gain: one rule covers every element, and the selector, the page-pattern POST registration, and the render-time path channel are all unnecessary
  script_free_form:
    element: form carrying server-action
    emits: action, method=post, a hidden selector field, and the hidden token of policy:html-update-csrf-protection
    url: the page's own pattern, so the address bar keeps showing the page
    several_forms: each form carries its own selector, so one page pattern serves several handlers and the generated dispatcher branches on that field
  script_free_submit_button:
    element: button inside a form, carrying its own server-action
    emits: formaction whose query carries that handler's selector
    why: formaction is the native per-button override, so one form also dispatches several handlers
    rejected_name_value: a submit button's name and value are submitted alongside the form's hidden selector, colliding on one key where the first value wins
  script_free_other_element:
    element: anything else, such as a bare button
    result: the scripted attribute alone and no native fallback, decided 2026-08-12
    was: a generation error, per the authoring rule of decision:script-free-render-mode
    why_it_changed: with both sets emitted that error would apply always, and a bare button is the common shape under today's default, so requirement:native-action-form-submit demotes it to an opt-in diagnostic
    unchanged: the generator still cannot wrap it in a form without knowing an ancestor chain it cannot see, which is why the author remains the one who decides
selector_precedence:
  rule: a selector in the query wins over one in the body
  reason: the two channels coexist only for the submit_button lowering
  value: the one opaque hash and name string of requirement:template-server-functions, spelled identically in both channels and in the direct entry point URL
progressive_enhancement:
  owner: this decision, as of 2026-08-12; a form carrying server-action reaches its handler natively in every build
  was: decision:script-free-render-mode, which a project selected when a native submit had to reach the handler, making progressive enhancement a property of that mode rather than of the default
  exposure: the generated route table no longer needs to record which set a route was lowered under, because every route carries both
  still_partial: only a form has a native path; a bare button is scripted-only, per script_free_other_element
resolution_dataflow:
  problem: the template compiler cannot compute the URL, because the route path and the hash are known only to route discovery
  pass_1: the compiler reports referenced action names, beside the existing template signature reader
  pass_2: route discovery matches each name to an exported function in the route package and derives its hash and URL
  pass_3: the resolved map returns as a compile option and is lowered into markup
  precedent: the optional context detection of requirement:async-external-functions already resolves a template fact between passes, by reading the package Go sources and handing the result to the compiler as an option
  rejected_runtime_lookup: a name-keyed map consulted at request time, refused by the no string-based dependency rule of decision:html-route-go-package-model
form_action_url:
  status: deleted 2026-08-12, never built
  was: the form lowering needs the concrete request path, so a render option would carry it and the CSRF token, read by a renderer-state operation rather than from component parameters, because the path is not held at the typed rung of decision:route-handler-shape
  why_it_is_unnecessary: a form with no action submits to the document URL, and a POST preserves that URL's query rather than replacing it, so the page pattern is already the target with nothing emitted
  measured: 2026-08-12, in the browser; a form with no action and method=post on /users/123?tab=x reached POST /users/123?tab=x
  base_element_does_not_reach_it: measured on a document carrying a base element whose relative resolution was confirmed active; the form still targeted the document URL, because an absent action never resolves anything
  what_it_unblocks: it was the most expensive part of the script-free set, and deleting it is most of why both_sets_emitted became affordable
both_sets_emitted:
  decided: 2026-08-12, per requirement:native-action-form-submit and decision:client-handler-seams
  implemented: 2026-08-12
  rule: one compile emits the scripted attribute and the native form markup together, and the runtime's presence selects which mechanism drives them
  narrowed_while_building: the page POST route is registered only for a handler a template names from a form; a bare button has no native submit to serve, so registering one would claim an address an application may want and buy nothing
  the_defect_it_fixes: the scripted set alone leaves a form declaring no method, which is a GET form to the current URL rather than inert markup, so a native submit silently does the wrong thing
  not_cloaking: the bytes are identical for every client, so the switch_placement argument of decision:script-free-render-mode is untouched; that argument was against a per-request switch and this is not one
  cost: the hidden selector, the page-pattern POST registration, and the generated dispatcher, paid by every project whether or not any client submits without a runtime
  profile: decision:action-lowering-profile may narrow the output for a framework that wants only one set
constraints:
  - the attribute value stays a static string literal, so the symbol resolves at generation
  - server-action stays an ordinary attribute name wherever no lowering applies
  - an author-written action, or a method other than post, on a form carrying server-action is a generation error
```

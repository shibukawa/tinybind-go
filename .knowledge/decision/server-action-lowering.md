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
  selected_by: decision:script-free-render-mode, which chooses one of the two sets below
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
    result: a generation error, per the authoring rule of decision:script-free-render-mode
    why: nothing can invoke it, and the generator cannot wrap it in a form without knowing an ancestor chain it cannot see
selector_precedence:
  rule: a selector in the query wins over one in the body
  reason: the two channels coexist only for the submit_button lowering
  value: the one opaque hash and name string of requirement:template-server-functions, spelled identically in both channels and in the direct entry point URL
progressive_enhancement:
  owner: decision:script-free-render-mode, which is what a project selects when a native submit must reach the handler
  consequence: the progressive enhancement claim of requirement:template-server-functions is a property of that mode rather than of the default
  exposure: the generated route table records which set a route was lowered under, so a framework can report it
resolution_dataflow:
  problem: the template compiler cannot compute the URL, because the route path and the hash are known only to route discovery
  pass_1: the compiler reports referenced action names, beside the existing template signature reader
  pass_2: route discovery matches each name to an exported function in the route package and derives its hash and URL
  pass_3: the resolved map returns as a compile option and is lowered into markup
  precedent: the optional context detection of requirement:async-external-functions already resolves a template fact between passes, by reading the package Go sources and handing the result to the compiler as an option
  rejected_runtime_lookup: a name-keyed map consulted at request time, refused by the no string-based dependency rule of decision:html-route-go-package-model
form_action_url:
  problem: the form lowering needs the concrete request path, which the component does not hold at the typed rung of decision:route-handler-shape
  channel: a render option carrying the request path and the CSRF token, read by a renderer-state operation rather than from component parameters
  precedent: the merged head operation already reads renderer state instead of parameters
  scope: the form lowerings only; other_element needs no such channel
constraints:
  - the attribute value stays a static string literal, so the symbol resolves at generation
  - server-action stays an ordinary attribute name wherever no lowering applies
  - an author-written action, or a method other than post, on a form carrying server-action is a generation error
```

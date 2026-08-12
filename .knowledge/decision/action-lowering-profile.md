---
id: decision:action-lowering-profile
type: decision
title: Action Lowering Profile
---
Let a framework choose the attribute names decision:server-action-lowering writes, so a generated action drives an existing client library instead of a tinybind runtime.

```yaml
source:
  - decision:server-action-lowering
  - requirement:custom-framework-generation-profile
  - user framework-arrangement discussion 2026-07-29
review_gate: proposed
motive:
  observed: a bare element lowering plus untouched attributes is already enough to drive an HTMX-style runtime
  example: server-action lowered to hx-post leaves hx-target and hx-swap to HTMX, with no glue code emitted or written
  conclusion: the emitted attribute name is the only thing that must move for a foreign runtime to work
customizable:
  render_mode: decision:script-free-render-mode, which selects a lowering set rather than renaming anything, and is therefore a different kind of setting from the names below
  url_prefix: the entry point prefix of requirement:template-server-functions, defaulting to /_action
  name_resolution: requirement:external-action-resolution, so a framework's own route table can address a handler the route tree does not hold
  element_attribute: the URL attribute of the other_element lowering, defaulting to data-tb-action
  element_attribute_value:
    what: whether that attribute carries the endpoint URL or the selector alone
    status: not built; recorded 2026-08-12 as the shape decision:client-handler-seams entry_point_discussion would take if the downstream stops resolving the asymmetry in its own runtime
    default_is_fixed: the URL, because this decision rests on lowering to hx-post and an existing library needs an address there, so the selector could only ever be a setting
    not_a_gap_yet: the reporter said it would fix the path-parameter asymmetry in its runtime, so nothing has asked for this
  hidden_fields: the selector and token field names of the form lowering
  selector_channel: whether the form lowering carries the selector in a hidden field or in the action query
  both_lowerings:
    what: whether one compile emits the scripted attribute and the native form markup together
    status: no setting exists; both are emitted whenever a selector resolves, per decision:server-action-lowering both_sets_emitted
    the_narrowing_that_does_exist: supplying no selector for a name leaves the scripted attribute alone, which is what requirement:external-action-resolution already produces; it is a consequence of not resolving rather than a knob
    corrected: 2026-08-12, having been listed here as a setting before it was built
  handler_attribute: the lowered attribute name of requirement:template-client-handlers
  parameter_attribute: the lowered attribute name of requirement:component-parameter-emission
  method_attribute: whether method is emitted
  dispatcher: the generated POST dispatcher, replaceable through the same registry template override requirement:generated-route-registration already exposes
fixed:
  handler_shape: the http.HandlerFunc signature of requirement:template-server-functions
  hash: its input, length, and determinism
  response_rule: writing nothing means a redirect on the form entry point, and a verbatim response everywhere else
  reason: these are the contract a handler is written against, so moving them would change what an author's Go already means
client_encoding:
  fact: api:bind already accepts JSON, urlencoded, and multipart payloads for one input struct
  consequence: a script posting JSON collected from framework attributes and a script-free form post reach the same Go type
  server_cost: none; the encoding choice is the framework's and needs no generated support
round_trip_state:
  observed: a framework may return state in an attribute and resubmit it on the next request, in the manner of a cookie
  relation: this is a lighter form of the opaque boundary continuation of requirement:partial-update-boundaries
  ownership: the format is the framework's, never tinybind's
  security: state exposed in the DOM is client-editable, so it must be signed or treated as untrusted input
surface:
  template_side: the htmlbind compile options that already carry the context external selection of requirement:async-external-functions
  registry_side: the routetree emitter symbols and templates
  tension: one feature's settings sit on two configuration surfaces, so a framework needs one object feeding both
prefix_resolved:
  was: requirement:template-server-functions fixed the /_action prefix and forbade configuring it
  problem: a framework free to choose attribute names but not the URL space could not mount under a sub-path or place actions in its own namespace
  now: configurable, decided 2026-07-29; the default is unchanged and the shadow check it requires is stated in requirement:template-server-functions
constraints:
  - a profile changes emitted markup and generated registration only, never the handler contract
  - rewriting generated source after generation stays prohibited under requirement:custom-framework-generation-profile
```

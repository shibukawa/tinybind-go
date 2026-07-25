---
id: requirement:html-component-api
type: requirement
title: HTML Component API
---
Generated HTML components bind parameters to a render plan and return a fragment; the shared runtime renders it and the caller owns the response.

```yaml
priority: must
source:
  - downstream framework CLI requirements for tinybind v0.1.12
  - decision:generated-render-plan
signature: func Component(params ComponentParams) htmlbind.Fragment
superseded:
  earlier_shape: func Component(w io.Writer, params ComponentParams) error
  reason: requirement:head-merging must write the document head before any body byte, which one writer pass cannot do
params_type:
  exported_component: "{ComponentName}Params"
  private_component: "render{Name}Params"
  rules:
    - always generated, including for zero-parameter and single-parameter components
    - a zero-parameter component still gets an empty struct and the same one-argument shape
    - one exported field per declared component parameter, preserving declaration order
    - field name derives deterministically from the parameter name
    - field type is the declared parameter type
    - a declaration already using the generated name fails with a conflict diagnostic
wrapper_binder:
  emitted_for: an exported component declaring an unnamed slot
  shape: func BindComponent(params ComponentParams) htmlbind.Wrapper
  reason: only a component with an unnamed slot can wrap another, so misuse is a compile error
component_owns:
  - context-aware escaping
  - typed field access
  - control flow and loops
  - requirement:explicit-output-control intrinsics and JsonForScript
  - requirement:html-slot-syntax insertion points
  - requirement:head-merging contributions, transitively over the call graph
component_never_owns:
  - http.ResponseWriter and *http.Request parameters
  - Content-Type and Content-Encoding decisions
  - content negotiation, compression, and response commit
  - error response rendering
html_slot_type: htmlbind.Fragment; its zero value means an absent optional slot
rendering:
  single: htmlbind.Render writes one fragment to an io.Writer
  chain: api:render-html-chain composes wrappers around a leaf
  lifetime: a fragment is immutable and safe to reuse across requests
response_runtime:
  owner: the caller
  rationale: tinybind ships no HTML response runtime; a handler passes http.ResponseWriter straight to Render and keeps header, negotiation, and compression decisions
capability_interaction:
  model: data:component-render-capabilities
  rules:
    - request-bound capabilities belong to the caller's response handling, not the component
    - request-bound capabilities include filesystem route roles, partial update boundaries, and request-keyed caches
    - an await boundary is supported through decision:async-component-signature once it ships
    - a component requiring an unshipped capability must fail generation with an actionable diagnostic
acceptance:
  - a one-parameter component generates UserPageParams and func UserPage(params UserPageParams) htmlbind.Fragment
  - a zero-parameter component generates an empty params struct and the same one-argument shape
  - a multi-parameter component generates one field per parameter in declaration order
  - generated template output references no net/http identifier and imports no HTTP runtime
  - rendering to a buffer needs no HTTP value
related:
  - requirement:template-code-generation
  - requirement:html-template-v1
  - requirement:html-rendering-compatibility
  - data:component-render-capabilities
  - requirement:explicit-output-control
  - requirement:custom-framework-generation-profile
```

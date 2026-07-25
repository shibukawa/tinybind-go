---
id: requirement:html-component-api
type: requirement
title: HTML Component API
---
Generated HTML components are HTTP-independent io.Writer functions with one generated parameter struct; the runtime owns the response.

```yaml
priority: must
source: downstream framework CLI requirements for tinybind v0.1.12
signature: func Component(w io.Writer, params ComponentParams) error
params_type:
  exported_component: "{ComponentName}Params"
  private_component: "render{Name}Params"
  rules:
    - always generated, including for zero-parameter and single-parameter components
    - a zero-parameter component still gets an empty struct and the same two-argument shape
    - one exported field per declared component parameter, preserving declaration order
    - field name derives deterministically from the parameter name
    - field type is the declared parameter type
    - a declaration already using the generated name fails with a conflict diagnostic
component_owns:
  - context-aware escaping
  - typed field access
  - control flow and loops
  - requirement:explicit-output-control intrinsics and JsonForScript
  - streaming writes to the supplied writer
  - returning write errors unchanged
component_never_owns:
  - http.ResponseWriter and *http.Request parameters
  - Content-Type and Content-Encoding decisions
  - content negotiation, compression, and response commit
  - error response rendering
html_slot_type: func(io.Writer) error
response_runtime:
  owner: the caller
  rationale: tinybind ships no HTML response runtime; http.ResponseWriter is an io.Writer, so a handler passes it directly and keeps header, negotiation, and compression decisions
caller_contract: the generated function is assignable to func(io.Writer, P) error, so any framework runtime can render it
capability_interaction:
  model: data:component-render-capabilities
  rules:
    - request-bound capabilities belong to the caller's response handling, not the component
    - request-bound capabilities include filesystem route roles, suspense streaming boundaries, partial update boundaries, and request-keyed caches
    - a component requiring one must fail generation with an actionable diagnostic until that capability ships
acceptance:
  - a one-parameter component generates UserPageParams and func UserPage(w io.Writer, params UserPageParams) error
  - a zero-parameter component generates an empty params struct and the same two-argument shape
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

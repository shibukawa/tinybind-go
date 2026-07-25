---
id: requirement:html-writer-api-mode
type: requirement
title: HTML Writer API Mode
---
Optional generation mode emits HTML components as HTTP-independent io.Writer functions with one generated parameter struct.

```yaml
priority: must
source: downstream framework CLI requirements for tinybind v0.1.12
option:
  owner: data:generator-options
  name: HTMLWriterAPI
  type: bool
  default: false
default_mode:
  signature: func Component(w http.ResponseWriter, r *http.Request, params...) error
  responsibilities: content type, content encoding, compression negotiation, response commit, error response
  status: unchanged default; remains compatible
writer_mode:
  signature: func Component(w io.Writer, params ComponentParams) error
  params_type: "{ComponentName}Params for exported components; render{Name}Params for private ones"
  params_rule:
    - always generated, including for zero-parameter and single-parameter components
    - a zero-parameter component still gets an empty struct and the same two-argument shape
    - one exported field per declared component parameter, preserving declaration order
    - field name derives deterministically from the parameter name
    - field type is the declared parameter type
    - a declaration already using the generated name fails with a conflict diagnostic
  html_slot_type: func(io.Writer) error
  keeps:
    - context-aware escaping
    - typed field access
    - control flow and loops
    - requirement:explicit-output-control intrinsics and JsonForScript
    - streaming writes to the supplied writer
    - write errors returned unchanged
  drops:
    - http.ResponseWriter and *http.Request parameters
    - Content-Type and Content-Encoding decisions
    - compression setup and response commit
    - error response rendering
  caller_contract: generated function is assignable to func(io.Writer, P) error, so the framework runtime owns the HTTP response
capability_interaction:
  model: data:component-render-capabilities
  rules:
    - mode selection is package-wide and fixed at generation time
    - request-bound capabilities are unavailable in writer mode
    - request-bound capabilities include filesystem route roles, suspense streaming boundaries, partial update boundaries, and request-keyed caches
    - a component requiring one must fail generation in writer mode with an actionable diagnostic when that capability ships
acceptance:
  - a one-parameter component generates UserPageParams and func UserPage(w io.Writer, params UserPageParams) error
  - a zero-parameter component still generates an empty params struct and the same two-argument shape
  - a multi-parameter component generates one field per parameter in declaration order
  - the same template generates the default HTTP signature when the option is off
  - writer-mode output for identical input bytes equals default-mode body bytes
related:
  - requirement:template-code-generation
  - requirement:html-template-v1
  - requirement:html-rendering-compatibility
  - data:generator-options
  - data:component-render-capabilities
  - requirement:explicit-output-control
  - requirement:custom-framework-generation-profile
```

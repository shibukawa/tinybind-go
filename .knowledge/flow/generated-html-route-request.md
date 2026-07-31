---
id: flow:generated-html-route-request
type: flow
title: Generated HTML Route Request Flow
---
Request path from generated ServeMux registration through whichever page rung the route uses.

```yaml
flow:
  trigger: a route registered by api:register-generated-html-routes matches the request
  branch: decision:route-handler-shape selects the rung from the presence and signature of func Page
  steps:
    - id: context
      owner: application
      action: run whatever middleware the application already wrapped around the mux
    - id: dispatch
      owner: generated
      action: enter the generated handler for the matched route
    - id: bind
      owner: generated
      rungs: [1, 2]
      action: decode the dynamic path segments and the declared query parameters, in the order requirement:colocated-route-logic fixes
    - id: load
      owner: author
      rungs: [2]
      action: call func Page, which returns the component parameter values, or an error, or a requirement:redirect-error target
    - id: raw
      owner: author
      rungs: [3]
      action: the handler decodes, loads, and calls the generated composer itself, owning the whole response
    - id: compose
      owner: generated
      action: assemble the requirement:nested-layout-composition chain and decision:html-document-shell, validating before any byte is written
    - id: query
      owner: generated
      action: invoke rule:render-external-query-semantics through data:html-route-dependencies for template-declared externals
    - id: respond
      owner: generated
      action: write the merged head, then the body, as complete HTML or a negotiated delta response
    - id: drain
      owner: generated
      action: run flow:chain-render-pipeline when the assembled chain is async, writing and flushing each data:async-boundary-content item
  failure:
    bind: configured invalid-parameter mapping, before func Page runs
    page_function: configured typed HTTP error mapping, or a redirect through requirement:redirect-error, with nothing written yet
    raw_handler: the handler writes whatever status it chooses
    compose_before_commit: configured mapping on rungs 1 and 2, or an error returned to a rung 3 handler
    async_after_commit: decision:async-boundary-syntax recover content and server diagnostics
```

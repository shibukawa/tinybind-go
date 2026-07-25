---
id: flow:generated-html-route-request
type: flow
title: Generated HTML Route Request Flow
---
Generated request path from ServeMux registration through typed data loading and document rendering.

```yaml
flow:
  trigger: api:register-generated-html-routes handler matches a page request
  steps:
    - id: context
      action: apply configured middleware, authentication, authorization, and observability hooks
    - id: bind-path
      action: decode requirement:typed-html-route-parameters from matched route
    - id: bind-query
      action: decode page-declared search parameter record
    - id: plan
      action: select data:html-render-route-plan document and requirement:nested-layout-composition chain
    - id: query
      action: invoke rule:render-external-query-semantics through data:html-route-dependencies
    - id: render
      action: render page into layouts and decision:html-document-shell with component capability lowering
    - id: respond
      action: send complete HTML, streaming updates, or negotiated delta response
    - id: drain
      action: run flow:chain-render-pipeline when the assembled chain is async, writing and flushing each data:async-boundary-content item
  failure:
    before_commit: configured typed HTTP error mapping
    async_after_commit: decision:async-boundary-syntax recover content and server diagnostics
```

---
id: requirement:generated-html-route-handlers
type: requirement
title: Generated HTML Route Handlers
---
Serve filesystem pages without requiring users to hand-write normal HTTP binding and rendering handlers.

```yaml
source: concept:filesystem-html-routing
registration: api:register-generated-html-routes
package_model: decision:html-route-go-package-model
flow: flow:generated-html-route-request
per_route: data:html-render-route-plan
handler_pipeline:
  - match generated stdlib ServeMux method and path pattern
  - establish request, authentication, authorization, tracing, and policy context through configured hooks
  - decode requirement:typed-html-route-parameters from path
  - decode declared typed page search parameters from query
  - invoke generated document, layout, and page render plan
  - dispatch template external calls through data:html-route-dependencies
  - apply async, cache, partial-update, bootstrap, compression, and error behavior from component capabilities
  - classify the assembled chain through requirement:chain-render-pipeline and range the merged decision:async-component-signature sequence, writing each data:async-boundary-content item as an identified template plus update record
  - finalize response and observability
generated_endpoints:
  page_navigation: GET complete HTML and negotiated partial navigation response
  component_update: protected endpoint for api:client-component-update
errors:
  invalid_path_or_query: configured 400 or 404 mapping before component execution
  external_or_render: before-commit error mapping or decision:async-boundary-syntax recover flow
  committed_stream: server observability and boundary-safe update only
compatibility:
  - filesystem route mode requires no ordinary page handler implementation
  - application can exclude or override a route explicitly and register a manual handler
  - existing manually registered handler discovery remains separate from generated route mode
acceptance:
  - page component receives typed path and search values without reading http.Request directly
  - one explicit registration call installs all valid routes
  - generated handlers never hide startup route conflicts
open_questions:
  - route-level middleware and authorization declaration syntax
  - generated update endpoint paths and media types
  - default error page ownership
```

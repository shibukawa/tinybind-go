---
id: requirement:generated-route-registration
type: requirement
title: Generated Route Registration
---
Derive route patterns from the filesystem and generate as much of the handler as the page's rung allows, down to none at all.

```yaml
source: concept:filesystem-html-routing
registration: api:register-generated-html-routes
package_model: decision:html-route-go-package-model
flow: flow:generated-html-route-request
per_route: data:html-render-route-plan
logic: requirement:colocated-route-logic
shape_decision: decision:route-handler-shape
always_generated:
  pattern: a stdlib method and path pattern derived from decision:route-segment-notation directory names
  registration: one GET registration per page, plus the POST form entry point of requirement:template-server-functions
  composer: a per-route Render that applies the requirement:layout-chain-discovery chain and decision:html-document-shell around a page fragment
  route_decoder: a typed decoder for requirement:typed-html-route-parameters and the declared query parameters
  route_table: a readable list of the derived patterns and dynamic segment names
  update_endpoints: the api:client-component-update endpoint when route capabilities require it
per_rung:
  rung_1:
    generated: the whole handler
    body: decode inputs, render the component, compose the chain, respond
    application_go: none on the request path; template external calls go through data:html-route-dependencies
  rung_2:
    generated: the handler around the function
    body: decode inputs, call func Page, map its results onto the component parameters, compose the chain, respond
    error: a returned error becomes the configured error response, or a redirect through requirement:redirect-error
  rung_3:
    generated: registration only
    body: none; the handler owns the response and calls the generated composer itself
composition_pipeline:
  owner: the generated composer
  steps:
    - assemble the layout chain and validate slots, types, and requirement:awaitable-parameters handles before writing
    - merge requirement:head-merging contributions and write the document head
    - dispatch template external calls through data:html-route-dependencies
    - apply async, cache, partial-update, bootstrap, and compression behavior from component capabilities
    - classify the assembled chain through requirement:chain-render-pipeline and range the merged decision:async-component-signature sequence, writing each data:async-boundary-content item as an identified template plus update record
  failure_before_commit: the configured mapping at rung 1, including a failing binding's error per decision:value-binding-hoisting, or an error returned to a rung 3 handler so it still chooses the status
  failure_after_commit: server observability and decision:async-boundary-syntax recover content only
never_generated:
  - what a page loads, which is its own external calls at rung 1 and the handler's business at rung 3
  - the rung 3 handler body
  - authentication, authorization, and any per-route policy, which the application wraps around the mux
errors:
  invalid_path_or_query: configured mapping before the page function runs
  page_function: a returned error maps through the configured mapping or requirement:redirect-error, before any byte is written
  committed_stream: server observability and decision:async-boundary-syntax recover content only
startup:
  conflicts: a duplicate normalized pattern is a startup error
  contract_failures: rung mismatches, parameter-order violations, and result-list mismatches fail at generation, per requirement:colocated-route-logic
customization:
  scope: which symbols the generated handler calls to bind, render, and report errors
  owner: data:generator-options and api:generator-call-registration
  reason: requirement:custom-framework-generation-profile forbids a downstream framework from rewriting generated source to get its own surface
  levels:
    symbols: the packages and declaration names generated code writes
    blocks: the named pieces of a built-in template, being imports, convert, error, handler, and the render block of requirement:framework-render-entry
    files: the decoder, composer, and registry templates end to end
  signature_axis:
    gap: a symbol repoints a package but never the shape of a call, so a framework needing the request at the render entry had to replace whole files
    closed_by: requirement:framework-render-entry for the render call and requirement:router-type-independence for the router parameter
    source: decision:framework-integration-seams
route_table_purpose:
  consumer: a downstream framework, per decision:route-feature-ownership
  supplies: what the filesystem knows, meaning patterns and which segments are dynamic
  omits: what only the application knows, meaning the concrete values a dynamic segment expands into
compatibility:
  - the application may exclude a route and register its own pattern manually
  - existing manually registered handler discovery remains separate from route mode
  - existing flat template mode is unaffected
acceptance:
  - a directory holding only page.tb.html adds a working route with no Go written anywhere
  - a page component receives typed path and query values without reading http.Request directly
  - one explicit registration call installs all valid routes
  - moving a page between rungs changes no other route
  - a rung 3 handler is testable with httptest without any registration
  - route conflicts surface at startup rather than at first request
open_questions:
  - route-level middleware declaration, given that the application already wraps the mux
  - generated update endpoint paths and media types
  - whether the composer is one function per route or one shared function taking the route plan
  - default error page ownership
```

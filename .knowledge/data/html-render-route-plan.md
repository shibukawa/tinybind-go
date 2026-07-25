---
id: data:html-render-route-plan
type: data
title: HTML Render Route Plan
---
Generated typed execution plan connecting one URL pattern to layouts, page, parameters, and reuse identities.

```yaml
source: concept:filesystem-html-routing
fields:
  route_id: stable identity derived from configured route root and normalized relative path
  method: GET by default; exact route API remains configurable
  pattern: static and named dynamic segments
  document: decision:html-document-shell and data:html-client-bootstrap plan for full responses
  path_parameters: ordered requirement:typed-html-route-parameters descriptors
  handler: generated binding and rendering handler registered by api:register-generated-html-routes
  dependencies: route-scoped data:html-route-dependencies groups used by page and layouts
  layouts:
    - component identity
    - scoped path-parameter inputs
    - slot contract
    - requirement:layout-reuse-boundaries identity
  page:
    component: generated page renderer
    path_parameters: all dynamic ancestors
    search_parameters: optional typed page input
selection:
  default: requirement:layout-chain-discovery convention chain
  page_override: auto, none, or explicit compatible chain
  runtime_override: optional compatible chain for advanced applications
validation:
  - unique normalized route and generated symbols
  - every supplied parameter exists and matches its declaration type
  - every layout owns exactly one unnamed requirement:html-slot-syntax slot declaration site
  - explicit chain is acyclic and ordered outermost to innermost
  - dependency groups and generated handler symbols are collision-free
```

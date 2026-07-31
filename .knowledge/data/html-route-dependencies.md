---
id: data:html-route-dependencies
type: data
title: HTML Route Dependencies
---
Generated typed provider surface for external functions used by one filesystem route tree.

```yaml
source: decision:html-route-go-package-model
scope:
  route_mode: optional; requirement:colocated-route-logic is the primary way a route obtains data, and a colocated loader needs no entry here
  still_used_for: template-declared external functions inside a route template
  flat_mode: unchanged and unaffected
generation:
  collect: every synchronous and requirement:async-external-functions declaration reachable from document, layouts, and pages
  group: stable route-relative template module identity
  validate: function name, typed parameters, result, error, and async context contract
conceptual_shape:
  root: Dependencies value passed once during api:register-generated-html-routes
  group: one named interface or provider struct per route template module
  member: statically typed method or function field per external declaration
runtime:
  - generated component calls the injected group member directly
  - no global mutable registry, reflection, service locator, or per-request symbol lookup
  - one immutable dependency value may be shared safely by all generated handlers
context:
  - request-scoped context is passed to operations that declare or require it
  - cancellation reaches async and synchronous I/O implementations
naming:
  - two folders may declare the same local external name without Go symbol collision
  - diagnostics show template-relative path and local external name
missing_dependency:
  generation: declaration without generated binding shape is a generator error
  startup: nil or absent required provider is rejected by registration before serving
```

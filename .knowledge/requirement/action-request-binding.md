---
id: requirement:action-request-binding
type: requirement
title: Action Request Binding
---
Make api:bind work inside a page or a server action by reporting the packages of a route tree, so a generation run covers them.

```yaml
priority: should
status: implemented 2026-07-30
source:
  - requirement:template-server-functions binder_gap
  - downstream framework integration request 2026-07-30
problem:
  reported_cause: binder generation is driven by user-written route registrations, and a server function is registered by generated code, so its request model is never discovered
  actual_cause: rule:request-model-discovery reads every api:bind call site of the package it analyzes and consults no registration at all; nothing filtered the handler out
  real_gap: generation is per package, and no run analyzed a route package, so the binder that api:bind dispatches through was never emitted
  found: 2026-07-30, by generating a binder for an unregistered handler and watching it succeed
implemented:
  package_list: the tree reports its Go packages, being the route root, every route directory, and every layout directory, root first and then by directory
  caller_loop: a caller runs the binder generator over that list, which is the same per-package generation the flat path already uses
  ordering: after the tree's own generated files are written, because analysis type-checks each package
  empty_case: a package with nothing to bind emits no binder and says so, rather than writing an empty file
  no_new_analysis: the api:bind call site detection is unchanged; only the set of packages a run covers grew
openapi:
  unchanged: neither an action endpoint nor a page route enters an OpenAPI document
  now_a_filter: rule:generated-source-not-discovered, keyed on the generated header
  why_needed: the generated registry is itself a discoverable registration site, so covering the route root would have documented every page and endpoint; the side effect the exclusion leaned on did not exist
unlocks:
  field_checking: the form control check of requirement:template-server-functions recovers its input type from the api:bind call inside the handler, which needs this discovery to run
acceptance:
  - httpbind.Bind[T] inside a server action returns a decoded T at runtime
  - a check tag on that request rejects an invalid submission before the handler writes
  - a generated OpenAPI document over the route root contains no page route and no action endpoint
  - a package with no bound request needs no binder
related:
  - requirement:template-server-functions
  - rule:request-model-discovery
  - rule:generated-source-not-discovered
  - concept:handler-discovery
  - concept:openapi-generation
  - decision:framework-integration-seams
```

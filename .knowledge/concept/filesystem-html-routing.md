---
id: concept:filesystem-html-routing
type: concept
title: Filesystem HTML Routing
---
Opt-in route tree that discovers pages and layouts from the filesystem and generates the router around hand-written handlers.

```yaml
evidence:
  source: user design discussion
  received: 2026-07-23
  extended: user app-router discussion 2026-07-27
review_gate: proposed requirements require user approval
scope: decision:route-feature-ownership
convention: decision:html-route-file-conventions
segment_notation: decision:route-segment-notation
handler_shape: decision:route-handler-shape
route_plan: data:html-render-route-plan
requirements:
  - requirement:generated-route-registration
  - requirement:colocated-route-logic
  - requirement:template-server-functions
  - requirement:redirect-error
  - requirement:typed-html-route-parameters
  - requirement:layout-chain-discovery
  - requirement:nested-layout-composition
  - requirement:layout-reuse-boundaries
  - requirement:html-runtime-bootstrap
compatibility: requirement:html-rendering-compatibility
go_package: decision:html-route-go-package-model
platform_constraint: rule:go-safe-route-directory
principles:
  - generate route structure and typed bindings; never parse templates at request time
  - remove the plumbing, so an author writes the data a page needs and nothing else
  - offer a ladder rather than one shape, so a page starts as a template alone and takes on Go only when it needs to
  - keep an ordinary net/http handler at the top of that ladder, so an unusual response never needs a framework feature
  - let a form name a Go function instead of a URL string, so the compiler checks what a hand-written action never could
  - derive names mechanically from the HTTP method, so nothing about the generated surface has to be looked up
  - keep route logic ordinary compiled Go, so every existing Go tool applies to it unchanged
  - scope layout inputs so unaffected ancestor wrappers remain reusable
  - keep document bootstrap ownership separate from the root page
  - stop at discovery, naming, registration, and composition; page metadata and site-wide files belong to a downstream framework
```

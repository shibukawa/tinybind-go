---
id: concept:html-render-runtime-extensions
type: concept
title: HTML Render Runtime Extensions
---
Post-v1 additions for runtime HTML composition, reusable component output, progressive asynchronous rendering, and client-driven partial updates.

```yaml
evidence:
  source: user design discussion
  received: 2026-07-22
review_gate: proposed requirements require user approval
baseline:
  - requirement:html-template-v1
  - requirement:template-code-generation
compatibility: requirement:html-rendering-compatibility
extensions:
  - requirement:nested-layout-composition
  - requirement:component-output-cache
  - requirement:async-external-functions
  - requirement:suspense-html-streaming
  - requirement:chain-render-pipeline
  - requirement:head-merging
  - requirement:scoped-component-style
  - requirement:static-asset-extraction
  - requirement:cross-template-components
  - requirement:partial-update-boundaries
  - requirement:component-delta-rendering
  - requirement:boundary-parameter-updates
runtime_flow: flow:suspense-html-render
partial_update_flow: flow:html-partial-update
boundary_update_flow: flow:boundary-parameter-update
component_analysis: decision:component-capability-lowering
route_generation: concept:filesystem-html-routing
scope:
  - preserve generated, statically checked rendering
  - add request-time composition without runtime template parsing
  - preserve HTML context safety across deferred and cached output
  - avoid sending unchanged update-boundary HTML after search parameter changes
  - rerender one explicit boundary after a declared client parameter changes
  - generate typed pages and reusable layout chains from an opt-in route tree
milestone: follows requirement:template-v1-scope; async remains outside v1
```

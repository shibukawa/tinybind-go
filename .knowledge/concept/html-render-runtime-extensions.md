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
  - requirement:awaitable-parameters
  - requirement:suspense-html-streaming
  - requirement:chain-render-pipeline
  - requirement:head-merging
  - requirement:scoped-component-style
  - requirement:static-asset-extraction
  - requirement:cross-template-components
  - requirement:partial-update-boundaries
  - requirement:component-delta-rendering
  - requirement:render-mode-negotiation
  - requirement:streaming-delta-response
  - requirement:client-navigation
  - requirement:delta-head-sync
  - requirement:component-redraw-endpoint
  - requirement:live-reconnect
  - requirement:action-response-update
two_mechanisms:
  navigation_delta: the server discovers what changed by re-executing and comparing, for URL-driven page updates
  component_redraw: the browser names one instance and supplies its inputs, for deliberate region reloads
  distinction: the first needs identity derivation and validators; the second needs neither
  action_response: requirement:action-response-update reuses the second shape from a mutating endpoint, so acting and refreshing cost one round trip
  - requirement:fragment-capability-introspection
  - requirement:client-managed-head
runtime_flow: flow:suspense-html-render
declaration_syntax:
  annotations: decision:template-annotation-syntax
  cache: decision:cache-component-declaration
  await: decision:async-boundary-syntax
cache_store: api:cache-store
partial_update_flow: flow:html-partial-update
client_apis:
  - api:client-component-update
  - api:client-navigate
component_analysis: decision:component-capability-lowering
dom_application: decision:dom-application-strategy
client_runtime: decision:client-runtime-ownership
route_generation: concept:filesystem-html-routing
follow_on:
  live_updates: concept:live-boundary-updates extends the one-shot await boundary into a repeatedly re-rendered one, and depends on both the async and partial-update halves of this concept
scope:
  - preserve generated, statically checked rendering
  - add request-time composition without runtime template parsing
  - preserve HTML context safety across deferred and cached output
  - avoid sending unchanged update-boundary HTML after search parameter changes
  - rerender one explicit boundary after a declared client parameter changes
  - serve document, navigation-delta, and boundary-delta renders from one route without changing the URL
  - apply each delta boundary as soon as it renders, reusing the async completion stream
  - generate typed pages and reusable layout chains from an opt-in route tree
milestone: follows requirement:template-v1-scope; async remains outside v1
rollout: requirement:client-update-rollout sequences the client update extensions
```

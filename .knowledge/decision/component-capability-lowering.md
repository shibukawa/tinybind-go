---
id: decision:component-capability-lowering
type: decision
title: Component Capability Lowering
---
Analyze independent component capabilities and assemble specialized render logic without combinatorial component implementations.

```yaml
source:
  - concept:html-render-runtime-extensions
  - user architecture discussion 2026-07-22
input: typed HTML AST and component call graph
analysis:
  - collect explicit and locally inferred data:component-render-capabilities
  - propagate async pending effects through calls until a decision:async-boundary-syntax await clause consumes them
  - validate dominance and cross-feature constraints through rule:component-capability-combinations
  - build one ordered lowering plan per component
  - leave slot continuation async classification to requirement:chain-render-pipeline instead of fixing it in the slot owner
logical_lowering:
  - base typed streaming renderer and context-safe writes
  - slot continuation invocation for slot-capable components
  - automatic requirement:layout-reuse-boundaries frame and child-slot validators for route layouts
  - async task scheduling at external call sites
  - await fallback, success, recover, and completion coordinator where pending effects are consumed
  - data:async-boundary-content yield sites where an await boundary settles
  - cache lookup, isolated render, successful publication, and replay for eligible settled regions
  - partial-update markers, validators, and redraw registration metadata
  - serialized response writes and optional encoding flush
generation:
  - emit only handlers required by the capability set
  - select the render signature through decision:async-component-signature
  - lower each component into a decision:generated-render-plan value consumed by the shared coordinator
  - allow implementation fusion while preserving logical ordering and failure semantics
  - retain requirement:html-rendering-compatibility for components with the baseline capability set
diagnostics:
  - report the originating declaration or call chain for incompatible propagated effects
  - do not defer known invalid combinations to request time
```

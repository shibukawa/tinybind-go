---
id: requirement:async-external-functions
type: requirement
title: Async External Functions
---
Allow an explicitly async external function to execute concurrently during generated HTML rendering.

```yaml
source: concept:html-render-runtime-extensions
baseline: typed external functions from requirement:template-language-core
declaration:
  activation: explicit async flag on an external declaration
  signature: statically typed; exact source syntax and Go mapping unresolved
execution:
  - invoke generated adapter in a goroutine with request cancellation context
  - start each invocation once and expose a typed pending result to its dependent subtree
  - send completion to one response coordinator; goroutines never write the response directly
  - wait for or cancel all request-owned work before the decision:async-component-signature sequence ends
usage:
  - a component subtree that reads a pending value must be enclosed by a decision:async-boundary-syntax await clause
  - use outside an await boundary is a generation error
failure:
  - normalize returned error, adapter panic, and configured timeout as data:async-render-error
  - route failure to the nearest decision:async-boundary-syntax recover clause
  - stop work on request cancellation when the Go implementation honors context
  - never replace fallback with partial or context-unsafe HTML
compatibility: synchronous external declarations retain existing behavior through requirement:html-rendering-compatibility
open_questions:
  - async flag placement and whether async externals require context.Context
  - exact result and error signature mapping to data:async-render-error
  - concurrency limit, timeout, and nested dependency scheduling
```

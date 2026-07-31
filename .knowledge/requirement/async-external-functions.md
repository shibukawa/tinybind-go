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
  shape: `external async LoadUser(id: string): User`
  placement: `async` follows the `external` keyword instead of becoming a decision:template-annotation-syntax annotation, because it changes the required Go signature rather than annotating behavior
  ambiguity: none, because an external name must be PascalCase
go_signature:
  async: func Name(args...) (Result, error)
  sync: func Name(args...) Result, unchanged
  shape: an ordinary blocking Go function; the only difference from a sync external is the error result a boundary can recover from
  optional_context:
    decision: an implementation declaring a leading context.Context receives the boundary context, approved 2026-07-26
    detection: generation reads the package's Go sources and passes the context to the functions that accept one
    syntactic: the check is on the parsed parameter list, because it runs before the package compiles; an unparsable file is skipped and a mismatched call shape stays an ordinary Go compile error
    template_surface: unchanged either way, so the choice belongs to whoever writes the implementation, function by function
    reason: application code stays a plain function and the runtime owns the concurrency, the way a caller promisifies a blocking call, while a call that can genuinely abort still gets what it needs
usage:
  - an async external may be called only in a decision:async-boundary-syntax await clause header
  - any other call site is a generation error naming the function and the position
  - the bound result is an ordinary typed value in the primary subtree
  - proposed requirement:awaitable-parameters adds a second source for the same binding form, where the caller starts the work and passes the pending result in
execution:
  - the runtime runs each binding of one await clause in its own goroutine and joins the results
  - two slow bindings of one clause cost the slower one rather than their sum
  - each invocation starts once per boundary instance
  - each binding writes only its own field of the generated boundary scope, so bindings share no memory and need no lock
  - completion goes to one response coordinator; adapter goroutines never write the response
cancellation:
  bounds: always the wait; the work only when the implementation took a context
  request_cancelled: the runtime stops waiting and the boundary emits nothing
  timeout: failure rather than cancellation, so the boundary renders recover with the timeout code, or surfaces the unrecovered failure when the clause omits recover
  context_taking: sees the cancellation and may return early
  plain: cannot be interrupted, so it is abandoned; it finishes on its own and its result is discarded
  scope_safety: an abandoned binding is still writing the boundary scope, so a failed or cancelled wait discards that scope instead of returning it
failure:
  - normalize returned error, adapter panic, and configured timeout as data:async-render-error
  - the first failing binding in declaration order decides the boundary, so two failing bindings fail the same way on every run
  - route failure to that clause's recover subtree, or out of the boundary to the caller when the clause omits recover, per decision:async-boundary-syntax omitted_recover
  - stop work on request cancellation, which produces no recover output
  - never replace fallback with partial or context-unsafe HTML
compatibility: synchronous external declarations retain existing behavior through requirement:html-rendering-compatibility
open_questions:
  - per-boundary concurrency limit defaults
  - whether a declared error classifier should replace the default data:async-render-error normalization
```

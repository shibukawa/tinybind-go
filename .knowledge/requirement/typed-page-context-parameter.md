---
id: requirement:typed-page-context-parameter
type: requirement
title: Typed Page Context Parameter
---
Let a typed page entry point declare a leading context.Context, excluded from the URL input list, so a page needing a request-scoped value keeps the rung whose results are checked.

```yaml
priority: should
source:
  - downstream framework request-context report 2026-08-05
  - requirement:colocated-route-logic
  - decision:route-handler-shape
review_gate: proposed
status: shipped 2026-08-05, as the reporter proposed it; the trim, the leading-only rule, and the unchanged diagnostic are all implemented
answers: the open question both requirement:colocated-route-logic and decision:route-handler-shape have carried since 2026-07-27, whether rung 2 may take a leading context before the path parameters
problem:
  today: routetree Validate checks every declared parameter with bindableType, so a leading context.Context is a generation error naming it a query parameter
  diagnostic_is_right: the message states the rule it enforces, and the rule is right about URL inputs
  gap: a context is not a URL input; it arrives from the request rather than from the address, which is why it cannot be spelled
  escape: the page drops to rung 3 and gives up the typed rung entirely
check_asymmetry:
  rung_2: generation compares the function's result list against the page component's parameter list and fails naming both
  rung_3: nothing is compared, because the response is the caller's
  consequence: the page that most needs the check, one assembling several values for a template, is the one that cannot have it, only because it also needs a pool
  weight: this is the argument for the requirement; the rung 3 workaround works, and what it costs is the check
shape:
  go: 'func Load(ctx context.Context, path..., query...) (values..., error)'
  position: leading only, matching Go's own convention and keeping the route-order rule for everything after it
  rung_3: unchanged and unaffected, because a handler reads the request's context already
proposed_implementation:
  flag: PageFunc gains a TakesContext flag, set when the first parameter is the file's spelling of context.Context
  detection: syntactic, in the style routetree isHandlerSignature already uses; it resolves net/http from the file's imports, and importName is already there to do the same for context
  trim: InspectLogic removes that parameter from PageFunc.Params
  validate: unchanged, because the route-order match and the bindable check keep reading the list they already read
  binding: routetree pageBinding prefixes its argument list with the request context; the generated handler holds the request, so the call site reads it from there
  preferred: the trim rather than an offset, because it keeps "Params are the URL inputs, in route order" true rather than making every reader of that code carry an offset
as_built:
  followed: the reporter's shape exactly, including the trim over the offset
  trim_position: after the result-shape check rather than before it, so a near-miss still reports the signature as written; trimming first would have described a parameter list the author did not type
  alias: the import name is resolved from the file, so a renamed context import is recognised; this matches the handler-shape check in the same file
  no_import_no_context: a file that does not import context cannot be declaring one, so the missing import answers no on its own
  scan_aligned: the external scan read the qualifier literally and now resolves the import the same way, so one rule decides in both places rather than an aliased import working for an entry point and not for an external beside it
not_asked:
  request_object: an http.Request in a typed entry point pulls the transport into a signature whose point is that it has none
  context_contents: pools, sessions, and tracers belong to the application; the module passes the value through and does not look inside it
  handler_rung: it stays the right escape hatch for a page that owns its response
  url_rule: every remaining parameter stays a bindable scalar, and the current diagnostic for one that is not stays exactly as it is
  non_leading_context: a context anywhere but first keeps the existing error, which falls out of trimming only the first parameter
relation_to_route_package_context_externals:
  order: requirement:route-package-context-externals is the cheaper fix and reaches further, because it makes one already-shipping behavior consistent across both compile paths
  effect: with it, a page reaches the context through a synchronous external and no new concept, which makes this requirement a convenience rather than the only typed way
  still_wanted: an external's result lands in template scope, while a typed entry point keeps the value in Go and has its results checked, so the two are not substitutes
constraints:
  - routetree gains no net/http or context dependency at generation time; the check reads parsed Go source
  - an entry point declaring no context generates exactly the bytes it generates today
  - rung selection is unchanged, because the handler-shape check still runs first
acceptance:
  - a typed entry point taking only a context, on a route with no dynamic segment, generates and builds
  - the context is counted as neither a path nor a query parameter, in any diagnostic or in the generated decoder
  - a context in a non-leading position keeps the current error
  - an entry point declaring no context regenerates unchanged
  - a rung 3 handler is unaffected
related:
  - requirement:typed-html-route-parameters
  - requirement:optional-query-parameter
  - requirement:generated-route-registration
open_questions:
  - whether a layout's typed entry point, if it gains one, takes the same leading context
  - whether the rung 1 template-only path can reach the context, having no Go function to declare it, or whether requirement:route-package-context-externals is its only route
```

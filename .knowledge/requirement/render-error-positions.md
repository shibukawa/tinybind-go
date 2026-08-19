---
id: requirement:render-error-positions
type: requirement
title: Template Positions On HTML Render Errors
---
Report the template position of the instruction whose execution failed, so an HTML render error names a template line the way a compile error does.

```yaml
priority: could
status: not implemented 2026-08-19; the parallel-list shape the request proposed does not reach the ops that need it, and which alternative to take is an open cost decision rather than a coding one
source:
  - downstream Popcorn Wave request 2026-08-19
  - upstream feasibility review 2026-08-19
problem:
  gap: requirement:template-source-positions maps compile-time positions, but htmlbind renders through decision:generated-render-plan, so a failing render's stack frame is in the shared coordinator and a line directive on the plan literal cannot reach it
  asymmetry: requirement:template-source-positions shipped, so sqlbind is repaired to the stack frame and htmlbind only to the compiler; that difference is now real and a downstream has to document it rather than hide it
  today: execOps returns an op error unchanged; nothing in the render path adds position, component, or instruction context
requested_shape:
  what: an exported OpSources slice on Plan, same length and order as Ops, holding one position per instruction
  precedent_claimed: requirement:head-contribution-provenance HeadSources, a parallel list beside Head
  renderer_change: wrap the error an op returns with its indexed position
blocking_defect:
  finding: Ops is only the top-level instruction list; a For body and an If branch are bare Op slices parameterized by their own scope type, reached through the For, ForCtx, and If constructors rather than through a Plan
  consequence: a list parallel to Plan.Ops covers the outermost instructions and misses every nested one, and a loop body is where a nil dereference or a failing binding is most likely
  why_the_precedent_does_not_transfer: Head is flat, so a parallel list describes all of it; Ops is a tree, so a parallel list describes only its root
  also: execOps ranges without an index and is called with nested lists, so even the top-level pairing needs the index threaded rather than read
alternatives:
  position_on_the_op:
    what: each op constructor takes its position and stores it, so nesting is free
    cost: a field on every op including the Static runs that dominate an instruction list, against requirement:tinygo-wasm size
    interface: reading it back needs a position accessor, which is the Op interface change the request set out to avoid
  paired_nested_lists:
    what: For, ForCtx, and If take positions beside their body slices
    cost: changes exported constructor signatures and the generated call shape, so every fixture is rewritten
  decline:
    what: specify that an HTML render error carries no template position
    consequence: the downstream writes that as a documented limit, and requirement:template-source-positions still delivers the compile-time half
constraints_any_shape_must_hold:
  presentation_safety: a template path is server-side detail; AsyncError carries only renderable fields and the Go error reaches the caller through WithErrorReporter, so a position follows the reporter path and never the recover clause
  no_fmt: the runtime wraps errors through its own wrappedError rather than fmt.Errorf, to keep the formatter out of a TinyGo build; a position wrapper does the same
  panic: nothing recovers a panic in the render path, so this requirement covers a returned error only; a panic keeps naming the coordinator whatever is decided here
  plan_immutability: a plan is built once at package initialization and shared across requests, so per-render cost is carrying an index and nothing more
acceptance:
  - an error returned by an instruction reports the template path and line of the instruction that returned it
  - an instruction inside a For body or an If branch reports its own position, not the position of the enclosing instruction
  - a rendered page never contains the position; only the error reporter sees it
  - a component whose render succeeds pays no allocation for position carrying
open_questions:
  - whether the asymmetry alone justifies the cost, given requirement:template-source-positions already repairs the compile-time half that produces most of the confusion
  - whether the position belongs on the op or beside the list, which is the binary-size question requirement:tinygo-wasm decides
```

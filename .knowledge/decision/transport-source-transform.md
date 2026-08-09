---
id: decision:transport-source-transform
type: decision
title: net/http Is Source, fasthttp Is Generated
---
Treat net/http handler source as the single authored form and generate the fasthttp handler from it, admitting a handler to the transform only when every occurrence of the writer and the request is one the rewriter recognizes.

```yaml
status: proposed 2026-08-08
decided_by: user, 2026-08-08
direction: net/http source -> generated fasthttp; never the reverse
arity_collapse:
  question: every existing emitter and analyzer assumes a writer and a request as two values, and fasthttp carries both in one
  answer_for_handlers: the transform rewrites both w and r to the same ctx, so arity collapses as a consequence of substitution rather than as a modelled change
  example:
    from: "httpbind.Write[T](w, r, out)"
    to: "fasthttpbind.Write[T](ctx, out)"
  answer_for_emitters: only the printed signature changes, which is naming, and routetree Symbols already parameterizes that half
  residual: a runtime entry point taking neither w nor r has nothing to collapse, so the arity question is confined to the emitted signature
eligibility_is_a_whitelist:
  test: every occurrence of the w and r identifiers in the handler body is one of the recognized forms below
  significance: this is decidable and conservative, unlike asking whether a handler semantically depends on net/http; an unrecognized occurrence is not analyzed, it is refused
  supersedes_objection: decision:transport-neutral-handler argues that portability cannot be inferred, which holds against a blacklist over behaviour and does not hold against a whitelist over occurrences
recognized_occurrences:
  runtime_call_argument: the value is an argument to a call the generator already knows, from the same registry parser uses for Bind, Write, WriteStatus, WriteError, and the stream entry
  framework_call_argument: the same, for calls a framework registered through the generator call patterns
  named_selector_rewrites:
    principle: a small, explicitly enumerated set, each with a written rewrite; never a general rule about methods
    example: "r.Context() -> ctx, valid because RequestCtx satisfies context.Context"
    constraint: each addition is a named entry, so the set is auditable and its growth is visible
refused_occurrences:
  - the value passed to a function the generator does not recognize, which is the common case for tracing, metrics, and session libraries
  - the value assigned to a variable, stored in a struct, or captured by a closure
  - the address of the value taken
  - any selector not in the named set
  - a handler whose body the parser cannot see, including one reached through a package selector
  outcome: the handler is ineligible and is registered through decision:fasthttpbind-adapter-boundary
transitivity_settled_2026_08_08:
  case: a same-package helper such as func renderError(w http.ResponseWriter, r *http.Request, err error), called from an otherwise eligible handler
  resolution: transform the helper too; the admission set closes over the same-package call graph rather than running per handler
  forced_by: decision:backend-build-tag-mode, whose refusal is a build error rather than a fallback, so one refused shared helper would make the application unbuildable
  cost: a fixpoint over the call graph, and refusals that must propagate to callers already admitted
  specified_by: rule:transform-eligibility
refusal_outcome_2026_08_08: a generation error naming the declaration, not an adapter registration; decision:fasthttpbind-adapter-boundary is not implemented
naming_2026_08_08: the rewritten calls keep the net/http names, because the tag keeps the two file sets from ever compiling together, so only argument lists move
generated_body_is_a_copy:
  fact: the emitted fasthttp handler carries a rewritten copy of the authored statements
  consequences:
    - panics and profiles point at generated source, not at the authored file
    - the emitter must print arbitrary Go, so it needs an AST-in, AST-out path; the binder and writer emitter builds strings today, while templates and artifacts already use format.Node
    - imports of the generated file are derived from the rewritten body, not from the source file's import block
streaming_precondition:
  requires: decision:stream-callback-shape
  why: a stream value the handler holds across statements is a value the rewriter would have to track through the body, while a callback keeps every use inside one expression
middleware:
  framework_owned: both backends supplied, with composition order verified identical across the pair
  application_owned: subject to the same eligibility test as a handler
related:
  - decision:backend-build-tag-mode
  - decision:fasthttpbind-adapter-boundary
  - decision:fasthttpbind-no-transport-interface
  - concept:handler-discovery
  - concept:handler-forms
  - requirement:transport-port-surface
```

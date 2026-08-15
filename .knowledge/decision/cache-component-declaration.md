---
id: decision:cache-component-declaration
type: decision
title: Cache Component Declaration
---
Enable requirement:component-output-cache with a `@cache(ttl: "...")` annotation on a component whose whole output is a pure function of its declared parameters.

```yaml
source:
  - requirement:component-output-cache
  - user syntax decision 2026-07-26
review_gate: approved 2026-07-26
syntax: decision:template-annotation-syntax
shape: |
  @cache(ttl: "5m")
  export component Sidebar(userId: string): html { ... }
options:
  ttl:
    required: for storage; writing one is what asks for it, and decision:cache-scope-declaration modes makes an annotation without one a scope declaration that stores nothing
    forbidden: on a slot owner or shell, which cannot store, so a duration there describes an expiry that cannot happen
    form: Go duration string parsed at generation time
    invalid: unparsable or non-positive duration is a generation error
  scope: decision:cache-scope-declaration; private or public, defaulting to private on a storing component
  vary:
    settled: 2026-08-09; not unspecified after all
    what_was_already_true: a component reaching a builtin element backed by a provider is refused, over the call graph, at the declaration position
    so: a component whose output depends on a request property is ineligible rather than mis-keyed, which is the stronger of the two forms decision:cache-scope-seams offered
    remaining: an element declaring Vary with no Provider is a registration question, not a cache one
  future: eviction and stale-while-revalidate stay unspecified rather than reserved
eligibility:
  applies_to: the storing mode only; decision:cache-scope-declaration eligibility_is_about_storage gives the reason each condition below has none once nothing is stored
  single_root:
    added: 2026-08-08 by the owner, for requirement:boundary-decomposed-render
    rule: a cached component renders exactly one root element, as decision:update-manifest-transport already requires of an update boundary
    why: a decomposition hole needs an element to hold the place, and an id on a stable root is what lets a parent fragment carry a placeholder for it
    not_needed_before: the byte cache replays a range and never addressed it, so root count did not matter; it matters the moment the component becomes a hole
    breaking: a cached component rendering several roots stops generating, which is the cost of joining the boundary set
  no_nested_boundary:
    decided: 2026-08-08; a cached component may not contain a nested reloadable component, and declaring one is a generation error
    follows_from: requirement:component-output-cache opaque_unit, which makes a cached output one unit with nothing reported inside it
    what_a_nested_boundary_would_force: its own fragment and id, and a hole in the cached output, which is structure the stored range cannot express — and it is the only thing that would have forced api:cache-store to carry structure
    same_shape_as_await: rejected at generation with the declaration position, for the reason no_await gives; the alternative is caching a form the consumer cannot use
    cost: a cached sidebar holding a reloadable widget has to give up the cache or move the widget out, which is the same trade await already carries
  no_html_parameters: an html parameter is a bound continuation, not a value, so it cannot enter decision:cache-key-derivation; declaring one is a generation error
  no_slots:
    rule: follows from the parameter rule, so a component that stores is never a requirement:chain-render-pipeline member
    narrowed: 2026-08-09; it was read as barring `@cache` from a layout outright, which decision:cache-scope-seams found is what made the round's own chain-union example unwritable
  no_await:
    was: the component and every component reachable from it must be free of decision:async-boundary-syntax boundaries
    narrowing_proposed: 2026-08-14, to a live boundary alone, per requirement:cached-settled-boundary; an await boundary settles once so a settled form exists to store, and a live one never settles
  no_shell: a cached component cannot own the document head, because requirement:head-merging output depends on the chain rather than on parameters
await_rationale:
  problem: a boundary emits a placeholder now and a completion later, so its output is not one byte range that can be stored and replayed
  v1: reject at generation time with the declaration position, instead of silently caching only the initial pass
  future: caching a fully settled boundary set requires storing completions too, and was deferred with requirement:suspense-html-streaming
  taken_up: requirement:cached-settled-boundary, 2026-08-14, which keeps the reason for a live boundary and drops it for an await one
  what_the_problem_statement_missed: the settled contiguous form already exists at runtime, because awaitOp falls back to a blocking in-place render when no coordinator is present and a cached subtree is rendered without one; the placeholder is what a miss emits, not what a cached subtree emits
  what_remains_hard: delivering the miss with its fallback while still storing the settled form, which is decision:cached-boundary-delivery
execution:
  hit: write stored bytes into the current stream; the component body does not run
  miss: render the subtree into an isolated buffer, then publish and write it
  error: a failed render publishes nothing and propagates; partial output is never stored
  no_store: with no api:cache-store supplied, a cached component renders normally with no key computed
acceptance:
  - two renders with equal parameters inside the TTL run the body once
  - a changed parameter, a changed plan, or an expired entry re-runs the body
  - an html parameter, an await boundary, or a document head in a storing component fails generation
  - a ttl written on a layout fails generation
  - an annotation carrying neither ttl nor scope fails generation
  - an annotation with no ttl stores nothing and computes no key, wherever it sits
```

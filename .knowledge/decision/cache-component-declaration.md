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
    required: true
    form: Go duration string parsed at generation time
    invalid: unparsable or non-positive duration is a generation error
  future: eviction, vary, and stale-while-revalidate stay unspecified rather than reserved
eligibility:
  no_html_parameters: an html parameter is a bound continuation, not a value, so it cannot enter decision:cache-key-derivation; declaring one is a generation error
  no_slots: follows from the parameter rule, so a cached component is never a requirement:chain-render-pipeline member
  no_await: the component and every component reachable from it must be free of decision:async-boundary-syntax boundaries
  no_shell: a cached component cannot own the document head, because requirement:head-merging output depends on the chain rather than on parameters
await_rationale:
  problem: a boundary emits a placeholder now and a completion later, so its output is not one byte range that can be stored and replayed
  v1: reject at generation time with the declaration position, instead of silently caching only the initial pass
  future: caching a fully settled boundary set requires storing completions too, and is deferred with requirement:suspense-html-streaming
execution:
  hit: write stored bytes into the current stream; the component body does not run
  miss: render the subtree into an isolated buffer, then publish and write it
  error: a failed render publishes nothing and propagates; partial output is never stored
  no_store: with no api:cache-store supplied, a cached component renders normally with no key computed
acceptance:
  - two renders with equal parameters inside the TTL run the body once
  - a changed parameter, a changed plan, or an expired entry re-runs the body
  - an html parameter, an await boundary, or a document head in a cached component fails generation
```

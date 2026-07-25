---
id: requirement:cross-template-components
type: requirement
title: Cross Template Component Use
---
Use a component declared in another template file, and fill a slot with a subtree the declaring file cannot see.

```yaml
source:
  - requirement:html-slot-syntax
  - user composition discussion 2026-07-25
problem:
  same_file: a slot filled by a call in the same file resolves statically today
  cross_file: a document shell cannot name the page that fills its body, and shared components live outside the caller
mechanisms:
  named_reference:
    shape: external component declaration naming the component, its typed parameters, its slots, and its html output
    scope: requirement:template-file-scope; the target must be exported from its own file
    model: mirrors the requirement:template-language-core external function declaration and reuses the external keyword
    resolution: generation time; the generator binds the declaration to the real generated function
    call_site: ordinary uppercase component call with named arguments and requirement:html-slot-syntax fill blocks
    slots:
      declaration: slots appear as html-typed parameters, so no separate slot clause is needed
      unnamed: the reserved children parameter
      requiredness: an optional html parameter matches an optional slot; a plain html parameter matches a required one
      usage: identical to a same-file component call, including fill blocks and default content behavior
    inspiration: importing a component the way a React or Vue module import does
    use_for: shared components extracted for reuse across template files
  go_continuation:
    shape: a slot parameter is a decision:generated-render-plan component value binding a plan to its params
    fill: the caller binds the target component with its generated params struct
    use_for: a handler or runtime choosing at call time which subtree fills a slot
    head: the bound value exposes its requirement:head-merging contributions before rendering starts
    ergonomics: generator emits a binder producing that value from a component plus its params
  static_chain:
    shape: requirement:layout-chain-discovery already resolves document, layouts, and page without either mechanism
    use_for: filesystem route mode
rejected:
  runtime_map:
    shape: map from slot name to a render function and an untyped params value
    reasons:
      - untyped params need decision:reflection-free forbidden reflection or unchecked assertions
      - slot names become runtime strings, losing the generation-time undeclared-slot diagnostic
      - data:html-route-dependencies already forbids a service locator or per-request symbol lookup for external dispatch
    note: the bound component value already pairs plan and typed params, which is what the map was reaching for
document_shell:
  model: decision:html-document-shell owns html, head, and body, exposing the body interior as an unnamed slot
  handler_template: a handler-invoked template renders only that interior and never emits the frame
  wiring: filesystem routes use static_chain; a manual handler passes both to api:render-html-chain
constraints:
  - every mechanism resolves component identity and parameter types at generation time
  - no runtime registry, name lookup, or reflection
  - a slot continuation stays lazy per requirement:html-slot-syntax; passing it never renders it
  - an async continuation carries its own sequence, so requirement:chain-render-pipeline still merges completions
acceptance:
  - a shared component in one file is callable from another with full parameter type checking
  - an external component carrying slots is filled exactly like a same-file component
  - a document shell compiles without knowing which page fills its body
  - a handler composes shell and page without naming a slot as a runtime string
  - a missing or type-incompatible external component fails generation with both positions
open_questions:
  - target notation in an external declaration, deferred to requirement:template-file-scope
  - how a generation unit maps to Go packages in decision:html-route-go-package-model
  - whether a binder helper is generated per component or one generic adapter suffices
```

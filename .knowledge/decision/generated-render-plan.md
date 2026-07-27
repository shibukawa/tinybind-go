---
id: decision:generated-render-plan
type: decision
title: Generated Render Plan And Coordinator
---
Emit each component as a typed render plan value consumed by one shared coordinator, instead of a self-contained one-pass render function.

```yaml
source:
  - requirement:template-code-generation
  - user architecture decision 2026-07-25
review_gate: approved 2026-07-25
problem: one function performing a single pass cannot serve slots, await boundaries, cross-file components, and head merging at once
forcing_case: a root head cannot be written until every reachable title, meta, and script contributor is known, so emission needs a phase before body output
forcing_case_note: styles left this set through decision:component-style-delivery, but the remaining contributions still require the phase
plan:
  produced: generation time, once per component
  immutable: shared by every request; never rebuilt per request
  carries:
    - coalesced static byte runs
    - typed steps for expressions, control flow, and component calls
    - requirement:html-slot-syntax insertion points and their bound parameters
    - decision:async-boundary-syntax boundaries with their fallback and recover subplans
    - requirement:head-merging contributions declared by this component, including requirement:static-asset-extraction reference tags
    - stable component version and source positions for diagnostics
  excludes:
    - parsed template text; parsing stays at generation time
    - per-request node trees
coordinator:
  role: one shared runtime walker that turns plan plus typed params into streamed HTML
  location: shared runtime package, not generated per package
  boundary: an HTML runtime leaf under decision:runtime-package-boundaries that excludes net/http, matching the HTTP-independent writer mode
  tinygo: one runtime copy instead of per-package emission also helps requirement:tinygo-wasm size
  imports: generated files reference it like any other runtime package, so rule:generated-source-self-contained is unaffected
  phases: requirement:chain-render-pipeline owns the ordered lifecycle
  entry: api:render-html-chain for manual composition; generated route handlers call the same coordinator
  writes: streams to the io.Writer supplied by requirement:html-component-api rendering
  flush: performed when the writer implements a flush method, since io.Writer alone cannot
  emits: data:async-boundary-content for settled boundaries
representation:
  form: ordered instruction list per component, not nested closures
  typing: the list is parameterized by that component's generated params struct, so steps stay typed and decision:reflection-free holds
  instructions:
    - emit coalesced static bytes
    - evaluate a typed expression into a checked insertion context
    - branch and loop over typed values
    - call a component with bound params
    - insert a requirement:html-slot-syntax slot
    - open a decision:async-boundary-syntax boundary with its fallback and recover subplans
  reason: an explicit list is inspectable, lets the coordinator suspend and resume at a boundary, and keeps phase logic out of generated code
inlining:
  rule: a private component using no phase-dependent capability is expanded into its caller instead of getting its own plan
  phase_dependent: await boundary ownership, slot declaration, partial-update boundary, and component-output cache
  preserved: requirement:head-merging contributions move to the caller contribution set; hoisting is generation time, so inlining does not lose them
  blocked: self-recursive or mutually recursive components keep their own plan
  diagnostics: inlined steps retain their original declaration positions
  benefit: removes plan indirection for the common private helper, offsetting the coordinator cost
component_value:
  shape: a value pairing the plan with its bound params, not a bare render func
  reason: a slot fill must expose its head contributions and async capability before rendering starts
  slot_argument: this same value shape, so requirement:cross-template-components passes composition without a map
  immutable: rendering writes no state back into the value; all per-render state stays in the coordinator
  lifetime: reusable across requests and safe to share concurrently, unlike the single-use sequence
  consequence: a parameterless wrapper such as a document shell can be built once at startup
constraints_preserved:
  no_runtime_parsing: the plan is compiled data, not template text
  no_reflection: every step is statically typed; decision:reflection-free is unchanged
  no_virtual_dom: nothing materializes a whole document; only head metadata is collected before streaming
  static_coalescing: adjacent static output stays merged in the plan, after requirement:static-whitespace-normalization has rewritten it
  streaming: body bytes leave as the coordinator walks, not after a build step
tradeoff:
  cost: one indirection per step versus straight-line generated writes
  gain: slots, async, cross-file reuse, and head merging share one execution model instead of four special cases
  mitigation: static byte runs remain single writes, so the indirection is per step, not per byte
open_questions:
  - measured TinyGo and WASM size and speed against straight-line emission, given generic instruction lists
  - whether the instruction list is a Go slice of a sum-type struct or one interface per instruction kind
  - inlining depth limit and its effect on generated file size
```

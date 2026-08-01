---
id: requirement:fragment-capability-introspection
type: requirement
title: Fragment Capability Introspection
---
Expose a bound component's render capabilities as public read-only data, so a caller can decide once per response whether a client runtime is needed.

```yaml
priority: should
source:
  - concept:html-render-runtime-extensions
  - user lifecycle discussion 2026-07-27
review_gate: approved 2026-07-27
status: shipped; the await-block flag, its two accessors, and the chain helper are implemented
problem:
  hidden_classification: requirement:chain-render-pipeline already classifies a chain as async, but only the runtime sees the result
  hardcoded_consequence: because the caller could not ask, api:render-html-chain injected the update runtime itself; decision:client-runtime-ownership removed that prepend on 2026-07-27
  duplication: a chain whose layout and page both open await boundaries must still produce exactly one runtime script, so the decision belongs to one aggregate rather than to each member
surface:
  value: the decision:async-component-signature bound component value, both Fragment and Wrapper forms
  shape: an exported accessor returning an immutable capability summary
  staging:
    first: the await-block flag alone, because it is what decision:client-runtime-ownership needs to move the streaming script out of the entry points
    later: a partial-update flag for requirement:partial-update-boundaries, once that capability ships
    asset_set: requirement:component-asset-requirements folds a required-asset set through the same shape; it is a set rather than a bool, so it is the first member that is not one accessor per capability
    shape_choice: one bool accessor per capability, decided 2026-07-27; adding a second method later is additive, so no summary struct is needed
  names:
    accessor: HasAwaitBlock on Fragment and on Wrapper
    plan_field: HasAwaitBlock, an exported bool beside Head, Ops, and Cache
    aggregate: package-level HasAwaitBlock over the MergeHead argument pair; a package function and a method may share the name
    vocabulary: await block follows the decision:async-boundary-syntax authoring term rather than the internal boundary word
  meaning: the member, or anything it composes, opens a decision:async-boundary-syntax await block
  parallel: MergeHead is the existing public aggregate over the same argument pair, so this follows a shape the package already has
derivation:
  origin: data:component-render-capabilities, computed by decision:component-capability-lowering at generation time
  storage: a constant on the decision:generated-render-plan value; reading it walks nothing
  transitive: includes capabilities propagated through the component call graph
  runtime_slots: a bound value filling a slot carries its own summary, so a runtime-assembled composition reports correctly
  no_reflection: the summary is generated data, so decision:reflection-free holds
aggregation:
  primary: the caller unions the accessor across the values it holds, which is the whole contract
  helper: an exported function over the MergeHead argument pair, as sugar for the common chain shape
  rule: the aggregate is the union of member flags
  once: the caller decides one time and adds one script, which is what keeps the runtime tag single
  dedup_backstop: requirement:head-merging still collapses identical script nodes, so a caller adding it twice cannot emit two tags
conservatism:
  rule: a member below a conditional slot that never renders still counts toward the aggregate
  reason: the same conservatism requirement:chain-render-pipeline already applies to head contributions, where a dropped member still contributed
  direction: over-inclusion ships an unused script; under-inclusion leaves completions unapplied, so the safe direction is to include
timing:
  available: before rendering starts, because the summary is bound to the value rather than produced by walking
  use: the caller selects requirement:render-time-script-contribution entries before calling the render entry
implementation:
  plan_field: HasAwaitBlock, an exported bool on the Plan struct beside Head, Ops, and Cache, written into the generated plan literal
  generator: the HTML compiler already knows which declarations open a boundary; it must propagate the flag through component calls so a caller of an async component is async too
  bind: Bind copies the flag onto Fragment exactly as it copies Head; BindWrapper does the same for Wrapper
  accessors: exported readers on Fragment and Wrapper, beside the existing Head and Present
  aggregate: an exported function over the same argument pair MergeHead takes, so a caller asks once for a whole chain
  assemble: the internal composed Fragment built while nesting wrappers must carry the aggregated flag, not the zero value
  inlining: decision:generated-render-plan inlines a private component into its caller, so the flag must be raised on the caller when an inlined body awaits
  scope_v1: plan field, Bind and BindWrapper copies, the two accessors, and the chain helper; no slot plumbing, per slot_parameter_propagation
  as_built:
    generator: the emitter reuses the compiler's existing reachesAwait call-graph walk, which already backed the cached-component diagnostic
    output_stability: the field is emitted only when true, so a project with no await boundary regenerates byte-identical Go
    wrapper: Head was added to Wrapper at the same time, since it had no head accessor while Fragment did
slot_parameter_propagation:
  finding: Bind copies only plan.Head, so a Fragment passed as a slot argument inside a params struct is not walked by the binder
  not_required_here:
    reason: the caller holds every Fragment it built, so it unions the flag across the values in its own hand
    covered_statically: a component the template calls is already folded into the plan flag transitively, so only caller-supplied fragments are outside the plan, and those are exactly the ones the caller holds
    conclusion: the flag ships with no slot plumbing; the contract is the accessor on Fragment, and how the value is filled stays an implementation detail
  decoupled_from_head:
    revision: an earlier reading said this must be fixed together with head merging; that is withdrawn
    why: a caller can union a bool across values it holds, but it cannot reconstruct a merged head that way, because MergeHead must produce the merged result itself
    status: the head case remains an open defect against requirement:cross-template-components and is tracked separately
  residual_risk:
    case: a helper builds a Fragment, embeds it in another component's params, and returns only the outer value, so the inner flag never reaches the caller
    likelihood: lower than the ordinary case, because a caller composing slots normally keeps the parts
    remedy: adopt the plan-carried slot accessor below if it bites
  option_if_needed:
    shape: an optional generated accessor field on Plan returning the Fragment-typed params, so Bind can union without reflection
    why_this_one: nil for a component with no slots, so it stays zero cost and adds no interface boxing, which also suits requirement:tinygo-wasm
    alternatives_considered:
      variadic_bind: passing slot fragments as extra Bind arguments; typed and cheap, but a hand-written call can silently omit one
      params_interface: a generated Slots method on the params type; call sites stay unchanged, at the cost of boxing params on every bind
      caller_supplied: making the caller pass extra fragments to the aggregate; pushes the problem outward
      reflection: excluded by decision:reflection-free
constraints:
  - the summary is immutable and safe to read concurrently, like the value carrying it
  - reading it never starts rendering and never consumes the single-use async sequence
  - a component gaining an await boundary changes only the summary, not the type of any call site
acceptance:
  - a component declaring an await boundary reports true
  - a component calling one, without declaring its own, also reports true
  - a component with no boundary anywhere in its call graph reports false
  - a chain whose layout and page both open boundaries reports one aggregate, so the caller emits one runtime script
  - a chain with no boundary reports false and the caller adds no script
  - reading the summary never renders, never consumes the async sequence, and can be repeated
  - the reported classification matches what the coordinator does when the same chain renders
  - an inlined private component that awaits raises the flag on the component that inlined it
related:
  - requirement:chain-render-pipeline
  - requirement:html-component-api
  - data:component-render-capabilities
open_questions:
  - whether the accessor set later grows to cache participation and route role, or stays limited to client-runtime decisions
  - whether the chain helper is exported separately or the chain options compute it internally
```

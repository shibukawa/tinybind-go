---
id: requirement:partial-update-boundaries
type: requirement
title: Explicit Partial Update Boundaries
---
Mark components whose rendered DOM and validators may participate in client-side partial updates.

```yaml
source: concept:html-render-runtime-extensions
declaration:
  activation:
    explicit: update flag on an ordinary component
    automatic: requirement:layout-reuse-boundaries for generated route layouts
    explicit_is_unimplemented:
      state: designed here and never built, so today every boundary is a chain member and a delta's granularity is the page and its layouts
      cost: changing a sort order on a five-hundred-row table transfers the whole page boundary
      guidance_has_no_mechanism: decision:manifest-state-ownership size_mitigations says to place boundaries at meaningful regions rather than per list row, and with no explicit flag there is no way to place one at all
      priority_asked: downstream framework partial transfer report 2026-08-08, which asks for scheduling rather than shape and declines to propose a syntax
      multiplier: requirement:structured-render-output decides what each boundary costs and this decides how many are sent, so the two compound; decision:partial-transfer-seams sequences this ahead of it because it is designed and blocks on nothing
      what_actually_blocks_it:
        not_the_syntax: the recorded open question is the flag's spelling, and that is not what is stopping an implementation
        is: requirement:component-redraw-endpoint shipped a reloadable modifier — exported, single rooted, carrying a kind id and an instance id, published as an endpoint — and nothing here states what an update flag adds that it does not
        three_questions: whether one component can be both and whether reloadable implies the flag; whether a flagged component carries a decision:author-declared-boundary-id or the automatic positional identity, since the manifest entry shape follows from it; and the exported_only cost above
      downstream_positions_2026_08_08:
        reading_matches: reloadable is client-addressed re-rendering and the flag is participation in server-discovered deltas
        both_allowed: one component should be able to be both
        no_implication: reloadable must not imply the flag, because a component a page can redraw on demand is not necessarily one whose markup a navigation delta should compare
        identity: the automatic positional identity is what the reporter's manifest already assumes, and an author-written id would be a second entry shape for it to carry
        exported_only: no position offered
      separate_from_a_unit: a requirement:structured-render-output unit is template-scoped and sent once per connection, while a boundary is instance-scoped and returned on every request, so the two stay separate declarations and a component may be either, both, or neither
      as_built:
        shipped: 2026-08-08
        runtime: 'Boundary gained Instance func(P) string, which a component sets when it names its own id; a component call opens a collector boundary only when it does, so a chain member is unchanged and an ordinary call still costs the manifest nothing'
        generation: emitBoundary writes Instance from the reloadable id parameter; no new Boundary emission was needed, because boundaryCandidate already covered every exported single-rooted component and a reloadable one is required to be both
        smaller_than_expected: the collector already carried a stack, a parent id, and frames excluding nested boundaries, so nesting needed nothing new there
        output_change: a page collecting a render now writes the instance attribute on a reloadable component's root, where before the attribute op found no pending boundary and wrote nothing
        verified: TestNamedComponentBecomesAnInstance, TestUnnamedComponentStaysOutOfTheManifest, TestOneChangedRowLeavesTheOtherFrameAlone, and TestNamedComponentWritesNothingExtraWhenNotCollecting for the non-collecting path
        gap_this_creates:
          what: a redraw replaces the region and the client keeps the validator the page render gave it, so the next navigation delta compares against a stale one and sends the region again
          new: before this, a reloadable component had no manifest entry at all, so there was no validator to go stale; the change introduces the staleness along with the entry
          not_corruption: the region is re-sent and its DOM replaced, which is waste and lost focus rather than wrong content
          why_not_fixed_here: requirement:component-redraw-endpoint json_body carries the validator in a field the shared body has, and that body lands with requirement:boundary-decomposed-render
          interim: the client drops its manifest entry for an instance it just redrew, which is rule:delta-consistency-model document_validator invalidation applied per instance; requirement:update-wire-contract carries it as a client obligation
      settled_2026_08_08:
        decided: the explicit activation is the existing reloadable modifier; a reloadable component is an update boundary, and no new marker is added
        by: owner, on the marker-set round
        what_it_makes_this: no longer a flag to design but a capability to add — reloadable gains a data:component-update-manifest entry and participation in requirement:component-delta-rendering comparison
        constraints_are_a_superset: requirement:component-redraw-endpoint already requires exported, single rooted, not the shell, plus query-carryable parameters, and decision:update-manifest-transport plus exported_only require the first three; no reloadable component can fail to be a valid boundary
        one_direction_only: updatable does not imply reloadable, because a region a delta compares must not thereby publish an HTTP endpoint
        cache_is_not_merged: an @cache component stays independent, which answers the shorthand question below
        cost_accepted:
          what: every reloadable component now carries a manifest entry and a validator the client returns on every request, which is the linear cost decision:manifest-state-ownership size_mitigations exists for
          bounded_by: registration; a reloadable component is deliberately registered and Register refuses a duplicate, so the boundary population is author-chosen rather than inferred, which is the argument the merge rests on
          relief: the size_mitigations already designed — truncated validators, omitted validators for boundaries that cannot change, and a request-size cap degrading to frame-level hints
        identity_consequence: a reloadable component takes a decision:author-declared-boundary-id, so the manifest carries author-written ids beside the automatic positional ones; the reporter names this as a second entry shape it does not yet carry
        downstream_dissent:
          position: the reporter asked that reloadable not imply the flag, on the ground that a component a page can redraw on demand is not necessarily one whose markup a navigation delta should compare
          decided_against: 2026-08-08 by the owner, with the cost above accepted and the registration argument as the reason
          recorded_because: it is the one point of the round where the two catalogs disagree, and a reader of either should find it rather than infer it
  exported_only:
    rule: only an exported component can be a boundary
    reason: becoming a boundary publishes an identity into the DOM and the protocol, so a file-private implementation detail must not be addressable from outside
    cost: a component must be exported to become updatable, which is worth revisiting at requirement:client-update-rollout m3 if it forces unwanted public surface
    subsumed_2026_08_08: the explicit activation is now the reloadable modifier, which already requires export so it can publish an endpoint, so this rule adds no constraint an author was not under anyway and the m3 revisit has nothing left in it
  cache_relation: independent from requirement:component-output-cache
rendering:
  - emit stable boundary markers using rule:component-instance-identity
  - add one entry to data:component-update-manifest
  - carry identity and validators per decision:update-manifest-transport, which requires one root element per update boundary
  - preserve normal complete HTML for initial navigation and non-update clients
parameters:
  source: generated page execution from current path, search parameters, request state, and typed parent inputs
  client_state: raw arguments need not be exposed or resubmitted
direct_redraw:
  capability: requirement:component-redraw-endpoint, for an explicitly registered component only
  identity: decision:author-declared-boundary-id
  inputs: supplied by the caller and therefore untrusted, per rule:redraw-input-trust
nested_boundaries:
  - track parent identity and stable child ordering
  - unchanged nested boundaries may be omitted independently when the protocol can preserve their DOM
  - otherwise retain them through data:component-delta-response holes, or replace the nearest safe changed ancestor
client_state: rule:preserved-client-subtree
consistency: rule:delta-consistency-model
constraints:
  - repeated component calls require stable explicit keys through rule:component-instance-identity
  - browser runtime cannot instantiate undeclared components or select arbitrary server arguments
  - boundary rerendering is side-effect-free and safe to repeat
acceptance:
  - update flag is opt-in and leaves ordinary component output compatible
  - generated route layouts participate automatically without making their parameters client-mutable
  - browser can locate every returned operation target without inspecting component arguments
open_questions:
  - nested boundary preservation versus ancestor replacement in the first milestone
resolved:
  update_flag_syntax:
    decided: 2026-08-08, there is no new syntax; the reloadable modifier is the explicit activation
    supersedes: the recorded question, which assumed a marker had to be spelled
  cache_shorthand:
    decided: 2026-08-08, no; an @cache component does not become update-enabled, because its declaration answers a different question and decision:cache-component-declaration eligibility is unrelated to being addressable
```

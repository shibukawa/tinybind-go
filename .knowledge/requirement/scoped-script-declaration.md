---
id: requirement:scoped-script-declaration
type: requirement
title: Scoped Script Ownership
---
Report on each asset which component declaration owns it, so a caller's runtime joins a script to the live instances it must start and stop with, using identity the manifest already carries.

```yaml
priority: should
source:
  - concept:scope-lifecycle
  - decision:lifecycle-from-declaration-block
  - user design discussion 2026-08-11
review_gate: proposed
status: module half shipped 2026-08-11; the client half is the caller's by decision:client-runtime-ownership and is unwritten
as_built:
  field: htmlbind.Asset gained Scope, documented as empty for document lifetime and the declaring component otherwise
  filled_by: the template-time Asset gained Owner, set by extraction from the component that declared the block
  emitted_by: templates/htmlbind/emit.go writes Scope into the generated asset literal only when it is set, so a project declaring no block regenerates byte for byte
  contract: docs/httpbind_update_wire_contract.md gained a Scoped scripts section and an eighth normative client obligation, plus a note on the head section that it only ever adds, which is why a released script cannot be one the head installed
  harness_gap_stated: that document now says outright that the lifecycle vocabulary and these rules have no wire form and so no harness coverage, rather than leaving a reader to discover it
  not_done: the client half itself, which is the caller's
scale: one field on htmlbind.Asset and a contract section; the authoring surface and its checks are requirement:component-script-block
what_is_missing_today:
  finding: 2026-08-11, verified against v0.5.3
  already_there: the initial render writes the instance attribute, data:component-update-manifest carries component_id beside every instance_id, and data:component-delta-response reports insert, remove, move, and replace by instance
  not_there: nothing says which asset belongs to which component declaration; htmlbind.Asset carries ID, Type, and URL, and Fragment.Assets flattens the per-component sets the compiler already built internally
  therefore: a component-scoped lifecycle needs this one link and no new wire record, no new marker, and no new body channel
asset_field:
  added: Scope on htmlbind.Asset, beside ID, Type, and URL
  empty: document lifetime; a head contribution script, which is every script that exists today, evaluates once and is never released
  set: the stable generated declaration identity of the component that declared the block, per requirement:component-script-block
  pairs_with: the component_id data:component-update-manifest already carries, so a caller joins an asset to a live instance with no second identity scheme
  why_one_field: emptiness is the lifetime class and the value is the owner, and a caller needing only the class reads the same field
  unchanged: ID stays the content hash, so two components declaring identical bytes still share one file and differ by Scope
  merge: MergeAssets still deduplicates by ID; a file shared by two owners is possible because identity is content, so a caller reads Scope per member rather than per file
  accessors: Fragment.Assets and Wrapper.Assets are unchanged in shape, and a caller reads Scope off members it already holds, which is the requirement:client-managed-head precedent of reading per-member state off values in hand rather than from an enriched return type
  determinism: identical input produces identical owners, unchanged from requirement:static-asset-extraction
solves_the_conservatism_problem:
  fact: Assets reports what a value could require, including a component below a slot that never renders, which htmlbind.Asset documents as deliberate
  naive_consequence: a scope chain built from Assets alone would mount a lifecycle for markup that is not on screen
  not_fixed_by_per_layer_chains: building from each layer's own set rather than the merged one solves the wrapper case and not the component case, because a conditional component's asset sits in its own layer's set either way
  fixed_by_keying_on_the_instance: an instance that did not render has no attribute and no manifest entry, so nothing mounts; the conservative set stays safe to read because it is read as a catalog rather than as a mount list
what_the_module_does_not_do:
  no_call: the module calls nothing and imports nothing; it publishes an owner and the caller's runtime decides what to do with it
  no_export_name: setup, its argument, and the teardown convention are the caller's, and the module reads no JavaScript
  no_mount_order: the module publishes composition order and the caller walks it, outermost first to mount and innermost first to release
  no_scope_kinds: page and layout are the caller's words; the module reports the declaring component and classifies it no further
  no_global: unchanged from decision:client-runtime-ownership constraints
client_obligations:
  written_in: docs/httpbind_update_wire_contract.md, as a Scoped scripts section after the head operations and an eighth entry in the normative list
  why_there: it is behavior a second implementation cannot infer from the bytes, which is the shape that document's own exception names
  states:
    - a scoped asset's lifecycle runs per live instance of its owning component, not once per document
    - an instance that did not render mounts nothing
    - release runs when data:component-delta-response removes or replaces that instance, before the incoming markup is in the DOM
    - a common prefix of the composition chain stays mounted across a navigation; only the tail below it is released and remounted
    - a throwing setup or teardown is caught and reported through the caller's diagnostics and never stops the apply loop, matching requirement:runtime-lifecycle-signals handler_isolation
  not_stated: the module-map behavior a client relies on, which is a browser property rather than a contract term
compatibility:
  bytes: unchanged; this adds a field and emits nothing
  callers: Scope is an added field on a struct callers construct nowhere, so reading it is opt-in and no existing call site changes
  tinygo: a string field adds no reflect and no allocation on the existing path, so requirement:tinygo-wasm targets are unaffected
acceptance:
  - a component declaring a script block produces an asset reporting that component as owner
  - a head contribution script produces an asset whose Scope is empty
  - two components declaring identical script bytes still emit one file, and each member reports its own owner
  - a chain's merged asset set is unchanged in membership and order from the same chain today
open_questions:
  - whether the field is named Scope holding a declaration identity, or ComponentID with the lifetime class read from its emptiness, which trades a clearer name for a less direct one
  - whether a wrapper may declare a script block, given a chain member's identity is derived by rule:component-instance-identity rather than author-declared, and whether that reaches the same manifest field
  - whether a lifecycle needs a way to say it wants one call per component rather than one per instance, which the singleton author will expect and the instance keying does not give
  - whether the module should reject a script block on a component the generator knows is never updateable, since nothing would ever release it, or leave that to the caller as a diagnostic
```

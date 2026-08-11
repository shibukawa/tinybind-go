---
id: requirement:scoped-script-declaration
type: requirement
title: Scoped Script Ownership
---
Report on each asset which component declaration owns it, so a caller's runtime can join a script to the live instances it must start and stop with.

```yaml
priority: should
source:
  - concept:scope-lifecycle
  - decision:lifecycle-from-declaration-block
  - user design discussion 2026-08-11
review_gate: proposed
status: module half shipped 2026-08-11; the module side is complete for the whole lifecycle, mount and release alike, and what remains is the client half, which is the caller's by decision:client-runtime-ownership
as_built:
  field: htmlbind.Asset gained Scope, empty for document lifetime and the declaring component's package-qualified identity otherwise
  marker: templates/htmlbind writes data-<prefix>-component onto the single root element of a component declaring a block, as static markup carrying that same identity
  marker_transport: an attribute rather than a manifest field, chosen with the downstream caller 2026-08-11; the manifest is a request header bounded at DefaultMaxManifestBytes and dropped whole past it, so a per-instance addition has a silent cliff that scales with page size, while an attribute compresses with the body, has no bound, and reaches a first load that has sent no manifest
  marker_precedent: requirement:client-managed-head moved the head off a bounded header into the body for the same reason, so this is the second time the bound decided a transport
  filled_by: the template-time Asset gained Owner, set by extraction from the component that declared the block
  emitted_by: templates/htmlbind/emit.go writes Scope into the generated asset literal only when it is set, so a project declaring no block regenerates byte for byte
  contract: docs/httpbind_update_wire_contract.md gained a Scoped scripts section and an eighth normative client obligation, plus a note on the head section that it only ever adds, which is why a released script cannot be one the head installed
  harness_gap_stated: that document now says outright that the lifecycle vocabulary and these rules have no wire form and so no harness coverage, rather than leaving a reader to discover it
  not_done: the client half itself, which is the caller's
scale: one field on htmlbind.Asset and a contract section; the authoring surface and its checks are requirement:component-script-block
what_is_missing_today:
  finding: 2026-08-11 against v0.5.3, corrected 2026-08-11 against v0.5.5
  already_there: the initial render writes the instance attribute, and data:component-delta-response reports insert, remove, move, and replace by instance
  not_there: nothing says which asset belongs to which component declaration; htmlbind.Asset carries ID, Type, and URL, and Fragment.Assets flattens the per-component sets the compiler already built internally
  correction: the original finding also claimed the manifest carries component_id beside every instance_id, and it does not; data:component-update-manifest as_built records the four fields that ship, and that claim is why this requirement was scoped as needing no wire change
  therefore: the asset link plus the declaration marker cover the whole lifecycle for every component, an ordinary call included; nothing further is needed on the wire
  why_the_marker_had_to_be_static: found 2026-08-11 while answering the transport question, and named by neither side before that; both proposed transports would have put an identity beside data-<prefix>-id, and an ordinary component call has no data-<prefix>-id to put anything beside, per the htmlbind nestedboundary tests, so a marker riding the collector would have missed exactly the multi-instance case it was for
asset_field:
  added: Scope on htmlbind.Asset, beside ID, Type, and URL
  empty: document lifetime; a head contribution script, which is every script that exists today, evaluates once and is never released
  set: the package-qualified declaration identity, componentKind over the package, the file stem, and the component name, which is the same string Boundary.ComponentID carries
  normalized_up_because: the declared name collides; two Counter components in two route directories are one name and two declarations, and a lifecycle keyed on the short form would run one component's module against the other's elements, which a long string never does
  joins_by: matching Scope against the data-<prefix>-component marker on rendered elements, with no mapping, no manifest change, and no second identity scheme
  marker_is_static: it rides the markup rather than an instruction, so it costs nothing per render and lands where the instance attribute does not; an ordinary component call opens no boundary and would otherwise carry nothing at all, and that call is exactly the multi-instance case
  names_a_declaration_not_an_instance: two elements of one component are marked identically, and that is enough, because a lifecycle asks what to start and what to release rather than which instance a thing is
  release_is_local: a replace fragment stops at its nested boundaries, so when an operation for an instance arrives the subtree under it is still the one the caller mounted against; scanning that subtree for markers and running their teardowns before applying the markup satisfies the release-before-the-markup-lands rule and asks nothing of the server, since which elements are about to be destroyed is knowable only to whoever owns the apply loop
  reorder_survives: a children operation carries no markup and moves live nodes, so anything mounted inside one is untouched, which is correct because the element did not go away
  what_a_boundary_would_add: addressing one instance from the server, such as redrawing a single row, which is requirement:partial-update-boundaries territory and a different feature from the lifecycle
  single_root: a component declaring a block must render exactly one root element, since the marker lives on it, which is the rule requirement:partial-update-boundaries already imposes on a reloadable component for the same reason
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
  - whether a wrapper may declare a script block, given a chain member's identity is derived by rule:component-instance-identity rather than author-declared
  - whether an author will want to redraw one instance of a scoped component from the server, which is the one thing the marker does not reach and which would need declaring a block to open an update boundary, against the manifest-stays-small rule of htmlbind.Boundary; nothing has asked for it yet
  - whether a lifecycle needs a way to say it wants one call per component rather than one per instance, which the singleton author will expect and the instance keying does not give
  - whether the module should reject a script block on a component the generator knows is never updateable, since nothing would ever release it, or leave that to the caller as a diagnostic
```

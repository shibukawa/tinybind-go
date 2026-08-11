---
id: concept:scope-lifecycle
type: concept
title: Scope Lifecycle Methods
---
Let a script declare setup and teardown bound to the lifetime of the thing that declared it, so a region leaving the screen releases what it registered without the author remembering to.

```yaml
evidence:
  source: user design discussion 2026-08-11, plus a downstream responsibility survey of the same date
  asked_for: a generic lifecycle, explicitly not a page-specific one, with the caller's half and the module's half separated
  surveyed: v0.5.3 source, templates/htmlbind/assets.go and html.go, against the downstream survey's assumptions
review_gate: proposed
baseline:
  - decision:client-runtime-ownership
  - requirement:static-asset-extraction
  - decision:script-load-mode-authoring
  - rule:component-instance-identity
  - concept:signal-channel
problem:
  evaluated_once: a page script is an ES module, so its top level runs once per URL for the life of the document and cannot be where per-visit setup goes; measured downstream, where removing and re-adding a module tag ran it once and a classic tag ran it twice
  classic_is_worse: re-executing a classic script re-runs customElements.define, which throws NotSupportedError, and re-adds every listener, so the naive delete-and-re-add is not merely ineffective
  nothing_releases: a component's script registers listeners, observers, and timers, and no mechanism exists to release them when its markup is replaced by requirement:component-delta-rendering
  symptom_looks_healthy: a forgotten cleanup re-registers on every revisit, and a handler firing twice still looks like it works, so the failure is found in production rather than in development
  head_never_retires: requirement:client-managed-head installs and retires head tags, but a script the head installed has already evaluated and owns whatever it registered; retiring the tag releases nothing
not_the_signal_channel:
  why_it_looks_like_one: concept:signal-channel already has a lifecycle vocabulary in requirement:runtime-lifecycle-signals, so a second lifecycle mechanism needs a reason
  the_difference: a signal reports that something happened to a table registered once for the document's life; a lifecycle method is a per-scope registration that must be released when that scope leaves
  cannot_be_expressed: release everything this region registered has no form in a name-to-callback table, because the table is keyed by name and holds one entry per name, not per region
  they_compose: setup is where a scoped handler registers into the signal table, and teardown is what unregisters it; requirement:runtime-lifecycle-signals says what happened and this says who is currently alive
  reading: the table is document-scoped by construction, which is correct for it and is exactly the property this feature does not have
what_generic_means:
  rejected_axis: page, which is a caller word; this module has fragments, wrappers, and components, and no definition of a page to attach a flag to
  chosen_axis: the declaring component, whatever it is; a page and a layout are then special cases rather than the two supported shapes
  reaches: a layout through requirement:layout-reuse-boundaries chain members, a leaf fragment, and an ordinary component inside one, all through one rule
  costs_nothing_extra: rule:component-instance-identity already applies to chain members and to updateable components alike, so the generic reading rides identity that already ships
  what_it_refuses: a scope that is not a component at all, such as a hand-marked region an author wraps in an element of their own; that stays the caller's affordance and needs nothing here
mechanism_status:
  finding: 2026-08-11 against v0.5.3, corrected 2026-08-11 against v0.5.5
  instances_are_marked: the initial render writes the instance attribute into the output, per decision:update-manifest-transport
  declarations_are_not_named: the original finding claimed data:component-update-manifest carries component_id beside every instance_id; it does not, and its as_built records the four fields that ship
  entry_and_exit_are_reported: data:component-delta-response insert, remove, move, and replace operations name instances, so the client already learns when one arrives and when one goes
  missing_link: nothing says which script asset belongs to which component declaration; htmlbind.Asset carries ID, Type, and URL, and Fragment.Assets flattens the per-component sets the compiler already built
  therefore: requirement:scoped-script-declaration supplies that link and it is enough for a chain member, whose one instance per render makes chain position an address; a component with many instances needs a wire field naming each instance's declaration, which is a new wire record after all
  the_authoring_side_is_separate: requirement:component-script-block adds where an author writes the script, which is a parser change the user asked for on its own merits; the two meet at requirement:scoped-script-declaration and are otherwise independent
responsibility_split:
  module_decides_what_a_thing_is:
    - that a script declares a lifecycle at all, per requirement:scoped-script-declaration
    - which component declaration owns it, reported as that component's declared name and not as any identity the manifest carries
    - the identity of the file, unchanged from requirement:component-asset-requirements
    - the composition order, unchanged from Assets and MergeAssets, outermost first
    - the generation-time rules that make a declaration honest, chiefly the module-mode requirement of requirement:component-script-block
  caller_decides_what_happens_to_it:
    - the export name, its argument, and the teardown convention; setup returning a cleanup is the downstream shape and the module specifies none of it
    - when a scope is entered and left, since only the client watches the DOM
    - mount and unmount direction, outermost first and innermost last, walked over the order the module published
    - the global object name, the module loader, and same-origin enforcement
    - whether a lifecycle exists for a region that is not a component
  the_rule_it_follows: htmlbind.Asset states it already, that the module decides what is required and what its identity is while where the bytes are served is the caller's; this adds who owns it and keeps the same line
  why_the_line_matters_here: decision:client-runtime-ownership moved the browser half to the caller, and the downstream framework's own convergence decision moved the rest for the same reason, that a client-side change must need no upstream release; handing the module a reference to a runtime object to call setup would rebuild that coupling and make every signature change an upstream release again
milestones:
  first:
    what: requirement:component-script-block, the single-file-component script block and its extraction
    why_first: an author has nowhere to write a component's own script today, so every later part has nothing to act on; and it is worth shipping alone, since a script block that merely extracts is already the familiar shape
    module_deliverable: a raw-text parser rule generalized one level out, three diagnostics, and the existing extraction path reused
  second:
    what: requirement:scoped-script-declaration, the owning-declaration field on htmlbind.Asset
    why_after: it names owners of blocks that must first exist, and it settles the vocabulary both catalogs then use
  third:
    what: the client half, which is the caller's; the loader, the call, the release, and the diff over the chain
    why_after: it reads the field the second milestone publishes, and decision:client-runtime-ownership puts it downstream either way
  independent:
    what: requirement:authored-language-transform, the language marker and the bridge from extracted content to the transform seam
    why_independent: it is a property of the block's content rather than of its lifetime, so it lands whenever the transform seam does and blocks nothing here
  not_scheduled:
    body_markup_script: a script among the rendered elements, which requirement:component-script-block keeps a diagnostic because the block expresses everything it would
    per_region_scopes: a lifecycle for a marked region that is not a component, which stays the caller's
non_goals:
  - a component model; this adds a lifetime hook to a script that already ships, and adds no state, no props, and no reactivity
  - a server-side lifecycle; setup and teardown run in the browser, and the module's own render lifecycle is unchanged
  - replacing requirement:runtime-lifecycle-signals, which reports arrivals to a document-lifetime table and stays the right shape for that
  - a veto; teardown observes a scope leaving and cannot block it, matching the concept:signal-channel non-goal
  - shipping the runtime that calls these methods, which decision:client-runtime-ownership already refuses
  - naming a global or emitting a call into one, which the module has never done
conservatism_caveat:
  fact: Assets reports what a value could require, including a component below a slot that never renders, which htmlbind.Asset documents as deliberate
  consequence: a scope chain built from Assets alone mounts a lifecycle for markup that is not on screen
  not_fixed_by_per_layer_chains: the downstream survey's advice to build from each layer's own set rather than the merged one solves the wrapper case and not the component case, because a conditional component's asset is in its own layer's set either way
  resolved_by: keying on the instance, per the_mechanism_already_exists; an instance that did not render has no attribute and no manifest entry, so nothing mounts
  reading: this is why the generic answer is also the correct one rather than merely the broader one
authoring: requirement:component-script-block
placement: decision:lifecycle-from-declaration-block
ownership: requirement:scoped-script-declaration
language: requirement:authored-language-transform
identity: rule:component-instance-identity
transport: data:component-update-manifest, unchanged
open_questions:
  - whether a wrapper's lifecycle and a component's lifecycle are one kind or two, given a chain member's identity is derived and a component's may be author-declared
  - whether teardown is guaranteed on document unload, or only on a delta that removes the instance, since a browser grants no reliable unload hook
  - whether two instances of one component share one setup call or receive one each, which the instance keying answers as one each and which an author writing a singleton will expect not to
```

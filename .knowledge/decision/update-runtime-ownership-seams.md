---
id: decision:update-runtime-ownership-seams
type: decision
title: Runtime Ownership Seam Requests From The Downstream Framework
---
Accept all seven asks of the fourth downstream round, and record that the headline one is not a new boundary but the exit from a milestone deviation this catalog already wrote down and never scheduled.

```yaml
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
  - decision:framework-integration-seams
  - decision:live-integration-seams
  - decision:library-component-seams
review_gate: proposed
round:
  when: 2026-08-01, the fourth round from this reporter, against the shipped v0.3.0 rather than against the plan
  previous: generation seams 2026-07-30, live integration 2026-07-31, component and asset seams 2026-07-31
  reporter_position: not blocked, and adopting the package; every item has a workaround and each costs a copy, a wrapper, or a name the reporter cannot choose
  not_in_question: the delta protocol, the two validators, the manifest encoding, the operation kinds, and the requirement:render-mode-negotiation rules, which the reporter names as the reason to adopt rather than rebuild
  scope: the module now ships a browser asset and a set of names in the document, and both used to be the caller's
verification:
  method: every claim checked against the v0.3.0 source before it was accepted
  runtime_shipped: confirmed; htmlupdate/runtime.go embeds, digests, paths, serves, and references runtime.js, and runtimeSource is unexported
  header_names_hardcoded: confirmed; runtime.js holds the render, manifest, and build header names and the id attribute as literals, while the same file reads the path prefix and build id from the script tag dataset
  author_attributes_branded: confirmed; 'data-tinybind-preserve' and 'data-tinybind-ignore' are literals, and data:generator-options DataAttributePrefix does not reach them
  prefix_reaches_half: confirmed; 'data-<prefix>-id' is configurable while the '<tb-boundary>' element and the 'tb-' boundary id allocation are not
  global_installed: confirmed; runtime.js installs eight methods and two constants on window.tinybind
  errors_bypass_caller: confirmed; RedrawHandler writes four failures with http.Error and Options carries no hook
  mount_concrete: confirmed; Mount takes '*http.ServeMux', while RedrawHandler and RuntimeHandler both return an http.Handler
  panics: confirmed; Register panics twice and the build identity fallback panics once
  cache_contradiction: confirmed; redraw.go sets no-store while requirement:component-redraw-endpoint asks for private plus an ETag that lets an unchanged redraw answer 304
  bounds_inconsistent: confirmed; one Options field and two package constants
headline_reading:
  reported_as: a reversal of the boundary decision:client-runtime-ownership states and docs/htmlbind_frameworkowner.md documents in three places
  catalog_holds: requirement:client-update-rollout m1 records exactly this as deviations.runtime_delivery, with the reason that requirement:static-asset-extraction and requirement:html-runtime-bootstrap injection did not exist yet
  code_agrees: the RuntimeHandler comment says serving the asset here keeps the first milestone free of the static asset pipeline
  what_is_true: the deviation was recorded, the exit was never scheduled, and a temporary shape reached a tagged release, where it reads as policy to anyone who did not read the milestone
  not_a_disagreement: both sides put the runtime with the caller; the report supplies the deadline the deviation lacked
  new_reading: a recorded deviation with no scheduled exit becomes a decision at the next release, so a deviation needs the milestone that retires it named at the time it is taken
accepted:
  - what: requirement:browser-runtime-asset-ownership
    value: highest; it is the only item whose workaround silently rots, since a vendored runtime copy drifts with no build failure and fails as a dead page
    cost: medium, and mostly already designed; requirement:component-asset-requirements url_function is the destination shape
    also_covers: the hardcoded 'tinybind.<digest>.js' asset file name
  - what: requirement:update-error-hook
    value: high; it is the only item in the round that loses information rather than costing a copy, and a redraw failing in production is invisible in the caller's logs
    cost: low; one hook on Options, or a handler shape returning an error
  - what: requirement:update-protocol-naming-ownership
    value: high, and mechanical once the asset belongs to the caller
    cost: low individually, spread across the generator and the runtime
    includes: the header names, the browser global, the author-facing preserve and ignore markers, and the half of the prefix that never reached the placeholder element or the id allocation
    priority_within: the author-facing markers first, because unlike a header name they are written by hand in application templates
  - what: requirement:redraw-cache-policy
    value: medium, and it is the one item that contradicts this catalog rather than the reporter's preference
    cost: low
  - what: requirement:update-endpoint-mounting
    value: medium; the seam is already open through RedrawHandler, so only the convenience is lost
    cost: low; one interface parameter
  - what: requirement:update-registration-diagnostics
    value: medium; the requirement is uncontested and only its failure form is
    cost: low; one signature change
  - what: requirement:update-configurable-bounds
    value: lowest; three knobs of the same kind in two shapes
    cost: lowest
sequencing:
  reporter_proposed: asset, then errors, then names, then mux and bounds
  chosen: errors, then the author-facing markers, then the asset, then names, then mux, bounds, and cache policy
  why_errors_first: the only item whose cost is unrecoverable while it is open, and it needs no design round
  why_markers_second: they are the smallest change in the round and the only one reaching application source, so every day they are open writes the dependency's brand into more of the reporter's templates
  why_asset_third: it retires four of the five naming items, but it needs the requirement:component-asset-requirements url_function shape, which is itself the largest open piece of the 2026-07-31 round
  why_the_rest_last: each is small, independent, and blocks nothing
  not_a_disagreement: the reporter sequenced by what each ask retires; this sequences by readiness and by how much each costs while open, and both put the asset at the top of what matters
severity:
  reading: nothing in the round can put wrong content on screen, leak anything, or break the protocol
  correctness: only the cache policy is a defect against a stated requirement; the rest are missing seams
  worst_case_asset: silent drift between a vendored copy and the module, producing a dead page after an upgrade with no build failure anywhere
  worst_case_errors: a redraw failure loop invisible to logging and tracing, in a request the browser retries
implemented_2026_08_01:
  what: all seven, in one pass rather than in the sequence below; each was small enough that the sequencing argument decided order within a day rather than across releases
  measured: every generator fixture and golden file regenerated unchanged, and no generated Go changed at all, so the generation half of the seam rule holds by construction
  three_breaking_changes:
    author_markers: the preserve and ignore attributes default to the 'tb' prefix, where they were 'tinybind'; an existing author's markers stop being recognized
    renamed_constants: MaxQueryBytes and StreamContentType became DefaultMaxQueryBytes and DefaultStreamContentType
    register_signature: Register returns an error, so a call site that ignored it no longer compiles
    reading: all three are the cost of the consistency the round asked for, and the module is pre-1.0
  one_default_output_change: ScriptTag writes one data-config attribute instead of two dataset attributes; the tag is written and read by this module alone, so nothing outside that loop depends on its shape
  stronger_than_asked: the runtime bytes carry no naming choice at all, so one asset serves every deployment and a merging framework needs no assembly step; the ask had offered an assembly entry as the alternative
  weaker_than_asked: shipping the asset is a disable switch rather than opt-in, because flipping the default would break a direct user who has no replacement until requirement:html-runtime-bootstrap injection exists
  found_while_building:
    element_name_rule: a custom element name must start with a lowercase letter while a data attribute may start with a digit, so the prefix naming an element is stricter than the prefix naming an attribute; Options.Validate enforces the stricter rule
    render_delta_signature: RenderDelta took no options and had no way to receive the boundary prefix, so it gained the variadic parameter every sibling entry already had
    startup_validation_generalizes: Options.Validate came out of requirement:update-registration-diagnostics and turned out to be the right home for every unusable option, not only a duplicate kind
principle:
  applies: the decision:framework-integration-seams rule unchanged, widen a seam whose default output stays identical and whose contract stays the caller's
  fits: every accepted item is additive or a default-preserving parameterization; none changes a shape an author's own template or Go is written against, and the author-facing markers move toward the author rather than away
  reporter_states_it_back: the report's own closing condition is that a project using none of these seams gets byte-identical output
not_asked_for:
  - the delta protocol, the two validators, the manifest encoding, and the operation kinds
  - the requirement:render-mode-negotiation rules; the reporter names Negotiate resolving anything unrecognized to a complete document as what lets its live-delivery token share one header with the update modes
  - bundling, minification, or any JavaScript toolchain
related:
  - decision:client-runtime-ownership
  - decision:update-manifest-transport
  - requirement:client-update-rollout
  - requirement:component-asset-requirements
  - requirement:html-runtime-bootstrap
```

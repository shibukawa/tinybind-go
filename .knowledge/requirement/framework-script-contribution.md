---
id: requirement:framework-script-contribution
type: requirement
title: Framework Script Contribution
---
Let a framework register browser script that merges into the root document head, so a global API such as a telemetry namespace is callable from any component script.

```yaml
priority: should
source:
  - concept:framework-template-extensions
  - user design discussion 2026-07-27
review_gate: proposed
model:
  contributor: a registration, not a component; the contribution exists without any template declaring it
  axes: Inclusion decides which documents carry it, Load decides how the browser evaluates it, Namespace names what it installs; the three are independent
  inclusion_not_only_static: requirement:render-time-script-contribution adds the render call as a third inclusion trigger, so registration fixes the asset while the call may still decide the audience
  destination: the requirement:head-merging root head, through the same merge pass component contributions use
  form: one script element per contribution, referencing an external file
  static_set: registration is fixed at build time, so the reachable contribution set is known at generation time and needs no per-request collection
registration:
  timing: passed to the generate command, like the requirement:builtin-element-registration whitelist
  fields:
    Name: contribution identifier used by data:builtin-element-definition Scripts and by diagnostics
    Source: external URL, or inline JavaScript source the generator extracts
    Namespace: optional name the script installs, such as otel
    Load: global or module, required with no default per decision:framework-script-delivery
    Attributes: author attributes such as defer preserved on the emitted tag
    Inclusion: always or on-demand
  owner: data:generator-options, registered with requirement:builtin-element-registration
inclusion:
  axis: where the script is available, independent of the Load axis that decides how its Namespace is installed
  always:
    meaning: contributed to every document in the generation unit, so a declared Namespace is callable from any component script
    use_for: a telemetry or logging namespace the application calls from arbitrary component script
    cost: every document carries the tag, so this is an explicit choice rather than a default
    typical: paired with Load global, which is the combination the script-global request describes
  on_demand:
    meaning: contributed only when a builtin element naming it is reachable in the composition
    use_for: script backing one builtin element, which a page not using that element should not pay for
    reachability: the same generation-time call graph requirement:head-merging already walks, plus runtime slot values carrying their own contributions
  render_selected:
    meaning: neither always nor reachability decides it; the render call names the contribution, per requirement:render-time-script-contribution
    use_for: a script needed on some paths only, which a build-time set cannot express
    registration_still_needed: the named form still registers here, so its file and URL are generated; only the inclusion decision moves to the call
  default: on-demand
delivery:
  inline_source: requirement:static-asset-extraction writes a content-hashed file under PublicDir and emits the PublicURLBase reference
  external_url: emitted as a script tag unchanged, producing no file, per the existing passthrough rule
  no_inline_block: an inline script element is never emitted, so a policy may forbid inline script
  ordering_and_dedup: decision:framework-script-delivery
per_request_data:
  rule: a contribution is static source; a per-request value never becomes generated JavaScript
  channel: data:html-client-bootstrap inert escaped metadata, read by the script at load
  reason: requirement:head-merging writes the head before the first body byte, and an inline value would defeat asset caching and CSP
relation_to_bootstrap:
  distinct: requirement:html-runtime-bootstrap stays a separate capability-selected runtime and is never merged into a framework contribution bundle
  independent: a document may carry the bootstrap, framework contributions, both, or neither
non_goals:
  - type checking or analyzing the registered JavaScript
  - verifying that a component script actually calls the declared namespace
  - bundling, transpiling, or resolving JavaScript imports
acceptance:
  - a framework registers a telemetry script as always-included with load mode global, and any component script may call otel.log without the page linking anything
  - a page using no builtin element that needs an on-demand script emits no script tag for it
  - two components triggering the same contribution emit one script tag
  - an inline registered source produces one hashed file and one src reference
  - an already-external registered URL produces a tag and no file
  - a per-request trace value reaches the script through escaped head metadata rather than generated JavaScript
  - a registration omitting the load mode fails, so a global-exposing script never inherits module semantics by accident
related:
  - requirement:static-asset-extraction
  - requirement:head-merging
  - rule:template-context-safety
open_questions:
  - whether contributions bundle per generation unit like component styles, or emit one file per contribution
  - whether a framework serves its own versioned runtime URL instead of letting the generator emit the file
  - CSP nonce provisioning for a project that cannot use external files
```

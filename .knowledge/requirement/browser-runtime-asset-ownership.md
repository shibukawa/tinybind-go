---
id: requirement:browser-runtime-asset-ownership
type: requirement
title: Browser Runtime Asset Ownership
---
Hand the browser runtime to the caller as bytes rather than as a served endpoint, so a framework that already ships a runtime merges one asset instead of vendoring a copy.

```yaml
priority: must
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
  - decision:client-runtime-ownership
  - requirement:client-update-rollout m1 deviations.runtime_delivery
review_gate: proposed
shipped_today:
  embed: htmlupdate/runtime.go embeds runtime.js as the unexported runtimeSource
  version: RuntimeVersion, a base64 digest of those bytes
  url: Options.RuntimePath, '<prefix>/runtime/tinybind.<digest>.js'
  serve: Options.RuntimeHandler, setting its own content type, ETag, and immutable cache header
  reference: Options.ScriptTag, the script element carrying the path prefix and build id
  gap: the exported surface is the version, the URL, the handler, and the tag; never the bytes
reported_as: a reversal of the boundary decision:client-runtime-ownership states and docs/htmlbind_frameworkowner.md documents
catalog_reading:
  not_a_reversal: requirement:client-update-rollout already records this as an m1 deviation, with the reason that requirement:static-asset-extraction and requirement:html-runtime-bootstrap injection did not exist yet
  runtime.go agrees: its own comment says serving the asset here keeps the first milestone free of the static asset pipeline
  what_is_actually_true: the deviation was recorded and its exit was never scheduled, so a temporary shape reached a release and reads as policy
  agreement: both readings put the runtime with the caller; only the reason for the current state differs
why_it_binds_now:
  reporter_already_ships_one: async boundary application, live delivery, and truncation detection, against v0.2.x
  v0_3_0_adds: navigation delta, redraw, action apply, rule:form-state-reconciliation, and rule:preserved-client-subtree islands
  two_runtimes_on_one_document: two boundary id spaces, two build identities, and two script tags with nothing deciding which owns a region
  consequence: the reporter merges them into one asset, and with no readable bytes merging means vendoring a copy
  vendoring_cost: a copy is not a version-pinned dependency, so upstream may change it and nothing in the downstream build fails
  failure_shape: a browser runtime that drifts produces a silently dead page rather than a compile error, which is the worst available failure shape
stated_rationale_no_longer_holds:
  comment: runtime.js says the protocol names are hardcoded because the runtime is shipped per framework
  true_of: a world where the module ships none
  false_of: its own package, since nobody is building a runtime for an overridden prefix once the module ships one
  detail: requirement:update-protocol-naming-ownership
ask:
  primary: export the runtime source, or an assembly entry taking the naming choices and returning the bytes
  preferred: make shipping the asset opt-in, so the module default ships none
  filename: with the asset exported, the hardcoded 'tinybind.<digest>.js' name falls away; otherwise the caller names the file
destination:
  shape: the runtime becomes one more entry of requirement:component-asset-requirements, with its identity from the module and its URL from the caller
  url_function: the caller-supplied identity-to-URL function that requirement already defines, generalizing the scaffolded runtime-script-URL pattern
  embedded_table: the same asset expressed as generated source, which requirement:tinygo-wasm targets need
  injection: requirement:html-runtime-bootstrap keeps selecting and placing exactly one runtime tag per document
constraints:
  - a project that keeps the shipped asset regenerates and re-serves byte-identical output, per decision:framework-integration-seams
  - the module still owns the protocol contract, the version constant, and what a conforming client must do
  - no bundling, minification, or JavaScript toolchain enters the module
acceptance:
  - a framework reads the runtime bytes from the module and merges them with its own into one asset, with no vendored copy
  - a module upgrade that changes the runtime changes the bytes the framework reads, so the merge is rebuilt rather than silently stale
  - a project using the module directly still gets a working runtime without writing protocol code
  - a document never carries two runtimes
as_built:
  shipped: 2026-08-01
  bytes: RuntimeSource returns a copy of the embedded source; RuntimeAsset adds the digest, the media type, and the file name, which is the data:generation-artifact shape requirement:component-asset-requirements wants
  no_assembly_entry_needed: the ask offered source or an assembly entry taking the naming choices; the runtime reads its names at load instead, so the bytes carry no naming choice at all and merging needs no build step
  stronger_than_asked: one asset serves every deployment, which the assembly-entry form would not have given
  file_name: Options.RuntimeFileName, so the hardcoded 'tinybind.<digest>.js' is the default rather than the name
  switch: Options.CallerOwnsRuntime; Mount registers no asset route and ScriptTag returns empty, because a tag pointing at an asset this build does not serve is worse than none
deviation_from_the_ask:
  asked: make shipping opt-in, so the module default ships none
  built: a disable switch, so the module default still ships one
  why:
    - the round's own closing rule is that a project using none of these seams gets byte-identical output, and flipping the default changes it
    - requirement:html-runtime-bootstrap injection does not exist, so a direct user losing the default would get a compiling program and a page that silently stops updating
  cost_to_the_reporter: none; it sets the switch either way
  revisit: when requirement:html-runtime-bootstrap selects and injects the runtime, at which point a direct user has a replacement and the default can flip
script_tag_not_byte_identical:
  what: ScriptTag now writes one data-config attribute holding the whole configuration, replacing two dataset attributes
  why_acceptable: the tag is written by this module and read by this module's runtime, so nothing outside that loop depends on its shape
  stated_because: it is the one place the default document bytes changed, and claiming byte-identical output without it would be false
related:
  - requirement:static-asset-extraction
  - requirement:framework-script-contribution
  - requirement:component-asset-requirements
  - decision:framework-integration-seams
open_questions:
  - whether the exported form is the source string, an assembly function taking naming choices, or both
  - whether opt-in serving keeps RuntimeHandler as a convenience or removes it
  - how a merged framework asset reports the protocol version it implements, given decision:client-runtime-ownership wants a mismatch to fail loudly
```

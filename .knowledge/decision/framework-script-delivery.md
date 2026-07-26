---
id: decision:framework-script-delivery
type: decision
title: Framework Script Delivery
---
Order framework script contributions ahead of component script in the merged head, keep every tag deferred so document order is execution order, and fail generation on a duplicated global namespace.

```yaml
source:
  - requirement:framework-script-contribution
  - user design discussion 2026-07-27
review_gate: proposed; requires user approval
problem:
  ordering: a component script calling a framework global runs only if the framework script already evaluated
  dedup: requirement:head-merging dedups identical nodes, but two registrations may define one namespace from different URLs
  namespaces: requirement:html-runtime-bootstrap avoids installed global names, while the requested otel.log surface is exactly one
ordering:
  groups: always-included contributions first, then on-demand contributions, then requirement:render-time-script-contribution render-supplied entries, then requirement:static-asset-extraction component scripts, in that order inside the head
  reason: an always-included contribution exists to be callable from anything later, so it must load before every other script
  render_supplied_position: after every generation-time contribution, because it may depend on them, and before component script, because component script may depend on it
  within_group: registration order for framework contributions, deterministic emission order for component scripts
  execution: a deferred classic script and a module script both execute in document order, so head order is the guarantee
  async_forbidden: an async attribute is rejected on a contribution declaring a Namespace, because async execution order is unspecified
  limit: ordering guarantees load order only; a component script still runs before a framework script that a later page adds
namespaces:
  declaration: a contribution declares the name it installs, or declares none
  collision: two contributions declaring one Namespace is a generation error naming both registrations
  render_supplied_collision: a requirement:render-time-script-contribution entry colliding with an installed name fails at render before the first byte, because no generation-time check can cover a per-call selection
  module_caveat: a module script installs nothing by itself, so a module contribution declaring a Namespace must assign it explicitly; the required load_mode makes that an author choice rather than an inherited default
  vocabulary: Namespace is the installed name; load_mode global is how it gets installed; requirement:framework-script-contribution Inclusion always is where it is available. the three are independent axes
  bootstrap_divergence: the requirement:html-runtime-bootstrap no-globals rule stands for the tinybind runtime itself; a framework accepting a global surface owns that choice
load_mode:
  values: [global, module]
  naming: the Vue distribution vocabulary, where a global build installs a name on the window and a module build does not
  rejected_name: classic, the HTML specification term; it describes the tag, not what the author cares about
  required: every registration states one; there is no default
  reason: a module installs no name, so a defaulted module mode would silently break a contribution that exists to expose one
  global:
    tag: an ordinary script tag with defer and no type attribute
    namespace: required; this mode exists to install a name such as otel
    use_for: a name any component script may call, including non-module script
  module:
    tag: a script tag with type module
    namespace: optional; declaring one means the script assigns it explicitly, and the generator does not verify that
  not_a_type_value: global names a mode, never a type attribute value; decision:script-load-mode-authoring records why type global would never execute
  authoring: decision:script-load-mode-authoring spells the same two modes inside a template
  policy: policy:frontend-convention-alignment favors modules, but not at the cost of an implicit choice the generator cannot verify
  diagnostic: a registration omitting the mode, or choosing global without a namespace, fails while naming the contribution
dedup:
  identity: resolved reference URL, so one contribution reached through several builtin elements emits one tag
  across_kinds: a framework contribution and a component script naming the same external URL collapse to one tag
  never: the bootstrap tag is never deduplicated away, as requirement:html-runtime-bootstrap already states
failure_modes:
  missing_contribution: a builtin element naming an unregistered contribution fails at registration time
  unreachable_asset: a PublicURLBase misconfiguration is already covered by requirement:static-asset-extraction option pairing
rejected:
  body_end_injection: placing contributions at body end would run them after component scripts in the head, inverting the dependency
  runtime_ordering: a client-side loader ordering scripts at runtime adds a dependency the framework wanted to avoid
  automatic_dependency_graph: contributions declaring dependencies on each other, deferred until a second contribution kind exists
open_questions:
  - whether a contribution may declare it must load before the bootstrap runtime
  - whether preload or modulepreload hints are emitted for on-demand contributions
  - subresource integrity for an externally hosted contribution URL
```

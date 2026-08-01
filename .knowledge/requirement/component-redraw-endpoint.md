---
id: requirement:component-redraw-endpoint
type: requirement
title: Component Redraw Endpoint
---
Publish an explicitly registered component as a GET endpoint that renders one instance from arguments the browser supplies.

```yaml
source:
  - user redraw proposal 2026-08-01
review_gate: proposed protocol surface requires user approval
model:
  contract: a reloadable component is a function of its declared parameters, and the browser passes all of them
  consequence: nothing is reconstructed server side, so decision:boundary-update-execution subtree-only execution becomes correct rather than a snapshot
  trust: rule:redraw-input-trust, because those arguments are now attacker controlled
registration:
  explicit: a component is registered as reloadable; being renderable is not enough
  reason: registration publishes an HTTP endpoint, which must be a deliberate act
  syntax: a reloadable modifier after export on the component declaration, matching how the language already spells export and external live
  both_halves: the declaration generates a registration value; the application still installs it, so publishing is visible in Go as well as in the template
  kind_id: the generated component identity, name plus a hash of parameters and compiled plan, supplied by the framework rather than chosen by the author
  kind_covers:
    primary_purpose: version, so a template edit changes the URL and a page loaded before a deploy gets a 404 and reloads instead of rendering under changed semantics
    also_distinguishes: two components sharing a name but differing in parameters or markup
    does_not_cover: the package, so two templates identical in name, parameters, and markup collide
    collision_harm: the plan text is identical but its external calls resolve per package, so the wrong component could answer
    guard: registering a kind twice fails at startup rather than overwriting, which catches any collision cause including a truncated digest
    rejected_alternative: adding the package name, which is not unique either; only the import path would be, and the generator does not know it
  id_parameter: the component declares a required string id, which the framework writes onto the root element and the endpoint fills from the path
  diagnostics:
    - a reloadable component must be exported, single rooted, and not the document shell
    - every other parameter must be a type a query string carries deterministically, so a record, a slice, and html are refused
    - unlike an automatic boundary these are errors, because the author asked for the endpoint
kind_attribute:
  rule: every render of a reloadable component emits its kind on the root element, alongside the instance id
  reason: the browser reads the kind from the element rather than being told it, and a redraw replaces that element, so a kind missing from the replacement would make the region redrawable exactly once
  found_by: implementation, when a second redraw of the same region failed
request:
  shape: 'GET <prefix>/redraw/<kind_id>/<instance_id>?<declared parameters>'
  prefix: configurable, defaulting to the module namespace
  instance_id: decision:author-declared-boundary-id value, carried so the returned root element arrives already addressable
  method: GET, because a redraw renders and must be side-effect free
  parameters: decoded by the generated typed decoder; an unknown name, a bad value, or an oversized query is rejected
response:
  body: the component's rendered subtree, a single root element carrying the instance id
  content_type: HTML fragment
  cache: private by default, because a redraw usually renders per-user content
  conditional: an ETag over the rendered bytes lets an unchanged redraw answer 304, reusing HTTP instead of a bespoke manifest
no_manifest:
  rule: a redraw needs no data:component-update-manifest, no validators, and no continuation
  reason: the client names the target and supplies the inputs, so there is nothing to compare or reconstruct
  contrast: requirement:component-delta-rendering still owns URL-driven page updates, where the server must discover what changed
version_skew:
  changed_template: the kind hash changes, so a page loaded before a deploy requests a kind that no longer exists
  behavior: 404 falls back to a complete page reload, consistent with every other unrecognized condition
client:
  api: redraw(instanceId, params) resolving after the element is swapped
  failure: any non-2xx, a malformed fragment, or a missing target falls back to a full page load
constraints:
  - parameter types are limited to what a query string can carry deterministically; records, files, and arbitrary objects need an explicit codec
  - a configured maximum query length, since a GET carries every argument
  - the component must render exactly one root element, per decision:update-manifest-transport
acceptance:
  - a registered component renders from a URL with no page execution
  - an unregistered component has no endpoint, even when it is exported and single rooted
  - a template edit changes the endpoint URL and old pages fall back rather than rendering stale semantics
  - an unchanged redraw can answer 304
delivered:
  transport: the endpoint, its registry, path parsing, query size bound, and the browser redraw call
  generation: the reloadable modifier, its diagnostics, the typed query decoder, the id and kind attributes, and the registration value
  decoding: a missing, undecodable, or repeated parameter is an error rather than a zero value, because a caller supplies them
open_questions:
  - whether an enum parameter should be checked against its members at decode time rather than accepted as any string
  - policy:html-update-csrf-protection applicability, given a side-effect-free GET with ambient credentials
  - whether the instance id belongs in the path, given its cost as a cache key
```

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
  superseded_as_normative: requirement:caller-addressed-redraw moves kind and instance into headers so the caller chooses the URL; this path form stays working as a compatibility shape and stops being the published contract
  prefix: configurable, defaulting to the module namespace
  instance_id: decision:author-declared-boundary-id value, carried so the returned root element arrives already addressable
  method: GET, because a redraw renders and must be side-effect free
  parameters: decoded by the generated typed decoder; an unknown name, a bad value, or an oversized query is rejected
response:
  body: the component's rendered subtree, a single root element carrying the instance id
  content_type: HTML fragment
  cache: private by default, because a redraw usually renders per-user content
  conditional: an ETag over the rendered bytes lets an unchanged redraw answer 304, reusing HTTP instead of a bespoke manifest
  as_built_conflict: redraw.go sets no-store, which forbids the conditional request above; requirement:redraw-cache-policy resolves it and moves the choice to the caller
  as_built_errors: the handler writes its four failures as plain text, so requirement:update-error-hook is what makes a redraw failure visible to the caller
  as_built_registration: Register panics rather than returning an error, per requirement:update-registration-diagnostics
also_an_update_boundary:
  decided: 2026-08-08, reloadable is the explicit activation requirement:partial-update-boundaries was going to spell as a new flag
  what_it_adds: a manifest entry and participation in requirement:component-delta-rendering comparison, on top of the endpoint this concept already publishes
  costs_nothing_in_diagnostics: this concept's requirements — exported, single rooted, not the shell — are a superset of what a boundary needs, so no reloadable component can fail to be a valid one
  one_direction: a boundary does not become reloadable; publishing an endpoint stays the deliberate act this concept describes
  also_a_structured_unit: requirement:structured-render-output unit_set includes it, which is why the identity it already carries is doing three jobs rather than one
json_body_2026_08_08:
  decided: the response body becomes the JSON shape the action path already writes, and the head moves into it
  shape: 'ops with one replace naming the instance, plus head; identical to what WriteUpdateStatus builds for one region'
  not_an_envelope_added: it is the removal of the one update path without one; the navigation delta and the action response both carry this shape already
  only_consumer: a redraw needs three request headers, so a browser navigation gets the page instead, and private no-cache keeps it out of every shared cache; the update runtime is the only reader, which is what makes agreeing with it sufficient
  curl_property_traded: the plain-fragment property this concept recorded is a developer convenience rather than a correctness one, and a redraw already requires three headers to provoke
  json_is_the_browser_s_own_parser: one of the few formats with a native parser, which is the reason to prefer it over a bespoke framing
  escaping_cost: JSON escapes the angle brackets, so assembled markup in a body costs roughly triple; accepted because the caller compresses and because requirement:structured-render-output moves markup out of the body entirely
  lands_with: requirement:structured-render-output, not before, so the assembled-markup regression window never exists and docs/httpbind_update_wire_contract.md takes one migration rather than two
  removes:
    - the head response header, encodeHead, and its base64 of JSON
    - DefaultMaxHeadBytes and the registration error telling an author to use Registry.RequiredHead instead, both of which exist only because the head travels in a header
    - the requirement:fragment-response-head asymmetry, which disappears rather than being patched a second time
  gains: a content type that differs from the page's, so a Vary-ignoring proxy substitution becomes visible instead of silent, which htmlupdate already relies on for the delta
  cache_policy_does_not_converge: the body shape joins the action path and the cache policy does not; requirement:redraw-cache-policy keeps private, no-cache and an ETag, while an action response stays no-store
no_live:
  rule: a redraw carries no live boundary; it is a client-triggered re-render of one registered component and nothing in it subscribes
  settled: 2026-08-08 by the owner, narrowing an earlier reading that had a redraw originate a subscription
  why_the_earlier_reading_failed: decision:live-transport-boundary reconstructs a subscription by executing the page, and a redrawn region's arguments came from the client rather than from the page, so page execution would deliver content for different arguments
  where_live_does_arrive: a navigation or a search, where the page did execute; renderDelta already sets the live field for exactly that case and the client opens its own connection
  diagnostic_needed: declaring reloadable on a component owning a live boundary should fail at generation with the declaration position, or this rule is a convention rather than a fact; Plan.HasLiveBlock already computes the condition over the call graph
  precedent: decision:cache-component-declaration await_rationale rejects the same shape for the same reason, that the alternative is silent — there, caching only the initial pass; here, a fallback that never updates
no_manifest:
  rule: a redraw response carries no continuation and reconstructs nothing, because the client names the target and supplies the inputs
  validator_is_no_longer_exempt:
    was_true_when: a reloadable component was not an update boundary
    now: requirement:partial-update-boundaries makes it one, so after a redraw the client holds a validator for markup the redraw replaced
    consequence: the next navigation delta compares against the stale validator, judges the region changed, and sends it again — discarding the DOM the redraw just installed along with its focus and form state
    fix: the response carries the instance's new validator, which is a field the shared body already has
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
resolved:
  instance_id_in_the_path: requirement:caller-addressed-redraw takes both the kind and the instance out, for authorization rather than for the cache-key cost this concept had raised; the stronger argument arrived from the caller that owns the route
```

---
id: requirement:update-protocol-naming-ownership
type: requirement
title: Update Protocol Naming Ownership
---
Derive every name the update protocol puts in a document — data attributes, the placeholder element, boundary ids, header names, the browser global, and the asset file name — from one caller-supplied configuration instead of a literal.

```yaml
priority: must
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
  - decision:update-manifest-transport naming
review_gate: proposed
premise: the names stop being the module's the moment requirement:browser-runtime-asset-ownership hands the runtime over; each item below is also fixable on its own
half_configured_axes:
  header_prefix:
    configurable: Options.HeaderPrefix, defaulting to 'X-Tinybind'
    reaches: the server half only
    literals_in_runtime.js: RENDER_HEADER, MANIFEST_HEADER, BUILD_HEADER, and ID_ATTR 'data-tb-id'
    module_states_the_gap: the Options.HeaderPrefix comment says an overriding deployment needs a runtime built for the same prefix
    circular: v0.3.0 ships the only runtime, so no deployment can build one, and the option configures nothing end to end
    pattern_already_present: the same file reads PREFIX and BUILD from the script tag dataset, with the comment that an endpoint namespace is a deployment choice rather than a protocol detail
    reading: a header namespace is a deployment choice for the same reason a path prefix is
  data_attribute_prefix:
    configurable: data:generator-options DataAttributePrefix, defaulting to 'tb', producing 'data-<prefix>-id'
    not_configurable:
      placeholder_element: the '<tb-boundary>' element htmlbind writes around a boundary
      boundary_id_prefix: the 'tb-' id allocation, an unexported plan field
    effect: a project setting the prefix to 'pw' gets 'data-pw-id' and '<tb-boundary id="tb-1">' in one document, so two naming systems coexist and only one is configurable
author_facing_names:
  what: 'data-tinybind-preserve' and 'data-tinybind-ignore' in runtime.js
  who_writes_them: application authors, by hand, in their own templates
  why_it_is_the_worst_case: unlike a header name these are an authoring surface, so the dependency's brand reaches the application's own source
  not_covered: DataAttributePrefix does not reach them
  reporter_priority: the item it would fix first if only one could be fixed
  related_rules: rule:preserved-client-subtree, requirement:client-navigation link and form interception
global_namespace:
  what: runtime.js installs eight methods and two constants on window.tinybind
  effect: a framework on top can only alias it, so its users call a dependency's name
  ask: export a module or a factory the caller installs under its own name
  precedent: requirement:html-runtime-bootstrap already asks for one namespaced module API rather than global function names
ask:
  one_prefix: one configuration covers the generated attributes, the placeholder element, the boundary id allocation, and the author-facing markers
  headers_read_the_same_way: every protocol name is read from the script tag dataset, or one generated configuration object is accepted
  global: the caller names what the runtime installs
  filename: the asset file name is the caller's, per requirement:browser-runtime-asset-ownership
contradicts:
  concept: decision:update-manifest-transport naming.runtime_binding and naming.hand_written_runtime
  they_say: the runtime hardcodes the prefix because it is framework-owned implementation, and a framework shipping its own build builds it for the configured prefix
  still_true_if: the module ships no runtime
  false_today: the module ships the only runtime, so the escape hatch that made hardcoding acceptable does not exist
  resolution: either the module stops shipping the asset, or the names become discovered; keeping both is what makes the option a lie
constraints:
  - a project overriding nothing emits byte-identical markup and byte-identical generated Go
  - field names after a prefix stay protocol surface and change with decision:update-protocol-version
  - prefix validation stays as decision:update-manifest-transport states it, lowercase letters and digits with no leading or trailing hyphen
acceptance:
  - a deployment renaming the headers is answered by the shipped runtime with no rebuild
  - a project setting the prefix to 'pw' emits one naming system across attributes, placeholder elements, and boundary ids
  - an author writes the preserve and ignore markers under the framework's own prefix
  - two frameworks on one page each install their runtime under their own name
as_built:
  shipped: 2026-08-01
  transport: one data-config JSON object on the script tag, not one dataset attribute per name; three derived header names made a per-name attribute the wrong unit, and a merging caller passes the same object to the factory directly
  runtime: createRuntime(config) builds every name from the configuration, and the file installs createPartialUpdateRuntime rather than installing itself
  global: RuntimeConfig.Global names the instance; an empty name installs nothing, which is what a merging caller sets
  headers: derived from Options.HeaderPrefix on both sides, so the option configures end to end for the first time
  author_markers: derived from Options.DataAttributePrefix, so an author writes them under the framework's prefix
  placeholder_and_ids: htmlbind.WithBoundaryPrefix names the placeholder element and the root id allocation; htmlupdate passes it on every render entry it drives, so the generator's prefix and the runtime's cannot disagree
  render_delta_signature: RenderDelta gained variadic options, which it needed to carry the prefix and had no other way to receive
  validation: Options.Validate, which reports every unusable name at once rather than one per startup
default_rename:
  what: the author-facing markers default to data-tb-preserve and data-tb-ignore, where they were data-tinybind-preserve and data-tinybind-ignore
  why: they now follow the same prefix as every other attribute, and the prefix defaults to 'tb'
  cost: an existing author's markers stop being recognized, which is a breaking change to an authoring surface rather than to a wire format
  reading: the inconsistency was the defect; a document carrying data-tb-id beside data-tinybind-preserve had two naming systems in it, only one of which anything could configure
stricter_than_the_generator:
  found: a custom element name must start with a lowercase letter, while a data attribute may start with a digit
  consequence: the prefix now names an element, so the element rule is the one that applies, and Options.Validate enforces it
  where_it_matters: a prefix like '9pw' generated valid attributes and an element no browser parses
related:
  - requirement:browser-runtime-asset-ownership
  - requirement:render-mode-negotiation
  - decision:update-manifest-transport
open_questions:
  - whether names travel as separate dataset attributes or one generated configuration object, given a header namespace has three derived names
  - whether the placeholder element rename is a protocol version bump, since a live document holds the old element name
  - whether a runtime reading its own names can still fail loudly on a prefix mismatch, which hardcoding gave for free
```

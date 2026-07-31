---
id: decision:route-feature-ownership
type: decision
title: Route Feature Ownership
---
Keep filesystem route discovery and router generation in tinybind, and leave page metadata, sitemaps, and their developer experience to a downstream framework.

```yaml
source:
  - concept:filesystem-html-routing
  - user scope decision 2026-07-27
review_gate: proposed
tinybind_owns:
  - route tree discovery through decision:html-route-file-conventions
  - segment naming through decision:route-segment-notation
  - route pattern derivation and ServeMux registration through requirement:generated-route-registration
  - the generated per-route composer and typed route decoder
  - the exposed route table a framework can read
framework_owns:
  - what metadata a page has and how an author declares it
  - sitemap, robots, and any other aggregated site-wide file
  - enumerating concrete URLs for a dynamic route pattern
  - code generation for all of the above, through requirement:custom-framework-generation-profile
rejected_metadata_declaration:
  shape: a metadata block in the page template, aggregated at generation into head tags and a sitemap
  primary_reason: a page title and description are derived from the same records the page renders, so a separate declaration or a separate metadata method fetches that data a second time
  secondary_reasons:
    - a static block cannot express a title derived from the loaded record, which is the common case
    - aggregating a sitemap requires enumerating concrete URLs, which is application data rather than a template fact
    - both concerns are framework developer experience, not binding or rendering
  status: withdrawn 2026-07-27, before any implementation
head_metadata_channel:
  replacement: requirement:render-time-head-metadata
  shape: the handler passes head content to the render call, after it has already loaded the page data
  gain: one fetch feeds both the document head and the body, because the same values are in scope at the call
  layer: htmlbind owns the channel because it owns requirement:head-merging; the framework decides what travels through it
sitemap_path:
  available_to_framework: the generated route table supplies patterns, methods, and which segments are dynamic
  supplied_by_application: the concrete values a dynamic segment expands into
  conclusion: a framework can build a sitemap from those two without tinybind defining a metadata format
scope_effect:
  removed: three proposed requirements covering metadata declaration, generated metadata routes, and dynamic route enumeration
  remaining: discovery, naming, registration, and composition
  reason: the smaller surface is what a binding and generation library should own
```

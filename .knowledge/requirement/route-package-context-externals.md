---
id: requirement:route-package-context-externals
type: requirement
title: Route Package Context Externals
---
Scan a route package's Go sources for context-taking externals and pass them to the template compiler, so one external declaration means the same thing in a route package as in a templates package.

```yaml
priority: must
source:
  - downstream framework request-context report 2026-08-05
  - requirement:render-context-externals
  - requirement:async-external-functions
review_gate: proposed
status: shipped 2026-08-05; the scan moved to a shared internal package, routetree fills the option per template directory, and the render context wiring below closed the half the report could not see
downstream_id: the reporting framework raised this as ask 1 of a three-ask report; asks 1 and 3 are one defect, and requirement:typed-page-context-parameter carries ask 2
defect_not_feature:
  mechanism: the generator's contextExternals pass reads a directory's .go files and reports which package-level functions declare a leading context.Context; it ships and is tested
  applied: the generator fills htmlbind.GenerateOptions.ContextExternals from it on both paths that compile a template, generateTemplate and the artifact path
  missing: routetree compileTemplate builds its GenerateOptions with the package, the server actions, the action resolver, and the action attribute, and never the map
  coverage: routetree names contextExternals nowhere, in source or in test, which is why the gap survived requirement:render-context-externals shipping
verified:
  method: htmlbind.Generate called directly on one async external declaration, with and without the map
  with_map: RecentMemos(ctx)
  without_map: RecentMemos()
  meaning: htmlbind is correct and complete on both call paths; only the caller differs
symptom:
  sync: an external whose Go function takes a leading context generates a bare call, and the route package fails to build with not enough arguments
  async: the same failure, in the await binding
  live: unaffected, because decision:live-external-signature makes the context mandatory and the emitter writes it unconditionally rather than from the map
subsumes_async_ask:
  reported: the report reads takesRenderContext, whose condition excludes async, and asks for the async clause to be dropped
  finding: that function gates the synchronous expression path only; the await binding path reads contextExternals directly and does prepend ctx
  cause: the reproduction ran in a route package, where the map is empty, so the async symptom is this requirement rather than that condition
  gate_is_correct: an async external may be called only in an await binding per requirement:async-external-functions usage, so excluding it from the sync path removes a call site that is already a generation error
  do_not_apply: dropping the async clause changes only that already-rejected path, and would not have fixed the reported build failure
scope:
  page_and_layout: both compile through the same compileTemplate, so both need it
  directory: the route directory itself, because decision:html-route-go-package-model makes each route directory its own package, which is the unit the generator already scans
  per_call: a layout is scanned in its own directory rather than the page's, so a route does not inherit its ancestor's externals
ownership:
  problem: the scan was unexported in the generator package, and routetree is a separate package that cannot call it
  offered: exporting it, or a ContextExternals field on routetree GenerateOptions for the caller to fill; the reporter had no preference
  chosen: neither. The scan moved to a shared internal package that both import, so one definition decides what counts as a context-taking external and neither caller can drift from the other
  not_an_option_field: a route package's externals sit beside the template by construction, so making the caller supply them would let a tree generate quietly wrong code by omission
  safe_before_compile: the scan is syntactic and skips a file that does not parse, so it runs before the route package compiles, which is the property routetree needs
second_half_not_in_the_report:
  found: implementation 2026-08-05, by serving the fixture page rather than reading its generated source
  defect: the generated route handler called the runtime's render entry with the caller's options only, so the render context defaulted to background and a sync external declaring one read a value belonging to no request
  why_invisible: the reported symptom was a build failure, and passing the map alone clears it; the code then compiles and returns the wrong value, which is worse than the error it replaced
  fix: the default render block passes the request's context as a render option, so the context an external receives is the one the request carries
  ordering: the option goes last, because the caller's options are installed once for the whole mux while this one is per request, so the specific value wins over the static one
  aliasing: the caller's slice is copied with a full slice expression rather than appended in place, because every handler closure shares it and two requests appending at once would write one backing array slot
  seam_preserved: the render block stays overridable per requirement:framework-render-entry, and an override still receives the request identifier to do its own thing
  live_and_async_unaffected: those entries already take a context argument directly rather than reading the render option
constraints:
  - detection stays syntactic, per requirement:render-context-externals; no type checking and no compiled package
  - a route package whose externals take no context regenerates byte-identical Go
  - no change to htmlbind, whose behavior on both call paths is already correct
acceptance:
  - a sync external declaring a leading context, in a route package, receives the render context and the package builds
  - an async external declaring one receives the boundary context in its await binding
  - the same declarations in a templates package keep generating exactly what they generate today
  - a layout's externals are found in the layout's own directory
  - a route tree using no context-taking external regenerates unchanged
as_built:
  scan_home: a shared internal package, imported by both the generator and routetree
  routetree_call: per compiled template, over that template's own directory, so a page and a layout each read their own package
  render_option: emitted by the default render block from the request identifier already in scope
  proof: the whole-tree fixture gained a route whose reader is put on the request context by the test, and both shapes read it back through a real ServeMux; asserting on generated source alone would have missed the background-context defect entirely
related:
  - requirement:colocated-route-logic
  - requirement:generated-html-route-handlers
  - requirement:template-server-functions
  - requirement:framework-render-entry
open_questions:
  - whether a route package's externals should also be visible to an ancestor layout's template, which no rule states today
  - whether a framework overriding the render block should be required to carry the request context, or whether losing it there is the override's business
```

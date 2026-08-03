---
id: requirement:hook-head-contribution
type: requirement
title: Hook Head Contribution
---
Let a reference hook transform declare head entries alongside its rewrite, so a conversion that produces a file no attribute can name still gets that file loaded.

```yaml
priority: should
review_gate: implemented 2026-08-04, needs user approval as built
as_built:
  surface: htmlbind.ReferenceResult.Head, a list of htmlbind.HeadEntry with an Element and an Attributes map
  where: templates/htmlbind/hook.go for the type, the validation, and the contribution accumulator; generator/hooks.go for the cache round trip
  injected_not_merged:
    what: the hook pass synthesizes the entries as ordinary element nodes and appends them to the component's own head declaration, creating one when the component wrote none
    why: everything downstream of that point already carries what an author wrote, so nothing in the merge path, the provenance path, or the emitter needed a case for a hook entry
    consequence: an entry is validated, scoped, and hoisted by the code that already does those things, and a contributed link is indistinguishable from a written one by the time it is emitted
    ordering_falls_out: appending puts contributions after the author's entries without an ordering rule anywhere
  attribute_order: sorted by name, so one registration produces one set of bytes rather than whatever the map iterated
  scope_settled: the component whose template held the matched attribute, as recommended; the accumulator resets between components in the same module
  escaping: a contributed value takes hookValueEscaper, the same replacer the rewrite path uses, so a transform hands over a plain URL
  cache_version: bumped to 2, because a version 1 entry is silent about head entries and that is indistinguishable from a conversion contributing none
  restricted_to: link, script, and style, refused with a diagnostic naming the element
  tested:
    - TestHookContributesHead, the driving case, including the contribution landing after the authored entry
    - TestHookContributesHeadWithoutADeclaration, a component with no head of its own
    - TestHookHeadContributionIsDeduplicated, one stylesheet from three references
    - TestHookHeadContributionsDisagree, two conversions naming one file differently
    - TestHookSkipContributesNoHead
    - TestHookHeadContributionIsRestricted, over title, meta, and base
    - TestHookHeadContributionEscapes, a value trying to leave its attribute
    - TestCachedConversionReplaysHeadContribution, the cache round trip producing identical bytes
  found_while_building:
    dedup_is_per_component: the identity set resets with the component, so two components each contributing one link keep one each, which is what the merge path already expects from two authors writing the same link
    conflict_scope: a disagreement is detected within a module, which is where a hook's own conversions meet; two modules disagreeing is the same situation as two authors disagreeing and is left where that already lives
source:
  - requirement:element-reference-hook open question, carried since 2026-08-02
  - concept:build-time-asset-transforms later_items.preload
  - downstream request from the Popcorn Wave derived asset pipeline, 2026-08-04
the_gap:
  what: ReferenceResult carries a value, a skip, produced files, and a read set, and nothing that reaches the document head
  consequence: a conversion whose output must be loaded by a tag the author did not write has no way to say so, and the produced file is written, declared, and never fetched
  why_the_value_result_cannot_do_it: a value replaces one attribute on the element that already exists; introducing a second element is a different act
  why_the_element_result_would_not_help: it replaces the matched element in place, so a link belonging in the head cannot be expressed by it either, and requirement:element-reference-hook records it as designed and not built
driving_case:
  what: a TypeScript entry point importing a CSS module
  produces: the built JavaScript, which the rewritten script src names, and a companion stylesheet, which nothing names
  today: the stylesheet reaches DerivedAssetDir and no page loads it
  blocks: only this case; a JavaScript build with no CSS import works with what already ships
second_case:
  what: a preload link for an asset the page will fetch
  status: the use the open question was originally written for, and weaker than the driving case, because a missing preload costs latency and a missing stylesheet costs correctness
  covered_by_the_same_shape: yes, which is why one entry form serves both
surface:
  where: a new field on htmlbind.ReferenceResult, so a transform declares it with the same return that carries the rewrite
  why_not_a_second_callback: the head entry depends on how the conversion turned out, exactly as the rewrite does, and a separate call would have to redo or cache what the transform already knows
  shape: a list of head entries, each naming an element, its attributes, and nothing else
  restricted_elements: link, script, and style only; a hook contributing a title, a base, or a meta charset would be rewriting the document rather than loading what it produced
  no_children: an entry declares attributes and no text content, because a transform that wants to inject a script body is writing the page rather than referencing a file
receiving_path:
  target: requirement:head-merging, which already carries head contributions from component declarations to the document
  provenance: requirement:head-contribution-provenance, so a merged entry names the hook that produced it and a conflict diagnostic can say where it came from
  arrival: a contribution enters at the same point a component head declaration does, so nothing downstream of the merge distinguishes them
  ordering: after every component contribution, because a hook entry is a consequence of a rewrite that already happened and never something an author wrote
scope_question_to_settle:
  problem: a contribution is produced while compiling one module, and the head it belongs in is the document that renders that module's component
  candidate_module: the entry attaches to the component whose template held the matched attribute, which is the narrowest correct answer and the one that survives a component being rendered on some pages and not others
  candidate_run: the entry attaches to every document in the run, which is wrong for a component used by one page out of forty
  recommended: module, and specifically the component, matching how requirement:static-asset-extraction already binds an extracted asset to the template that declared it
  consequence: a hook matching inside a layout contributes to every page that layout serves, which is correct and needs no special case
deduplication:
  rule: two conversions declaring an identical entry contribute one
  why: one stylesheet imported by two entry points is one link, and the run memo already makes the second conversion a memo hit rather than a second call, so the duplicate arrives only across modules
  identity: the element name and the full attribute set, compared as written
  conflict: two entries sharing a href or src and disagreeing on any other attribute is a generation error naming both, following the requirement:element-reference-hook overlap rule rather than letting one win
caching:
  requirement: a head entry is part of the conversion outcome, so it is stored and replayed with the value, the files, and the skip
  consequence: the cache entry shape changes, which bumps the stored version so an older entry is ignored rather than misread as one carrying no head
  skip_case: a skip contributes nothing, because a conversion that declined produced no file to load
determinism:
  order: contributions are emitted in first-occurrence order within a module and module order across the run, matching how the rewrite report is already built
  check: identical inputs produce an identical merged head, so the order-independent byte comparison requirement:component-asset-requirements states for `--check` is unaffected
constraints:
  - a project registering no hook contributes nothing and regenerates byte-identical Go
  - a contribution never reaches a document that does not render the contributing component
  - a hook cannot replace or remove an existing head entry, only add one
  - nothing is looked up at request time; the merged head is generated
acceptance:
  - a transform declaring a stylesheet link produces a page whose head loads it, once, in a document that renders the component
  - a component contributing through a hook and not rendered on a page leaves that page's head unchanged
  - two entry points importing one CSS module produce one link
  - two conversions declaring the same href with different attributes fail generation naming both
  - a cached conversion replays its head entry without calling the transform
  - a skip contributes no entry
  - a hook entry cannot introduce a title, a base, or a meta charset
related:
  - requirement:element-reference-hook
  - requirement:derived-asset-generation
  - requirement:head-merging
  - requirement:head-contribution-provenance
  - requirement:static-asset-extraction
  - concept:build-time-asset-transforms
open_questions:
  - whether a contribution declared inside a layout should be distinguishable in the report from one declared in a page, given both are correct
  - whether a style entry is worth allowing at all, since a transform holding CSS bytes could produce a file and link it instead
  - whether a contribution belongs in GenerateResult.Rewrites, which reports what a hook did to attributes and says nothing about what it added to a head
```

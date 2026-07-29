---
id: requirement:head-contribution-provenance
type: requirement
title: Head Contribution Provenance
---
Report which component declared each head contribution, so a caller that cannot deliver one can name the component to change.

```yaml
priority: should
source:
  - requirement:head-merging
  - framework owner review 2026-07-30, petitweb-go
review_gate: approved 2026-07-30
status: shipped; Plan.HeadSources, the two accessors, and per-tag Head granularity are implemented
problem:
  caller: a response with no document shell has no head to merge into, so a contribution is rejected rather than dropped; petitweb-go decision:fragment-head-rejection is that caller
  message: the rejection could only print the head markup, so an author grepped their templates to find which component's style block was responsible
  available_but_discarded: generation already knows the declaring component and the tag position, and threw both away when it rendered the contribution to a string
surface:
  plan_field: HeadSources, an exported '[]string' beside Head, Ops, HasAwaitBlock, and Cache
  accessors: HeadSources on Fragment and on Wrapper, beside Head
  form: 'ComponentName (file:line:col)', ready to print in a diagnostic
  parallel_list:
    rule: same length and same order as Head, so index i of either describes the same tag
    why_not_a_struct: requirement:fragment-capability-introspection already settled that an additive accessor beats a summary type, and requirement:client-managed-head already specifies HeadIDs as a parallel view of Head
  merged_view:
    absent: MergeHead drops duplicates, so no source list can stay parallel to its result
    instead: a caller asks the chain member it holds, which is the same principle the await flag follows
granularity:
  change: Head became one entry per contributed tag; it was one concatenated string per contributing component
  why: requirement:head-merging deduplicates on the tag and states identity is per tag, so per-component strings could not satisfy it
  defect_fixed:
    acceptance: 'two components declaring the same stylesheet emit one link'
    was_broken: two components each linking a shared stylesheet and then declaring their own styles produced two different concatenated strings, so neither MergeHead nor generation collapsed the shared link
    now: the shared link collapses and both style blocks survive
  dedup_layers:
    generation: transitiveHead collapses a repeated tag within one member's reachable set, and the first declarer keeps the attribution
    runtime: MergeHead still collapses across chain members, because a member cannot see the members it is composed with
  whitespace: authoring whitespace between contributed tags is dropped, because the merged head is assembled from the tags rather than from the authored block
output_stability:
  rule: HeadSources is emitted only beside a non-empty Head
  effect: a project with no head contribution anywhere regenerates byte-identical Go
  measured: every existing generator fixture had 'Head: nil' and none changed; examples/demo kept its self digest across all four generated files
rejected_alternatives:
  generation_time_call_site_check:
    what: have the framework generator reject a 'WriteHTMLFragment' call whose component argument contributes to the head, at generation time instead of per request
    why_not:
      - the generator has no HTML render call operation; data:generator-options Calls covers request bind, response write, stream, JSON, rows, config, route, and error response, and petitweb-go registers none for its HTML writers
      - it would therefore need a new tinybind call operation plus argument-to-component resolution, so it is not the framework-local change it looked like
      - detection reaches only the direct-call form, so a passing generate run is not a guarantee, while the runtime check already fails loud and pre-commit on the first request
    status: declined 2026-07-30; the message fix delivers the same information at the moment of failure
  comment_marker:
    what: emit a source comment inside the contributed markup
    why_not: it ships attribution to every client in production output
  head_string_only:
    what: keep Head per component and add nothing
    why_not: it leaves the caller grepping, and it keeps the requirement:head-merging dedup defect above
verified:
  fixture: testdata/templates/htmlbind/headcontrib
  runtime_tests: TestHeadTagsCarryTheirSource, TestSharedStylesheetCollapses, TestWrapperExposesTheSamePair, TestNoContributionReportsNothing, TestMergeStillDeduplicatesAcrossMembers
acceptance:
  - a component with one head element reports one entry per tag it declared
  - HeadSources has the same length and order as Head for every component
  - a component reporting no contribution reports an empty source list rather than an empty entry
  - two components sharing a stylesheet link emit that link once, with the first declarer attributed
  - a wrapper exposes the same pair as a fragment
  - a project with no head contribution regenerates byte-identical Go
related:
  - requirement:head-merging
  - requirement:client-managed-head
  - requirement:fragment-capability-introspection
  - decision:generated-render-plan
open_questions:
  - whether the form should carry the component and position as separate fields once a caller needs to reformat it
```

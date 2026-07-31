---
id: decision:library-component-seams
type: decision
title: Library Component Seam Requests From The Downstream Framework
---
Accept the four component-seam asks from the framework built on this module, and record that two of them are the design this catalog already holds, reached independently from three features that hit the same closed seam in one week.

```yaml
source:
  - downstream framework component seam report 2026-07-31, against v0.2.8
  - decision:framework-integration-seams
  - decision:live-integration-seams
review_gate: proposed
round:
  when: 2026-07-31, the third round from this reporter and the second on the same day
  previous: generation seams 2026-07-30, live integration 2026-07-31
  reporter_position: not blocked; the CSRF plumbing works, the runtime reference works, and the scriptless handoff has a shape that works and is not conforming HTML
  question_asked: whether the seam is worth opening before three more features arrive at it
  driving_features: a CSRF token, a scriptless-client handoff marker, and a capability that needs a script
  reporter_ids: the report carries the same content in machine form as three requirements named render-time-request-context, framework-provided-components, and framework-head-contribution, recorded in that project's catalog rather than this one
verification:
  method: every claim checked against the v0.2.8 source, and against the current tree where the two differ
  sync_external_has_no_context: confirmed; the emitter prepends ctx only for await bindings, and the ordinary expression path emits a bare call
  async_optional_live_required: confirmed; the discovery pass exists and the live call shape hardcodes the leading ctx
  components_come_only_from_local_template_files: confirmed; requirement:template-file-scope still leaves what an external declaration may name an open question, and no registration seam is implemented
  head_merge_injects_nothing: confirmed
  style_hoisted_script_not: true at v0.2.8, closed for the authoring case on 2026-07-31 by requirement:static-asset-extraction script extraction, which landed after the reported version
  builtin_elements_unimplemented: confirmed; requirement:builtin-element-registration, requirement:builtin-element-lowering, and requirement:render-value-provider are all designed and none is built, so the reporter could not have found them by reading the source
accepted:
  - ask: a synchronous call may receive the render context
    verdict: new, and it is the round's cheapest item
    becomes: requirement:render-context-externals
    value: high; it is the same parameter two other external forms already take, and it removes the plumbing from every form-bearing page
    cost: small but larger than reported, because a plan value closure has no ctx in scope; the fix is a context-carrying op variant rather than one more call argument
  - ask: components the library implements, callable by name
    verdict: already designed here, and the design answers it in the terms the reporter used
    covered_by: requirement:builtin-element-registration, requirement:builtin-element-lowering, requirement:render-value-provider, data:builtin-element-definition
    agreement: csrf-token is the worked example in this catalog and the first example in the report, arrived at independently
    added_this_round: a declared vary axis, and head-or-body placement stated on the definition rather than left to the insertion context
    value: highest of the four, because it is the item both sides reached on their own and the one the other three lean on
  - ask: assets a component brings with it
    verdict: new, and the largest
    becomes: requirement:component-asset-requirements
    value: it changes what a third-party component library can be, which nothing else in the round does
    cost: highest; an embedded asset table, a plan-level required set, and a caller-supplied URL function
  - ask: head contributions from the caller, not only from components
    verdict: already designed here
    covered_by: requirement:render-time-script-contribution for script, requirement:render-time-head-metadata for the rest of the head
    agreement: both already state the reporter's constraint, that a call argument is available strictly before the head pass so nothing about streaming changes
    added_this_round: the node-kind gap below, which is what actually blocks the handoff marker
new_findings:
  noscript_not_an_allowed_head_node:
    found: head contributions accept link, meta, style, script, and title; noscript is rejected, and every contributed attribute must be static
    why_it_matters: the scriptless handoff is one noscript refresh in the head, so the feature is blocked by the allowed-node set and not only by the missing caller channel
    consequence: requirement:render-time-head-metadata must state which node kinds a caller may contribute, and requirement:head-merging must say whether noscript joins the authored set
    scale: small, and it is the difference between the reporter shipping the handoff and continuing to hold it
  fragment_returning_sync_external_already_works:
    found: `external CSRFField(): html` compiles in v0.2.8 and lowers to a Slot op, so the result renders as a subtree under the ordinary context checks
    consequence: the report's secondary ask under ask 1 is already satisfied; only the context is missing
    caveat: a Fragment produced while rendering contributes no head, which is why ask 3 cannot be met by returning one
  head_of_a_render_time_fragment:
    reading: this is the same defect requirement:fragment-capability-introspection recorded against requirement:cross-template-components for slot-carried fragments, seen from the asset side
implemented_2026_07_31:
  what: the first two items of the sequence below, plus the caller-head channel the fourth ask needs
  requirement:render-context-externals: shipped; see its as_built
  head_noscript: shipped in both the authored set and the caller set, so the scriptless handoff has a conforming shape for the first time
  requirement:render-time-head-metadata: shipped as WithHead over typed nodes, which is the fourth ask's channel; requirement:render-time-script-contribution stays unimplemented, and an external script tag is expressible through WithHead in the meantime
  still_open: the registration seam of the second ask, and every part of requirement:component-asset-requirements
  measured: every generator fixture and golden file regenerated unchanged, so no accepted item cost a project that uses none of it
sequencing:
  chosen: context first, then the head node kinds, then the registration seam with its vary and placement declarations, then assets
  why_context_first: it is small, additive, and blocks nothing else that has to wait for it; it is also the item the reporter itself put first
  why_node_kinds_second: it is the smallest change in the round and it unblocks a feature that is being held rather than shipped
  why_registration_third: it is the item both catalogs already describe, so it needs implementation rather than a design round, and asks 3 and 4 both read better once it exists
  why_assets_last: it is the largest, it has the most open questions, and its hardest property is the static required set, which wants its own design round
  not_a_disagreement: the reporter sequenced by what each ask unblocks; this sequences by readiness, and both put assets last
constraints_confirmed:
  unused_is_free: every accepted item states byte-identical output for a project using none of it, which is the decision:framework-integration-seams test
  escaping_unchanged: component output and contributed markup keep rule:template-context-safety escaping from their declaring context
  no_inline_script: already the requirement:static-asset-extraction and requirement:framework-script-contribution rule, restated by requirement:component-asset-requirements
  no_cloaking: reading the request decides delivery and never content; the module states the equivalent rule where the mode header never reaches template scope
  no_authorization_in_a_component: unchanged; requirement:render-value-provider already forbids a provider writing the response
  tinygo: the embedded asset table exists because of it, and no accepted item adds reflection, a filesystem read, or an init-order dependency
  fragment_path_honesty: requirement:component-asset-requirements carries the reporter's condition, that a required asset is reported on the fragment path rather than discovered in a browser
not_asked_for:
  - children or slots on a registered component, and anything asynchronous in one
  - a plugin surface for application-authored components, which concept:framework-template-extensions already lists as a non-goal
  - the module choosing URLs, serving files, or setting cache headers
principle:
  applies: the decision:framework-integration-seams rule unchanged, widen a seam whose default output stays identical and whose contract stays the caller's
  new_reading: when a downstream reaches a design this catalog already holds, from features rather than from the documents, the design's priority moves and its shape does not
related:
  - concept:framework-template-extensions
  - requirement:render-context-externals
  - requirement:component-asset-requirements
  - requirement:builtin-element-registration
  - requirement:render-time-head-metadata
```

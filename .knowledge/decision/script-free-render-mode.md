---
id: decision:script-free-render-mode
type: decision
title: Script Free Render Mode
---
Offer one document-level mode that turns off every feature depending on a browser runtime at once, rather than a switch per feature.

```yaml
source:
  - requirement:template-server-functions
  - decision:server-action-lowering
  - user default-inversion decision 2026-07-29
review_gate: proposed
default:
  chosen: the scripted mode, where a browser runtime is assumed present
  inverts: requirement:template-server-functions previously treated the script-free path as the baseline and the scripted path as an optimization
  why: a native form submit is a full navigation, so scroll position, focus, and client state reset and the reload is visible; the earlier argument for a script-free baseline is not refuted, only outweighed by that cost
  scope_of_inversion: which mode is default; both remain supported
nickname: bot mode, because a crawler is the caller that most obviously has no runtime
audience:
  - a crawler or snapshot renderer
  - HTML delivered by mail, where script never runs
  - a static export
  - a project choosing a script-free posture for its own reasons
turns_off:
  runtime_bootstrap: requirement:html-runtime-bootstrap emits neither the runtime script nor the token metadata
  partial_update: requirement:partial-update-boundaries emits no boundary markers and no data:component-update-manifest
  async_streaming: requirement:suspense-html-streaming does not run; a decision:async-boundary-syntax await settles in place instead
  element_actions: the non-form lowering of decision:server-action-lowering has no caller
turns_on:
  nothing_about_actions: as of 2026-08-12; the three entries below moved out of this mode and are now unconditional, per requirement:native-action-form-submit
  was_form_post: a form carrying server-action posts to the page's own pattern rather than to the direct entry point
  was_redirect: the 303 post-redirect-get default of requirement:template-server-functions, so a reload does not resubmit
  was_form_csrf: the hidden field transport of policy:html-update-csrf-protection, because a plain form cannot set a header
lowering_sets:
  status: removed 2026-08-12; this mode selects no lowering set, because decision:server-action-lowering both_sets_emitted emits them together
  was: the scripted set lowered every element alike to the URL attribute, and the script-free set emitted the form and submit-button markup, with the mode choosing
  why_the_gain_argument_fell: it named the selector, the page-pattern POST registration, and the render-time request path channel as script-free-only costs; the third does not exist, per decision:server-action-lowering form_action_url, and the first two are static markup and one route registration
  what_this_mode_still_does: four of its five jobs, all of them about suppressing a runtime rather than about what markup an action lowers to
async_already_present:
  fact: the blocking await path is what the synchronous render entry already takes
  failure: that path returns the failure instead of writing a fallback, so a caller rendering into a buffer can still choose the status
  consequence: the mode selects a render entry rather than adding an execution model
switch_placement:
  chosen: generation time, so one build compiles one lowering set
  rejected_per_request: selecting the mode from a request header or user agent
  why_rejected: serving different HTML to crawlers than to people is cloaking, and a render-time switch would also force both lowering sets to be compiled and branched
  cost_accepted: one build serves one mode, so an application wanting both deploys or mounts them separately
cache_identity:
  rule: the mode participates in the component version that validates requirement:component-output-cache and requirement:layout-reuse-boundaries entries
  reason: the same component emits different markup in each mode, so output cached in one is invalid in the other
  narrowed: 2026-08-12; an action no longer contributes to that difference, because both lowerings are emitted in either mode. The rule survives on the boundary markers, the async placeholders, and the bootstrap, which still differ.
downstream_dependency:
  reported: 2026-07-30, through decision:framework-integration-seams
  fact: a downstream framework carries an acceptance condition requiring pages to work with no browser runtime
  effect: a route adopting the server actions of requirement:template-server-functions cannot meet that condition until the form markup is emitted
  status: design decided here and unimplemented, so the gap is sequencing rather than shape
  stated_because: a complete-reading design hides the fact that adopting one feature suspends someone else's acceptance condition
  raised_again: 2026-08-11, through decision:client-handler-seams, with the reading sharpened; the emitted form is not inert without a runtime but a working GET form, so the condition is not merely unmet, it fails silently
  moved_out: 2026-08-12; requirement:native-action-form-submit owns the fix and it is no longer a phase of this mode, because the caller cannot take a per-build mode without turning one application into two deployments
  read_this_way: the request was for the markup rather than for the mode, and that is the distinction two rounds took to surface
unaffected:
  framework_script: requirement:framework-script-contribution and requirement:render-time-script-contribution stay available; the mode suppresses the tinybind runtime, not whatever the application ships
authoring_rule:
  status: retired 2026-08-12, never built as an error
  was: under this mode server-action must sit on a form, and on any other element it is a generation error naming the position and saying to wrap it, decided 2026-07-29
  why_retired: with both lowerings emitted there is no mode for the rule to be scoped to, so it would apply always, and a bare button is the common shape under today's default; the rule would break the ordinary case to describe a missing fallback
  now: a bare button compiles and lowers to the scripted attribute alone, with an opt-in diagnostic naming the missing native path, per requirement:native-action-form-submit
  why_author_still_decides: only the author knows whether an ancestor form already encloses the element and whether the action needs inputs at all, which is why the diagnostic never becomes an error
  portability:
    one_way: a form carrying server-action reaches its handler with or without a runtime; a bare button needs one
    guidance: put server-action on a form when a client with no runtime must reach it
    no_longer_about_modes: the guidance is about which clients can invoke the element, which is a property of the markup rather than of a build setting
rejected_auto_wrap:
  shape: the generator wraps a bare button in a generated form
  blocker: a form cannot nest, and whether an ancestor form encloses the element is invisible across component boundaries, so the wrap is unprovable in general
  failure_mode: a browser silently drops the inner form, producing a button that does nothing rather than invalid markup anyone would notice
  layout: solvable with display:contents, the trick the async boundary placeholder already uses, but that is the smallest of the three problems
rejected_form_attribute:
  shape: an empty form emitted at a document slot, with the button associated through the HTML form attribute
  correct: yes; the form attribute associates a control with a form anywhere in the document and sidesteps nesting entirely
  blocker: a button rendered inside a loop needs a per-iteration identity through rule:component-instance-identity and its form hoisted to a document slot, and the renderer has no deferred emission mechanism
  status: the shape to return to if deferred emission ever exists for another reason
buffering:
  rule: the whole document renders into a buffer before any byte reaches the response
  decided: 2026-07-29
  gain_status: no failure is ever after-commit, so the failure_after_commit case of requirement:generated-route-registration does not arise in this mode and every error still chooses its status
  gain_length: a complete document has a known length, which suits a crawler, a mail body, and a static export alike
  fits_existing: the blocking await path already returns its failure instead of writing a fallback, precisely so a caller rendering into a buffer can discard it
  cost: one document held in memory per request, accepted because the audience of this mode is not a latency-sensitive interactive client
  streaming: no flush point exists in this mode, so the streaming entries and their flush calls do nothing
constraints:
  - the mode is one setting, so no combination of half-disabled client features is representable
  - the generator still writes no structural markup; it only replaces the attribute it reserved
```

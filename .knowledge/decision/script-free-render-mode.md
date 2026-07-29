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
  form_post: a form carrying server-action posts to the page's own pattern rather than to the direct entry point
  redirect: the 303 post-redirect-get default of requirement:template-server-functions, so a reload does not resubmit
  form_csrf: the hidden field transport of policy:html-update-csrf-protection, because a plain form cannot set a header
lowering_sets:
  scripted: every element lowers alike, to the URL-carrying attribute; a form is intercepted by the runtime and needs no action, method, or hidden field
  script_free: the form and submit button lowerings of decision:server-action-lowering
  gain: the selector, the page-pattern POST registration, and the render-time request path channel are needed by the script-free set only, so the scripted set carries none of them
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
unaffected:
  framework_script: requirement:framework-script-contribution and requirement:render-time-script-contribution stay available; the mode suppresses the tinybind runtime, not whatever the application ships
authoring_rule:
  rule: under this mode server-action must sit on a form; on any other element it is a generation error naming the position and saying to wrap it
  decided: 2026-07-29, closing whether a bare button is an error or dead markup
  why_author: only the author knows whether an ancestor form already encloses the element and whether the action needs inputs at all
  common_case: a bare button is the usual shape under the default mode, so this is the cost the script-free mode charges
  portability:
    one_way: a form carrying server-action works under both modes; a bare button works under the default mode only
    guidance: put server-action on a form when the script-free mode is a possibility
    supersedes: an earlier constraint claiming a template compiles unchanged under either mode
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

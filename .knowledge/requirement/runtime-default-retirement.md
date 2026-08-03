---
id: requirement:runtime-default-retirement
type: requirement
title: Runtime Default Retirement
---
Ship no browser asset by default, move the reference client out of this module, and make a missing runtime a startup error rather than a silently dead page.

```yaml
priority: must
source:
  - downstream framework caller-owned runtime report 2026-08-04, against v0.3.3
  - requirement:client-update-rollout m1 deviations.runtime_delivery fully_retired_when
  - user sequencing decision 2026-08-04
review_gate: proposed
completes: requirement:browser-runtime-asset-ownership, which exported the bytes and made serving switchable on 2026-08-01 and left the default shipping one
gate_as_recorded: requirement:client-update-rollout fully_retired_when names requirement:html-runtime-bootstrap selecting and injecting the runtime, at which point a direct user has a replacement
gate_as_chosen:
  what: the replacement is requirement:update-wire-contract plus reference code in the framework-facing docs, not the injection machinery
  why: the recorded condition is that a direct user has a replacement, and the embedded bytes with serving opted back on already are one; the contract and the reference code are what a framework author needs instead
  ordering: the contract lands, then the reference code is written against it, then the default flips
  cheaper_than_recorded: this removes requirement:html-runtime-bootstrap from the critical path rather than merely reordering around it
reference_client:
  where: docs/httpbind_reloadable_componet.md and its Japanese mirror, as reference code
  why_there: it already carries the live and runtime material, and requirement:live-mode-token-contract names it as the document whose mode and availability tables must be corrected, so the reference code and the tables it illustrates stay in one place
  audience: a framework author writing its own client, who reads it beside requirement:update-wire-contract rather than copying it
  decided: 2026-08-04 by the user
  restores: decision:client-runtime-ownership reference_runtime.today, which already put a conforming client in the guides as code the caller copies or reimplements
vendoring_objection_dissolved:
  raised_here: reference code a direct user copies is vendoring, which requirement:browser-runtime-asset-ownership rejected for the framework case and this would reintroduce for the direct user
  why_it_does_not_apply: a direct user is not the audience for the reference code; the module keeps embedding the bytes, so opting serving back on is one option field and no copy exists to drift
  who_copies_then: a framework author, who is copying a starting point for a client it was going to write anyway and who owns the result deliberately
  what_would_reintroduce_it: removing RuntimeSource from the module, which this requirement does not do
  reading: the objection was against losing the bytes, not against publishing reference code; only the default serving flips
startup_diagnostic:
  problem: with the default shipping nothing, a direct user who changes no code gets a program that compiles and a page that silently stops updating
  fix: Options.Validate fails at startup when the composition uses update features, no runtime is served, and CallerOwnsRuntime is not set
  why_there: decision:update-runtime-ownership-seams found Options.Validate to be the right home for every unusable option rather than only a duplicate kind
  effect: the real retirement condition becomes that the one-line fix is discoverable, rather than that a replacement mechanism exists
  smallest_item_here: it is what turns a breaking default into a visible one, and nothing else in the change depends on it
what_the_module_keeps:
  bytes: RuntimeSource and RuntimeAsset, so a framework still merges rather than copies
  switch: Options.CallerOwnsRuntime, whose default inverts
  serving: RuntimeHandler as a convenience for a caller that opts back in
  contract: requirement:update-wire-contract, which is what module ownership of the protocol now means
constraints:
  - a direct user who does nothing gets a startup error, never a dead page
  - a caller already setting CallerOwnsRuntime sees no change
  - this is the only breaking item in the change, and the module is pre-1.0
acceptance:
  - the module default serves no browser asset and writes no script tag
  - a project using update features with no runtime fails to start
  - a direct user reaches a working client by opting serving back on, never by copying reference code
  - the framework-facing docs carry reference code a framework author can read against the contract
as_built:
  shipped: 2026-08-04
  default: Options.ServeRuntime, off by default; Mount registers no asset route and ScriptTag returns empty unless it is set
  diagnostic: Options.Validate refuses both flags set and neither set, so a build that would serve a silently dead page fails at startup instead
  why_two_flags_rather_than_an_inversion: a single inverted flag would have let a caller say nothing and mean something; the question is asked because its wrong answer is the only one here that is invisible at run time
  bytes_stay: RuntimeSource, RuntimeAsset, and RuntimeHandler are unchanged, so a direct user opts serving back on with one field and never copies anything
  reference_client: docs/httpbind_reloadable_componet.md and its Japanese mirror gained a section on writing your own, naming the four obligations a conforming client cannot break
related:
  - requirement:browser-runtime-asset-ownership
  - requirement:client-update-rollout
  - requirement:html-runtime-bootstrap
  - requirement:update-wire-contract
  - decision:client-runtime-ownership
open_questions:
  - what requirement:html-runtime-bootstrap still covers once the default ships nothing
```

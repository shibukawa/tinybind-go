---
id: requirement:slot-fragment-head-merge
type: requirement
title: Head Of A Fragment Passed As A Parameter
---
Walk the head contributions of a fragment supplied through a component's parameter struct, or report them, so a slot-filled component's styles are never silently dropped.

```yaml
priority: should
source:
  - downstream framework composition seam report 2026-08-02, against v0.3.1
  - requirement:fragment-capability-introspection slot_parameter_propagation
review_gate: proposed
promoted:
  from: a finding recorded against requirement:cross-template-components and read again from the asset side in decision:library-component-seams
  why_now: a second reporter reached it from a third direction, the incompleteness of a safety check, which is the reading that makes it a defect rather than a scope note
defect:
  mechanism: Bind copies only the plan's own head, so a Fragment passed as a slot argument inside a params struct is not walked by the binder
  effect: the composed value reports the outer component's contributions and none of the slot's
why_the_third_reading_is_the_sharp_one:
  existing_check: a response with no document shell rejects a head contribution rather than dropping it, per requirement:head-contribution-provenance; the downstream's own fragment-head rejection decision is that caller
  incompleteness: the check can only reject what it is told about, so a slot-supplied component's styles are dropped before the check sees them
  reading: the failure is not that a contribution goes unmerged, which a caller might notice; it is that a guard designed to make the loss loud stays silent for exactly the cross-file composition case it was built for
  scale: the case is less common than the ordinary one, which is why requirement:fragment-capability-introspection shipped without slot plumbing, and it is the case a component library produces
ask:
  either: walk parameter-carried fragments when merging
  or: report them, so a caller can refuse rather than lose them
  reporter_accepts: either, and the second is what makes the existing guard honest
shapes_already_considered:
  plan_carried_slot_accessor: requirement:fragment-capability-introspection names it, nil for a component with no slots, so it stays zero cost and adds no interface boxing
  variadic_bind: passing slot fragments as extra Bind arguments; typed and cheap, and a hand-written call can silently omit one
  reading: the first was already preferred there and nothing in this round changes that
constraints:
  - a component with no slots costs nothing, per requirement:tinygo-wasm and decision:reflection-free
  - a project using no slot-carried fragment regenerates byte-identical Go
  - whatever is walked for head must be walked for requirement:component-asset-requirements too, or the same hole reopens one layer down
acceptance:
  - a slot-supplied component's head contributions are delivered or reported, never dropped
  - a fragment response carrying a slot-supplied contribution is refused by the same guard that refuses a direct one
  - a component with no slots produces identical output and identical generated code
related:
  - requirement:cross-template-components
  - requirement:head-merging
  - requirement:component-asset-requirements
  - decision:library-component-seams
open_questions:
  - whether reporting alone is enough for the asset case, where a caller that cannot deliver an asset has nothing to do but refuse
```

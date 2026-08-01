---
id: rule:live-boundary-content
type: rule
title: What May Live Inside A Live Boundary
---
Keep browser-owned state out of a live boundary, because its subtree is replaced on the server's clock and nothing warns the user first.

```yaml
source:
  - requirement:live-boundary-rendering
  - user authoring guidance 2026-07-30
review_gate: proposed
status: the diagnostic is shipped for form, input, textarea, and select in a live clause's primary subtree, including nested if, for, slot default, and nested boundary subtrees. The annotation escape hatch, the transitive walk through component calls, and every residual case below remain guidance.
problem:
  replace_destroys: applying a replace operation discards the value of an input, the caret position, the selection, and the element identity the browser attached that state to
  no_user_signal: a navigation is something the user asked for, so losing an unsaved field is at least explicable; a live delivery arrives on the server's clock while the user is typing
  frequency: requirement:component-delta-rendering already had this exposure once per navigation, and a live boundary turns it into once every few seconds
rule: a live boundary renders output, not input
diagnostic:
  detectable: the compiler already walks a live clause's primary subtree for rule:template-context-safety, so it can see a form control there without new analysis
  reported: form, input, textarea, select, and any element carrying contenteditable, inside a live clause primary subtree
  severity: an error rather than a warning, because the failure mode is silent loss of something the user typed
  escape_hatch: an explicit decision:template-annotation-syntax annotation on the boundary, for a control that legitimately resets, such as a disabled input showing a rendered value
  not_in_fallback_or_recover: those subtrees are rendered once and are not replaced by a delivery, so the rule does not apply to them
authoring_shape:
  pattern: the form stays outside the boundary and the live data goes inside it
  parameter_path: api:client-component-update is how a control outside the boundary changes what the boundary shows, per requirement:live-boundary-rendering parameter_interaction
  reason: that keeps the input's DOM stable while the output re-renders, which is the split the protocol can actually guarantee
residual_cases:
  not_covered_by_the_diagnostic:
    focus: a link or button inside the boundary that holds focus loses it when its subtree is replaced, and no static rule can forbid a focusable element in output
    scroll: the boundary's own scroll position, and the page's position relative to a boundary that changes height
    media: a playing video or audio element inside the boundary restarts
    animation: a CSS animation or transition restarts from its initial state
  status: authoring guidance, not enforcement; a caller's client runtime may restore focus if it chooses, and the module states the exposure rather than hiding it
  granularity_helps: an unchanged nested boundary is omitted, so a delivery that only appends does not touch the nodes holding this state; the exposure is real for a replace and small for an insert
announcement:
  decided: authoring guidance, not a module mechanism; the author writes the aria attributes because only the author knows whether an update is worth interrupting for
  by_use_case:
    timer_paced: a dashboard, gauge, or clock re-rendering every few seconds is aria-live off, because announcing a changing number on the server's cadence makes the page unusable with a screen reader
    arrival_paced: a chat log, notification list, or feed is polite, because the arrival is the information and the user does want to know
    role_log: role log is the precise fit for a chat or feed and already implies polite, so it is preferred over spelling out aria-live
    assertive: essentially never correct for a live boundary; it interrupts whatever the user is reading, which is warranted for an error or a session expiry rather than for content
  no_default:
    reason: neither default is safe, because silence fails a chat and noise fails a dashboard, and nothing in the template says which one a boundary is
    consequence: the module emits no politeness of its own and does not get in the author's way
  placement:
    rule: put the attribute on an element that survives a delivery, meaning a wrapper the author writes around the boundary, never inside the primary subtree
    why: an aria-live attribute inside the replaced content is destroyed and recreated with it, which resets the live region and can lose the announcement entirely
    shape: the author's own element wraps the boundary and carries role or aria-live, so nothing has to be threaded through the generated placeholder
  granularity_is_an_accessibility_property:
    mechanism: a screen reader announces from DOM mutations, not from what the server rendered, so an insert announces the new item and a replace announces the whole region
    consequence: a chat log whose delta degrades to the requirement:component-delta-rendering nearest-safe-ancestor replacement re-announces every message on every delivery
    implication: keeping structural operations granular is what makes polite announcement usable, so rule:live-boundary-delivery suppression is not only a bandwidth concern
    atomic: aria-atomic stays false, its default, so only the changed nodes are read; setting it true is right for a short status region and wrong for a log
  reduced_motion:
    scope: prefers-reduced-motion governs any transition a caller's client script animates when applying an operation, and nothing about delivery pacing
    reason: an update is not motion; there is no media query expressing a preference for fewer content changes
    owner: the caller, since decision:client-runtime-ownership puts the applying script there
open_questions:
  - whether the diagnostic extends to a component call whose body contains a control, which needs the transitive walk decision:component-capability-lowering already performs
  - whether the guidance is worth a generation-time hint when a live boundary renders a list, which is the shape most likely to want role log
```

---
id: requirement:layout-reuse-boundaries
type: requirement
title: Automatic Layout Reuse Boundaries
---
Treat generated route layouts as automatic partial-update boundaries while keeping output caching explicit.

```yaml
source: concept:filesystem-html-routing
boundary:
  activation: every layout in data:html-render-route-plan
  identity: stable route-prefix and layout declaration identity through rule:component-instance-identity
  direct_client_mutation: disabled unless separately enabled by requirement:boundary-parameter-updates
reuse_model:
  frame: layout markup and own typed inputs excluding child slot output
  frame_validator:
    - layout generated-code version
    - scoped path parameters and other explicit layout inputs
    - canonical wrapper structure excluding request-unique markers
  child_slot: independent descendant manifest and validator
navigation:
  unchanged_frame: retain existing wrapper DOM and update only child slot descendants
  changed_frame: replace layout boundary and regenerate its descendant manifest
  changed_deeper_parameter: does not invalidate an ancestor layout that cannot receive that parameter
  slot_presence_change: an absent requirement:html-slot-syntax slot leaves no anchor, so replace the nearest enclosing boundary instead of patching that position
cache:
  automatic_boundary: does not imply requirement:component-output-cache
  explicit_full_output_cache: includes slot identity or bytes through rule:component-capability-combinations
safety:
  - frame and slot splitting preserves valid HTML structure and rule:template-context-safety
  - retained wrapper cannot preserve server-only state outside generated manifest rules
acceptance:
  - search-only page updates can reuse all layout frames that exclude search parameters
  - navigating between sibling dynamic IDs reuses layouts above the changed dynamic segment
  - child replacement does not resend unchanged ancestor wrapper HTML
open_questions:
  - focus and scroll behavior when a boundary is replaced because a conditional slot changed presence
  - focus, scroll, and client-owned state retention policy on layout replacement
```

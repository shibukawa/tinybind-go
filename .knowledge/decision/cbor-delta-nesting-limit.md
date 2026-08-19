---
id: decision:cbor-delta-nesting-limit
type: decision
title: Delta Nesting Is Not A Design Constraint
---
The profile's nesting limit is raised to 10000, matching the JSON reading, so a delta's depth stops bounding how deep a world may be and the generator builds no check for it.

```yaml
status: accepted 2026-08-19, by the maintainer
decided_where: the driver's profiles, upstream; this module reads whatever limit the profile carries
supersedes: the generation-time depth check proposed 2026-08-19, which is not built
what_prompted_it:
  measured: a delta of a d-level hierarchy nests 2d+1 containers, so world over city over house is already seven levels
  against: a wire-profile budget of eight, which would have made a four-level hierarchy a generation error
  the_formula_survives_the_decision: 2d+1 is still what a delta costs in containers, and it is still the reason a hierarchy level is two array heads rather than one; it stops being a limit and stays an explanation of size
why_raising_it_is_reasonable:
  the_shape_is_bounded_by_the_schema: a generated codec walks a fixed schema, so its own depth is the schema's and not an attacker's
  the_alternative_was_worse: a generator refusing a four-level world would have been this module deciding how deep a game may be, from a number it does not own
what_is_not_built_because_of_it:
  - a depth computation over the delta type set
  - a generation error naming a chain that exceeds a profile
  - the remedies that error would have offered
the_limit_is_a_recursion_bound:
  fact: Skip calls skipItem per level and Profile.Validate recurses the same way, so the number is Go stack frames rather than bytes
  on_a_host_target: the stack grows, and 10000 costs nothing that matters
  on_tinygo_wasm: the stack is fixed at link time, so the effective bound there is the stack rather than the profile, and a hostile input deep enough to reach the limit reaches the stack first
  why_it_matters_here: requirement:tinygo-wasm makes a js/wasm client a first-class target, so the client is exactly where an untrusted delta arrives
  the_remedy_costs_nothing: a Profile is constructed per side, so a wasm client may hold a small limit through WithMaxNestedLevels while a server holds 10000; a legitimate delta is shallow and neither side notices
  not_this_module_s_to_set: the client builds its own reader, so the number is the framework's; it is recorded here because the generator is what made the depth interesting
related:
  - data:cbor-state-delta
  - requirement:cbor-state-delta-generation
  - system:tinygodriver-cbor
  - requirement:tinygo-wasm
  - requirement:cbor-world-codec
```

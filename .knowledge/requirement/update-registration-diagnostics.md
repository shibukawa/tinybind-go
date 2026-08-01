---
id: requirement:update-registration-diagnostics
type: requirement
title: Update Registration Diagnostics
---
Return an error from redraw registration instead of panicking, so a caller collecting startup diagnostics reports every problem together rather than aborting on the first.

```yaml
priority: should
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
review_gate: proposed
shipped_today:
  redraw.go Register: panics on a missing kind and on a duplicate kind
  build.go processID: panics when the randomness source fails, the fallback identity of requirement:component-redraw-endpoint build identity
  reason_given: registration happens at startup, so failing there is the cheapest place to find a collision
agreement:
  contested: nothing about the requirement
  reporter_agrees: a duplicate kind must fail at startup, and the decision:framework-integration-seams reading behind it is right; a same-named, same-shaped component from another package would otherwise answer with the wrong package's external calls
  distinction: failing at startup and panicking are not the same requirement
why_it_costs_the_caller:
  call_site: Register is called from generated registry code during the caller's startup validation phase
  that_phase: collects diagnostics and reports them together, rather than aborting the process on the first
  effect: a panic replaces a diagnostic list with one stack trace, and the remaining registrations are never checked
ask: Register returns an error
constraints:
  - a caller ignoring the error still fails, because an unregistered kind answers 404 and the page reloads
  - the collision message stays as specific as today's, naming the kind and why the package is not part of it
  - a build identity that cannot be derived still fails rather than falling back to a constant, since a constant is the unsafe direction
acceptance:
  - a startup validation phase collects two duplicate-kind errors and reports both
  - a project registering one component at a time and ignoring the error behaves as it does today
as_built:
  shipped: 2026-08-01
  shape: Register returns an error; MustRegister panics, for a caller with nowhere to return one
  first_registration_stands: a refused duplicate leaves the earlier registration serving, so a caller ignoring the error has a working endpoint rather than a half-replaced one
  build_identity_open_question_resolved: the panic stays; BuildID is a lazily evaluated package value with no call site to return to, and a constant fallback is the unsafe direction
  related_addition: Options.Validate collects every unusable option at once, which is the same startup-validation shape this asked for
related:
  - requirement:component-redraw-endpoint
  - rule:redraw-input-trust
  - requirement:analysis-diagnostics
open_questions:
  - whether generated registry code should call Register or MustRegister, which decides where a duplicate surfaces in a project that writes no startup pass
```

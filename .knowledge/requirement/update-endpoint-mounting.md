---
id: requirement:update-endpoint-mounting
type: requirement
title: Update Endpoint Mounting
---
Accept a one-method router when mounting the framework-owned update endpoints, so a caller with its own mux can install the whole surface in one call.

```yaml
priority: should
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
  - requirement:router-type-independence
review_gate: proposed
shipped_today:
  signature: 'htmlupdate/runtime.go Options.Mount(mux *http.ServeMux, registry *Registry)'
  registers: the runtime asset handler and, when a registry is supplied, the redraw handler, both under the configured path prefix
  effect: a caller whose router is not '*http.ServeMux' cannot call Mount at all
not_blocking:
  workaround: RedrawHandler and RuntimeHandler both return an http.Handler, so the caller registers each itself
  cost: none beyond losing the one-call convenience and the guarantee that the prefix surface stays complete as endpoints are added
precedent:
  inside_this_module: requirement:router-type-independence made the generated registry's router a configurable symbol, with net/http as a default rather than a constraint
  known_feasible: '*http.ServeMux satisfies a one-method interface, because Mount registers through Handle alone'
  reading: the same finding as the 2026-07-30 round, one package further in; the runtime half kept the concrete type the generated half gave up
ask: a one-method router interface parameter, matching the shape requirement:router-type-independence already accepts
tension:
  decision:stdlib-servemux: core defaults require no framework-specific router
  unaffected: an interface parameter keeps net/http as the only import and '*http.ServeMux' as a valid argument
  earlier_deferral: decision:framework-integration-seams deferred an interface for the generated registry because MuxType already let a framework name its own type; no such escape exists here, since Mount is runtime Go rather than a template
constraints:
  - a caller passing '*http.ServeMux' compiles unchanged
  - the interface names one method, so a router with a different registration shape wraps rather than conforms
acceptance:
  - a framework mounts the update surface on its own router with one call
  - existing Mount call sites need no edit
as_built:
  shipped: 2026-08-01
  shape: 'Mount takes a Router interface naming one method, Handle(string, http.Handler)'
  compatibility: '*http.ServeMux satisfies it, so every existing call site compiles unchanged'
  open_question_resolved: Mount registers the runtime asset only when this build serves it, per requirement:browser-runtime-asset-ownership; owning the runtime does not affect the redraw endpoint
related:
  - decision:stdlib-servemux
  - decision:framework-integration-seams
  - requirement:generated-route-registration
open_questions:
  - whether the interface should grow HandleFunc, which a router exposing only that method would need
```

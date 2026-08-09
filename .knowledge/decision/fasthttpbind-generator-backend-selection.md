---
id: decision:fasthttpbind-generator-backend-selection
type: decision
title: Backend Selection Through Generator Symbols
---
Select the transport at generation time through a symbol seam, extending the mechanism routetree already has rather than inventing a second one.

```yaml
status: proposed 2026-08-08
precedent:
  implemented: requirement:router-type-independence already splits MuxImport, MuxAlias, MuxType and MuxConstructor from the request package in routetree Emitter Symbols
  reading: the registration half of the discovered router is already backend-selectable; only the request half is nailed down
gaps_found_2026_08_08:
  routetree_decoder_template:
    hardcodes: r.PathValue and r.URL.Query as method names on the request value
    fix: route them through Symbols as calls, the way the error constructors already are
    note: fasthttp has no transport PathValue at all, so this seam must also let the router publish path parameters
  generator_emit:
    hardcodes: the binder signature naming *http.Request, the writer signature naming http.ResponseWriter, and every httpbind. helper call
    fix: give the binder and writer emitter the same Symbols shape routetree has
    status: no seam exists here today; routetree has one and the core emitter does not
standalone_value:
  claim: closing both gaps is worth doing before any fasthttp code exists
  evidence: DefaultOptions already discovers tinygodriver/httpmux beside net/http.ServeMux, so a second request-side target is the pattern continuing, not a new one
  effect: httpbind output stays byte-identical while the emitter stops assuming one transport
selection_surface:
  where: a generator option, not a build tag and not an environment variable
  why: decision:runtime-package-boundaries makes the import graph the mechanism, and the graph is decided by what is emitted
  default: net/http; requirement:fasthttpbind-product-goals makes fasthttp additive
mixed_build:
  allowed: one module may generate both, in different packages
  forbidden: one generated file importing both transports
related:
  - requirement:router-type-independence
  - requirement:generated-route-registration
  - decision:framework-integration-seams
  - decision:fasthttpbind-runtime-package
  - rule:usage-directed-generation
```

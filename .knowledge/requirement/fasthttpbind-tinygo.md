---
id: requirement:fasthttpbind-tinygo
type: requirement
title: fasthttpbind TinyGo Position
---
fasthttpbind must compile under TinyGo and must not change what an existing TinyGo binary links, but owes that target nothing further.

```yaml
status: proposed 2026-08-08
scope_set_by: decision:fasthttpbind-tinygo-not-first-class
transport: system:tinygodriver-fasthttp
required:
  compiles: tinygo build of fasthttpbind plus generated fixtures succeeds
  build_tags: fasthttp_nozstd, which also removes the need for noasm
  no_reflect: generated binders and writers compile without reflect, per decision:reflection-free, which is a generation property rather than a TinyGo one
  isolation: importing httpbind never pulls fasthttpbind, so an existing TinyGo binary is byte-identical to one built before this package existed
isolation_is_the_real_protection:
  reasoning: the size and throughput costs of this transport are real, and requirement:tinygo-wasm exists to keep them out of binaries that did not ask for them
  mechanism: decision:fasthttpbind-runtime-package makes them separate packages, so the import graph enforces this without a build tag
  acceptance: a dependency check asserts httpbind's graph excludes fasthttpbind and the fork
not_required:
  - a size budget or a published size comparison
  - a throughput target
  - runtime verification of behaviour under TinyGo
  - TLS serving or HTTP/2, neither of which the transport can offer there
  - js/wasm evaluation; requirement:tinygo-wasm already records that the net/http runtime fails compiling roundtrip_js.go, and this package inherits no obligation to do better
ci_shape:
  add: a compile-only step, alongside the existing scripts/tinygo-check.sh coverage
  do_not_add: a behavioural matrix, which would imply a commitment this package does not make
related:
  - decision:fasthttpbind-tinygo-not-first-class
  - requirement:tinygo-wasm
  - decision:fasthttpbind-runtime-package
  - requirement:fasthttpbind-product-goals
```

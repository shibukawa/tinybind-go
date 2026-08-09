---
id: decision:fasthttpbind-tinygo-not-first-class
type: decision
title: TinyGo Is Compile-Only For fasthttpbind
---
fasthttpbind targets host Go; TinyGo is held to compiling and nothing else, so no size or throughput budget governs its design.

```yaml
status: accepted 2026-08-08
decided_by: user, 2026-08-08
commitment: the package builds under TinyGo; behaviour, size, and throughput there are unverified and untargeted
scope_note: this narrows fasthttpbind alone; requirement:tinygo-wasm continues to govern every other runtime package unchanged
why_the_question_arose:
  size: two routes cost 1.21 MB on net/http and 2.77 MB on the fork under TinyGo, so the transport reverses part of what the reflection-free work bought
  throughput: about 40 percent of standard Go, because TinyGo sync.Pool is one mutex-guarded slice and that lock is the gap
  reading: the pooling that makes fasthttp fast is the same mechanism that is contended there, so a TinyGo-first framing could not motivate this package
released_by_this_decision:
  - the size tension against requirement:tinygo-wasm; no budget applies
  - the throughput caveat, which becomes informational
  - any obligation to verify runtime behaviour on TinyGo
  - TLS serving returns to scope on host Go, where the fork is upstream behaviour for behaviour
not_released:
  http2: fasthttp implements no HTTP/2 server on any toolchain, so this stays out of scope everywhere
  requestctx_lifetime: rule:fasthttpbind-requestctx-lifetime is a correctness rule, not a TinyGo accommodation, and is unaffected
settles_the_dependency:
  question: depend on upstream valyala/fasthttp or on system:tinygodriver-fasthttp
  answer: the fork, because upstream does not build under TinyGo and compiling there is still required
  cost: the fork's types are distinct from upstream's, so a third-party package importing valyala/fasthttp cannot wrap a fasthttpbind handler; RequestHandler is func(*RequestCtx) over a different RequestCtx
  why_the_cost_is_bounded:
    - a user's own handlers and middleware are unaffected; the fork is drop-in for code you control
    - decision:fasthttpbind-adapter-boundary already places third-party middleware on the net/http side, so the ecosystem this breaks is one the design was not reaching for
  rejected_alternative:
    shape: ship fasthttpbind over upstream and a second package over the fork
    why_rejected: a maintenance multiplier spent on a target that is explicitly not first-class
related:
  - requirement:fasthttpbind-tinygo
  - requirement:fasthttpbind-product-goals
  - system:tinygodriver-fasthttp
  - decision:fasthttpbind-runtime-package
  - requirement:tinygo-wasm
```

---
id: decision:cbor-codecs-are-application-side
type: decision
title: The Game CBOR Codecs Are Application-Side
---
Delete the cborbind package and its generator pass: the wire and world profiles are one application's protocol, and a shared generator shipping them claims an authority it does not have.

```yaml
status: implemented 2026-08-20, by the maintainer's instruction
what_was_removed:
  runtime: the cborbind package and its six declarations, GenerateWireCodec through GenerateWorldDelta
  generator: the whole CBOR codec and delta pass, generator/cborbind*.go; its usage bits, call operations, features and the CborName/CborPath wiring
  fixtures: testdata/cmd/tinygo-cbor-smoke and docs/cborbind.md
what_stayed:
  http_negotiation: requirement:cbor-http-body is untouched; its codecs call the driver directly from generated code and never named a game profile
  driver_dependency: go.mod keeps tinygodriver v1.2.7, which is what the HTTP codecs generate against
precedent: the driver made the same cut in v1.2.7, per system:tinygodriver-cbor profiles_reshaped_v1_2_7; wire and world became struct literals in the game server's own project, and this module follows the same line one level up — the mechanism stays generic, the application names its subsets
why_deletion_and_not_neutralization:
  asked: whether to keep a profile-neutral array/map codec mechanism with application-supplied restrictions
  answered: full removal; the game framework owns its codec generation, and a mechanism kept for one consumer that is leaving is dead weight
downstream:
  consumer: ebigentserver examples/phase0 declares GenerateWireCodec[PlayerInput] and GenerateWorldDelta[WorldState], pinned to tinybind-go v0.5.17
  consequence: that example keeps building until it bumps tinybind; the bump is the framework's migration to its own codegen, which is its decision to schedule
history: the pass was built 2026-08-19 at the framework's request and removed the next day when the driver's v1.2.7 reshape made the ownership question explicit; the emitter migration that briefly hardcoded the wire and world literals into this module is what surfaced it
related:
  - system:tinygodriver-cbor
  - requirement:cbor-http-body
  - rule:generator-feature-disable
  - decision:runtime-package-boundaries
```

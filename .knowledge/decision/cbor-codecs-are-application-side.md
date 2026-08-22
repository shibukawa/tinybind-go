---
id: decision:cbor-codecs-are-application-side
type: decision
title: The Game CBOR Codecs Are Application-Side
---
The wire and world profiles are one application's protocol, and a shared generator shipping them claims an authority it does not have; they left with the whole pass that carried them, and the codec mechanism returns without them per decision:cbor-shape-is-the-only-axis.

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
reopened_2026_08_22:
  what: the maintainer asks for cborbind back as a jsonbind-shaped declaration mechanism; the removal was right about the names and wider than the reason required
  what_this_decision_still_holds: no preset standing for one application's protocol ships from here, and the delta pass stays gone -- a state diff is a game's sync strategy, not a codec
  what_it_no_longer_holds: that the codec generation itself is application-side; wire and world were a container shape wearing an application name, and the shape is mechanism
  where_the_profile_goes_instead: decision:cbor-shape-is-the-only-axis
  the_mechanism_returns_as: requirement:cbor-codec-generation
  cost_of_the_round_trip: the pass, its usage bits and its TinyGo smoke were deleted rather than renamed, so reviving them is a re-implementation against git history rather than a rename; 5436485 is the commit to read
related:
  - decision:cbor-shape-is-the-only-axis
  - requirement:cbor-codec-generation
  - system:tinygodriver-cbor
  - requirement:cbor-http-body
  - rule:generator-feature-disable
  - decision:runtime-package-boundaries
```

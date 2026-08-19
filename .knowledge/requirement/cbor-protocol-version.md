---
id: requirement:cbor-protocol-version
type: requirement
title: CBOR Protocol Version Derivation
---
Derive one identifier from wire-observable schema shape alone, so a generator upgrade that emits identical bytes does not force a lockstep client and server redeploy.

```yaml
priority: must
status: implemented 2026-08-19
as_built:
  emitted: two constants in the generated file, CBORSchema holding the canonical description and CBORProtocolVersion holding the first 8 bytes of its SHA-256 as hex
  where_the_description_lives: a generated Go constant, settling that open question the cheap way; it is diffable in review because it sits in a file review already reads, and a sidecar can still be added beside it
  covers: profile, field order, wire key, kind and width per field, and the identity of a self-encoding type by name
  covers_nothing_else: not the generator, not go.mod, not the directions asked for; verified by test that narrowing GenerateWireCodec to GenerateWireEncoder leaves the version unchanged while widening a field or changing the profile moves it
  stable_across_targets: the same schema digests identically on darwin/arm64 and on wasip1 under TinyGo, printed by testdata/cmd/tinygo-cbor-smoke on both
  one_version_per_package: the digest covers every codec in the run, which is what a handshake compares
source:
  - downstream game framework CBOR requirements 2026-08-19, its section 7
  - the framework's protocol-version data concept, which says derived and never hand maintained
emits: one identifier covering every generated schema in the run
covers:
  - field order
  - field types and their declared widths
  - the identity of each field's type, which is what carries a scale, per decision:cbor-scale-lives-in-the-type
  - the profile each schema was generated for
  - map key ordering, for a world schema
  - the identity field of an element type, and the presence-mask width, once requirement:cbor-state-delta-generation exists; both are wire-observable, and a moved identity is a delta an old receiver keys wrongly
derived_never_hand_maintained:
  why: the framework makes a mismatch a hard connection error, so a stale hand-written constant takes a fleet down rather than degrading
covers_wire_observable_shape_only:
  the_requirement_most_likely_to_be_got_wrong: yes
  forward: regenerating with a newer generator that emits identical bytes must produce the identical version
  backward: any change that moves one byte on the wire must move the version
  consequence_of_getting_it_wrong: every generator upgrade becomes a coordinated redeploy of both ends, which is the operational cost the framework's must-match rule already names
not_the_regeneration_hash:
  other: rule:generation-input-hash, which decides whether to regenerate
  legitimately_covers: the generator binary, go.mod, template files, options; things the wire never sees
  therefore: two hashes, and folding them into one would make a generator upgrade a protocol change
stability:
  across: runs, platforms and Go versions
  computed_over: a canonically serialized schema description
  not_over: generated source text, and not over anything map-ordered
the_description_itself_should_be_emittable:
  why: a version mismatch is then diagnosed by diffing two schemas rather than by observing that two opaque numbers differ
  open: whether it lives in a generated Go constant, a sidecar file, or both; a sidecar is diffable in review, which is where a protocol change should be caught
acceptance:
  - regenerating with an unchanged schema produces an unchanged version
  - changing one field's width changes it
  - changing a field from one scale's type to another's changes it
  - upgrading the generator with no schema change leaves it unchanged
  - the same schema produces the same version on darwin/arm64, linux/amd64 and js/wasm
related:
  - requirement:cbor-wire-codec
  - requirement:cbor-world-codec
  - decision:cbor-scale-lives-in-the-type
  - requirement:declared-cbor-codec
  - rule:generation-input-hash
  - data:generation-artifact
open_questions:
  - where the schema description lives, per the_description_itself_should_be_emittable
```

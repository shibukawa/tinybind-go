---
id: rule:update-validator-computation
type: rule
title: Update Validator Computation
---
Define what an input validator and a content validator cover, so a skipped boundary is provably unchanged.

```yaml
source:
  - data:component-update-manifest
  - user input-hash design 2026-07-26
roles:
  input_validator: digest of component version plus canonical typed inputs; predicts identical output only for a pure component
  content_validator: digest of the canonical rendered boundary HTML; the authority for omitting a boundary
short_circuit:
  allowed: a requirement:component-output-cache eligible component may reuse output and its validator without rendering
  forbidden: skipping rendering on input-validator equality alone for a component reading time, storage, globals, locale, or request state outside its declared inputs
  reason: the user-facing input hash is a cache and diagnostic key, not a proof of output equality
canonical_input_encoding:
  form: generated type-aware encoding, ordered by declaration, each field length-prefixed and type-tagged
  rejected: encoding a Go value as JSON, because float formatting, map ordering, optional versus empty, and time zone rendering are ambiguous
  included: generated component version, so a template edit invalidates every validator
  excluded: server-only values, request identity, CSRF token, CSP nonce, and transport marker IDs
canonical_content:
  input: the boundary's rendered bytes as written
  excluded: compression framing, request-unique marker attributes, and injected bootstrap metadata
  no_normalization: bytes are hashed as emitted; whitespace-insensitive comparison is not attempted
hashing:
  algorithm: one declared cryptographic hash with a build-identity tag, which decision:caller-owned-wire-versioning substitutes for the protocol-version tag it removes
  why_the_build_identity: it is the axis that actually moves, it already covers a changed client and a changed external function, and Options.BuildID makes its value the caller's
  not_the_component_version: canonical_input_encoding already mixes the generated component version so a template edit invalidates every validator; the tag here guards against the wire shape changing instead
  belt_and_braces: Negotiate already answers a build mismatch with a complete document before any validator is read, so the tag matters only where the build header was dropped in transit
  keyed: keyed with a per-server secret so an attacker cannot confirm guessed low-entropy content by replaying a hash
  truncation: truncate only to a length where collision is negligible, because a collision silently keeps stale DOM
  encoding: opaque URL-safe text, safe in an HTML attribute and in protocol fields
determinism:
  expectation: an update boundary renders deterministically for equal inputs and equal server state
  nondeterministic_output: a boundary embedding wall-clock text or unstable ordering never matches, so it is resent every time; this is correct but wasteful and belongs in diagnostics guidance
  no_static_guarantee: external function purity cannot be proven at generation time, so this stays a documented contract
authority:
  - client validators are hints; the server never derives arguments or access decisions from them
  - an unparseable, oversized, or build-mismatched validator set is ignored in favor of a larger safe delta
  - a boundary the server cannot validate is sent in full
acceptance:
  - two renders with equal declared inputs and equal state produce one content validator
  - a template edit changes every validator without any input change
  - replaying a captured validator against another session reveals nothing about content
  - an input-validator match alone never suppresses a non-cacheable boundary
open_questions:
  - key rotation and multi-instance key distribution
  - whether the input validator is published to the client at all, given its limited authority
  - validator length versus manifest size in decision:manifest-state-ownership
```

---
id: decision:cache-key-derivation
type: decision
title: Cache Key Derivation
---
Build a cache key from a generation-time plan fingerprint plus a generated length-prefixed encoding of every declared parameter, with no reflection and no hashing in the runtime.

```yaml
source:
  - requirement:component-output-cache
  - decision:cache-component-declaration
  - user architecture decision 2026-07-26
review_gate: approved 2026-07-26
parts:
  identity: template file path plus component name
  version: fingerprint of the emitted decision:generated-render-plan instruction list and head contributions
  parameters: canonical encoding of the generated params struct in declaration order
version_rationale:
  covers: markup edits, expression edits, nested component edits reaching this plan, and scoped style renaming
  effect: regenerated code cannot read entries written by the previous code
encoding:
  emitter: one generated key function per cached component, typed by its params struct
  per_type: a dedicated encoder per template type, so two distinct types cannot produce the same bytes
  framing: every scalar is written length-prefixed, so concatenated fields cannot alias across boundaries
  records: generated per-record encoder walking declared fields in declaration order
  optional: a distinct absent marker, never an empty value
  arrays: element count followed by framed elements
  html: unreachable; decision:cache-component-declaration rejects html parameters
hashing:
  runtime: none; the key is the framed string
  store: an api:cache-store implementation may hash, because it owns its own key-length limits
  reason: hashing in the runtime would trade a correctness risk for a saving the store can make itself
constraints:
  - decision:reflection-free is preserved; every encoder is statically typed generated code
  - equal keys imply equal declared inputs and equal generated code
  - request, session, and locale variation must be declared parameters or the component must not be cached
```

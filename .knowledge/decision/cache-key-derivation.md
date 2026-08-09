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
  scope: the render's decision:cache-scope-declaration value, framed and prepended, present only for a component declaring private
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
scope_prefix:
  added: 2026-08-09, per decision:cache-scope-declaration
  framing: the existing length-prefixed form, so a scope value cannot spell out another component's key
  absent: a private component rendered with no scope value stores nothing; an empty scope would be a shared entry wearing a private label
  reflection_free: one string in front of a string generated code already builds, so decision:reflection-free is untouched
  above_the_store: the prefix is in the key, so api:cache-store and every adapter written against it are unchanged
constraints:
  - decision:reflection-free is preserved; every encoder is statically typed generated code
  - equal keys imply equal declared inputs and equal generated code
  - locale variation must be a declared parameter or the component must not be cached
  - a request property an element declares is not keyed; decision:cache-component-declaration options.vary makes the component ineligible instead
amended_2026_08_09:
  was: request, session, and locale variation must be declared parameters or the component must not be cached
  why_it_moved: it left a component reading per-user state with no cached form at all, and the scope prefix gives it one keyed per user
  what_survives: the rule still holds for anything the scope value does not cover, which is why locale stays a parameter
```

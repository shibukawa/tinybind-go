---
id: decision:generic-method-migration
type: decision
title: Which Package Functions Become Methods, And When
---
Convert the three entries today's Go already allows, and hold the rest against a trigger that needs a Go release and a TinyGo release carrying it.

```yaml
source:
  - downstream framework change request 2026-08-13, asks 2 and 3
  - decision:cache-key-generator-seams
review_gate: proposed
axis: an operation reached as a package function because a method cannot carry its own type parameter, where the receiver would otherwise be the obvious home
available_today:
  why: these three carry no type parameter beyond the receiver's own, so the language never blocked them; they sat in the blocked list and were read as blocked
  entries:
    - at: htmlbind/ops.go Require
      to: func (Builder[P]) Require(check func(P) error) Op[P]
    - at: htmlbind/plan.go Bind
      to: func (p *Plan[P]) Bind(params P) Fragment
    - at: htmlbind/render.go BindWrapper
      to: func (p *Plan[P]) BindWrapper(params P, setChildren func(*P, Fragment)) Wrapper
  no_new_receiver: Plan[P] already carries Exec and Sequence
  value: Builder[P] reads as methods everywhere else, so a generated plan currently spells one surface two ways
  verified: 2026-08-13 against v0.5.8; all three are `[P any]` and introduce nothing beyond it
  implemented: 2026-08-13; each method carries the body and each function forwards to it, marked deprecated
  no_collisions: neither Builder[P] nor *Plan[P] declared any of the three names
  tested: an equivalence test per entry, pinning that the function and the method are one operation rather than two that drift
  generated_output_unchanged: the emitter still writes the function form, so nothing regenerates and the move stays a caller's choice
deferred:
  instruction: do not act; the reporter filed the inventory for planning only
  trigger: a Go release permitting a method to declare its own type parameters, and a TinyGo release carrying it, both
  why_both: the reporter targets TinyGo and WebAssembly, so a conversion available only on upstream Go splits their build rather than tidying it
  expected: Go 1.27 with TinyGo 0.42, to be re-filed against the releases that actually ship it
  priority_order:
    - what: firestorebind.Tx typed reads
      entries: [LoadTx, LoadAllTx, QueryPageTx]
      value: the only one worth more than tidiness; writes are already methods, so one transaction is written two ways in adjacent lines
      survives_the_change: LoadTx gives two reasons for the function form and only one is the language; a context-carried handle would still make one call site mean two things depending on which context reached it, so the operation stays reached through the transaction value
    - what: the *On entries on Handle
      why_a_method_is_possible: Handle is a concrete type, so the explicit form becomes a method while the context-resolving form is untouched
      dynamobind: [LoadOn, LoadAllOn, StoreOn, StoreAllOn, StoreReturningOn, RemoveOn, RemoveReturningOn, UpdateOn, QueryPageOn, QueryOn, ScanPageOn, ScanOn]
      firestorebind: [LoadOn, LoadAllOn, StoreOn, StoreAllOn, InsertOn, InsertAllOn, UpdateOn, RemoveOn, RemoveAllOn, QueryPageOn, QueryOn]
      weight: the reporter wraps none of these, so they are what an application author writes
      verified: 2026-08-13; all twelve and all eleven exist under those names
    - what: htmlbind.Builder[P]
      entries: [For, ForCtx, Await, Live, Provide]
      value: least visible, since no application reads generated plans
    - what: jsonbind.Parser
      entries: [ParseSlice, ParseMap]
      shape: a struct with a dozen methods, and the two operations parameterized on the decoded element standing outside it
      caller: this module's own generated action decoders
    - what: sqlbind.AppendValues
      value: last, because it is one function beside Builder's Arg and Statement
  permanently_excluded:
    what: sqlbind.ScanRows
    why: Rows is an interface, so no language change lets the package give it a method
    verified: 2026-08-13; sqlbind/statement.go declares Rows as an interface
    recorded_because: the reporter asked that it be recorded rather than rediscovered each round
migration_shape:
  applies_to: both halves
  form: the method becomes the body, the existing function stays as a deprecated wrapper
  breakage: none; nothing stored, generated, or on the wire changes, so no caller is forced to move
related:
  - decision:cache-key-generator-seams
  - decision:reflection-free
```

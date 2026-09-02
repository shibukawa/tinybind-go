---
id: decision:generic-method-migration
type: decision
title: Which Package Functions Become Methods, And When
---
Every entry on the deferred list became a method on 2026-09-02, once Go 1.27 and TinyGo 0.42 both shipped; each old function stays as a deprecated wrapper, and the generator reads both spellings.

```yaml
source:
  - downstream framework change request 2026-08-13, asks 2 and 3
  - decision:cache-key-generator-seams
  - downstream instruction 2026-09-02, once the toolchain baseline moved
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
  trigger_fired_2026_09_02:
    what: both halves shipped and the project moved onto them; requirement:tinygo-wasm now reads TinyGo 0.42.0 + Go 1.27.x
    verified_directly: a method declaring its own type parameter compiles on Go 1.27 and builds under tinygo -target=wasip1 on 0.42.0, so the split build the why_both clause guarded against does not happen
    gated_on_the_module_line: go 1.26.0 refuses it with "generic method requires go1.27 or later"; the module now declares go 1.27.0, so the gate is open
  migrated_2026_09_02:
    instruction: the downstream owner asked for the list in the catalog's own priority order, with the upstream half done here
    order_kept: the transaction reads, then the On entries, then the builder, then the parser, then the SQL builder; each below records what it became
    call_shape: a method declaring its own type parameter is instantiated at the call as h.Load[Reading](ctx, key), with a constraint-bound second parameter such as PT inferred from the first; an entry whose type is inferable from an argument spells none, so h.Store(ctx, v) reads as a plain call
    receiver_for_a_generic_type: Builder[P] can declare a method with further parameters of its own, For[E, S], so the html entries needed no new receiver either
  priority_order:
    - what: firestorebind.Tx typed reads
      entries: [LoadTx, LoadAllTx, QueryPageTx]
      value: the only one worth more than tidiness; writes are already methods, so one transaction is written two ways in adjacent lines
      survives_the_change: LoadTx gives two reasons for the function form and only one is the language; a context-carried handle would still make one call site mean two things depending on which context reached it, so the operation stays reached through the transaction value
      became: tx.Load[T], tx.LoadAll[T] and tx.QueryPage[T], carrying the bodies; the three functions forward to them
      also_moved: QueryKeysPageTx and CountTx, which declare no type parameter and could have been methods all along; they went with the rest so the transaction reads one way rather than two, per the same reasoning the downstream memo store applied to its own keyless entries
      generated_twins_unchanged: a declared query's <Name>Tx twin still calls the function form, per generated_output_unchanged above; moving the emitter is a separate change that regenerates every fixture
    - what: the *On entries on Handle
      why_a_method_is_possible: Handle is a concrete type, so the explicit form becomes a method while the context-resolving form is untouched
      dynamobind: [LoadOn, LoadAllOn, StoreOn, StoreAllOn, StoreReturningOn, RemoveOn, RemoveReturningOn, UpdateOn, QueryPageOn, QueryOn, ScanPageOn, ScanOn]
      firestorebind: [LoadOn, LoadAllOn, StoreOn, StoreAllOn, InsertOn, InsertAllOn, UpdateOn, RemoveOn, RemoveAllOn, QueryPageOn, QueryOn]
      weight: the reporter wraps none of these, so they are what an application author writes
      verified: 2026-08-13; all twelve and all eleven exist under those names
      became: the same names without the suffix, as methods on Handle; the Context form now delegates to the method, and the On function forwards to it, so the implementation still lives in exactly one place
      also_moved: the keyless firestorebind twins RemoveKeysOn, QueryKeysPageOn, CountOn, KeyForOn, KeysForOn, RunOn and RunReadOnlyOn, for the reason given under the transaction entry
      discovery: a Method pattern per entry beside each Function pattern, in generator/options.go, so a package written in the method spelling generates what the On spelling generated; requirement:parameter-api-call-discovery records the positions
    - what: htmlbind.Builder[P]
      entries: [For, ForCtx, Await, Live, Provide]
      value: least visible, since no application reads generated plans
      became: methods on Builder[P] declaring E and S, S and R, or V of their own, every one inferred from an argument
      generated_output_unchanged: the emitter still writes the function form; the fixtures under testdata/templates/htmlbind would all regenerate otherwise
    - what: jsonbind.Parser
      entries: [ParseSlice, ParseMap]
      shape: a struct with a dozen methods, and the two operations parameterized on the decoded element standing outside it
      caller: this module's own generated action decoders
      became: p.ParseSlice, p.ParseMap and, for the same reason, p.ParseArray, which arrived after the inventory was filed and had the same shape
      generated_output_unchanged: as above; generated decoders still spell jsonbind.ParseSlice(p, ...)
    - what: sqlbind.AppendValues
      value: last, because it is one function beside Builder's Arg and Statement
      became: b.AppendValues; generated SQL still binds the function as _tinybindSQLArgs, per decision:generated-runtime-in-module
  permanently_excluded:
    what: sqlbind.ScanRows
    why: Rows is an interface, so no language change lets the package give it a method
    verified: 2026-08-13; sqlbind/statement.go declares Rows as an interface
    recorded_because: the reporter asked that it be recorded rather than rediscovered each round
migration_shape:
  applies_to: both halves
  form: the method becomes the body, the existing function stays as a deprecated wrapper
  breakage: none; nothing stored, generated, or on the wire changes, so no caller is forced to move
  tested_2026_09_02:
    equivalence: one test per package pins that the function and the method are one operation; htmlbind and jsonbind and sqlbind compare results, the NoSQL packages write through one spelling and read through the other against the fake servers, and the zero Handle answers ErrNoClient through both
    discovery: the two usage tables in the generator carry a row per method spelling, explicit and inferred, beside the row for its On form
    toolchain: go test across the module and scripts/tinygo-check.sh, which now exercises a generic method under TinyGo
open:
  emitter: whether generated plans, decoders, SQL and query twins should move to the method spelling; each is a fixture regeneration and none is forced by this change
related:
  - decision:cache-key-generator-seams
  - decision:reflection-free
  - requirement:parameter-api-call-discovery
  - decision:firestore-transaction-scope
```

---
id: requirement:parameter-api-call-discovery
type: requirement
title: Discovery Finds The Handle-Taking Twins
---
Register a CallPattern for every On entry of api:dynamobind-operations and api:firestorebind-operations, so a package written against the parameter form is read by the same rule:usage-directed-generation that reads the Context form.

```yaml
status: implemented
proposed: 2026-08-11
implemented: 2026-08-11, in generator/options.go
source: downstream framework request 2026-08-11, filed against v0.5.1
the_gap:
  what_was_added: requirement:dynamo-parameter-api and requirement:firestore-parameter-api gave every runtime entry an On twin, and an option publishing declared queries in that form
  what_was_not: neither touched canonicalRuntimeCalls or canonicalFirestoreCalls, so no pattern ever named an On entry
  consequence: a package calling only On entries has no discovered call, so nothing is generated for it, and the call sites that required the codec cannot compile
  not_a_partial_result: the type reads as unused rather than as under-served, so the output is empty rather than missing a method
  same_in_both_stores: one omission made twice, rather than a difference between DynamoDB and Firestore
why_it_surfaced_late:
  dynamodb: the downstream scaffold wrote StoreOn and LoadOn, so a generated project failed to build on first use
  firestore: the scaffold carried struct tags and a declaration and no Go call at all, so the same hole was never reached
  reading: the asymmetry is in how it showed, not in what was broken
the_emitter_option_is_not_the_switch:
  what: DynamoParameterAPI and FirestoreParameterAPI choose the signature of a generated declared query, read only by generator/dynamoquery_emit.go and generator/firestorequery_emit.go
  not_discovery: neither reaches the pattern set, so there was no way to ask discovery to look for the On entries
  why_not_to_bind_them: declaring queries in the Context form while calling items in the parameter form is an ordinary configuration, so gating detection on the emitter's option would replace one hole with another
always_registered:
  rule: both forms are in the canonical set unconditionally
  cost_to_an_existing_project: none; a name nothing calls matches nothing, so a Context-only package generates what it generated before
  precedent: the canonical set is already spelled once per runtime package, and most of its names resolve nowhere
positions:
  read_side: the type parameter is where it was, since T appears only in the result
  write_side: one argument later, the Handle sitting between the Context and the table
  dynamo: ArgumentType 3, against 2 for "Store(ctx, table, v, opts...)"
  firestore: ArgumentType 2, against 1 for "Store(ctx, v, opts...)"
no_twin_to_register:
  tx_entries: LoadTx, LoadAllTx, QueryPageTx and the Tx write methods take a receiver already carrying the handle, so no On form exists
  keyless_entries: KeyForOn, KeysForOn, CountOn, QueryKeysPageOn, RemoveKeysOn, RunOn and RunReadOnlyOn name no model, so they carry nothing for discovery to read
why_downstream_could_not_work_around_it:
  fact: Options.Calls.Set replaces the canonical set rather than adding to it, and the canonical constructors are unexported
  consequence: a downstream adding one pattern restates the whole set, and then silently loses whatever a later version adds to it
  therefore: this is fixable upstream only
downstream_state:
  what_they_did_meanwhile: moved the DynamoDB scaffold and both storage guides to the Context form, and added a declaration to the scaffold the Firestore one already had
  why_recorded: it is a workaround against v0.5.1 rather than a preference, and they intend to move back to the Handle form, which is the one their own client design points at
  what_they_need_from_here: a release carrying this, and nothing else; no option to set and no call site to change
verification:
  tests: generator/dynamobind_test.go TestDynamoUsageFollowsHandleCallSites and generator/firestorebind_test.go TestFirestoreUsageFollowsHandleCallSites, one case per twin
  before: every case fails against the unpatched set, which is what makes the table a guard rather than a restatement
acceptance:
  - a package whose only dynamobind call is StoreOn generates EncodeItem
  - a package whose only firestorebind call is StoreOn generates EncodeEntity
  - every On entry carrying a model is registered, in both packages
  - a package calling only Context entries generates what it generated before
related:
  - requirement:dynamo-parameter-api
  - requirement:firestore-parameter-api
  - rule:usage-directed-generation
  - requirement:configurable-generator-discovery
  - api:dynamobind-operations
  - api:firestorebind-operations
  - decision:nosql-client-supply-modes
  - requirement:dynamobind-generated-item-codec
  - requirement:firestorebind-generated-entity-codec
```

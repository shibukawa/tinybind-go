---
id: decision:firestore-framework-requests
type: decision
title: Firestore Scope After The Downstream Round
---
Seven asks arrived from the framework building on this stack; three were the driver's and shipped in v1.1.7, and the four left are firestorebind surface rather than design disagreements.

```yaml
source: downstream framework Firestore change request 2026-08-05
review_gate: proposed
shape_source: decision:dynamo-framework-requests, the same split done for the DynamoDB side
consumer:
  who: the framework whose auth and session backends select one Firestore value
  stores: [session records, ceremony state, an OIDC allowlist, passkey credentials, bootstrap credentials]
  reads: every one is a key lookup, a batch lookup or a single-property equality query, so decision:firestore-no-schema-artifact needs no composite index for any of them
  writes: every predicate over a stored value is a transaction of one read plus one commit, per decision:firestore-transaction-scope
  filed_against: tinygodriver v1.1.6 and this module's firestorebind at 9219e98, before either had a tag carrying it
what_the_round_confirms:
  every_design_question_was_already_answered: the consumer reports finding each one in one of the two catalogs, which is what the catalogs are for and is worth recording as evidence rather than as praise
  nothing_here_is_a_disagreement: all seven are surface or documentation; no decision is being reopened
upstream_and_shipped:
  where: system:tinygodriver-firestore round_four_2026_08_05
  tag: v1.1.7
  single_use_transaction: beginTransaction is gone, and every transaction shape lost a round trip rather than only the one asked about
  readme_contradiction: the not-in-scope list no longer names SUM and AVG, and a test fails when it names anything the package exports
  read_time_bound: WithReadTime states both windows and whose duty truncation is
  effect_here: no source in this module changes for any of the three; what changes is the pin, three fixture assertions, and two statements this catalog made about the driver
this_module:
  - what: requirement:firestore-key-batch-delete
    value: highest; QueryKeysPage hands back keys and nothing takes them, so the one shape every cleanup needs is the one that has to be hand-rolled
    order: first
  - what: requirement:firestore-namespace-stamping
    value: high; the escape hatch writes to the default namespace silently, and no test running in the default namespace can see it
    order: beside the batch delete, since it is what that caller reaches for next
  - what: requirement:firestore-mutation-sizing
    value: a correctness fix wearing a cleanup's clothes, now that the driver measures what the local constant was guessing
    order: independent of the other three
  - what: requirement:firestore-expiry-property-declaration
    value: the only ask that reverses a position this catalog recorded, and the only one needing a decision rather than an implementation
    order: last, and gated on that decision
pin:
  driver: v1.1.7, from the v1.1.6 in go.mod
  what_it_buys: the fold and the doc fixes, neither of which needs a source change here
  what_it_does_not_buy: MutationSize, which has been available since v1.1.6 and is unused, per requirement:firestore-mutation-sizing
  order: first, ahead of the four; it is what makes the fold real and it is what the fixture has to be corrected for
  fixture_cost:
    measured: 2026-08-05, by bumping and running internal/firestorefixture; the package builds and three tests fail
    failures:
      - TestTransactionReadModifyWrite, asserting one beginTransaction and now seeing none
      - TestTransactionRestartsOnAbort, asserting two and now seeing none
      - TestDeclaredQueryTransactionTwin, reading a beginTransaction count as the proxy for having run inside a transaction at all
    it_is_a_fixture_update_not_a_regression: the driver stopped sending a request; every one of these counted that request
    what_replaces_the_proxy: the transaction is now visible where it actually happens - readOptions.newTransaction on the first read, and the handle or singleUseTransaction on the commit
    the_third_is_the_interesting_one: it asserts a property worth keeping, and only its evidence changed; the fake server has the request bodies to assert against directly
    nothing_else_moved: go build and go test over the whole module at v1.1.7 report these three and nothing else; the pin was reverted after measuring, so go.mod still says v1.1.6
framework_owns:
  namespace_teardown: there is no API that deletes a namespace, so a test run's teardown is a keys-only query per kind and a batch delete; this module supplies both halves and owns neither the sweep nor the isolation policy
  ttl_policy_application: gcloud firestore fields ttls update stays deployment tooling, per the consumer's own decision and system:tinygodriver-firestore ttl
  which_kinds_exist: the framework publishes its own kind list; requirement:firestore-expiry-property-declaration supplies one fact per type, not the roster
agreed_declined_positions:
  fact: the consumer restates and accepts each of these rather than leaving them to be inferred from silence
  index_derivation_from_a_query: declined by the driver as RequiredIndex, and by decision:firestore-no-schema-artifact in the generator; both stand
  admin_api: for indexes or for TTL, assigned to deployment tooling; the framework says it would not call these if it had them
  property_transformations: excluded by the driver, and the two arithmetic-shaped operations route through transactions instead
  firestore_native_mode: out of scope for both modules and for the consumer
  a_portable_facade_over_dynamodb_and_datastore: declined in both catalogs and in the consumer's own
related:
  - decision:dynamo-framework-requests
  - system:tinygodriver-firestore
  - api:firestorebind-operations
  - requirement:firestorebind-product-goals
  - rule:firestorebind-driver-passthrough
  - decision:firestore-transaction-scope
```

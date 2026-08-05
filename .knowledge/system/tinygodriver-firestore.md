---
id: system:tinygodriver-firestore
type: system
title: tinygodriver Firestore Datastore Client
---
TinyGo-buildable client for Firestore in Datastore mode, speaking the Datastore v1 JSON transport directly; the binding target of requirement:firestorebind-product-goals.

```yaml
package: github.com/shibukawa/tinygodriver/nosql/datastore
naming:
  driver: datastore, after the API it speaks
  binding: firestorebind, after the product, per decision:firestorebind-runtime-package
  effect: the one pair in this catalog where the binding name does not repeat the driver package name
reason_for_existing:
  grpc_client: cloud.google.com/go/datastore is gRPC only, and gRPC is TLS-only against a tinygo crypto/tls stub
  rest_client: google.golang.org/api/datastore/v1 speaks the same JSON transport and still fails, through the empty tinygo net/http.Transport reached by its credential layer
  reading: the obvious fallback exists and does not help, which the driver records, measured rather than assumed, in its own catalog
release_status:
  introduced_in: tinygodriver v1.1.4, commit c5ee07d
  bound_surface: v1.1.9, which go.mod requires; it names the commit envelope as CommitOverheadBytes, v1.1.7 folded beginTransaction away and fixed the README and the read-time bound, and v1.1.6 before that added OR filters, SUM and AVG, MutationSize, and made key partitioning recursive. Every API fact below was read from a tag rather than from a working copy
  v1_1_7_and_v1_1_8_are_one_tree: the proxy serves both from commit b3ad16f, so v1.1.8 buys the v1.1.7 changes and nothing further; v1.1.9 is 17f792b and is where the envelope arrives
  read_a_version_as_what_go_get_yields: not as what the driver's working copy holds under that name. See round_five.commit_envelope, where the two disagreed for a release
  not_in: v1.1.3, which carries nosql/dynamodb only
  driver_own_catalog: the tag ships .knowledge concepts for the client, the value codec, the retry policy, the write preconditions, the emulator endpoint, and a DynamoDB comparison; read them there rather than restating them here
reflection_path:
  api: [MarshalEntity, UnmarshalEntity]
  landed: v1.1.5, ff4c947; the v1.1.4 package had none, and requirement:firestorebind-product-goals was first written against that absence
  tag: datastore, the cloud.google.com/go/datastore spelling, so an official-client example ports over
  key_field: a field tagged "__key__" carries the entity's own key and must be a Key or *Key; Datastore reserves __key__ as the pseudo-property naming the key in queries, so no real property collides with it
  options: ",noindex", ",omitempty", and "-" to skip
  no_maps: deliberately unsupported, because a map's property names would come from run-time data rather than from the struct, which is the one thing the mapping exists to avoid; data:firestore-property-mapping follows the same reasoning
  cost: reflect, linked only when called
  authoritative_for_this_path_only: the driver's own doc comment says so, and instructs a generator over it to treat a field carrying the datastore tag but not the generator's own as an error rather than as agreement; rule:firestore-tag-options carries that check
  effect_here: the codec is no longer the only way a struct reaches this driver, so requirement:firestorebind-product-goals argues from drift and from the two-path hazard rather than from an absence
identity_model:
  Key: "{Namespace string; Path []PathElement}"
  PathElement: "{Kind string; ID int64; Name string}", exactly one of ID and Name
  constructors: [NameKey, IDKey, IncompleteKey, "Child(PathElement)", "WithNamespace(string)"]
  helpers: [Kind, Incomplete, Valid, Equal, String]
  incomplete: neither ID nor Name; legal only in an insert or AllocateIDs, where the server fills it in
  ancestry: earlier path elements are ancestors, so the parent is part of identity rather than a property
  partition: projectId and namespaceId are added by the client at encode time, so a Key stays portable inside a program
entity_model:
  Entity: "{Key *Key; Properties map[string]Value; Version int64; UpdateTime string; CreateTime string}"
  helpers: ["NewEntity(Key)", "Set(name, Value) Entity", "Get(name) (Value, bool)"]
  schemaless: two entities of one kind need not carry the same properties
  read_only_fields: Version, UpdateTime and CreateTime come back from a read and are ignored on the way out; they feed WithBaseVersion and WithUpdateTime
  key_is_not_a_property: unlike a DynamoDB partition key, which is an attribute in the item
value_model:
  Value: "one-member union over {Null bool; Bool; Integer *string; Double; Timestamp; Key; String; Blob; GeoPoint; Entity; Array}, plus ExcludeFromIndexes outside the union"
  constructors: [String, "Int[T Integer]", IntString, Float, Bool, Time, Blob, KeyValue, GeoPoint, Nested, Array, Null, Unindexed]
  accessors: [Kind, AsString, AsInt, AsNumber, AsFloat, AsBool, AsTime, AsBytes, AsKey, AsGeoPoint, AsEntity, AsArray, IsNull]
  union_rule: zero members is ErrEmptyValue and more than one is ErrAmbiguousValue; this is the proto3 oneof encoding, not a house style
  integers: int64 travels as text, the same trick DynamoDB N uses and for the same precision reason
  doubles: a real JSON number, and a distinct type from Integer; collapsing the two breaks sort order and equality filters
  timestamps: RFC 3339 to nanoseconds on the wire, microseconds in storage, so a round trip truncates
  nested_entity: an entityValue carries no key; encoding an Entity with a key inside a value is rejected rather than silently dropped
  absent_vs_null: a missing property and a Null property differ to a filter, and the map keeps them different
operations:
  reads:
    Get: "(ctx, key Key, opts ...ReadOption) (*Entity, error)"
    GetMulti: "(ctx, keys []Key, opts ...ReadOption) (*LookupResult, error)"
  writes:
    Put: "(ctx, e Entity, opts ...WriteOption) (Key, error)", an upsert
    Insert: "(ctx, e Entity, opts ...WriteOption) (Key, error)", fails ALREADY_EXISTS
    Update: "(ctx, e Entity, opts ...WriteOption) error", fails NOT_FOUND
    Delete: "(ctx, key Key, opts ...WriteOption) error"
    Mutate: "(ctx, ms []Mutation) (*CommitResult, error)", the only multi-verb commit, and it takes no options; per-mutation options ride Mutation.With
  queries:
    Run: "(ctx, q *Query, opts ...ReadOption) (*Batch, error)"
    Count: "(ctx, q *Query, opts ...ReadOption) (int64, error)"
  transactions:
    RunInTransaction: "(ctx, fn func(*Tx) error, opts ...TxOption) error"
    RunReadOnly: "(ctx, fn func(*Tx) error, opts ...TxOption) error"
  keys:
    AllocateIDs: "(ctx, keys []Key) ([]Key, error)"
  aggregation: [Count, Sum, Avg]
  sizing: "MutationSize(m Mutation) (int, error)", a Client method because only the client knows the partition an Entity-level figure would omit
  admin: [Close, ProjectID, Endpoint, Namespace]
mutations:
  constructors: [InsertOp, UpdateOp, UpsertOp, DeleteOp]
  option_carrier: "Mutation.With(opts ...WriteOption) Mutation"
  shape: four verbs are members of one commit request, not four endpoints
transactions:
  Tx_reads: [Get, GetMulti, Run, Count], each taking a ctx and no options
  Tx_writes: [Put, Insert, Update, Delete, Mutate], queued and returning nothing, sent with the commit
  happy_path: a closure returning an error writes nothing, so no rollback is needed; a panic or an expired context rolls back
  no_explicit_begin:
    since: v1.1.7
    how: a Tx starts empty, the first read carries readOptions.newTransaction and the reply carries the handle everything after it uses; a commit with no read at all carries singleUseTransaction
    why_it_is_a_fold_and_not_a_guess: the client does not bet on the closure's shape, it starts the transaction inside a read it was going to send anyway
    measured: one read then commit costs 2 round trips where it cost 3, N reads then commit N+1 where it cost N+2, a write-only closure 1 where it cost 3, and a closure that neither read nor wrote 0 where it cost 2
    wider_than_asked: the ask scoped itself to one read plus one commit; every shape improved, and the write-only row is the larger proportional saving
    rollback: fires only when a handle exists, since a closure that never started one has nothing to release
  restart: ABORTED re-runs the whole closure, budgeted by WithTxRetries
  bound: the closure must be side-effect free outside the transaction, stated in godoc and not enforceable
  ErrTxClosed: using a Tx after its closure returned
queries:
  builder: "NewQuery(kind).Where(Condition).Filter(prop, op, Value).Ancestor(key).Order(prop).OrderDesc(prop).Project(...).KeysOnly().DistinctOn(...).Limit(int32).Offset(int32).Start(Cursor).End(Cursor)"
  operators: [LessThan, LessThanOrEqual, GreaterThan, GreaterThanEqual, Equal, NotEqual, HasAncestor, In, NotIn]
  conditions:
    types: "Condition, built with Prop(property, op, Value), And(...), Or(...) and AncestorOf(key)"
    attaching: "Query.Where(Condition); Query.Filter and Query.Ancestor are sugar over it, so the flat AND case reads as it always did"
    composition: repeated Where and Filter calls combine with AND, so an Or belongs inside one call rather than across two
    changed_in_v1_1_6: this was AND-only, and the package said so, until this side asked whether the claim still held; it did not
    MaxDisjunctions: 30 once the filter is put in disjunctive normal form, so an Or nested inside an And multiplies rather than adds; the driver exports the number and leaves the check to the service
  aggregations:
    Count: "(ctx, q, opts...) (int64, error)"
    Sum and Avg: "(ctx, q, property, opts...) (Value, error)", returning a Value so the integer-versus-double distinction survives, and Avg answering null rather than zero when nothing matched
    added_in_v1_1_6: on the argument that counting by paging can be keys-only and summing cannot
  Batch: "{Entities []Entity; EndCursor Cursor; More MoreResults; SkippedResults int32}", with HasMore()
  MoreResults: [NOT_FINISHED, MORE_RESULTS_AFTER_LIMIT, MORE_RESULTS_AFTER_CURSOR, NO_MORE_RESULTS]
  paging: one batch per call; EndCursor feeds Start, and nothing loops for the caller
  skipped_results: entities an offset stepped over, which were read and billed
  filter_on_any_property: single-property indexes are automatic, so a filter is not confined to the key the way a DynamoDB key condition is
  composite_indexes: declared out of band through the admin API; a query needing one fails at run time with FAILED_PRECONDITION, per decision:firestore-no-schema-artifact
options:
  client: [WithEndpoint, WithDatabase, WithNamespace, WithCredentials, WithTokenSource, WithTimeout, WithHTTPClient, WithMaxIdleConns, WithRetry]
  read: [WithEventualConsistency, WithReadTime]
  read_time_bound:
    stated_since: v1.1.7; before it the option published no bound at all, so the only way to learn one was to be refused
    windows: any microsecond-granularity instant within the past hour, and from one hour to seven days back only whole-minute timestamps, only with point-in-time recovery enabled
    caller_duty: truncate a read older than an hour; the option formats RFC3339Nano and does not truncate, since that would change the instant asked for and the boundary between the windows moves while the request is in flight
    failure: the service answers "read_time is too old", naming the age when the precision was what was wrong
    no_local_check: the client cannot see whether PITR is enabled or what earliestVersionTime is, so a range check would refuse reads that work; the same position MaxDisjunctions takes
    effect_here: nothing in firestorebind reaches for it yet, and a store that does passes the duty to its own caller per rule:firestorebind-driver-passthrough
  write: [WithBaseVersion, WithUpdateTime]
  transaction: [WithTxRetries]
  typing: ReadOption, WriteOption and TxOption are separate interfaces, so a consistency option on a write is a compile error
  doc_drift_2026_08_03: the driver's own client concept lists WithCredentialsFile and WithPropertyMask; the released code declares neither, and every option above was read from the tag rather than from that concept
errors:
  sentinels: [ErrNoSuchEntity, ErrAlreadyExists, ErrAborted, ErrFailedPrecondition, ErrInvalidArgument, ErrPermissionDenied, ErrUnauthenticated, ErrUnavailable, ErrDeadlineExceeded, ErrResourceExhausted, ErrInternal]
  wrapper: "*datastore.Error with Op, Kind, StatusCode, Status, Message, Unwrap, Retryable"
  discrimination: the Status string, never the HTTP code
  why: ABORTED and ALREADY_EXISTS are both 409 and mean opposite things, one retryable and one terminal
  miss: a key matching nothing is ErrNoSuchEntity from Get, and a Missing entry from GetMulti
retry:
  owned_by: the driver
  request: 3 attempts, exponential backoff with full jitter, 25ms base, 1s cap
  transaction: 3 closure re-runs on ABORTED, budgeted separately
  once_only: INTERNAL is retried exactly once, per Google's own guidance
  token_refresh: a 401 with a cached token refreshes once and resends, outside the request budget
  documented_effect: a request can be delivered up to attempts x 2 times; a replayed transactional commit fails rather than double-writing, because the handle is consumed
  idempotency: insert, update and delete are replayable by construction, which DynamoDB's ADD update is not
auth:
  scheme: one bearer token from cloud/google, minted about once an hour, rather than a per-request signature
  emulator: DATASTORE_EMULATOR_HOST over http, and it ignores the Authorization header entirely
limits:
  exported_since: v1.1.5, 1aa42bf, as named constants rather than numbers a consumer copies out of the documentation
  constants:
    MaxLookupKeys: 1000; GetMulti checks it before sending and answers ErrTooManyKeys
    MaxRequestBytes: 10 MiB
    MaxTransactionBytes: 10 MiB
    MaxEntityBytes: 1 MiB minus 4 bytes
    MaxKeyBytes: 6 KiB
    MaxIndexedStringBytes: 1500; a longer string is stored and simply not indexed
    MaxNestingDepth: 20 levels of entity value
  no_mutation_count:
    fact: Google documents no count limit on a commit, and the driver deliberately exports none
    bound_instead: bytes, through MaxRequestBytes and MaxTransactionBytes
    misleading_number: the documented 500 is property transformations per entity, which the driver excludes
    consequence: a batch write chunks by size, not by count; api:firestorebind-operations follows
composite_indexes:
  landed: v1.1.5, 1aa42bf
  types:
    Index: "{Kind string; Ancestor bool; Properties []IndexProperty}", with Valid, String and Equal
    IndexProperty: "{Name string; Direction Direction}", where Direction is Ascending or Descending
    MarshalIndexYAML: "([]Index) ([]byte, error)", producing what gcloud datastore indexes create consumes
  it_is_a_description_not_a_request: applying an index stays an admin-API operation and out of scope; the shape of an index is a property of the API, which is why the type lives here
  no_RequiredIndex:
    what: the driver declines to derive the needed index from a *Query
    reason: the rule for when a composite index is required is subtle, and a derivation that is quietly wrong is worse than none, since it names an index that does not fix the query
    effect_here: decision:firestore-no-schema-artifact inherits both the type and the warning
ttl:
  answer_2026_08_03: TTL is not expressible on this wire in Datastore mode
  what_it_is_instead: a field-level policy applied out of band, with gcloud firestore fields ttls update over an ordinary timestamp property
  consequence: an expiring entity needs nothing from this package or from firestorebind to expire; a plain timestamp property is the whole story, per rule:firestore-tag-options
  what_this_does_not_settle: which property a deployment points its policy at, which is a fact someone has to publish rather than apply; requirement:firestore-expiry-property-declaration reopens that half only
  contrast: system:tinygodriver-dynamodb is blocked the other way, where UpdateTimeToLive is absent and requirement:dynamo-ttl-attribute waits on it
excluded_by_the_driver: [GQL, ReserveIds, the admin API, watch and listeners, property transformations, auto-pagination, explain options]
exclusion_list_drift:
  what: this list named SUM and AVG until 2026-08-05, two days after v1.1.6 shipped them and with their arrival recorded under queries.aggregations in this same concept
  same_fault_the_driver_had: its README carried the identical contradiction, reported as the consumer's second ask and fixed in v1.1.7 with a test that fails when the not-in-scope list names anything exported
  reading: a list of absences is the part of prose a machine can check, and neither side was checking it; the driver now is
property_transformations_note:
  what: server-side increment and array-append, which exist on the wire inside commit
  why_excluded: they are the non-idempotent-retry hazard the retry policy is built to avoid
  effect_here: firestorebind has no read-modify-write to bind; that is decision:firestore-transaction-scope instead
upstream_requests:
  sent: 2026-08-03, from this catalog; answered in v1.1.5
  composite_index_descriptor:
    status: shipped, as Index, IndexProperty and MarshalIndexYAML
    plus: the driver declined RequiredIndex(*Query) and said why, which is the more useful half of the answer
  service_limits:
    status: shipped, as the constants above, and on the DynamoDB side too as MaxBatchGet, MaxBatchWrite, MaxItemBytes and MaxRequestBytes
    question_answered: there is no per-commit mutation count to export, so chunking by size is not a workaround but the correct shape
  Mutate_options:
    status: not taken; Mutate still takes no WriteOption
    assessment: correct to skip, since Mutation.With covers it per mutation and the asymmetry costs a reader one lookup
  round_three_2026_08_04:
    int_wrap:
      reported: datastore.Int admitted ~uint and converted through int64, so a uint above MaxInt64 stored a wrong number with no error, while the same value through IntString was correctly refused
      status: fixed in v1.1.6 by dropping ~uint from the Integer constraint, which is what was suggested
      effect_here: data:firestore-property-mapping still rejects uint, uint64 and uintptr at generation time; the driver no longer has a silent path, and this keeps the failure at the field rather than at the value
    or_filters:
      asked: whether the AND-only comment was still true, since the wire type carried an Op and we could not check the service
      answer: it was not; v1.1.6 added Condition, Prop, And, Or and Query.Where, keeping Query.Filter as sugar over Where(Prop(...))
      effect_here: requirement:firestore-typed-queries builds a filter tree, implemented 2026-08-04
      why_the_naming_worked_for_a_generator: Prop, And and Or all take and return one Condition, so the emitter is a recursive walk with one case per node; keeping Filter meant the flat declaration generates byte-identical code to before
    sum_and_avg:
      argued: counting by paging can be keys-only and summing cannot, so the reasoning that included COUNT applies harder to SUM and AVG
      status: added in v1.1.6, returning Value rather than a Go number so the integer-versus-double distinction survives, and Avg returning null rather than zero when nothing matched
    mutation_size:
      asked: a way to size a mutation without marshalling it twice
      status: added as Client.MutationSize, a method on the client rather than on Entity because only the client knows the partition; better than what was asked for
    key_partitioning:
      reported: a key used directly as a filter value got a partition and the same key inside an array did not, which meant the code contradicted itself whichever way the service behaves
      wider_than_reported: following it upstream found that every stored key property was also going out unpartitioned, which is the half that writes data and the half this side could not see
      status: fixed in v1.1.6
  round_four_2026_08_05:
    filed_by: the downstream framework rather than this catalog, against v1.1.6; three of its seven asks were the driver's, per decision:firestore-framework-requests
    answered_in: v1.1.7, all three
    single_use_transaction:
      reported: wireCommitRequest.SingleUseTransaction was declared and never assigned, and wireReadOptions carried no newTransaction at all, so the read half was not expressible; runOneTransaction therefore always began explicitly
      why_it_was_the_first_ask: with no condition expression on the wire, every predicate over a stored value is a transaction, so the extra round trip was paid per conditional write rather than occasionally
      status: shipped; see transactions.no_explicit_begin above
      landed_wider_than_filed: the ask assumed only one-read-plus-one-commit could fold, and the implementation removed the begin from every shape
    readme_contradiction:
      reported: the not-in-scope list still excluded SUM and AVG about two hundred lines below the section documenting both, so a consumer reading the list would write a paging loop to sum a property the service will sum
      also_reported: SkippedResults, DistinctOn, Project and RunReadOnly were implemented and unmentioned
      status: fixed, and now guarded by a test that fails when the not-in-scope list names anything the package exports
      same_fault_here: see exclusion_list_drift below, which this catalog carried simultaneously
    read_time_bound:
      reported: WithReadTime published no staleness bound, so an INVALID_ARGUMENT on a reasonable-looking value was the only way to learn one
      filed_before_anything_depended_on_it: nothing downstream reaches for read times yet
      status: documented; see options.read_time_bound above
  round_five:
    commit_envelope:
      ask: whether the bytes a commit request adds around its mutations are inside what MutationSize reports, or a figure the driver should name
      why: requirement:firestore-mutation-sizing drops a local 512-byte constant for the driver's measure, and the envelope is the only part that measure does not cover
      why_not_solved_here: rule:firestorebind-driver-passthrough forbids a local number, and a smaller local number is the same mistake
      answered_in: v1.1.9, as Client.CommitOverheadBytes(n) and Tx.CommitOverheadBytes(n), measured by marshalling the real request struct rather than returned as a constant, so a field added to the wire shape is counted without the caller knowing. It counts the n-1 commas itself, so a caller adding a per-mutation separator counts them twice
      adopted: 2026-08-06; firestorebind holds no envelope figure of its own, and the single-entity refusal is now CommitOverheadBytes(1)+size against the limit rather than size alone
      the_release_it_took_two_tags_to_reach:
        what_happened: the commit landed as 17f792b and the driver's working copy tagged it v1.1.8, but v1.1.8 had already been published from b3ad16f. Proxy versions are immutable, so go get returned a client without the method while the checkout showed one with it
        resolved: re-published as v1.1.9, on the same commit
        keep: check a version claim against the fetched module, not the checkout. The downstream framework filed this ask as already satisfied in v1.1.8, reading the driver's source, and it was satisfied in nothing fetchable. The reserve inside it is still the guess
  doc_drift:
    status: fixed for WithCredentialsFile, WithPropertyMask, RunReadOnly and the Tx method list
    new_drift_v1_1_5: the driver's own value concept still carries a NOT IMPLEMENTED marker on the struct mapper, added in 1aa42bf and left standing when ff4c947 implemented it two commits later; the code is authoritative, and this is worth sending back
related:
  - requirement:firestorebind-product-goals
  - decision:firestorebind-runtime-package
  - api:firestorebind-operations
  - data:firestore-property-mapping
  - rule:firestorebind-driver-passthrough
  - decision:firestore-key-identity
  - decision:firestore-transaction-scope
  - system:tinygodriver-dynamodb
  - concept:dynamobind-firestorebind-mapping
  - decision:firestore-framework-requests
  - requirement:firestore-mutation-sizing
```

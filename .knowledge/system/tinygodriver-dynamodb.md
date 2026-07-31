---
id: system:tinygodriver-dynamodb
type: system
title: tinygodriver DynamoDB Client
---
TinyGo-buildable DynamoDB client speaking the DynamoDB JSON protocol directly; the binding target of requirement:dynamobind-product-goals.

```yaml
package: github.com/shibukawa/tinygodriver/nosql/dynamodb
reason_for_existing: aws-sdk-go-v2 reaches full net/http.Transport through smithy-go and imports net/http/httputil, neither of which builds under TinyGo
release_status:
  released_in: tinygodriver v1.1.3, tag 70463f5
  verified_2026_07_31: the package at that tag is byte-identical to the driver checkout these concepts were read from, so every API fact below is confirmed against a published tag
  not_in: v1.1.2, tag aadc28f, which carries no nosql/dynamodb package
  driver_own_catalog: the tag also ships .knowledge concepts for the client, attribute value, retry policy, connection policy, JSON codec, and local endpoint; read them there rather than restating them here
item_model:
  Item: alias of map[string]AttributeValue
  Key: alias of map[string]AttributeValue
  AttributeValue: struct with S, N, B, BOOL, NULL, L, M, SS, NS, BS; exactly one set
  numbers: held as text; DynamoDB carries 38 significant digits and float64 does not
constructors: [S, "N[T Number]", NString, B, Bool, Null, List, Map, SS, NS, BS]
accessors: [Kind, AsString, AsInt, AsFloat, AsNumber, AsBytes, AsBool, AsList, AsMap, IsNull]
operations:
  GetItem: "(ctx, table string, key Key, opts ...GetOption) (Item, error)"
  PutItem: "(ctx, table string, item Item, opts ...WriteOption) (*WriteResult, error)"
  UpdateItem: "(ctx, table string, key Key, update string, opts ...WriteOption) (*WriteResult, error)"
  DeleteItem: "(ctx, table string, key Key, opts ...WriteOption) (*WriteResult, error)"
  Query: "(ctx, table, keyCond string, opts ...QueryOption) (*Page, error)"
  Scan: "(ctx, table string, opts ...ScanOption) (*Page, error)"
  BatchGetItem: "(ctx, keys map[string][]Key, opts ...BatchOption) (*BatchGetResult, error)"
  BatchWriteItem: "(ctx, writes map[string][]WriteRequest) (*BatchWriteResult, error)"
  table_admin: [CreateTable, DeleteTable, DescribeTable, ListTables]
result_types:
  WriteResult: "{Attributes Item}"; populated only under WithReturnValues
  Page: "{Items []Item; LastEvaluatedKey Key; Count int; ScannedCount int}"; LastEvaluatedKey non-nil means more, whatever Count says
  BatchGetResult: "{Items map[string][]Item; UnprocessedKeys map[string][]Key}"
  BatchWriteResult: "{UnprocessedItems map[string][]WriteRequest}"
schema_types:
  TableDefinition: "{Name; PartitionKey KeyAttribute; SortKey *KeyAttribute; BillingMode; ReadCapacity; WriteCapacity; GlobalIndexes; LocalIndexes}"
  KeyAttribute: "{Name string; Type AttributeType}"
  AttributeType: TypeString "S", TypeNumber "N", TypeBinary "B"; DynamoDB permits no other key type
  BillingMode: PayPerRequest default, Provisioned
errors:
  sentinels: [ErrItemNotFound, ErrResourceNotFound, ErrConditionalCheck, ErrThroughputExceeded, ErrThrottled, ErrValidation, ErrTableInUse, ErrTableNotFound, ErrRequestTooLarge, ErrTransactionConflict, ErrBadCredentials, ErrChecksumMismatch, ErrServerFailure]
  wrapper: "*Error with Op, Table, StatusCode, RequestID, Unwrap, Retryable"
  miss: a key matching nothing is ErrItemNotFound, not an empty Item
retry:
  owned_by: the driver
  defaults: DefaultAttempts 3, DefaultRetryBase 25ms
  documented_effect: a write can be delivered up to attempts x 2 times
reflection_path:
  api: [MarshalItem, UnmarshalItem]
  tag: dynamodbav, the aws-sdk-go-v2 spelling
  honored_options: only "-" and omitempty; stringset, numberset, binaryset and unixtime are read as nothing
  time: time.Time marshals to S in UTC RFC 3339 nano
  cost: linked only when called
excluded_by_the_driver: [transactions, PartiQL, Streams, DAX]
excluded_table_admin: TTL, global tables, backup, autoscaling, tags; UpdateTable index changes
transaction_note:
  asked: 2026-07-31, whether dynamobind supports DynamoDB transactions
  answer: no, and it cannot; the driver declares no TransactWriteItems or TransactGetItems
  misleading_sentinel: ErrTransactionConflict exists, and is not a trace of transaction support; DynamoDB returns TransactionConflictException to an ordinary PutItem whose item is held by someone else's transaction, and the driver maps it as retryable
  shape_if_ever_taken: a transaction spans types and tables, so it cannot be a single-type generic like Store; it needs a builder that accumulates encoded writes
  chunking_warning: TransactWriteItems caps at 100 items, and unlike StoreAll that cap must not be chunked, since splitting a transaction stops it being one; an oversized transaction is an error
upstream_requests:
  ranked_2026_07_31: by how much one upstream change unlocks downstream, per decision:dynamo-framework-requests
  UpdateTimeToLive:
    priority: first, and the smallest of the three
    unlocks: requirement:dynamo-ttl-attribute here, the framework's session and auth-state backends, and the TTL half of its migration
    natural_shape: a TableDefinition field, so it rides the create step rather than adding a workflow
  UpdateTable:
    priority: second
    unlocks: adding a secondary index to a live table, without which an index-bearing table can only ever be created, never evolved
    note: the same call site as the TTL work, which is why the two are worth sending together
  transactions:
    priority: third, and larger than the other two combined
    note: a separate request rather than a rider on the TTL one
related:
  - requirement:dynamobind-product-goals
  - decision:dynamobind-runtime-package
  - api:dynamobind-operations
  - data:dynamodb-attribute-mapping
  - rule:dynamobind-driver-passthrough
  - decision:dynamobind-json-transport-deferred
```

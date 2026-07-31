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
related:
  - requirement:dynamobind-product-goals
  - decision:dynamobind-runtime-package
  - api:dynamobind-operations
  - data:dynamodb-attribute-mapping
  - rule:dynamobind-driver-passthrough
  - decision:dynamobind-json-transport-deferred
```

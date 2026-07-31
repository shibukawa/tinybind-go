---
id: api:dynamobind-operations
type: api
title: dynamobind Operations
---
Typed wrappers over the driver item calls: the caller passes and receives application types, and every driver result the wrapper cannot carry stays reachable.

```yaml
status: accepted
package: github.com/shibukawa/tinybind-go/dynamobind
dispatch: decision:dynamobind-static-dispatch
single:
  Load: "func Load[T any, PT interface{*T; ItemDecoder}](ctx, c *dynamodb.Client, table string, key dynamodb.Key, opts ...dynamodb.GetOption) (T, error)"
  Store: "func Store[T ItemEncoder](ctx, c, table string, v T, opts ...dynamodb.WriteOption) error"
  Remove: "func Remove[T Keyer](ctx, c, table string, v T, opts ...dynamodb.WriteOption) error"
  Update: "func Update[T Keyer](ctx, c, table string, v T, update string, opts ...dynamodb.WriteOption) error"
returning:
  StoreReturning: "func StoreReturning[T ItemEncoder, PT interface{*T; ItemDecoder}](ctx, c, table string, v T, opts ...dynamodb.WriteOption) (T, bool, error)"
  RemoveReturning: "func RemoveReturning[T Keyer, PT interface{*T; ItemDecoder}](ctx, c, table string, v T, opts ...dynamodb.WriteOption) (T, bool, error)"
  constraint_shape: T carries the write-side interface and PT the decode side, so both are inferred from the argument as in Load
  meaning: the returned value is the item that was replaced or deleted; the bool is false when there was none
  implementation: send WithReturnValues ALL_OLD and decode WriteResult.Attributes into T
  scope: PutItem and DeleteItem accept only NONE and ALL_OLD, so ALL_OLD is the whole feature
  no_condition_failure_attributes: the driver exposes no ReturnValuesOnConditionCheckFailure, so a failed condition carries no attributes
  reason_for_a_separate_name: Store and Remove keep the error-only signature, so the common call pays nothing and no raw dynamodb.Item reaches a caller
paged:
  QueryPage: "func QueryPage[T any, PT ...](ctx, c, table, keyCond string, opts ...dynamodb.QueryOption) (Page[T], error)"
  ScanPage: "func ScanPage[T any, PT ...](ctx, c, table string, opts ...dynamodb.ScanOption) (Page[T], error)"
  Page: "type Page[T any] struct { Items []T; LastEvaluatedKey dynamodb.Key; Count int; ScannedCount int }"
  HasMore: "func (p Page[T]) HasMore() bool"
iterated:
  Query: "func Query[T any, PT ...](ctx, c, table, keyCond string, opts ...dynamodb.QueryOption) iter.Seq2[T, error]"
  Scan: "func Scan[T any, PT ...](ctx, c, table string, opts ...dynamodb.ScanOption) iter.Seq2[T, error]"
  measured: range-over-func and iter.Seq2 build under tinygo 0.41.1
  continuation: each page after the first re-sends the caller opts plus WithExclusiveStartKey from the previous LastEvaluatedKey; a caller-supplied WithExclusiveStartKey is the starting point, not an override of it
  stop: an early break ends the iteration without a further request
  error: a failed page yields one zero value with the error and ends
  cost_disclosure: the godoc states that one range can issue many requests and that an unfiltered Scan walks the whole table
  discards: Count, ScannedCount, and the final LastEvaluatedKey; a caller who needs to diagnose a filter-heavy query or resume a run uses QueryPage
  page_level_iterator: not shipped; add one yielding "Page[T]" only if resumption or ScannedCount is actually needed
batch:
  StoreAll: "func StoreAll[T ItemEncoder](ctx, c, table string, vs []T) (unprocessed []T, err error)"
  LoadAll: "func LoadAll[T any, PT ...](ctx, c, table string, keys []dynamodb.Key, opts ...dynamodb.BatchOption) (items []T, unprocessed []dynamodb.Key, err error)"
  limits: exported as MaxBatchWrite 25 and MaxBatchGet 100, so a caller sizing its own input reads the same numbers the chunking uses
  matching_back: StoreAll matches a declined write to the value that produced it by comparing encoded items, since the service returns the item rather than an index; the comparison is written out because AttributeValue holds pointers and slices and reflect.DeepEqual is not available here
  order: LoadAll returns items in DynamoDB's reply order, and a key matching nothing is absent rather than an error or an unprocessed key
  chunking:
    where: the runtime, with the fixed DynamoDB limits of 25 writes and 100 reads per request
    not_generated: the size limits, 16 MB per request and 400 KB per item, depend on the data rather than the type, so generated code could bound only fixed-width fields and would learn nothing from a string or slice; the sole compile-time facts are the two constants
    driver_note: the driver does not chunk; it sends what it is given
    oversize: surfaces as ErrRequestTooLarge, per rule:dynamobind-driver-passthrough
  driver_shape: the driver batch calls are multi-table maps; these helpers bind one table and fill the map
  unprocessed: returned, never retried, per rule:dynamobind-driver-passthrough
errors:
  passthrough: errors.Is against every driver sentinel and errors.As to *dynamodb.Error keep working through every helper
  decode: field-level; attribute name, expected kind, got kind
  type: "dynamobind.Error{Attribute, Expected, Got, Message}", built by TypeError for a wrong kind and ValueError for a value the field cannot hold
  finding_it: dynamobind.AsError walks the chain by type assertion, as jsonbind.AsError does, because errors.As needs reflection
partial_update:
  Update: takes the driver update expression verbatim and supplies only the key from v.ItemKey()
  no_expression_generation: no per-field setter or expression builder is generated; that is a separate project
  contrast: Store is PutItem, which replaces the whole item, so it is not a partial update
string_key_condition:
  status: the escape hatch, now that requirement:dynamo-typed-queries generates the declared form
  unchecked: keyCond and its expression values are untyped here, so a renamed attribute still compiles, and reserved words are the caller's to alias
deferred:
  - a page-level Query and Scan iterator
  - generated update and condition expressions
  - a typed wrapper over the multi-table batch form
related:
  - requirement:dynamobind-product-goals
  - system:tinygodriver-dynamodb
  - decision:dynamobind-runtime-package
  - decision:dynamobind-static-dispatch
  - rule:dynamobind-driver-passthrough
  - requirement:dynamobind-generated-item-codec
```

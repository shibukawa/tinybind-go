# dynamobind guide

`dynamobind` binds Go structs to DynamoDB items on top of
[`github.com/shibukawa/tinygodriver/nosql/dynamodb`](https://github.com/shibukawa/tinygodriver).
Declare the struct once, run the generator, and no call site handles a
`map[string]dynamodb.AttributeValue` again.

The driver stays where it is. `dynamobind` adds typing and takes nothing away:
every driver error, every retry decision and every page boundary is still yours.

## What generation automates

- `EncodeItem` and `DecodeItem` for each bound type, without reflection
- `ItemKey`, built from the same tags the table definition uses
- `<Type>Table`, so the key names in the schema and in the request cannot drift
- one named function per declared query, from a `.tb.dynamo` file
- compile-time assertions that the type satisfies the runtime interfaces

## What you provide

1. A struct whose fields carry `dynamo` tags
2. At least one `dynamobind` call naming the type
3. A `go:generate` line, or a `tinybind-gen generate` run

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .

type Reading struct {
	Sensor  string    `dynamo:"sensor,partitionkey"`
	At      int64     `dynamo:"at,sortkey"`
	Celsius float64   `dynamo:"celsius"`
	Flags   []string  `dynamo:"flags,stringset,omitempty"`
	Taken   time.Time `dynamo:"taken"`
	Ignored string    `dynamo:"-"`
}
```

```go
got, err := dynamobind.Load[Reading](ctx, client, "readings", want.ItemKey())
```

## Why not the driver's `MarshalItem`

The driver ships a reflection-based mapper. It works, and two things are wrong
with it for a struct that is known at compile time.

The first is drift. `TableDefinition.PartitionKey.Name`, the struct tag and the
`Key` you pass to `GetItem` are three unrelated strings. Rename one and the
program still compiles; it fails at run time with `ValidationException`.
Generation makes all three come from one declaration.

The second is cost, and it is the smaller problem: the reflection path is about
24 KB of binary and 0.8 µs per item. On the measurements in
[Sizes](#sizes) below, the generated codec is 19 KB smaller than the reflection
one and within 200 bytes of a codec written by hand.

## The `dynamo` tag

```text
dynamo:"<attribute name>[,<option>...]"
```

An empty name uses the Go field name. `dynamo:"-"` skips the field. Unexported
fields are always skipped.

| Option | Meaning |
|--------|---------|
| `partitionkey` | this field is the table partition key |
| `sortkey` | this field is the table sort key |
| `omitempty` | write no attribute at all when the field is its zero value |
| `stringset` | store a slice as `SS` rather than `L` |
| `numberset` | store a slice as `NS` |
| `binaryset` | store a slice as `BS` |
| `unixtime` | store a `time.Time` as `N` seconds since the epoch |

An option this list does not contain is a generation error. That is the
difference worth having: the driver's reflection path reads an unknown option as
nothing and quietly stores an `L` where you asked for a set.

The tag is spelled `dynamo`, not the SDK's `dynamodbav`. A field carrying
`dynamodbav` and no `dynamo` is a generation error rather than a field silently
stored under its Go name.

## Types

| Go | Attribute | Note |
|----|-----------|------|
| `string` | `S` | the empty string is a value, and is stored |
| `int`…`int64`, `uint`…`uint64` | `N` | via `strconv`, never through `float64` |
| `float32`, `float64` | `N` | |
| `bool` | `BOOL` | |
| `[]byte` | `B` | |
| `time.Time` | `S` as RFC 3339 nano, or `N` with `unixtime` | |
| `[]T` | `L`, or `SS`/`NS`/`BS` with a set option | |
| `map[string]T` | `M` | a non-string key is a generation error |
| nested struct | `M` | must be declared in the same package |
| `*T` | the pointee, or `NULL` when nil | |
| `dynamodb.AttributeValue` | stored as it stands | the escape hatch |

Numbers are text from end to end. A DynamoDB number carries 38 significant
digits and `float64` does not, so nothing here routes one through a float. A
value wider than the field is a decode error rather than a silent wrap:

```go
item["count"] = dynamodb.NString("70000") // the field is uint16
err := reading.DecodeItem(item)           // error, not 4464
```

A number with more digits than any Go type holds still round-trips, through a
`dynamodb.AttributeValue` field.

## Operations

```go
Load[T](ctx, c, table, key, opts...) (T, error)
Store(ctx, c, table, v, opts...) error
Remove(ctx, c, table, v, opts...) error
Update(ctx, c, table, v, expression, opts...) error

StoreReturning(ctx, c, table, v, opts...) (T, bool, error)
RemoveReturning(ctx, c, table, v, opts...) (T, bool, error)

QueryPage[T](ctx, c, table, keyCond, opts...) (Page[T], error)
ScanPage[T](ctx, c, table, opts...) (Page[T], error)
Query[T](ctx, c, table, keyCond, opts...) iter.Seq2[T, error]
Scan[T](ctx, c, table, opts...) iter.Seq2[T, error]

StoreAll(ctx, c, table, vs) (unprocessed []T, err error)
LoadAll[T](ctx, c, table, keys, opts...) (items []T, unprocessed []dynamodb.Key, err error)
```

Dispatch is by type constraint, not by a registry. A type with no generated
codec fails to compile, instead of failing at run time on a registration nobody
made.

`Store` is `PutItem`: it replaces the whole item. `Update` takes a DynamoDB
update expression verbatim and supplies only the key, which is the part a struct
tag can actually provide.

`StoreReturning` and `RemoveReturning` ask for `ALL_OLD` and decode what was
replaced or deleted. The bool is false when there was nothing there, which is
not an error.

## Pages and iterators

`QueryPage` is one request and returns `LastEvaluatedKey`, `Count` and
`ScannedCount`. `Query` iterates instead:

```go
for reading, err := range dynamobind.Query[Reading](ctx, c, "readings", "sensor = :s",
	dynamodb.WithExpressionValues(values)) {
	if err != nil {
		return err
	}
	use(reading)
}
```

One `range` can issue many requests, and the iterator reports none of the
per-page numbers. A query whose filter scans a hundred times what it returns
looks exactly like one that does not, and an interrupted run cannot be resumed.
Reach for `QueryPage` when any of that matters; `Scan` costs the same and walks
the whole table.

Breaking out of the loop stops without issuing another request.

## Declared queries

A query is declared in a `.tb.dynamo` file beside the package, and generation
turns each declaration into one named function:

```text
export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> {
  key sensor = {sensor} and at > {from}
}
```

```go
for reading, err := range ReadingsSince(ctx, client, "readings", "room-1", from) {
	if err != nil {
		return err
	}
	use(reading)
}
```

The result type picks the request shape rather than a row count, since a Query
always returns many: `dynamo.many<T>` iterates every page, `dynamo.page<T>`
issues one request and returns a `Page[T]`.

Every attribute the declaration names is checked against your `dynamo` tags, so
a renamed tag fails generation instead of failing in production. The key clause
accepts what DynamoDB accepts there and nothing else: the partition key with
`=`, and at most one sort key predicate from `=`, `<`, `<=`, `>`, `>=`,
`between` and `begins_with`. Naming a non-key attribute is an error that says so:

```text
readings.tb.dynamo:5: statement ReadingsByNote: note is not a key of Reading;
a key condition reaches sensor and at, and a non-key attribute belongs in a filter
```

Filter expressions are not implemented yet, so that message names a clause that
does not exist. They join the same declaration when they land.

### Reserved words are handled for you

DynamoDB reserves 573 words, including `status`, `name`, `size`, `type`, `data`,
`year`, `count` and `timestamp`, and an expression naming one literally is
rejected. Generated queries alias every attribute unconditionally, so the
question never arises:

```go
const eventsByStatusKeyCondition = "#k0 = :v0"

var eventsByStatusAttributeNames = map[string]string{"#k0": "status"}
```

The expression and the alias map are constants, fixed when the attribute names
are known, so nothing is assembled per call.

### The string form is still there

`Query` and `QueryPage` still take a key condition as text, for what a
declaration cannot express. Nothing checks that text against your tags, and the
reserved words above are yours to alias:

```go
// ValidationException: Attribute name is a reserved keyword
dynamobind.Query[Event](ctx, c, "events", "status = :s", values)

// Alias it yourself
dynamobind.Query[Event](ctx, c, "events", "#n0 = :s",
	dynamodb.WithExpressionNames(map[string]string{"#n0": "status"}),
	values)
```

## Batches

`StoreAll` and `LoadAll` split the input into requests DynamoDB accepts: 25
writes or 100 reads each. That much is arithmetic, and it lives in the runtime.

Retrying is not arithmetic and does not. What the service declined comes back:

```go
unprocessed, err := dynamobind.StoreAll(ctx, c, "readings", readings)
if err != nil {
	return err
}
// Retry policy is yours. The driver already retried the transport, and it
// documents that a write can be delivered attempts × 2 times; a loop here
// would multiply that silently.
```

`LoadAll` returns items in whatever order DynamoDB replies with, and a key that
matches nothing is simply absent — not an error and not an unprocessed key.

## Errors

Every driver sentinel survives:

```go
_, err := dynamobind.Load[Reading](ctx, c, "readings", key)
if errors.Is(err, dynamodb.ErrItemNotFound) {
	// a miss stays a miss; it never arrives as a zero value
}

var driverError *dynamodb.Error
if errors.As(err, &driverError) {
	log.Println(driverError.Op, driverError.RequestID, driverError.Retryable())
}
```

A decode failure names the attribute and both kinds:

```go
if mapping, ok := dynamobind.AsError(err); ok {
	log.Println(mapping.Attribute, mapping.Expected, mapping.Got) // at N S
}
```

`AsError` walks the chain without `errors.As`, which needs reflection.

## The table definition

```go
func ReadingTable(name string) dynamodb.TableDefinition
```

Generated whenever the type declares a `partitionkey`, from the same tags as the
codec. Tests need it to create tables, and even a program that never calls
`CreateTable` gets the key names from one place. The driver's `CreateTable` is
about 22 KB and the linker drops it when nothing calls it.

Pass `-disable item-table` to leave tables entirely to CloudFormation or
Terraform; the codec and the key builder stay. `-disable item-codec` turns the
whole mode off.

## What generation emits, and when

Generation is directed by what the package calls. `Store` produces an encoder,
`Load` a decoder, and a type nothing names produces nothing at all. A nested
struct inherits its parent's operations.

The key builder is the exception: a type that declares a `partitionkey` gets
`ItemKey` and its table definition whether or not a call needs them. The
documented way to read an item is `Load(ctx, c, table, v.ItemKey())`, and using
a method is not a call the generator can discover — waiting for one would mean
the method never existed to call. It is three lines, and the linker drops it
when nothing calls it.

## Sizes

Measured with TinyGo 0.41.1 on `wasip1`, for one program that stores and reads
one four-field item:

| Build | Bytes | Against the hand-written codec |
|-------|-------|-------------------------------|
| raw driver, item map built by hand | 3,543,805 | — |
| hand-written codec through `dynamobind` | 3,568,434 | — |
| **generated codec through `dynamobind`** | **3,568,604** | **+170** |
| driver `MarshalItem` reflection | 3,588,094 | +19,660 |

The generated codec costs 170 bytes more than the same codec written by hand,
and saves about 19 KB against the reflection mapper. The 24 KB between the first
two rows is the `dynamobind` API surface itself, not the codec: a program that
wants neither can still call the generated methods directly.

`encoding/json` and `reflect` are linked either way — the driver marshals its
request bodies with `encoding/json` — so no amount of generated code removes
them. Recovering those bytes needs a byte-level JSON path in the driver, and the
API above does not change when that lands.

## Constraints

- Transactions, PartiQL, Streams and DAX are out of scope, because the driver
  excludes them.
- A nested struct must be declared in the same package; a codec cannot be
  generated into someone else's.
- Secondary index tags are not implemented yet, so a declared query runs against
  the table's own keys.
- Filter, projection, condition and update expressions are not generated; pass
  those yourself.

# dynamobind guide

`dynamobind` binds Go structs to DynamoDB items on top of
[`github.com/shibukawa/tinygodriver/nosql/dynamodb`](https://github.com/shibukawa/tinygodriver).
Declare the struct and its access patterns once, run the generator, and no call
site handles a `map[string]dynamodb.AttributeValue` or names an attribute.

The driver stays where it is. `dynamobind` adds typing and takes nothing away:
every driver error, every retry decision and every page boundary is still yours.

- [What you write, what you get](#what-you-write-what-you-get)
- [The `dynamo` tag](#the-dynamo-tag)
- [Attribute types](#attribute-types)
- [Query declarations](#query-declarations)
- [Resolving the client from a Context](#resolving-the-client-from-a-context)
- [Runtime operations](#runtime-operations)
- [Pages and iterators](#pages-and-iterators)
- [Batches](#batches)
- [Errors](#errors)
- [The table definition](#the-table-definition)
- [Generation](#generation)
- [Generation errors](#generation-errors)
- [Sizes](#sizes)
- [Not implemented](#not-implemented)

## What you write, what you get

You write a tagged struct, optionally a `.tb.dynamo` file of access patterns,
and a `go:generate` line:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .

type Sensor string

type Reading struct {
	Sensor  Sensor    `dynamo:"sensor,partitionkey"`
	At      int64     `dynamo:"at,sortkey"`
	Celsius float64   `dynamo:"celsius"`
	Flags   []string  `dynamo:"flags,stringset,omitempty"`
	Taken   time.Time `dynamo:"taken"`
	Ignored string    `dynamo:"-"`
}
```

```text
export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> {
  table readings
  key sensor = {sensor} and at > {from}
}
```

Generation writes two files:

| File | Contents |
|------|----------|
| `dynamobind_gen.go` | `EncodeItem`, `DecodeItem`, `ItemKey`, `<Type>Table`, interface assertions |
| `dynamoquery_gen.go` | one function per declaration, plus its expression constants |

and you call them:

```go
if err := dynamobind.Store(ctx, client, "readings", reading); err != nil {
	return err
}

got, err := dynamobind.Load[Reading](ctx, client, "readings", reading.ItemKey())

for reading, err := range ReadingsSince(ctx, client, "room-1", from) {
	if err != nil {
		return err
	}
	use(reading)
}
```

None of that names an attribute. Every attribute name lives in a tag and in
generated code, so renaming a tag breaks compilation or generation rather than
production. A declared query names no table either: the declaration does.

### Why not the driver's `MarshalItem`

The driver ships a reflection-based mapper. Two things are wrong with it for a
struct known at compile time, and only the second is about speed.

The first is drift. `TableDefinition.PartitionKey.Name`, the struct tag and the
`Key` passed to `GetItem` are three unrelated strings. Rename one and the program
still compiles; it fails at run time with `ValidationException`.

The second is cost: the reflection path is about 24 KB of binary and 0.8 µs per
item. See [Sizes](#sizes).

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

## Attribute types

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

A named type works wherever its underlying type does, so `type Sensor string` is
an `S` and the generated code converts.

Numbers are text from end to end. A DynamoDB number carries 38 significant
digits and `float64` does not, so nothing here routes one through a float. A
value wider than the field is a decode error rather than a silent wrap:

```go
item["count"] = dynamodb.NString("70000") // the field is uint16
err := reading.DecodeItem(item)           // error, not 4464
```

A number with more digits than any Go type holds still round-trips through a
`dynamodb.AttributeValue` field.

Decoding leaves a field alone when the item carries no such attribute, so an
item written by an older version of the struct decodes without error.

## Query declarations

A `.tb.dynamo` file beside the package declares access patterns. Generation turns
each into one named function.

```text
export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> {
  table readings
  key sensor = {sensor} and at > {from}
}

export statement ReadingsBetween(sensor: Sensor, lo: int64, hi: int64): dynamo.page<Reading> {
  table readings
  key sensor = {sensor} and at between {lo} and {hi}
}

statement readingsForSensor(sensor: Sensor): dynamo.many<Reading> {
  table readings; key sensor = {sensor}
}
```

### Grammar

```text
[export] statement <Name>(<param>: <GoType>, ...): dynamo.<shape><<ItemType>> {
  table <name>
  key <attribute> = {param} [and <attribute> <predicate>]
}
```

- `export` must agree with the name's own casing, as Go decides visibility by the
  name: `export statement ReadingsSince` and `statement readingsForSensor` are
  both fine, and either one without the other is a generation error rather than a
  silent rename.
- Parameter types are Go types as your package spells them, including named types
  and `[]byte`.
- Both clauses are required. `table` names the table this pattern runs against,
  and the generated function takes no table parameter as a result.
- Clauses may appear in either order, and `;` separates them on one line.
- `//` starts a comment to end of line.

The result shape picks the request shape rather than a row count, since a Query
always returns many:

| Shape | Generated return | Requests |
|-------|------------------|----------|
| `dynamo.many<T>` | `iter.Seq2[T, error]` | one per page, as the range advances |
| `dynamo.page<T>` | `(dynamobind.Page[T], error)` | exactly one |

Sort key predicates, at most one per declaration:

| Written | Sends |
|---------|-------|
| `at = {p}` | `=` |
| `at < {p}`, `at <= {p}`, `at > {p}`, `at >= {p}` | the comparison |
| `at between {lo} and {hi}` | `BETWEEN` |
| `begins_with(at, {p})` | `begins_with`, on a string sort key only |

The partition key predicate is mandatory, comes first, and is always `=`, because
DynamoDB allows nothing else there.

### Generated signature

```go
func ReadingsSince(ctx context.Context, c *dynamodb.Client,
	sensor Sensor, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]
```

There is no table parameter: the `table` clause supplies it. The variadic options
reach the driver, so `dynamodb.WithLimit`, `WithScanForward`, `WithConsistentRead`
and `WithIndex` all work. The generated expression names and values are appended
last, so a caller option cannot replace the condition the declaration describes.

With `-dynamo-context-api`, a second function is generated beside this one that
takes neither the client nor the table. See
[Resolving the client from a Context](#resolving-the-client-from-a-context).

### Why the `table` clause is in the body

It belongs to the statement rather than to the type, because a type is not one
table: the same struct can be stored in a test table and a production one, so a
table on the type would assert something untrue. An access pattern names exactly
one, so the fact is complete where it is written. It is also the right direction:
the result type is the decode target, an output, while the table is an input, and
inputs belong in the body with the key clause and the parameters.

It is required rather than optional because one declaration form has to yield one
signature. An optional clause would produce a function with a table parameter and
one without, from bodies that look alike.

The name is checked against what DynamoDB accepts — three to 255 characters of
letters, digits, `_`, `-` and `.` — so a name the service would reject is a
generation error rather than a `ValidationException` on the first call.

A deployment prefix is not written here. It is resolved at run time, from the
Context.

Item operations keep their table parameter. They have no declaration to read one
from; that is the absence of a declaration rather than an inconsistency.

### Everything is checked against your tags

A declaration is text, so text alone would close nothing. What closes the drift
is that generation matches every name in it against the type's `dynamo` tags:

```text
readings.tb.dynamo:5: statement ReadingsByNote: note is not a key of Reading;
a key condition reaches sensor and at, and a non-key attribute belongs in a filter
```

This is a check the SQL template cannot make, having no schema. Here the tags are
one.

### Reserved words are handled for you

DynamoDB reserves 573 words, including `status`, `name`, `size`, `type`, `data`,
`year`, `count` and `timestamp`. An expression naming one literally is rejected
with `ValidationException`. Generated queries alias every attribute
unconditionally, so the question never arises:

```go
const readingsSinceKeyCondition = "#k0 = :v0 AND #k1 > :v1"

var readingsSinceAttributeNames = map[string]string{"#k0": "sensor", "#k1": "at"}
```

Because the names are known at generation time, the expression and the alias map
are constants: nothing is assembled per call, and no reserved-word list has to be
carried or kept current.

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

## Resolving the client from a Context

A client and a deployment table prefix are facts of one process, and threading
both through every handler is noise. Generation can add a Context-resolved
wrapper beside the explicit function:

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -dynamo-context-api
```

```go
func ReadingsSince(ctx context.Context, c *dynamodb.Client, sensor Sensor, from int64,
	opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]

func ReadingsSinceContext(ctx context.Context, sensor Sensor, from int64,
	opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]
```

The explicit form is always generated and never changes shape. The wrapper
resolves and delegates; it opens nothing and closes nothing.

Middleware installs the client once:

```go
ctx := dynamobind.WithClient(r.Context(), client, dynamobind.WithTablePrefix("staging-"))
```

and `ReadingsSinceContext` reads `staging-readings`, the declared name with the
prefix prepended.

### A missing prefix is an error

```go
WithClient(ctx context.Context, c *dynamodb.Client, options ...ClientOption) context.Context
WithTablePrefix(prefix string) ClientOption

ClientFromContext(ctx context.Context) (*dynamodb.Client, error)
TableFromContext(ctx context.Context, table string) (*dynamodb.Client, string, error)
```

There is no empty-prefix default. A Context carrying a client and no prefix is
`ErrNoTablePrefix`, and a deployment that uses the declared names unchanged says
so with `WithTablePrefix("")`.

That is stricter than the SQL executor of the same shape, and deliberately: a
missing executor cannot execute at all, while a missing prefix would read the
unprefixed table and answer with a normal empty page. Silently reading the wrong
table is indistinguishable from a table holding nothing.

The `dynamo.page` form returns the resolver error. The `dynamo.many` form cannot,
so it yields the error once and stops, which is how a failed page already reports.

### Item operations

`Load`, `Store` and the rest keep their client and table parameters. They are
runtime generics rather than generated code, so a Context variant of each would
double the exported surface for a line the caller can write:

```go
c, table, err := dynamobind.TableFromContext(ctx, "readings")
if err != nil {
	return err
}
return dynamobind.Store(ctx, c, table, reading)
```

`ClientFromContext` returns the client alone, for a table name the prefix does
not apply to.

### The two other modes

`-dynamo-context-only-api` publishes only the Context-resolved surface under the
declared name: `ReadingsSince` becomes the Context form, the client-taking one
becomes unexported, and no `ReadingsSinceContext` is generated, so that name stays
free.

`Options.DynamoClientResolver` selects a framework resolver instead of
`TableFromContext`, and implies the Context API. It has the same signature, so a
framework can map a declared name onto a physical one however it likes:

```go
func Table(ctx context.Context, table string) (*dynamodb.Client, string, error)
```

The mode is fixed at generation time and applies to the whole package.

## Runtime operations

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

Dispatch is by type constraint, not by a registry. A type with no generated codec
fails to compile, instead of failing at run time on a registration nobody made.

`Store` is `PutItem`: it replaces the whole item. `Update` takes a DynamoDB update
expression verbatim and supplies only the key, which is the part a struct tag can
actually provide.

`StoreReturning` and `RemoveReturning` ask for `ALL_OLD` and decode what was
replaced or deleted. The bool is false when there was nothing there, which is not
an error.

## Pages and iterators

`QueryPage` is one request and returns `LastEvaluatedKey`, `Count` and
`ScannedCount`. `Query` iterates instead, requesting each page as the range
advances.

One `range` can issue many requests, and the iterator reports none of the
per-page numbers. A query whose filter scans a hundred times what it returns
looks exactly like one that does not, and an interrupted run cannot be resumed.
Reach for `QueryPage` — or declare `dynamo.page<T>` — when any of that matters.
`Scan` costs the same and walks the whole table.

Breaking out of the loop stops without issuing another request.

## Batches

`StoreAll` and `LoadAll` split the input into requests DynamoDB accepts:
`MaxBatchWrite` is 25 and `MaxBatchGet` is 100, both exported so a caller sizing
its own input reads the same numbers the chunking uses. That much is arithmetic,
and it lives in the runtime.

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

This is the table's *shape*, not its name, which is why `name` is a parameter: a
type is not one table, and the same definition creates the test table and the
production one. The `table` clause of a declaration names one; this describes
what any of them looks like.

## Generation

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

Generation is directed by what the package calls. `Store` produces an encoder,
`Load` a decoder, and a type nothing names produces nothing at all. A nested
struct inherits its parent's operations. A `.tb.dynamo` declaration counts as a
use of its result type, so a package whose only DynamoDB use is a declaration
still gets the decoder its generated query needs.

The key builder is the exception: a type that declares a `partitionkey` gets
`ItemKey` and its table definition whether or not a call needs them. The
documented way to read an item is `Load(ctx, c, table, v.ItemKey())`, and using a
method is not a call the generator can discover — waiting for one would mean the
method never existed to call. It is three lines, and the linker drops it when
nothing calls it.

Every generated file records the SHA-256 of its inputs, so a rerun whose sources,
`.tb.dynamo` files, `go.mod`, options and generator binary all hash to the
recorded value exits without regenerating. `-force` regenerates regardless.

CLI flags:

| Flag | Effect |
|------|--------|
| `-dynamo-context-api` | also generate `<Name>Context` wrappers |
| `-dynamo-context-only-api` | publish only the Context-resolved surface |
| `-force` | regenerate regardless of the recorded input hash |

The rest live on `generator.Options` rather than on the CLI:

```go
options := generator.DefaultOptions()
options.DisableFeatures = []generator.Feature{generator.FeatureItemTable}
options.DynamoTemplatePattern = "*.query.dynamo"
options.DynamoClientResolver = &generator.SymbolPattern{PackagePath: "app/dynactx", Name: "Table"}
```

| Setting | Effect |
|---------|--------|
| `FeatureItemCodec` | turns the whole DynamoDB mode off, queries included |
| `FeatureItemTable` | drops `<Type>Table` only; the codec and key builder stay |
| `DynamoTemplatePattern` | the declaration glob; the default is `*.tb.dynamo` |
| `DynamoClientResolver` | a framework Context resolver; implies the Context API |

There is no CLI flag for these yet, unlike `-html-template-pattern` and
`-sql-template-pattern`. Drive them through `generator.New` for now.

## Generation errors

Every check names the type and the field, or the statement and the attribute,
because a message you can act on is the whole reason for failing here rather than
in production.

Tag and type checks:

- an unknown `dynamo` tag option
- a `dynamodbav` tag on a field with no `dynamo` tag
- two fields mapping to one attribute name
- two `partitionkey` fields, two `sortkey` fields, or a `sortkey` without a
  `partitionkey`
- a key field whose attribute is not `S`, `N` or `B`
- a Go type with no attribute form, a map with a non-string key, or a set option
  whose element type does not match
- a nested struct declared in another package
- a type that already declares `EncodeItem`, `DecodeItem` or `ItemKey` by hand

Query checks:

- a statement with no `table` clause, or with two
- a table name DynamoDB would reject
- an item type with no `dynamo` tags, or one with no `partitionkey`
- an attribute the type does not have
- a non-key attribute in the key clause
- a partition key predicate that is not `=`, or one that is not first
- more than one sort key predicate
- `begins_with` on an attribute that is not stored as a string
- a parameter whose type does not match the attribute's Go type
- a placeholder naming no declared parameter, or a parameter never used
- two statements with one name, or, under the Context API, a statement whose name
  another statement's wrapper would take

## Sizes

Measured with TinyGo 0.41.1 on `wasip1`, for one program that stores and reads
one four-field item:

| Build | Bytes | Against the hand-written codec |
|-------|-------|-------------------------------|
| raw driver, item map built by hand | 3,543,805 | — |
| hand-written codec through `dynamobind` | 3,568,434 | — |
| **generated codec through `dynamobind`** | **3,568,604** | **+170** |
| driver `MarshalItem` reflection | 3,588,094 | +19,660 |

The generated codec costs 170 bytes more than the same codec written by hand, and
saves about 19 KB against the reflection mapper. The 24 KB between the first two
rows is the `dynamobind` API surface itself, not the codec: a program that wants
neither can still call the generated methods directly.

`encoding/json` and `reflect` are linked either way — the driver marshals its
request bodies with `encoding/json` — so no amount of generated code removes them.
Recovering those bytes needs a byte-level JSON path in the driver, and the API
above does not change when that lands.

## Not implemented

- **Filter, projection, condition and update expressions.** A `filter` clause is
  rejected with a message saying so; pass those expressions yourself for now.
  They join the same declaration when they land.
- **Secondary indexes.** There is no `gsi` tag, so a declared query runs against
  the table's own keys. `dynamodb.WithIndex` still reaches the driver, but nothing
  checks the condition against that index's keys.
- **Single-table design.** One struct owns one table. The codec itself is
  indifferent to who else stores items in that table, but `<Type>Table` describes
  one type and a typed read decodes every item as one type, so a shared table
  needs those two written by hand.
- **Optimistic locking and TTL.** A `version` tag and a `ttl` tag are designed but
  not built; TTL also waits on `UpdateTimeToLive` in the driver.
- **Transactions, PartiQL, Streams and DAX.** The driver excludes them, so
  nothing here can offer them.

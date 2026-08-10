# firestorebind guide

`firestorebind` binds Go structs to Firestore entities in Datastore mode, on top of
[`github.com/shibukawa/tinygodriver/nosql/datastore`](https://github.com/shibukawa/tinygodriver).
Declare the struct and its access patterns once, run the generator, and no call
site builds a `datastore.Value` or names a property.

Rename a DynamoDB attribute and the service tells you: `ValidationException`, on
the first call. Rename a Datastore property and nothing tells you. The filter
still parses, the query still runs, and it matches nothing. An empty batch is the
whole failure signal, and an empty batch is also what a correct query returns
when there is genuinely nothing there. That silence is what this package is
built against.

The driver stays where it is. `firestorebind` adds typing and takes nothing away:
every driver error, every retry decision, every transaction restart and every
batch boundary is still yours.

- [What you write, what you get](#what-you-write-what-you-get)
- [The `firestore` tag](#the-firestore-tag)
- [Property types](#property-types)
- [Keys are paths, not properties](#keys-are-paths-not-properties)
- [Query declarations](#query-declarations)
- [Composite indexes](#composite-indexes)
- [The client and the namespace come from the Context](#the-client-and-the-namespace-come-from-the-context)
- [Runtime operations](#runtime-operations)
- [Conditional writes](#conditional-writes)
- [Transactions](#transactions)
- [Pages and iterators](#pages-and-iterators)
- [Batches](#batches)
- [Errors](#errors)
- [Generation](#generation)
- [Generation errors](#generation-errors)
- [Sizes](#sizes)
- [Not implemented](#not-implemented)

## What you write, what you get

You write a tagged struct, optionally a `.tb.firestore` file of access patterns,
and a `go:generate` line:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .

type SensorID string

type Reading struct {
	ID      SensorID      `firestore:"-,name"`
	Site    datastore.Key `firestore:"-,parent"`
	Version int64         `firestore:"-,version"`

	Sensor  SensorID  `firestore:"sensor"`
	At      time.Time `firestore:"at"`
	Celsius float64   `firestore:"celsius"`
	Note    string    `firestore:"note"`
	Tags    []string  `firestore:"tags,omitempty"`
	Body    string    `firestore:"body,noindex"`
	Ignored string    `firestore:"-"`
}
```

```text
export statement ReadingsSince(sensor: SensorID, from: time.Time): firestore.many<Reading> {
  where sensor == {sensor} and at > {from}
  order at desc
}
```

Generation writes two files:

| File | Contents |
|------|----------|
| `firestorebind_gen.go` | `EncodeEntity`, `DecodeEntity`, `Kind`, `EntityKey`, `EntityVersion`, interface assertions |
| `firestorequery_gen.go` | one function per declaration, plus its kind constant and any declared index |

and you call them, having installed the client once:

```go
ctx = firestorebind.WithClient(ctx, client)

key, err := firestorebind.Store(ctx, reading)

got, err := firestorebind.Load[Reading](ctx, reading.EntityKey())

for reading, err := range ReadingsSince(ctx, "room-1", from) {
	if err != nil {
		return err
	}
	use(reading)
}
```

None of that names a property. None of it names a kind either, and that is the
part with no DynamoDB equivalent: a table is a deployment fact, so `dynamobind`
declarations carry a `table` clause, but a kind belongs to the type. The result
type names `Reading`, `Reading` knows its own kind, and a declaration that
disagreed with the codec about what it queries cannot be written.

### Why not the driver's `MarshalEntity`

The driver ships a reflection-based mapper, and it reads the `datastore` struct
tag — the spelling `cloud.google.com/go/datastore` uses, so an example ported
from the official client works unchanged.

That courtesy is also the hazard. Two tags on one field are two mappings that
look interchangeable and disagree on every renamed property, silently, with both
paths compiling and both producing an `Entity`. Only one of them matches what a
query filters on. So a field carrying `datastore` and no `firestore` is a
generation error naming both spellings — which is exactly what the driver's own
documentation asks a generator over it to do.

## The `firestore` tag

```text
firestore:"<property name>[,<option>...]"
```

An empty name uses the Go field name. `firestore:"-"` skips the field, unless an
identity option below claims it. Unexported fields are always skipped.

| Option | Meaning |
|--------|---------|
| `name` | this string field supplies the key's name |
| `id` | this `int64` field supplies the key's numeric id |
| `parent` | this field supplies the ancestor path |
| `version` | this `int64` field receives the entity version a read returned |
| `ttl` | this `time.Time` property is what a TTL policy expires this kind by |
| `noindex` | store the property but keep it out of every index |
| `omitempty` | write no property at all when the field is its zero value |

An option this list does not contain is a generation error. Nothing softens a
typo here: the driver's mapper reads a different tag entirely, so a misspelled
option would otherwise become a property that simply never appears.

Two options carry a cost worth knowing about.

`omitempty` makes the property *absent*, not null. Datastore treats an absent
property and a null one as different things to a filter, and both are
representable, so the choice is real rather than cosmetic.

`noindex` is cheaper than it looks and narrower than it looks. Every Datastore
property is indexed by default, and each index costs write throughput and
storage, so excluding a long text field you never filter on is worth doing. But
an excluded property is in *no* index, which means a query can never match on it
— and generation enforces that, rejecting a declaration that filters, orders or
projects on a `noindex` field.

### `ttl` declares a property; it does not expire anything

Datastore mode has no expiry on the wire. Nothing in this package, and nothing in
the driver, can make an entity go away on a deadline. Expiry is a field-level
policy applied out of band:

```bash
gcloud firestore fields ttls update expires_at --collection-group=Session
```

So what is the tag for? A policy has to name a property, and something has to
tell the deployment step *which* property. Without the tag that list is kept by
hand beside your types, and the day someone renames the property there is no
compile error and no run-time error — just a policy pointed at a property that no
longer exists, and records that quietly never expire.

```go
type Session struct {
	Token     string    `firestore:"-,name"`
	ExpiresAt time.Time `firestore:"expires_at,ttl"`
}
```

The tag changes nothing about the write. `ExpiresAt` is encoded as an ordinary
`timestampValue`, byte for byte what it would be without the option. What you get
is one generated fact:

```go
func (v Session) ExpiryProperty() (string, bool) { return "expires_at", true }
```

satisfying `firestorebind.Expirer`, which is emitted only for a type that carries
the tag — so the assertion succeeding is itself the declaration. A deployment tool
walks your types, asks each one, and generates the `gcloud` calls from the same
declaration the codec came from.

At most one `ttl` field per type, and it must be a `time.Time`: a policy over
anything else expires nothing. A `ttl` on a field that is not stored is a
generation error for the same reason `noindex` on one is — it describes a policy
that can never fire.

One thing this does *not* yet answer: whether a TTL policy needs the property to
stay indexed, which would make `ttl` with `noindex` a contradiction rather than a
combination. It is not confirmed either way, so nothing here rejects it. If you
are excluding your expiry property from indexes, check the policy actually fires.

## Property types

| Go | Value | Note |
|----|-------|------|
| `string` | `stringValue` | the empty string is a value, and is stored |
| `int`…`int64` | `integerValue` | text on the wire, never through `float64` |
| `uint8`, `uint16`, `uint32` | `integerValue` | every value fits an `int64` |
| `float32`, `float64` | `doubleValue` | a real JSON number |
| `bool` | `booleanValue` | |
| `[]byte` | `blobValue` | base64 on the wire |
| `time.Time` | `timestampValue` | storage keeps microseconds; a round trip truncates |
| `datastore.Key` | `keyValue` | the nearest thing to a foreign key, and nothing enforces it |
| `datastore.LatLng` | `geoPointValue` | |
| `[]T` | `arrayValue` | |
| nested struct | `entityValue` | must be declared in the same package, and carries no key |
| `*T` | the pointee, or null when nil | |
| `datastore.Value` | stored as it stands | the escape hatch |

A named type works wherever its underlying type does, so `type SensorID string`
maps as a string.

Three rejections are worth their own explanation, because each one is a case
where the obvious mapping exists and is still wrong.

**Integer and double are different types.** DynamoDB has one `N` and folds every
numeric Go type into it. Datastore does not: it stores, orders and compares
`integerValue` and `doubleValue` separately. An `int64` field therefore decodes
only from an integer and a `float64` field only from a double, and a value
written by an earlier schema as the other one is a decode error rather than a
conversion. Coercing would produce a value that the very query that found it can
no longer find.

**`uint`, `uint64` and `uintptr` are generation errors.** A Datastore integer is
an `int64`, and the driver refuses to marshal anything wider, so nothing above
`math.MaxInt64` reaches the wire at all. A field that accepted `uint64` would
compile, store small values for months, and fail on the first large one. Use
`int64`, or a string property where the value really is that wide and is never
ordered on.

**Maps are generation errors.** A map would become a nested entity whose property
names come from run-time data rather than from the struct — and property names
coming from anywhere but a tag is the one thing this codec exists to prevent. Use
a nested struct, whose names are declared, or a `datastore.Value` field where the
names really are dynamic. The driver's own mapper refuses maps for the same
reason.

## Keys are paths, not properties

A DynamoDB partition key is an attribute in the item. A Datastore key is not a
property at all: it travels beside them, and it is a path of kind-and-identifier
pairs whose earlier elements are ancestors. Every structural difference between
this package and `dynamobind` follows from that one fact.

```go
type Reading struct {
	ID   SensorID      `firestore:"-,name"`   // key path leaf
	Site datastore.Key `firestore:"-,parent"` // everything before it
	// ...
}
```

Generation emits four things from that:

```go
func (v Reading) Kind() string             { return "Reading" }
func (v Reading) EntityKey() datastore.Key // the full path, ancestors included
func (v Reading) EncodeEntity() datastore.Entity
func (v *Reading) DecodeEntity(e datastore.Entity) error
```

The identity fields are **absent from the property map**. Writing them as
properties too would store identity twice and let the two copies drift, so the
encoder lifts them out and the decoder fills them back from the key. A value
returned by `Load` therefore carries its own identity without a second read.

That has a consequence for queries: you cannot filter on a property that is not
stored. A declaration naming `ID` in a `where` clause is a generation error
saying so, and pointing at the ancestor clause instead.

If you *want* the duplicate — because you need to filter on it — give the tag a
real name:

```go
ID SensorID `firestore:"sensor,name"` // key leaf and a stored property
```

That is an opt-in, and keeping the two in step is then yours.

The kind defaults to the Go type name. Nothing configures it, and nothing needs
to: a `Reading` is a `Reading` wherever it is stored, unlike a table name. That
is also why the single-table question `dynamobind` has to decline never arises
here — one type is one kind by construction.

An identifier field left at its zero value produces an **incomplete key**, which
is legal on insert and is where the server allocates an id. `Store` and `Insert`
return the stored key, so a caller assigns it back:

```go
task := Task{Title: "write it down"}       // Number is 0
key, err := firestorebind.Insert(ctx, task)
task.Number = key.Path[0].ID
```

`Remove` on an incomplete key is an error before a request is sent, since there
is nothing to identify.

## Query declarations

A `.tb.firestore` file beside the package declares access patterns. Generation
turns each into one named function.

```text
export statement RecentReadings(from: time.Time, size: int): firestore.batch<Reading> {
  where at > {from}
  order at desc
  limit {size}
}

export statement HotOrNamed(sensor: SensorID, note: string, from: time.Time): firestore.batch<Reading> {
  where (sensor == {sensor} or note == {note}) and at > {from}
  index sensor, at
}

statement readingsUnder(parent: datastore.Key): firestore.many<Reading> {
  ancestor {parent}; order at
}
```

### Grammar

```text
[export] statement <Name>(<param>: <GoType>, ...): firestore.<shape><<EntityType>> {
  where <condition>
  ancestor {param}
  select <property>, ...
  distinct <property>, ...
  order <property> [asc|desc], ...
  start {param}
  end {param}
  limit <n>|{param}
  offset <n>|{param}
  index <property> [asc|desc], ...
}
```

Every clause is optional. `export` must agree with the name's own casing, since
Go decides visibility by the name: either one without the other is a generation
error rather than a silent rename. Parameter types are Go types as your package
spells them. Clauses may appear in any order, `;` separates them on one line, and
`//` starts a comment to end of line.

There is no `kind` clause. The result type names the bound type and that type
supplies its kind, so a declaration cannot disagree with the codec about what it
queries. Item operations name no kind either, which makes the two consistent
rather than merely different.

The result shape picks the request shape, since a query always returns a batch:

| Shape | Generated return | Requests |
|-------|------------------|----------|
| `firestore.batch<T>` | `(firestorebind.Page[T], error)` | exactly one |
| `firestore.many<T>` | `iter.Seq2[T, error]` | one per batch, as the range advances |
| `firestore.count<T>` | `(int64, error)` | one aggregation query, no entities decoded |
| `firestore.keys<T>` | `(firestorebind.KeyPage, error)` | one keys-only request |

### Conditions

```text
where sensor == {s} and at > {from}
where sensor == {s} or note == {n}
where (sensor == {s} or note == {n}) and at > {from}
where sensor in {sensors}
where sensor not in {retired}
```

Comparisons are `==`, `!=`, `<`, `<=`, `>`, `>=`, `in` and `not in`. Writing `=`
is an error that tells you to write `==`, as in Go.

`and` binds tighter than `or`, also as in Go, and parentheses override it. Both
operators are needed because Datastore composes with both — which it did not
always do. This grammar rejected `or` by name for a while, citing an AND-only
wire; that was true when it was written and stopped being true when the driver
gained disjunctive queries.

`in` and `not in` take the candidates, so the parameter is a slice of what the
property stores: `sensor in {sensors}` wants `sensors: []SensorID`, and anything
else is a generation error naming both types.

A disjunctive query costs more than it looks. Datastore puts the whole filter
into disjunctive normal form and caps the result at
`datastore.MaxDisjunctions`, so an `or` nested inside an `and` multiplies rather
than adds. Nothing counts that here: the expansion rule is the service's, and a
count that disagreed would refuse a query that works. The generated godoc names
the constant so the number is one lookup away.

### Ordering, bounds and cursors

`order` takes one or more properties, ascending unless you write `desc`.

`limit` and `offset` take a literal or a parameter. An offset is worth avoiding:
the entities it steps over are read and billed. A cursor is the cheaper way to
resume, which is what `start` and `end` are for:

```text
export statement ReadingsFrom(cursor: datastore.Cursor, size: int): firestore.batch<Reading> {
  order at
  start {cursor}
  limit {size}
}
```

The parameter must be `datastore.Cursor`, so a string cannot be passed by
mistake. `Page[T].EndCursor` is what you feed back.

### Projections

`select` narrows what comes back to the named properties. The result type does
not change: the bound type arrives with everything unselected at its zero value,
which is already what the decoder does with an absent property. A projection is
bandwidth, not a different shape.

The hazard is on the way out, not the way in. Datastore has no partial update, so
`Store` and `Update` replace the whole entity — and writing back a projected
value erases every property it did not ask for. The generated godoc says so on
every projecting function. Nothing prevents it: the value is still the bound type
and still satisfies `EntityEncoder`.

A projection reads from an index, so every selected property must be indexed —
`noindex` on a selected field is a generation error. Selecting an array property
makes the service return one result per element rather than one per entity, and
the generator says so in the godoc, having seen the field's own Go type. One rule
it does not check: the service rejects projecting a property an equality filter
already fixes. That is published and unmeasured here, and a check that was wrong
would refuse a query that works, so the godoc names it instead.

`distinct` collapses results sharing the named properties. Datastore requires
those properties to lead the ordering, and both clauses sit in the same
declaration, so that is checked structurally rather than guessed at.

### Everything is checked against your tags

A declaration is text, and text alone would close nothing. What closes the drift
is that generation matches every name in it against the type's `firestore` tags:

```text
readings.tb.firestore:5: statement ReadingsByNote: Reading has no property "nte"
readings.tb.firestore:9: statement A: parameter s is string, but property sensor is stored from SensorID
readings.tb.firestore:14: statement B: body is tagged noindex on Reading, so it is
in no index and a query naming it can never match
```

The checks reach into a condition tree, so a renamed property nested inside an
`or` fails exactly as loudly as one at the top, and the message names the line
the comparison was written on.

### Generated signature

```go
func RecentReadings(ctx context.Context,
	from time.Time, size int, opts ...datastore.ReadOption) (firestorebind.Page[Reading], error)
```

No kind, no client, no query builder. The variadic options reach the driver, so
`datastore.WithEventualConsistency` and `WithReadTime` work.

A declaration also generates a transaction twin for the shapes a transaction can
serve — `batch`, `count` and `keys`:

```go
func RecentReadingsTx(ctx context.Context, tx *firestorebind.Tx,
	from time.Time, size int) (firestorebind.Page[Reading], error)
```

The iterator shape gets none. A `range` inside a transaction issues an unbounded
number of round trips inside something that has to commit, and a wrapper that
made that easy would be hiding the cost rather than binding it. Page explicitly
with the batch form when you need every entity inside a transaction.

### The driver's builder is still there

`Query`, `QueryPage`, `Count` and `QueryKeysPage` take a `*datastore.Query` built
by hand, for a query whose shape is decided at run time. Every `datastore.Query`
method now has a clause, so this is no longer the way to say something the
grammar cannot — it is the way to say something you do not know until the request
arrives. Nothing checks a hand-built query against your tags:

```go
q := datastore.NewQuery("Reading").Filter("sensor", datastore.Equal, datastore.String(id))
page, err := firestorebind.QueryPage[Reading](ctx, q)
```

## Composite indexes

Single-property indexes are automatic. A composite index is not: a query
combining an equality filter with an inequality, or ordering on a second
property, needs one declared out of band and applied with `gcloud`. Without it
the query compiles, passes every check here, and fails on its first run with
`FAILED_PRECONDITION`.

An `index` clause emits a value a deploy step can apply:

```text
export statement WarmReadings(sensor: SensorID, from: time.Time): firestore.many<Reading> {
  where sensor == {sensor} and at > {from}
  order at desc
  index sensor, at desc
}
```

```go
yaml, err := datastore.MarshalIndexYAML([]datastore.Index{WarmReadingsIndex})
// feed to: gcloud datastore indexes create index.yaml
```

The index is exported when its statement is, because a deploy step has to reach
it.

Nothing derives it. The generator could look at a declaration's filters and
orders and work out which index it needs — and that is exactly what the driver
declined to do, on the grounds that the rule is subtle and a derivation that is
quietly wrong names an index that does not fix the query. That argument is
stronger downstream, where a wrong diagnostic in a build log reads as
authoritative and the author who adds the named index still has a broken query.

What you get instead is a hint that names nothing. A declaration whose shape
commonly needs a composite index, and that declares none, gets a godoc line
saying it *may* need one. It claims no certainty, so there is nothing to act on
wrongly.

A declaration is not a deployment. An author who writes no `index` clause gets a
hint at most.

## The client and the namespace come from the Context

A client is a fact of one process. By default nothing takes it as a parameter:
install it once, and no call site and no generated signature carries it. That is
the default, not the only form — see [Passing the client instead](#passing-the-client-instead).

```go
ctx := firestorebind.WithClient(r.Context(), client)
```

```go
WithClient(ctx context.Context, c *datastore.Client, options ...ClientOption) context.Context
ClientFromContext(ctx context.Context) (*datastore.Client, error)
```

`ClientFromContext` is the escape hatch, for reaching the driver directly for
something this package does not wrap. It hands back the client and nothing else
— in particular it applies no namespace, so a key you pass through it is sent
exactly as you built it. That is the escape hatch's one sharp edge, and it is a
quiet one: a key with no namespace lands in the default namespace, succeeds, and
looks right. `KeyFor` places a key the way every wrapped entry above places one.

```go
KeyFor(ctx context.Context, key datastore.Key) datastore.Key
KeysFor(ctx context.Context, keys []datastore.Key) []datastore.Key
```

```go
client, _ := firestorebind.ClientFromContext(ctx)
entity, err := client.Get(ctx, firestorebind.KeyFor(ctx, key))
```

A key that already names a namespace keeps it, so an explicitly placed key is
never moved. With no client, no resolver, or a resolver returning the empty
string, the key comes back untouched — there is no error to return, so there is
none in the signature.

`dynamobind` needs a table-name resolver here, because a declared table name and
a deployed one differ. This package needs none: a kind is intrinsic to the type,
and no deployment renames it.

What varies instead is tenancy. A namespace is who is asking, not what the type
is, so putting it on the type would make one struct unusable for a second tenant.
The driver's own `datastore.WithNamespace` covers a namespace fixed for the
process; this covers one that varies per request:

```go
ctx := firestorebind.WithClient(r.Context(), client,
	firestorebind.WithNamespace(func(ctx context.Context) string {
		return tenantOf(ctx)
	}))
```

A generated key carries no namespace. The runtime stamps the resolved one on the
way out, so a bound type stays portable across tenants, and a key that already
names a namespace keeps it.

A Context with no client is `ErrNoClient`, reported in whatever way the result
shape allows: a function returning an error returns it, and an iterator yields it
once with the zero value and stops.

A second project, a second database or a test client is a second Context rather
than a second signature.

## Passing the client instead

The client and the namespace resolver together are a `Handle`. `WithClient`
stores one in the Context; `NewHandle` builds the same value to pass directly.

```go
type Handle struct{ /* opaque */ }

NewHandle(c *datastore.Client, options ...ClientOption) Handle
WithHandle(ctx context.Context, h Handle) context.Context
HandleFromContext(ctx context.Context) (Handle, error)

func (h Handle) Client() *datastore.Client
```

Every runtime entry has a twin suffixed `On` that takes the `Handle`, including
the key placement above:

```go
h := firestorebind.NewHandle(client, firestorebind.WithNamespace(tenantOf))

reading, err := firestorebind.LoadOn[Reading](ctx, h, key)
key, err := firestorebind.StoreOn(ctx, h, reading)
err = firestorebind.RunOn(ctx, h, func(tx *firestorebind.Tx) error { ... })

placed := firestorebind.KeyForOn(ctx, h, key)
```

The transactional entries inside a `Run` — `LoadTx`, `LoadAllTx`, `QueryPageTx`,
`QueryKeysPageTx`, `CountTx` — take a `*Tx` that already carries the client and
the tenancy. They look nothing up, so they have no twin and need none.

The `On` forms hold the implementation and the Context forms delegate to them, so
the two cannot drift. The `Context` is still the first argument in both: it
carries the deadline, and the driver needs it. What the `On` form drops is the
`ctx.Value` lookup, not the `Context`. `NamespaceResolver` still takes a
`Context` in both forms, because a per-request tenant is read from one even when
the client is not.

The zero `Handle` is `ErrNoClient`, exactly as a Context carrying no client is,
and `KeyForOn` returns the key untouched for it.

There is no method form. Go does not allow type parameters on methods, and every
entity entry is generic in the entity type, so `h.Load[Reading](...)` cannot
exist.

## Runtime operations

```go
Load[T](ctx, key, opts...) (T, error)
Store[T](ctx, v, opts...) (datastore.Key, error)
Insert[T](ctx, v, opts...) (datastore.Key, error)
Update[T](ctx, v, opts...) error
Remove[T](ctx, v, opts...) error

QueryPage[T](ctx, q, opts...) (Page[T], error)
Query[T](ctx, q, opts...) iter.Seq2[T, error]
Count(ctx, q, opts...) (int64, error)
QueryKeysPage(ctx, q, opts...) (KeyPage, error)

LoadAll[T](ctx, keys, opts...) (values []T, missing, deferred []datastore.Key, err error)
StoreAll[T](ctx, vs, opts...) ([]datastore.Key, error)
InsertAll[T](ctx, vs, opts...) ([]datastore.Key, error)
RemoveAll[T](ctx, vs, opts...) error
RemoveKeys(ctx, keys, opts...) error
```

None of these names a kind. Identity is complete in the key, so there is nothing
left for a signature to carry — which is the one place these are shorter than
their `dynamobind` counterparts, where every entry names a table.

Dispatch is by type constraint, not by a registry. A type with no generated codec
fails to compile, instead of failing at run time on a registration nobody made.

`Store` upserts. `Insert` and `Update` carry their preconditions in the name, and
that is the whole of it: they are the wire's own verbs, not conditions this
package composes. There is no partial update — Datastore has none, so every write
replaces the entity.

There are no returning forms. A commit carries back no prior entity, so
`StoreReturning` and `RemoveReturning` have nothing to decode. Read inside a
transaction when you need the old value; that is the honest cost.

## Conditional writes

Three levels, in increasing order of what they cost.

**The verbs.** `Insert` fails if the key exists, `Update` fails if it does not.
Put-if-absent and put-if-present are the two conditions callers write most often,
and here they cost nothing at all. `dynamobind` has to generate a condition
expression to get the first of them.

**A `version` tag.** The decoder fills it from the entity version a read
returned, and a later `Store` or `Update` sends it as a precondition:

```go
r, err := firestorebind.Load[Reading](ctx, key) // r.Version is now set
r.Celsius = 21.5
_, err = firestorebind.Store(ctx, r)            // applies only if nothing else wrote
```

A conflict is `datastore.ErrFailedPrecondition`, reaching you unchanged. A value
that was never read has version zero and sends no precondition, so a first write
is an ordinary `Store`. `Insert` takes none: it already fails if the key exists,
and a precondition on an entity that must not exist yet says nothing.

This is where the two backends diverge most cleanly. `dynamobind` has to reserve
the single `ConditionExpression` slot for a generated condition, which means a
caller's own condition and a `version` tag cannot coexist. Here `baseVersion` is
a mutation field, so the tag takes nothing away from a caller who also wants a
filter or an ancestor.

**A transaction.** Anything predicate-shaped. Nothing on this wire evaluates a
predicate over a property value, so reading inside a transaction and deciding in
Go is not a fallback — it is the only path.

## Transactions

```go
err := firestorebind.Run(ctx, func(tx *firestorebind.Tx) error {
	task, err := firestorebind.LoadTx[Task](ctx, tx, key)
	if err != nil {
		return err
	}
	task.Title = "after"
	tx.Store(task)
	return nil
})
```

```go
Run(ctx, fn func(*Tx) error, opts ...datastore.TxOption) error
RunReadOnly(ctx, fn func(*Tx) error, opts ...datastore.TxOption) error

LoadTx[T](ctx, tx, key, opts...) (T, error)
LoadAllTx[T](ctx, tx, keys) (values []T, missing, deferred []datastore.Key, err error)
QueryPageTx[T](ctx, tx, q) (Page[T], error)
QueryKeysPageTx(ctx, tx, q) (KeyPage, error)
CountTx(ctx, tx, q) (int64, error)

func (tx *Tx) Store(v EntityEncoder, opts ...datastore.WriteOption)
func (tx *Tx) Insert(v EntityEncoder, opts ...datastore.WriteOption)
func (tx *Tx) Update(v EntityEncoder, opts ...datastore.WriteOption)
func (tx *Tx) Remove(v Keyer, opts ...datastore.WriteOption)
```

`dynamobind` offers no transactions, because the DynamoDB driver declares none.
The reasoning that excluded them there includes them here: they are the only way
to express a read-modify-write.

The reads are separate functions rather than methods because Go methods cannot
take type parameters, and separate from `Load` itself because a transactional
read must travel through the handle. A Context carrying the handle instead would
make one call site mean two different things depending on which Context reached
it.

Two properties are not hidden, because hiding them would be lying.

**The closure can run more than once.** Contention makes the server answer
`ABORTED`, and the driver re-runs the whole closure rather than resending the
commit, because the reads it was built on are stale. So the closure must be free
of side effects outside the transaction: a message sent or a file written inside
it can happen several times.

**Queued writes return nothing.** `tx.Store` has no error to give because nothing
has happened yet — the mutations travel with the commit. A closure that returns
an error therefore writes nothing and needs no rollback.

No retry loop is added here. The driver's own restart budget applies, and
`datastore.WithTxRetries` configures it.

## Pages and iterators

`QueryPage` is one request. `Query` iterates instead, requesting each batch as
the range advances.

One `range` can issue many requests, and the iterator reports none of the batch
numbers. Reach for `QueryPage` — or declare `firestore.batch<T>` — when any of
that matters. Breaking out of the loop stops without issuing another request.

`Page[T]` keeps two things a bool would lose:

```go
type Page[T any] struct {
	Values         []T
	EndCursor      datastore.Cursor
	More           datastore.MoreResults
	SkippedResults int32
}
```

`More` says *why* a batch ended — ran out, hit a limit, hit a cursor — and a
batch that ended at a limit has a successor worth asking for while one that ran
out does not. `SkippedResults` counts what an offset stepped over, which was read
and billed.

`QueryKeysPage` returns a `KeyPage` of the same shape, for a keys-only query. It
does not set `KeysOnly` for you: a wrapper that did would return keys where the
caller's query said entities.

## Batches

`LoadAll` chunks at `datastore.MaxLookupKeys`, the driver's own constant, and
returns three lists rather than two:

```go
values, missing, deferred, err := firestorebind.LoadAll[Reading](ctx, keys)
```

Those are three different facts. A **missing** key holds nothing. A **deferred**
key is one the server chose not to read this time, and retrying it is your
decision — collapsing it into "absent" would lose exactly that. Values come back
in the server's reply order, not the order of your keys.

`StoreAll` chunks by encoded size against `datastore.MaxRequestBytes`, not by
count. That is not an implementation shortcut: Google documents no per-commit
mutation limit, a commit is bounded in bytes, and a count-based chunker would be
a number this package made up. Sizing is `datastore.Client.MutationSize`, which
measures the mutation as it will be sent — including the key with its project,
database and namespace attached, which only the client knows.

No service limit is written down here. `MaxLookupKeys`, `MaxRequestBytes` and the
rest are the driver's constants, because a copied limit is what drifts when the
service changes it.

`RemoveKeys` deletes by key rather than by value:

```go
err := firestorebind.RemoveKeys(ctx, keys)
```

It is the counterpart of `QueryKeysPage`. Find these keys, then delete them is the
shape of every cleanup, teardown and administrative sweep, and `RemoveAll` cannot
express it because it needs a bound value to take the key from. `RemoveAll` is now
`RemoveKeys` over the keys its values carry, so the two cannot drift. Deleting a
key that holds nothing succeeds, as it does on the wire; an incomplete key is
refused before anything is sent.

A namespace teardown is the case that needs this most, because there is no API
that deletes a namespace: page a keys-only query per kind, then sweep.

A chunked batch is **not** a transaction. A large write commits in pieces, and a
failure leaves the earlier pieces written. Use `Run` when the batch has to be
all-or-nothing, subject to `datastore.MaxTransactionBytes`.

## Errors

Every driver sentinel survives:

```go
_, err := firestorebind.Load[Reading](ctx, key)
if errors.Is(err, datastore.ErrNoSuchEntity) {
	// a miss stays a miss; it never arrives as a zero value
}

var driverError *datastore.Error
if errors.As(err, &driverError) {
	log.Println(driverError.Op, driverError.Status, driverError.Retryable())
}
```

Discriminate on `Status`, never on the HTTP code. `ALREADY_EXISTS` and `ABORTED`
are both 409 and mean opposite things — one terminal, one retryable — so code
that keyed on 409 would retry a duplicate insert forever. Nothing in this package
keys on a status code either.

A decode failure names the property and both kinds:

```go
if mapping, ok := firestorebind.AsError(err); ok {
	log.Println(mapping.Property, mapping.Expected, mapping.Got) // scale double integer
}
```

`AsError` walks the chain without `errors.As`, which needs reflection.

## Generation

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

Generation is directed by what the package calls. `Store` produces an encoder,
`Load` a decoder, and a type nothing names produces nothing at all. A nested
struct inherits its parent's operations but never a key, since an `entityValue`
carries none. A `.tb.firestore` declaration counts as a use of its result type,
so a package whose only Firestore use is a declaration still gets the decoder its
generated query needs.

Either client form counts. `StoreOn` is discovered exactly as `Store` is, so a
package that passes its `Handle` at every call site generates what the Context
form generates, and a package mixing the two — declared queries on the Context,
entity operations on a `Handle` — needs no setting to be seen.

Three methods are the exception, emitted from the tag rather than from a
discovered call: `Kind`, `EntityKey` and `EntityVersion`. The documented way to
read an entity is `Load(ctx, v.EntityKey())`, and the runtime asks for a version
by interface assertion — neither is a call the generator can discover, so waiting
for one would mean the method never existed to call. They are leaf functions and
the linker drops them when nothing calls them.

Every generated file records the SHA-256 of its inputs, so a rerun whose sources,
`.tb.firestore` files, `go.mod`, options and generator binary all hash to the
recorded value exits without regenerating. `-force` regenerates regardless.

```go
options := generator.DefaultOptions()
options.DisableFeatures = []generator.Feature{generator.FeatureEntityCodec}
options.FirestoreTemplatePattern = "*.query.firestore"
```

| Setting | Effect |
|---------|--------|
| `FeatureEntityCodec` | turns the whole Firestore mode off, queries included |
| `FirestoreTemplatePattern` | the declaration glob; the default is `*.tb.firestore` |
| `FirestoreParameterAPI` | generated queries take a leading `firestorebind.Handle` |
| `FirestoreHandleResolver` | generated queries read the Handle from a function you name |

The last two choose where a generated query gets its client, and behave exactly
as their DynamoDB counterparts do — see
[the dynamobind guide](dynamobind.md#generation) for the full account. In short:

```go
// FirestoreParameterAPI
func BySensor(ctx context.Context, h firestorebind.Handle, sensor Sensor, opts ...datastore.ReadOption) iter.Seq2[Reading, error]

// FirestoreHandleResolver: the signature is unchanged, the Handle comes from you
options.FirestoreHandleResolver = &generator.SymbolPattern{
	PackagePath: "example.com/app/pw",
	Name:        "DatastoreHandle",
}
```

```go
func DatastoreHandle(ctx context.Context) (firestorebind.Handle, error)
```

Setting neither is the default and emits exactly what it emitted before they
existed. The transactional twin of a declaration is unchanged in every mode,
because it takes a `*Tx` that already carries the client.
`-firestore-parameter-api` is the CLI flag for the first; the resolver is a
Go-API setting only.

`tinybind-gen fmt` formats `.tb.firestore` sources alongside the other three
template languages, with `--firestore-template-pattern` and `-as firestore`.
Formatting is idempotent, keeps comments, and puts clauses in a fixed order
regardless of how they were written — so reading order follows what the query
does rather than what the author happened to type first.

## Generation errors

Every check names the type and the field, or the statement and the property,
because a message you can act on is the whole reason for failing here rather than
in production.

Tag and type checks:

- an unknown `firestore` tag option
- a `datastore` tag on a field with no `firestore` tag
- two fields mapping to one property name
- more than one `name`, `id`, `parent`, `version` or `ttl` field, or both a `name` and an `id`
- `name` on a non-string field, `id` or `version` on a non-`int64` field, `ttl` on
  a non-`time.Time` field, `parent` on a field that is neither `datastore.Key` nor
  a bound type
- a parent chain that reaches its own type
- an identity option on a nested type, which has no key
- `noindex` or `ttl` on a field that is not stored as a property
- a Go type with no property form: a map, a `uint`/`uint64`/`uintptr`, a slice of
  slices, a channel, a function, an interface
- a nested struct declared in another package
- a type that already declares `EncodeEntity`, `DecodeEntity`, `EntityKey`,
  `Kind`, `EntityVersion` or `ExpiryProperty` by hand

Query checks:

- an entity type with no `firestore` tags
- a property the type does not have, at any depth of a condition tree
- a filter, order, projection or index naming a `noindex` property
- a filter naming an identity or version field, which is not stored
- a parameter whose type does not match the property's Go type, or one that is
  not a slice where `in` needs one
- a placeholder naming no declared parameter, or a parameter never used
- an ancestor parameter that is not `datastore.Key`, or a cursor parameter that
  is not `datastore.Cursor`
- `select`, `start` or `end` on a `count`, or `select` on a `keys` query
- `distinct` properties that do not lead the ordering
- `or` written where the grammar needs a comparison, or `=` where it needs `==`
- two statements with one name

## Sizes

Not measured. `dynamobind` has a measured table because its case needed one: the
DynamoDB driver ships a reflection mapper, so generating had to justify itself
against a working alternative, and the measurement showed the generated path is
the larger of the two.

One number carries over, because it is the same mechanism. Resolving the client
from the Context cost `dynamobind` about 38 KB on `wasip1` — a bare
`context.WithValue` plus one type assertion, which pulls in type-descriptor
machinery TinyGo otherwise drops. Expect the same here; it is inherent to the
pattern rather than to either package.

If a target is tight enough for that to matter, the generated `EncodeEntity`,
`DecodeEntity` and `EntityKey` are ordinary methods. Call the driver directly with
them and none of this package is linked.

## Not implemented

- **GQL.** A second request shape for the same queries, needing its own escaping
  story.
- **SUM and AVG in declarations.** The driver has `Sum` and `Avg`; no result
  shape reaches them yet, so call them with a hand-built query.
- **`AllocateIDs`.** No typed helper. Use the driver's when you need a key before
  the write.
- **A kind override.** The kind is the Go type name. A `kind=` option remains the
  shape if one is ever wanted; nothing has needed it.
- **Applying a TTL.** Expiry is not expressible on this wire in Datastore mode.
  The `ttl` tag declares which property a policy targets and emits
  `ExpiryProperty`; applying the policy stays `gcloud firestore fields ttls
  update`, run by whoever owns your deployment. Nothing here calls it, and
  nothing here can tell you whether it has been applied. Contrast `dynamobind`,
  where the `ttl` tag waits on `UpdateTimeToLive` in the driver — there the
  *declaration* is blocked on an apply; here only the apply is out of scope.
- **Watch, listeners and property transformations.** The driver excludes them.
  Transformations are excluded deliberately: server-side increment and
  array-append reintroduce exactly the non-idempotent-retry hazard the driver's
  retry policy is built to avoid, and this package documents its writes as
  replayable on the strength of that.

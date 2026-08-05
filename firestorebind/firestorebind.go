// Package firestorebind provides typed, reflection-free entity binding for
// Firestore in Datastore mode, on top of
// github.com/shibukawa/tinygodriver/nosql/datastore.
//
// A struct declares its properties once with firestore tags, tinybind-gen emits
// the codec, and the call site never builds a datastore.Value:
//
//	type Reading struct {
//		ID      string    `firestore:"-,name"`
//		Sensor  string    `firestore:"sensor"`
//		At      time.Time `firestore:"at"`
//		Celsius float64   `firestore:"celsius"`
//	}
//
//	ctx = firestorebind.WithClient(ctx, client)
//	got, err := firestorebind.Load[Reading](ctx, datastore.NameKey("Reading", "r-1"))
//
// The client is not a parameter. It, the namespace and the database are facts of
// one process or one request, installed once with WithClient, so no call site
// and no generated signature carries them; see context.go.
//
// Nothing here names a kind either. A kind belongs to the type, not to the
// deployment, so the generated Kind method supplies it and a key carries it.
// That is the one place this package's signatures are shorter than dynamobind's.
//
// Dispatch is by type constraint rather than by a registry, so a type without
// generated code fails to compile instead of failing at run time on a missing
// registration. Nothing here reflects on application fields.
//
// # The driver has its own mapper, and it is a different one
//
// nosql/datastore ships MarshalEntity behind the datastore struct tag. This
// package reads firestore instead, and the two disagree on every renamed
// property. Generation treats a field carrying datastore but not firestore as an
// error rather than as agreement; the driver's own documentation asks for
// exactly that.
//
// # What this package does not do
//
// It adds no retry loop: the driver already retries with backoff, and restarts a
// transaction closure on contention. It hides no batch boundary: Query iterates,
// but QueryPage stays public and returns the cursor and the reason the batch
// ended. It declares no service limit of its own: MaxLookupKeys and the rest are
// the driver's constants, because a copied limit is what drifts. It swallows no
// error: every driver sentinel survives errors.Is and *datastore.Error survives
// errors.As through every helper here.
package firestorebind

import (
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// EntityEncoder converts a value into a Datastore entity. Generated code
// implements it on the value receiver.
//
// The returned entity carries the key when the type declares one, and its
// properties never include the key fields: Datastore stores a key beside the
// properties, so writing them as properties too would store identity twice.
type EntityEncoder interface {
	EncodeEntity() datastore.Entity
}

// EntityDecoder fills a value from a Datastore entity. Generated code implements
// it on the pointer receiver.
//
// It fills the key fields from the entity's key as well as the properties, so a
// decoded value carries its own identity without a second read.
type EntityDecoder interface {
	DecodeEntity(e datastore.Entity) error
}

// Keyer reports the key of a value. Generated code implements it when the type
// carries a name or id tag.
type Keyer interface {
	EntityKey() datastore.Key
}

// Versioner reports the entity version a value was read at. Generated code
// implements it when the type carries a version tag.
//
// A non-zero version makes Store and Update conditional on the stored entity
// still being at that version, which is optimistic concurrency under its own
// name. A zero version sends no precondition, so a value that was never read
// writes unconditionally.
type Versioner interface {
	EntityVersion() int64
}

// Kinder reports the Datastore kind of a value. Generated code implements it for
// every bound type, defaulting to the Go type name.
type Kinder interface {
	Kind() string
}

// Expirer reports which property a TTL policy for this kind should expire
// entities by. Generated code implements it when the type carries a ttl tag, and
// only then, so the assertion succeeding is itself the declaration.
//
// Nothing in this package or in the driver applies a TTL. Datastore mode has no
// expiry on the wire: a policy is applied out of band, with
//
//	gcloud firestore fields ttls update <property> --collection-group=<kind>
//
// over an ordinary timestamp property. The tag changes nothing about how that
// property is written. It exists so the deployment step can be told which
// property to name, rather than that list being kept by hand beside the types
// and drifting the next time one is renamed — a drift with no compile error and
// no run-time error, just a policy pointed at a property that no longer exists
// and records that never expire.
//
// The boolean is always true for a generated type, and is there so a caller
// reaching this through the interface does not have to know that.
type Expirer interface {
	ExpiryProperty() (string, bool)
}

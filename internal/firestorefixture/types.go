// Package firestorefixture exercises the generated Firestore entity codec
// against the firestorebind runtime and the driver's wire protocol.
//
// The calls below are what generation is directed by: each one names the type it
// binds, and the generator emits exactly the methods those calls need.
package firestorefixture

import (
	"context"
	"iter"
	"time"

	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// SensorID is a named string, so the codec has to convert rather than assign.
type SensorID string

// Site is a nested entity, stored as an entityValue. It declares no identity,
// because an entityValue carries no key.
type Site struct {
	City string `firestore:"city"`
	Zip  string `firestore:"zip,omitempty"`
}

// Reading covers every property form the generator supports.
//
// ID carries the key's name and is absent from the properties: Datastore stores
// a key beside them, so writing it as a property too would store identity twice.
type Reading struct {
	ID     SensorID         `firestore:"-,name"`
	Parent datastore.Key    `firestore:"-,parent"`
	Ver    int64            `firestore:"-,version"`
	Note   string           `firestore:"note"`
	At     time.Time        `firestore:"at"`
	Scale  float64          `firestore:"scale"`
	Small  float32          `firestore:"small"`
	Count  int64            `firestore:"count"`
	Tiny   uint16           `firestore:"tiny"`
	Active bool             `firestore:"active"`
	Blob   []byte           `firestore:"blob"`
	Ref    datastore.Key    `firestore:"ref"`
	Where  datastore.LatLng `firestore:"where"`

	Tags   []string        `firestore:"tags,omitempty"`
	Nested Site            `firestore:"site"`
	Extra  *string         `firestore:"extra"`
	Body   string          `firestore:"body,noindex"`
	Raw    datastore.Value `firestore:"raw"`
}

// Task uses an integer id rather than a name, and stores nothing but its own
// identity plus one property.
type Task struct {
	Number int64  `firestore:"-,id"`
	Title  string `firestore:"title"`
}

// LoadReading is a decode-side use: T appears only in the result, so the AST
// carries it even before any codec exists.
func LoadReading(ctx context.Context, key datastore.Key) (Reading, error) {
	return firestorebind.Load[Reading](ctx, key)
}

// StoreReading is an encode-side use, whose type comes from the value argument.
func StoreReading(ctx context.Context, r Reading) (datastore.Key, error) {
	return firestorebind.Store(ctx, r)
}

// InsertTask exercises the put-if-absent verb over the id-keyed type.
func InsertTask(ctx context.Context, t Task) (datastore.Key, error) {
	return firestorebind.Insert(ctx, t)
}

// RemoveReading is a key-only use.
func RemoveReading(ctx context.Context, r Reading) error {
	return firestorebind.Remove(ctx, r)
}

// QueryReadings iterates a kind, which is what Scan would have been.
func QueryReadings(ctx context.Context, q *datastore.Query) iter.Seq2[Reading, error] {
	return firestorebind.Query[Reading](ctx, q)
}

// StoreReadings exercises the size-chunked batch write.
func StoreReadings(ctx context.Context, rs []Reading) ([]datastore.Key, error) {
	return firestorebind.StoreAll(ctx, rs)
}

// LoadReadings exercises the key-chunked batch read and its three results.
func LoadReadings(ctx context.Context, keys []datastore.Key) ([]Reading, []datastore.Key, []datastore.Key, error) {
	return firestorebind.LoadAll[Reading](ctx, keys)
}

// RenameInTransaction is a read-modify-write, which is the only thing a
// transaction is for here: nothing on this wire evaluates a predicate.
func RenameInTransaction(ctx context.Context, key datastore.Key, title string) error {
	return firestorebind.Run(ctx, func(tx *firestorebind.Tx) error {
		task, err := firestorebind.LoadTx[Task](ctx, tx, key)
		if err != nil {
			return err
		}
		task.Title = title
		tx.Store(task)
		return nil
	})
}

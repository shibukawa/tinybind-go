package firestorebind

import (
	"context"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// Tx is a typed view of one Datastore transaction.
//
// Datastore has no condition expression over property values, so a transaction
// is the only way to express a read-modify-write: read inside, decide in Go,
// commit. That is why this package binds transactions where dynamobind does not
// — there, they were a convenience the driver did not offer; here they are the
// only conditional path beyond the insert and update verbs.
//
// Writes queue and return nothing, matching the driver. Nothing has happened
// yet, and a signature that returned an error would be inventing one.
//
// Reads are methods too, so one transaction is written one way: tx.Load[Task]
// beside tx.Store(task). They became methods with Go 1.27, which is the first
// release letting a method declare its own type parameter; LoadTx, LoadAllTx,
// QueryPageTx, QueryKeysPageTx and CountTx are the earlier spelling and remain
// as deprecated wrappers.
type Tx struct {
	tx        *datastore.Tx
	namespace NamespaceResolver
	ctx       context.Context
}

// Run executes fn inside a read-write transaction and commits what it queued.
//
// fn can run more than once. Contention makes the server answer ABORTED, and the
// driver re-runs the whole closure rather than resending the commit, because the
// reads it was built on are stale. So fn must be free of side effects outside
// the transaction: a message sent or a file written inside it can happen several
// times. That is stated here rather than enforced, because it cannot be.
//
// A closure that returns an error writes nothing and needs no rollback, since
// the mutations travel with the commit that never happens.
//
// No retry loop is added here. The driver's own restart budget applies, and
// datastore.WithTxRetries configures it.
func Run(ctx context.Context, fn func(*Tx) error, opts ...datastore.TxOption) error {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return h.Run(ctx, fn, opts...)
}

// Run is Run on a Handle the caller already holds. The entries inside the
// closure are methods on the *Tx, which already carries the tenancy, so they
// have no second form.
func (h Handle) Run(ctx context.Context, fn func(*Tx) error, opts ...datastore.TxOption) error {
	c, ns, err := h.resolve()
	if err != nil {
		return err
	}
	return c.RunInTransaction(ctx, func(tx *datastore.Tx) error {
		return fn(&Tx{tx: tx, namespace: ns, ctx: ctx})
	}, opts...)
}

// RunReadOnly executes fn against a consistent snapshot.
//
// It queues no writes, so a read-only transaction never contends and never
// re-runs. Use it when several reads have to agree with each other and nothing
// is being changed.
func RunReadOnly(ctx context.Context, fn func(*Tx) error, opts ...datastore.TxOption) error {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return h.RunReadOnly(ctx, fn, opts...)
}

// RunReadOnly is RunReadOnly on a Handle the caller already holds.
func (h Handle) RunReadOnly(ctx context.Context, fn func(*Tx) error, opts ...datastore.TxOption) error {
	c, ns, err := h.resolve()
	if err != nil {
		return err
	}
	return c.RunReadOnly(ctx, func(tx *datastore.Tx) error {
		return fn(&Tx{tx: tx, namespace: ns, ctx: ctx})
	}, opts...)
}

// Store queues an upsert of v.
func (t *Tx) Store(v EntityEncoder, opts ...datastore.WriteOption) {
	t.tx.Put(t.encode(v), withBaseVersion(v, opts)...)
}

// Insert queues an insert of v, which fails the commit if the key exists.
func (t *Tx) Insert(v EntityEncoder, opts ...datastore.WriteOption) {
	t.tx.Insert(t.encode(v), opts...)
}

// Update queues a whole-entity update of v, which fails the commit if the key
// does not exist.
func (t *Tx) Update(v EntityEncoder, opts ...datastore.WriteOption) {
	t.tx.Update(t.encode(v), withBaseVersion(v, opts)...)
}

// Remove queues a delete of the entity identified by v's key.
func (t *Tx) Remove(v Keyer, opts ...datastore.WriteOption) {
	t.tx.Delete(applyNamespace(t.ctx, t.namespace, v.EntityKey()), opts...)
}

func (t *Tx) encode(v EntityEncoder) datastore.Entity {
	return withNamespace(t.ctx, t.namespace, v.EncodeEntity())
}

// Load reads one entity by key inside the transaction.
//
// It is reached through the transaction value rather than through Load itself
// because a transactional read has to travel through the transaction handle. A
// Context carrying the handle instead would make one call site mean two
// different things depending on which Context reached it.
func (t *Tx) Load[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, key datastore.Key, opts ...datastore.ReadOption) (T, error) {
	var out T
	entity, err := t.tx.Get(ctx, applyNamespace(t.ctx, t.namespace, key))
	if err != nil {
		return out, err
	}
	if entity == nil {
		return out, KeyError("Get returned no entity and no error")
	}
	if err := PT(&out).DecodeEntity(*entity); err != nil {
		return out, err
	}
	return out, nil
}

// LoadAll reads many entities by key inside the transaction. The three results
// mean what they do in the package-level LoadAll.
func (t *Tx) LoadAll[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, keys []datastore.Key) (values []T, missing, deferred []datastore.Key, err error) {
	keys = applyNamespaceAll(t.ctx, t.namespace, keys)
	values = make([]T, 0, len(keys))
	for chunk := range chunksOf(keys, datastore.MaxLookupKeys) {
		result, err := t.tx.GetMulti(ctx, chunk)
		if err != nil {
			return nil, nil, nil, err
		}
		if result == nil {
			continue
		}
		for _, entity := range result.Found {
			var decoded T
			if err := PT(&decoded).DecodeEntity(entity); err != nil {
				return nil, nil, nil, err
			}
			values = append(values, decoded)
		}
		missing = append(missing, result.Missing...)
		deferred = append(deferred, result.Deferred...)
	}
	return values, missing, deferred, nil
}

// QueryPage runs one query inside the transaction and decodes its batch.
func (t *Tx) QueryPage[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, q *datastore.Query) (Page[T], error) {
	batch, err := t.tx.Run(ctx, q)
	if err != nil {
		return Page[T]{}, err
	}
	return decodeBatch[T, PT](batch)
}

// QueryKeysPage runs one keys-only query inside the transaction.
//
// As outside a transaction, the query must already be keys-only.
func (t *Tx) QueryKeysPage(ctx context.Context, q *datastore.Query) (KeyPage, error) {
	batch, err := t.tx.Run(ctx, q)
	if err != nil {
		return KeyPage{}, err
	}
	return keysFromBatch(batch)
}

// Count counts matching entities inside the transaction.
func (t *Tx) Count(ctx context.Context, q *datastore.Query) (int64, error) {
	return t.tx.Count(ctx, q)
}

package firestorebind

import (
	"context"
	"encoding/json"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// mutationOverhead allows for what sizing cannot see: the partitionId a Client
// adds to every key on the way out, and the JSON wrapping one mutation in a
// commit request. It is deliberately generous, because overshooting the request
// limit costs a failed commit and undershooting costs one extra round trip.
const mutationOverhead = 512

// LoadAll reads many entities by key.
//
// The three results are three different facts and are not collapsed: values are
// what came back, missing are keys with no entity, and deferred are keys the
// server chose not to read this time. A deferred key is not a missing one, and
// retrying it is the caller's decision, per the driver's own contract.
//
// Values come back in the server's reply order, which is not the order of keys.
// A caller that needs its own order matches on the decoded key.
//
// Keys are chunked at datastore.MaxLookupKeys, so a caller passing more than one
// lookup accepts does not meet datastore.ErrTooManyKeys.
func LoadAll[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, keys []datastore.Key, opts ...datastore.ReadOption) (values []T, missing, deferred []datastore.Key, err error) {
	c, ns, err := clientFor(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	keys = applyNamespaceAll(ctx, ns, keys)
	values = make([]T, 0, len(keys))
	for chunk := range chunksOf(keys, datastore.MaxLookupKeys) {
		result, err := c.GetMulti(ctx, chunk, opts...)
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

// StoreAll upserts many entities, chunked into as few commits as the request
// limit allows.
//
// It chunks by encoded size rather than by count because Datastore publishes no
// per-commit mutation limit: a commit is bounded by datastore.MaxRequestBytes,
// and a count-based chunker would be a number this package made up. Sizing costs
// one JSON marshal per entity, which is not the marshal the driver then does.
//
// The returned keys are the stored ones, in the order the values were given, so
// an insert whose key was incomplete comes back completed at the same index.
//
// A commit is not a transaction. Chunking means a large batch commits in pieces,
// and a failure leaves the earlier pieces written; the error says which commit
// failed but not which entities within it. Use Run when the batch has to be
// all-or-nothing, subject to datastore.MaxTransactionBytes.
func StoreAll[T EntityEncoder](ctx context.Context, vs []T, opts ...datastore.WriteOption) ([]datastore.Key, error) {
	return mutateAll(ctx, vs, opts, datastore.UpsertOp)
}

// InsertAll inserts many entities. Each fails independently with
// datastore.ErrAlreadyExists if its key exists; see StoreAll on chunking.
func InsertAll[T EntityEncoder](ctx context.Context, vs []T, opts ...datastore.WriteOption) ([]datastore.Key, error) {
	return mutateAll(ctx, vs, opts, datastore.InsertOp)
}

// RemoveAll deletes the entities identified by the keys of vs.
//
// Deleting a key that holds nothing succeeds, as it does on the wire, so a
// caller cannot tell from the result which of them existed.
func RemoveAll[T Keyer](ctx context.Context, vs []T, opts ...datastore.WriteOption) error {
	c, ns, err := clientFor(ctx)
	if err != nil {
		return err
	}
	mutations := make([]datastore.Mutation, 0, len(vs))
	sizes := make([]int, 0, len(vs))
	for _, v := range vs {
		key := applyNamespace(ctx, ns, v.EntityKey())
		if key.Incomplete() {
			return KeyError("cannot remove an entity whose key is incomplete")
		}
		mutations = append(mutations, datastore.DeleteOp(key).With(opts...))
		sizes = append(sizes, len(key.String())+mutationOverhead)
	}
	for chunk := range chunksBySize(mutations, sizes, datastore.MaxRequestBytes) {
		if _, err := c.Mutate(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

func mutateAll[T EntityEncoder](
	ctx context.Context,
	vs []T,
	opts []datastore.WriteOption,
	op func(datastore.Entity) datastore.Mutation,
) ([]datastore.Key, error) {
	c, ns, err := clientFor(ctx)
	if err != nil {
		return nil, err
	}
	mutations := make([]datastore.Mutation, 0, len(vs))
	sizes := make([]int, 0, len(vs))
	for _, v := range vs {
		entity := withNamespace(ctx, ns, v.EncodeEntity())
		encoded, err := json.Marshal(entity)
		if err != nil {
			return nil, ValueError("", "cannot size the entity for chunking", err)
		}
		size := len(encoded) + mutationOverhead
		if size > datastore.MaxRequestBytes {
			return nil, ValueError("", "one entity is larger than a request", nil)
		}
		mutations = append(mutations, op(entity).With(opts...))
		sizes = append(sizes, size)
	}
	keys := make([]datastore.Key, 0, len(vs))
	for chunk := range chunksBySize(mutations, sizes, datastore.MaxRequestBytes) {
		result, err := c.Mutate(ctx, chunk)
		if err != nil {
			return nil, err
		}
		if result != nil {
			keys = append(keys, result.Keys...)
		}
	}
	return keys, nil
}

// chunksOf yields consecutive runs of at most n elements. A non-positive n
// yields the whole slice, since a zero-length chunk would not terminate.
func chunksOf[T any](items []T, n int) func(func([]T) bool) {
	return func(yield func([]T) bool) {
		if n <= 0 {
			if len(items) > 0 {
				yield(items)
			}
			return
		}
		for start := 0; start < len(items); start += n {
			end := min(start+n, len(items))
			if !yield(items[start:end]) {
				return
			}
		}
	}
}

// chunksBySize yields consecutive runs whose summed sizes stay under limit. An
// element at or over the limit on its own gets a chunk to itself; the caller has
// already rejected the cases where that cannot work.
func chunksBySize[T any](items []T, sizes []int, limit int) func(func([]T) bool) {
	return func(yield func([]T) bool) {
		start, total := 0, 0
		for i := range items {
			if i > start && total+sizes[i] > limit {
				if !yield(items[start:i]) {
					return
				}
				start, total = i, 0
			}
			total += sizes[i]
		}
		if start < len(items) {
			yield(items[start:])
		}
	}
}

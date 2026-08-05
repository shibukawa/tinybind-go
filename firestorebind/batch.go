package firestorebind

import (
	"context"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// mutationSeparator is the comma between two mutations in the request's JSON
// array. It is a fact of the encoding rather than a service limit, which is why
// it is counted here and not read from the driver.
const mutationSeparator = 1

// commitEnvelopeReserve is held back from datastore.MaxRequestBytes for what a
// commit request wraps around its mutations: the mode, the databaseId, and an
// optional transaction handle or singleUseTransaction block.
//
// datastore.Client.MutationSize measures one mutation and not the request, and
// the driver publishes no figure for the difference. Measured against a stub
// server, the wrapper is 42 bytes on a default client and 75 with an
// eighteen-character database name, so this is roughly an order of magnitude of
// room. It is held back once per commit rather than added per mutation, which
// is what the constant it replaced got wrong: at 512 bytes each, a thousand
// small entities lost half a megabyte of budget and an entity within 512 bytes
// of the limit was refused locally although the service would have taken it.
//
// This is the one figure here that is still a guess. It is asked upstream, and
// goes away when the driver names it.
const commitEnvelopeReserve = 4096

// commitBudget is the room one commit has for mutations.
func commitBudget(limit int) int { return limit - commitEnvelopeReserve }

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
// and a count-based chunker would be a number this package made up. Sizing is
// the driver's own datastore.Client.MutationSize, which measures the mutation
// as it will be sent, including the key's project, database and namespace.
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
// caller cannot tell from the result which of them existed. It is RemoveKeys
// over the keys the values carry, and shares its chunking and its refusal of an
// incomplete key.
func RemoveAll[T Keyer](ctx context.Context, vs []T, opts ...datastore.WriteOption) error {
	keys := make([]datastore.Key, 0, len(vs))
	for _, v := range vs {
		keys = append(keys, v.EntityKey())
	}
	return RemoveKeys(ctx, keys, opts...)
}

// RemoveKeys deletes the entities named by keys.
//
// It is the counterpart of QueryKeysPage, which hands back keys: find these
// keys, then delete them is the shape of every cleanup, teardown and
// administrative sweep, and RemoveAll cannot express it because it needs a bound
// value to take the key from.
//
// Deleting a key that holds nothing succeeds, as it does on the wire, so the
// result cannot say which of them existed. An incomplete key is refused before
// anything is sent, since it names no entity to delete.
//
// A commit is not a transaction. Chunking means a large sweep commits in pieces,
// and a failure leaves the earlier pieces deleted; use Run when the deletion has
// to be all-or-nothing, subject to datastore.MaxTransactionBytes.
func RemoveKeys(ctx context.Context, keys []datastore.Key, opts ...datastore.WriteOption) error {
	c, ns, err := clientFor(ctx)
	if err != nil {
		return err
	}
	keys = applyNamespaceAll(ctx, ns, keys)
	mutations := make([]datastore.Mutation, 0, len(keys))
	sizes := make([]int, 0, len(keys))
	for _, key := range keys {
		if key.Incomplete() {
			return KeyError("cannot remove an entity whose key is incomplete")
		}
		mutation := datastore.DeleteOp(key).With(opts...)
		size, err := c.MutationSize(mutation)
		if err != nil {
			return ValueError("", "cannot size the deletion for chunking", err)
		}
		mutations = append(mutations, mutation)
		sizes = append(sizes, size+mutationSeparator)
	}
	for chunk := range chunksBySize(mutations, sizes, commitBudget(datastore.MaxRequestBytes)) {
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
	budget := commitBudget(datastore.MaxRequestBytes)
	for _, v := range vs {
		entity := withNamespace(ctx, ns, v.EncodeEntity())
		mutation := op(entity).With(opts...)
		size, err := c.MutationSize(mutation)
		if err != nil {
			return nil, ValueError("", "cannot size the entity for chunking", err)
		}
		// Measured against the limit itself rather than against the chunking
		// budget: this check is "this entity can never be sent", and holding
		// back the envelope reserve here would refuse one the service takes.
		// Reserving for a wrapper is a batching concern, not an entity's.
		if size > datastore.MaxRequestBytes {
			return nil, ValueError("", "one entity is larger than a request", nil)
		}
		mutations = append(mutations, mutation)
		sizes = append(sizes, size+mutationSeparator)
	}
	keys := make([]datastore.Key, 0, len(vs))
	for chunk := range chunksBySize(mutations, sizes, budget) {
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

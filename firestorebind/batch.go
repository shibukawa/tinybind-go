package firestorebind

import (
	"context"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// Nothing here estimates what a commit request wraps around its mutations. That
// was a local reserve, held back from datastore.MaxRequestBytes and guessed at
// roughly an order of magnitude of room because no released driver named the
// figure; datastore.Client.CommitOverheadBytes names it as of tinygodriver
// v1.1.9. It is measured by marshalling the request the client will actually
// send, so a field added to the wire shape is counted without this package being
// touched, which is the property a constant here could never have.
//
// It counts the commas between n mutations itself. That is why no per-mutation
// separator is added below: doing both would count every comma twice.
//
// Both chunked paths are non-transactional, so the Client method is the right
// one. A commit inside a transaction carries a handle or a singleUseTransaction
// block as well, and Tx.CommitOverheadBytes is what would measure that.

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
	h, err := HandleFromContext(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	return LoadAllOn[T, PT](ctx, h, keys, opts...)
}

// LoadAllOn is LoadAll taking its Handle as an argument.
func LoadAllOn[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, h Handle, keys []datastore.Key, opts ...datastore.ReadOption) (values []T, missing, deferred []datastore.Key, err error) {
	c, ns, err := h.resolve()
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
	h, err := HandleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return StoreAllOn(ctx, h, vs, opts...)
}

// StoreAllOn is StoreAll taking its Handle as an argument.
func StoreAllOn[T EntityEncoder](ctx context.Context, h Handle, vs []T, opts ...datastore.WriteOption) ([]datastore.Key, error) {
	return mutateAll(ctx, h, vs, opts, datastore.UpsertOp)
}

// InsertAll inserts many entities. Each fails independently with
// datastore.ErrAlreadyExists if its key exists; see StoreAll on chunking.
func InsertAll[T EntityEncoder](ctx context.Context, vs []T, opts ...datastore.WriteOption) ([]datastore.Key, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return InsertAllOn(ctx, h, vs, opts...)
}

// InsertAllOn is InsertAll taking its Handle as an argument.
func InsertAllOn[T EntityEncoder](ctx context.Context, h Handle, vs []T, opts ...datastore.WriteOption) ([]datastore.Key, error) {
	return mutateAll(ctx, h, vs, opts, datastore.InsertOp)
}

// RemoveAll deletes the entities identified by the keys of vs.
//
// Deleting a key that holds nothing succeeds, as it does on the wire, so a
// caller cannot tell from the result which of them existed. It is RemoveKeys
// over the keys the values carry, and shares its chunking and its refusal of an
// incomplete key.
func RemoveAll[T Keyer](ctx context.Context, vs []T, opts ...datastore.WriteOption) error {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return RemoveAllOn(ctx, h, vs, opts...)
}

// RemoveAllOn is RemoveAll taking its Handle as an argument.
func RemoveAllOn[T Keyer](ctx context.Context, h Handle, vs []T, opts ...datastore.WriteOption) error {
	keys := make([]datastore.Key, 0, len(vs))
	for _, v := range vs {
		keys = append(keys, v.EntityKey())
	}
	return RemoveKeysOn(ctx, h, keys, opts...)
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
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return RemoveKeysOn(ctx, h, keys, opts...)
}

// RemoveKeysOn is RemoveKeys taking its Handle as an argument.
func RemoveKeysOn(ctx context.Context, h Handle, keys []datastore.Key, opts ...datastore.WriteOption) error {
	c, ns, err := h.resolve()
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
		sizes = append(sizes, size)
	}
	for chunk := range chunksByCommitSize(mutations, sizes, c.CommitOverheadBytes, datastore.MaxRequestBytes) {
		if _, err := c.Mutate(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

func mutateAll[T EntityEncoder](
	ctx context.Context,
	h Handle,
	vs []T,
	opts []datastore.WriteOption,
	op func(datastore.Entity) datastore.Mutation,
) ([]datastore.Key, error) {
	c, ns, err := h.resolve()
	if err != nil {
		return nil, err
	}
	mutations := make([]datastore.Mutation, 0, len(vs))
	sizes := make([]int, 0, len(vs))
	// The smallest request that can carry one mutation is that mutation plus the
	// envelope for a commit of one. The check below asks whether an entity can
	// ever be sent, so it wants exactly that and nothing more.
	//
	// It used to be measured against datastore.MaxRequestBytes alone, because
	// the envelope was a reserve big enough that including it would have refused
	// entities the service takes - the observable bug of the constant before it.
	// A measured figure has no such margin to protect against.
	alone := c.CommitOverheadBytes(1)
	for _, v := range vs {
		entity := withNamespace(ctx, ns, v.EncodeEntity())
		mutation := op(entity).With(opts...)
		size, err := c.MutationSize(mutation)
		if err != nil {
			return nil, ValueError("", "cannot size the entity for chunking", err)
		}
		if alone+size > datastore.MaxRequestBytes {
			return nil, ValueError("", "one entity is larger than a request", nil)
		}
		mutations = append(mutations, mutation)
		sizes = append(sizes, size)
	}
	keys := make([]datastore.Key, 0, len(vs))
	for chunk := range chunksByCommitSize(mutations, sizes, c.CommitOverheadBytes, datastore.MaxRequestBytes) {
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

// chunksByCommitSize yields consecutive runs that fit in one commit: the run's
// mutation sizes plus the envelope a commit of that many wraps around them stay
// within limit. An element that does not fit even on its own gets a chunk to
// itself; the caller has already rejected the cases where that cannot work.
//
// The envelope is asked for per candidate length rather than subtracted once up
// front, because it is not a constant: it grows with the commas between the
// mutations, so the run's own length is part of it. That is also why overhead
// takes a count, which lets this stay a running total instead of re-measuring
// the whole chunk on every step.
func chunksByCommitSize[T any](items []T, sizes []int, overhead func(n int) int, limit int) func(func([]T) bool) {
	return func(yield func([]T) bool) {
		start, total := 0, 0
		for i := range items {
			if i > start && overhead(i-start+1)+total+sizes[i] > limit {
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

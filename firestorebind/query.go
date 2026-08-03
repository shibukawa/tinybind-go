package firestorebind

import (
	"context"
	"iter"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// Page is one batch of decoded entities.
//
// More says why the batch ended and is the authority on whether another
// follows. It is kept rather than flattened to a bool because the reason
// matters: a batch that ended at a limit and one that ended because the results
// ran out are different facts, and only the first has a successor worth asking
// for.
type Page[T any] struct {
	Values    []T
	EndCursor datastore.Cursor
	More      datastore.MoreResults

	// SkippedResults counts entities an offset stepped over. They were read
	// and billed.
	SkippedResults int32
}

// HasMore reports whether running the query again from EndCursor could return
// anything.
func (p Page[T]) HasMore() bool {
	return p.More == datastore.NotFinished || p.More == datastore.MoreResultsAfterLimit
}

// QueryPage runs one query and decodes its batch.
//
// This is the form that keeps the request count visible: one call is one
// request. Query iterates instead, at the cost of hiding how many requests that
// takes.
func QueryPage[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, q *datastore.Query, opts ...datastore.ReadOption) (Page[T], error) {
	c, _, err := clientFor(ctx)
	if err != nil {
		return Page[T]{}, err
	}
	batch, err := c.Run(ctx, q, opts...)
	if err != nil {
		return Page[T]{}, err
	}
	return decodeBatch[T, PT](batch)
}

// Query iterates every entity a query matches, requesting batches as the range
// advances.
//
// One range can issue many requests, and a query with only a kind walks every
// entity of that kind. The iterator reports no batch boundary, no cursor and no
// SkippedResults, so a query that steps over far more than it returns looks the
// same as one that does not, and an interrupted run cannot be resumed. Use
// QueryPage when any of that matters.
//
// Iteration stops at the first error, which is yielded once with the zero value
// of T. A break stops it without issuing a further request.
func Query[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, q *datastore.Query, opts ...datastore.ReadOption) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		next := q
		for {
			page, err := QueryPage[T, PT](ctx, next, opts...)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, value := range page.Values {
				if !yield(value, nil) {
					return
				}
			}
			// An empty batch that still claims more would loop forever with
			// no cursor to advance past, so an absent cursor ends it.
			if !page.HasMore() || page.EndCursor == "" {
				return
			}
			next = next.Start(page.EndCursor)
		}
	}
}

// KeyPage is one batch of a keys-only query.
//
// A keys-only query reads no properties, which is the cheap way to test
// existence in bulk or to collect keys for a later LoadAll. It pages like any
// other query, and the reason a batch ended is kept for the same reason.
type KeyPage struct {
	Keys      []datastore.Key
	EndCursor datastore.Cursor
	More      datastore.MoreResults

	SkippedResults int32
}

// HasMore reports whether running the query again from EndCursor could return
// anything.
func (p KeyPage) HasMore() bool {
	return p.More == datastore.NotFinished || p.More == datastore.MoreResultsAfterLimit
}

// QueryKeysPage runs one keys-only query and collects the keys of its batch.
//
// It is not generic: nothing is decoded, so there is no type to infer. The
// query must already be keys-only; this does not set that for the caller,
// because a query that silently returned keys where the caller expected
// entities would be the surprise this package avoids.
func QueryKeysPage(ctx context.Context, q *datastore.Query, opts ...datastore.ReadOption) (KeyPage, error) {
	c, _, err := clientFor(ctx)
	if err != nil {
		return KeyPage{}, err
	}
	batch, err := c.Run(ctx, q, opts...)
	if err != nil {
		return KeyPage{}, err
	}
	if batch == nil {
		return KeyPage{}, nil
	}
	out := KeyPage{
		Keys:           make([]datastore.Key, 0, len(batch.Entities)),
		EndCursor:      batch.EndCursor,
		More:           batch.More,
		SkippedResults: batch.SkippedResults,
	}
	for _, entity := range batch.Entities {
		if entity.Key == nil {
			// A keys-only reply that carries no key is a reply this cannot use,
			// and skipping it silently would under-report the batch.
			return KeyPage{}, KeyError("a keys-only result carried no key")
		}
		out.Keys = append(out.Keys, *entity.Key)
	}
	return out, nil
}

// Count runs an aggregation query and returns how many entities match.
//
// It is not generic: a count decodes no entity, so there is no type to infer. It
// exists because counting by paging through keys costs a read per entity, and a
// wrapper that omitted it would push callers toward the expensive thing.
func Count(ctx context.Context, q *datastore.Query, opts ...datastore.ReadOption) (int64, error) {
	c, _, err := clientFor(ctx)
	if err != nil {
		return 0, err
	}
	return c.Count(ctx, q, opts...)
}

func decodeBatch[T any, PT interface {
	*T
	EntityDecoder
}](batch *datastore.Batch) (Page[T], error) {
	if batch == nil {
		return Page[T]{}, nil
	}
	out := Page[T]{
		Values:         make([]T, 0, len(batch.Entities)),
		EndCursor:      batch.EndCursor,
		More:           batch.More,
		SkippedResults: batch.SkippedResults,
	}
	for _, entity := range batch.Entities {
		var decoded T
		if err := PT(&decoded).DecodeEntity(entity); err != nil {
			return Page[T]{}, err
		}
		out.Values = append(out.Values, decoded)
	}
	return out, nil
}

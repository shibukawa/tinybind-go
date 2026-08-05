package dynamobind

import (
	"context"
	"iter"

	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// Page is one page of decoded items.
//
// LastEvaluatedKey is the continuation and is the authority on whether more
// pages follow: a non-nil key means more, whatever Count says, because a filter
// can empty a page that still has a successor.
type Page[T any] struct {
	Items            []T
	LastEvaluatedKey dynamodb.Key
	Count            int
	ScannedCount     int
}

// HasMore reports whether another page follows this one.
func (p Page[T]) HasMore() bool { return len(p.LastEvaluatedKey) > 0 }

// QueryPage runs one Query and decodes its page.
//
// This is the form that keeps the request count visible: one call is one
// request. Query iterates instead, at the cost of hiding how many requests that
// takes.
func QueryPage[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, table, keyCond string, opts ...dynamodb.QueryOption) (Page[T], error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return Page[T]{}, err
	}
	return QueryPageOn[T, PT](ctx, h, table, keyCond, opts...)
}

// QueryPageOn is QueryPage taking its Handle as an argument.
func QueryPageOn[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, h Handle, table, keyCond string, opts ...dynamodb.QueryOption) (Page[T], error) {
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return Page[T]{}, err
	}
	page, err := c.Query(ctx, name, keyCond, opts...)
	if err != nil {
		return Page[T]{}, err
	}
	return decodePage[T, PT](page)
}

// ScanPage runs one Scan and decodes its page.
func ScanPage[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, table string, opts ...dynamodb.ScanOption) (Page[T], error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return Page[T]{}, err
	}
	return ScanPageOn[T, PT](ctx, h, table, opts...)
}

// ScanPageOn is ScanPage taking its Handle as an argument.
func ScanPageOn[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, h Handle, table string, opts ...dynamodb.ScanOption) (Page[T], error) {
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return Page[T]{}, err
	}
	page, err := c.Scan(ctx, name, opts...)
	if err != nil {
		return Page[T]{}, err
	}
	return decodePage[T, PT](page)
}

// Query iterates every item of a query, requesting pages as the range advances.
//
// One range can issue many requests. The iterator reports no page boundary, no
// Count, no ScannedCount, and no final LastEvaluatedKey, so a query that scans
// far more than it returns looks the same as one that does not, and an
// interrupted run cannot be resumed. Use QueryPage when any of that matters.
//
// Iteration stops at the first error, which is yielded once with the zero value
// of T. A break stops it without issuing a further request.
func Query[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, table, keyCond string, opts ...dynamodb.QueryOption) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		h, err := HandleFromContext(ctx)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		QueryOn[T, PT](ctx, h, table, keyCond, opts...)(yield)
	}
}

// QueryOn is Query taking its Handle as an argument. The Handle is resolved once
// for the whole range rather than once per page.
func QueryOn[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, h Handle, table, keyCond string, opts ...dynamodb.QueryOption) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var start dynamodb.Key
		first := true
		for first || len(start) > 0 {
			first = false
			pageOpts := opts
			if len(start) > 0 {
				pageOpts = append(append([]dynamodb.QueryOption(nil), opts...), dynamodb.WithExclusiveStartKey(start))
			}
			page, err := QueryPageOn[T, PT](ctx, h, table, keyCond, pageOpts...)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			start = page.LastEvaluatedKey
		}
	}
}

// Scan iterates every item of a table or index scan.
//
// An unfiltered scan walks the whole table, one page per request. Everything
// said about Query's hidden request count applies here and costs more.
func Scan[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, table string, opts ...dynamodb.ScanOption) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		h, err := HandleFromContext(ctx)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		ScanOn[T, PT](ctx, h, table, opts...)(yield)
	}
}

// ScanOn is Scan taking its Handle as an argument. The Handle is resolved once
// for the whole range rather than once per page.
func ScanOn[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, h Handle, table string, opts ...dynamodb.ScanOption) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var start dynamodb.Key
		first := true
		for first || len(start) > 0 {
			first = false
			pageOpts := opts
			if len(start) > 0 {
				pageOpts = append(append([]dynamodb.ScanOption(nil), opts...), dynamodb.WithExclusiveStartKey(start))
			}
			page, err := ScanPageOn[T, PT](ctx, h, table, pageOpts...)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			start = page.LastEvaluatedKey
		}
	}
}

func decodePage[T any, PT interface {
	*T
	ItemDecoder
}](page *dynamodb.Page) (Page[T], error) {
	if page == nil {
		return Page[T]{}, nil
	}
	out := Page[T]{
		Items:            make([]T, 0, len(page.Items)),
		LastEvaluatedKey: page.LastEvaluatedKey,
		Count:            page.Count,
		ScannedCount:     page.ScannedCount,
	}
	for _, item := range page.Items {
		var decoded T
		if err := PT(&decoded).DecodeItem(item); err != nil {
			return Page[T]{}, err
		}
		out.Items = append(out.Items, decoded)
	}
	return out, nil
}

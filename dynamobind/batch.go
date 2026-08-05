package dynamobind

import (
	"bytes"
	"context"

	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// DynamoDB's per-request batch limits. They are fixed by the service, so the
// chunking that respects them is mechanical and belongs here rather than in
// generated code. The size limits, 16 MB per request and 400 KB per item, are
// properties of the data rather than of the type, so nothing at generation time
// could bound them; an oversized request surfaces as dynamodb.ErrRequestTooLarge.
const (
	MaxBatchWrite = 25
	MaxBatchGet   = 100
)

// StoreAll writes every value, splitting the input into requests of at most
// MaxBatchWrite items.
//
// What DynamoDB declined comes back in unprocessed, unretried: the driver
// already retries the transport, and whether a partial success is worth a
// second attempt is the caller's decision, not this package's. Passing
// unprocessed back to StoreAll is the retry, and it belongs in caller code
// where a backoff can live.
func StoreAll[T ItemEncoder](ctx context.Context, table string, vs []T) ([]T, error) {
	if len(vs) == 0 {
		return nil, nil
	}
	h, err := HandleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return StoreAllOn(ctx, h, table, vs)
}

// StoreAllOn is StoreAll taking its Handle as an argument.
func StoreAllOn[T ItemEncoder](ctx context.Context, h Handle, table string, vs []T) ([]T, error) {
	var unprocessed []T
	if len(vs) == 0 {
		return nil, nil
	}
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return nil, err
	}
	for start := 0; start < len(vs); start += MaxBatchWrite {
		end := min(start+MaxBatchWrite, len(vs))
		chunk := vs[start:end]

		requests := make([]dynamodb.WriteRequest, 0, len(chunk))
		items := make([]dynamodb.Item, 0, len(chunk))
		for _, v := range chunk {
			item := v.EncodeItem()
			items = append(items, item)
			requests = append(requests, dynamodb.PutRequest(item))
		}
		result, err := c.BatchWriteItem(ctx, map[string][]dynamodb.WriteRequest{name: requests})
		if err != nil {
			return unprocessed, err
		}
		if result == nil {
			continue
		}
		// Map each declined write back to the value that produced it. The
		// service returns the item it did not write, not an index, so the
		// encoded items are matched by value; a matched entry is consumed so
		// duplicate inputs report the right count.
		matched := make([]bool, len(chunk))
		for _, request := range result.UnprocessedItems[name] {
			if request.Put == nil {
				continue
			}
			for i := range chunk {
				if matched[i] || !equalItem(items[i], request.Put) {
					continue
				}
				matched[i] = true
				unprocessed = append(unprocessed, chunk[i])
				break
			}
		}
	}
	return unprocessed, nil
}

// LoadAll reads every key, splitting the input into requests of at most
// MaxBatchGet keys.
//
// DynamoDB does not promise to return batch items in request order, and a key
// that matches nothing is simply absent rather than an error, so len(items) can
// be smaller than len(keys) with no error and no unprocessed key. Keys DynamoDB
// declined to read come back in unprocessed, unretried.
func LoadAll[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, table string, keys []dynamodb.Key, opts ...dynamodb.BatchOption) ([]T, []dynamodb.Key, error) {
	if len(keys) == 0 {
		return nil, nil, nil
	}
	h, err := HandleFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	return LoadAllOn[T, PT](ctx, h, table, keys, opts...)
}

// LoadAllOn is LoadAll taking its Handle as an argument.
func LoadAllOn[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, h Handle, table string, keys []dynamodb.Key, opts ...dynamodb.BatchOption) ([]T, []dynamodb.Key, error) {
	var (
		items       []T
		unprocessed []dynamodb.Key
	)
	if len(keys) == 0 {
		return nil, nil, nil
	}
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return nil, nil, err
	}
	for start := 0; start < len(keys); start += MaxBatchGet {
		end := min(start+MaxBatchGet, len(keys))
		result, err := c.BatchGetItem(ctx, map[string][]dynamodb.Key{name: keys[start:end]}, opts...)
		if err != nil {
			return items, unprocessed, err
		}
		if result == nil {
			continue
		}
		for _, item := range result.Items[name] {
			var decoded T
			if err := PT(&decoded).DecodeItem(item); err != nil {
				return items, unprocessed, err
			}
			items = append(items, decoded)
		}
		unprocessed = append(unprocessed, result.UnprocessedKeys[name]...)
	}
	return items, unprocessed, nil
}

// equalItem compares two items attribute by attribute. It exists because
// AttributeValue holds pointers and slices, so == does not describe it, and
// reflect.DeepEqual is not available to a package that must not link reflect.
func equalItem(a, b dynamodb.Item) bool {
	if len(a) != len(b) {
		return false
	}
	for name, left := range a {
		right, ok := b[name]
		if !ok || !equalAttribute(left, right) {
			return false
		}
	}
	return true
}

func equalAttribute(a, b dynamodb.AttributeValue) bool {
	if a.Kind() != b.Kind() {
		return false
	}
	switch a.Kind() {
	case dynamodb.KindString:
		return *a.S == *b.S
	case dynamodb.KindNumber:
		return *a.N == *b.N
	case dynamodb.KindBinary:
		return bytes.Equal(a.B, b.B)
	case dynamodb.KindBool:
		return *a.BOOL == *b.BOOL
	case dynamodb.KindNull:
		return true
	case dynamodb.KindList:
		if len(a.L) != len(b.L) {
			return false
		}
		for i := range a.L {
			if !equalAttribute(a.L[i], b.L[i]) {
				return false
			}
		}
		return true
	case dynamodb.KindMap:
		return equalItem(a.M, b.M)
	case dynamodb.KindStringSet:
		return equalStrings(a.SS, b.SS)
	case dynamodb.KindNumberSet:
		return equalStrings(a.NS, b.NS)
	case dynamodb.KindBinarySet:
		if len(a.BS) != len(b.BS) {
			return false
		}
		for i := range a.BS {
			if !bytes.Equal(a.BS[i], b.BS[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

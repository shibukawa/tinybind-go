package dynamobind

import (
	"context"

	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// Every entry here comes in two forms. The plain one resolves its Handle from
// the Context and is the default. The one suffixed On takes the Handle as an
// argument, for a caller that already holds one and wants no lookup on the
// operation path.
//
// The On form holds the implementation and the Context form delegates to it, so
// the two cannot drift and a program calling neither WithClient nor a Context
// form links none of the Context machinery.

// Load reads one item by key and decodes it into T.
//
// A key that matches nothing keeps the driver's dynamodb.ErrItemNotFound rather
// than returning a zero value, so a miss cannot be mistaken for an empty item.
func Load[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, table string, key dynamodb.Key, opts ...dynamodb.GetOption) (T, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		var out T
		return out, err
	}
	return LoadOn[T, PT](ctx, h, table, key, opts...)
}

// LoadOn is Load taking its Handle as an argument.
func LoadOn[T any, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, h Handle, table string, key dynamodb.Key, opts ...dynamodb.GetOption) (T, error) {
	var out T
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return out, err
	}
	item, err := c.GetItem(ctx, name, key, opts...)
	if err != nil {
		return out, err
	}
	if err := PT(&out).DecodeItem(item); err != nil {
		return out, err
	}
	return out, nil
}

// Store writes v as a whole item, replacing any item with the same key.
//
// It is PutItem, not a partial update: every attribute of the stored item comes
// from v. Use Update for a partial change.
func Store[T ItemEncoder](ctx context.Context, table string, v T, opts ...dynamodb.WriteOption) error {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return StoreOn(ctx, h, table, v, opts...)
}

// StoreOn is Store taking its Handle as an argument.
func StoreOn[T ItemEncoder](ctx context.Context, h Handle, table string, v T, opts ...dynamodb.WriteOption) error {
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return err
	}
	_, err = c.PutItem(ctx, name, v.EncodeItem(), opts...)
	return err
}

// StoreReturning is Store, and also decodes the item it replaced.
//
// The bool is false when the key held no item, which is not an error. It asks
// the driver for ALL_OLD, so it costs a write capacity unit more than Store on
// a table that charges for it.
func StoreReturning[T ItemEncoder, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, table string, v T, opts ...dynamodb.WriteOption) (T, bool, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		var old T
		return old, false, err
	}
	return StoreReturningOn[T, PT](ctx, h, table, v, opts...)
}

// StoreReturningOn is StoreReturning taking its Handle as an argument.
func StoreReturningOn[T ItemEncoder, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, h Handle, table string, v T, opts ...dynamodb.WriteOption) (T, bool, error) {
	var old T
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return old, false, err
	}
	result, err := c.PutItem(ctx, name, v.EncodeItem(), withAllOld(opts)...)
	if err != nil {
		return old, false, err
	}
	if result == nil || len(result.Attributes) == 0 {
		return old, false, nil
	}
	if err := PT(&old).DecodeItem(result.Attributes); err != nil {
		return old, false, err
	}
	return old, true, nil
}

// Remove deletes the item identified by v's key. Only the key of v is read.
func Remove[T Keyer](ctx context.Context, table string, v T, opts ...dynamodb.WriteOption) error {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return RemoveOn(ctx, h, table, v, opts...)
}

// RemoveOn is Remove taking its Handle as an argument.
func RemoveOn[T Keyer](ctx context.Context, h Handle, table string, v T, opts ...dynamodb.WriteOption) error {
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return err
	}
	_, err = c.DeleteItem(ctx, name, v.ItemKey(), opts...)
	return err
}

// RemoveReturning is Remove, and also decodes the item it deleted.
//
// The bool is false when the key held no item, which is not an error.
func RemoveReturning[T Keyer, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, table string, v T, opts ...dynamodb.WriteOption) (T, bool, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		var old T
		return old, false, err
	}
	return RemoveReturningOn[T, PT](ctx, h, table, v, opts...)
}

// RemoveReturningOn is RemoveReturning taking its Handle as an argument.
func RemoveReturningOn[T Keyer, PT interface {
	*T
	ItemDecoder
}](ctx context.Context, h Handle, table string, v T, opts ...dynamodb.WriteOption) (T, bool, error) {
	var old T
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return old, false, err
	}
	result, err := c.DeleteItem(ctx, name, v.ItemKey(), withAllOld(opts)...)
	if err != nil {
		return old, false, err
	}
	if result == nil || len(result.Attributes) == 0 {
		return old, false, nil
	}
	if err := PT(&old).DecodeItem(result.Attributes); err != nil {
		return old, false, err
	}
	return old, true, nil
}

// Update applies a DynamoDB update expression to the item identified by v's key.
//
// The expression is passed to the driver verbatim; nothing here generates or
// validates it. Only the key is typed, which is the part a struct tag can
// actually supply. Attribute values in the expression come from
// dynamodb.WithExpressionValues as usual:
//
//	err := dynamobind.Update(ctx, "readings", key, "SET celsius = :c",
//		dynamodb.WithExpressionValues(map[string]dynamodb.AttributeValue{":c": dynamodb.N(21.5)}))
func Update[T Keyer](ctx context.Context, table string, v T, update string, opts ...dynamodb.WriteOption) error {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return UpdateOn(ctx, h, table, v, update, opts...)
}

// UpdateOn is Update taking its Handle as an argument.
func UpdateOn[T Keyer](ctx context.Context, h Handle, table string, v T, update string, opts ...dynamodb.WriteOption) error {
	c, name, err := h.Table(ctx, table)
	if err != nil {
		return err
	}
	_, err = c.UpdateItem(ctx, name, v.ItemKey(), update, opts...)
	return err
}

// withAllOld appends the ALL_OLD request. It is appended rather than prepended
// so a caller who set ReturnValues explicitly still loses to it: this helper
// cannot decode anything else.
func withAllOld(opts []dynamodb.WriteOption) []dynamodb.WriteOption {
	out := make([]dynamodb.WriteOption, 0, len(opts)+1)
	out = append(out, opts...)
	return append(out, dynamodb.WithReturnValues("ALL_OLD"))
}

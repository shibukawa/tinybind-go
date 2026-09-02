package firestorebind

import (
	"context"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// Every entry here comes in two forms. The plain one resolves its Handle from
// the Context and is the default. The other is a method on Handle, for a caller
// that already holds one and wants no lookup on the operation path.
//
// The method holds the implementation and the Context form delegates to it, so
// the two cannot drift and a program calling neither WithClient nor a Context
// form links none of the Context machinery. The functions suffixed On are the
// method's earlier spelling, from before a Go method could declare its own type
// parameter; each forwards to the method and is deprecated.
//
// The transactional entries are methods on *Tx, which already carries the
// tenancy, so they have no Context form and need none.

// Load reads one entity by key and decodes it into T.
//
// A key that matches nothing keeps the driver's datastore.ErrNoSuchEntity rather
// than returning a zero value, so a miss cannot be mistaken for an empty entity.
//
// The decoded value carries its own key: the key fields declared with the name,
// id and parent tags are filled from the entity's key, not from its properties.
func Load[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, key datastore.Key, opts ...datastore.ReadOption) (T, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		var out T
		return out, err
	}
	return h.Load[T, PT](ctx, key, opts...)
}

// Load is Load on a Handle the caller already holds.
func (h Handle) Load[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, key datastore.Key, opts ...datastore.ReadOption) (T, error) {
	var out T
	c, ns, err := h.resolve()
	if err != nil {
		return out, err
	}
	entity, err := c.Get(ctx, applyNamespace(ctx, ns, key), opts...)
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

// LoadOn is Load taking its Handle as an argument.
//
// Deprecated: use the Load method on Handle, which carries the body. This
// function remains so no caller is forced to move.
func LoadOn[T any, PT interface {
	*T
	EntityDecoder
}](ctx context.Context, h Handle, key datastore.Key, opts ...datastore.ReadOption) (T, error) {
	return h.Load[T, PT](ctx, key, opts...)
}

// Store writes v as a whole entity, replacing any entity with the same key.
//
// It is an upsert. The returned key is the stored one, which differs from v's
// own only when v carried an incomplete key and the server allocated an id; a
// caller storing a new entity assigns the result back rather than expecting v to
// have been mutated, since v was passed by value.
func Store[T EntityEncoder](ctx context.Context, v T, opts ...datastore.WriteOption) (datastore.Key, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return datastore.Key{}, err
	}
	return h.Store(ctx, v, opts...)
}

// Store is Store on a Handle the caller already holds.
func (h Handle) Store[T EntityEncoder](ctx context.Context, v T, opts ...datastore.WriteOption) (datastore.Key, error) {
	return writeOne(ctx, h, v, withBaseVersion(v, opts), (*datastore.Client).Put)
}

// StoreOn is Store taking its Handle as an argument.
//
// Deprecated: use the Store method on Handle, which carries the body. This
// function remains so no caller is forced to move.
func StoreOn[T EntityEncoder](ctx context.Context, h Handle, v T, opts ...datastore.WriteOption) (datastore.Key, error) {
	return h.Store(ctx, v, opts...)
}

// Insert writes v and fails if its key already exists.
//
// This is put-if-absent, and it is a precondition the wire evaluates rather than
// a condition this package composes: the driver sends an insert mutation, and a
// collision is datastore.ErrAlreadyExists.
func Insert[T EntityEncoder](ctx context.Context, v T, opts ...datastore.WriteOption) (datastore.Key, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return datastore.Key{}, err
	}
	return h.Insert(ctx, v, opts...)
}

// Insert is Insert on a Handle the caller already holds.
func (h Handle) Insert[T EntityEncoder](ctx context.Context, v T, opts ...datastore.WriteOption) (datastore.Key, error) {
	return writeOne(ctx, h, v, opts, (*datastore.Client).Insert)
}

// InsertOn is Insert taking its Handle as an argument.
//
// Deprecated: use the Insert method on Handle, which carries the body. This
// function remains so no caller is forced to move.
func InsertOn[T EntityEncoder](ctx context.Context, h Handle, v T, opts ...datastore.WriteOption) (datastore.Key, error) {
	return h.Insert(ctx, v, opts...)
}

// Update writes v and fails if its key does not exist.
//
// This is put-if-present. It replaces the whole entity: Datastore has no partial
// update, so every property of the stored entity comes from v.
func Update[T EntityEncoder](ctx context.Context, v T, opts ...datastore.WriteOption) error {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return h.Update(ctx, v, opts...)
}

// Update is Update on a Handle the caller already holds.
func (h Handle) Update[T EntityEncoder](ctx context.Context, v T, opts ...datastore.WriteOption) error {
	c, ns, err := h.resolve()
	if err != nil {
		return err
	}
	return c.Update(ctx, withNamespace(ctx, ns, v.EncodeEntity()), withBaseVersion(v, opts)...)
}

// UpdateOn is Update taking its Handle as an argument.
//
// Deprecated: use the Update method on Handle, which carries the body. This
// function remains so no caller is forced to move.
func UpdateOn[T EntityEncoder](ctx context.Context, h Handle, v T, opts ...datastore.WriteOption) error {
	return h.Update(ctx, v, opts...)
}

// Remove deletes the entity identified by v's key. Only the key of v is read.
func Remove[T Keyer](ctx context.Context, v T, opts ...datastore.WriteOption) error {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return err
	}
	return h.Remove(ctx, v, opts...)
}

// Remove is Remove on a Handle the caller already holds.
func (h Handle) Remove[T Keyer](ctx context.Context, v T, opts ...datastore.WriteOption) error {
	c, ns, err := h.resolve()
	if err != nil {
		return err
	}
	key := applyNamespace(ctx, ns, v.EntityKey())
	if key.Incomplete() {
		return KeyError("cannot remove an entity whose key is incomplete")
	}
	return c.Delete(ctx, key, opts...)
}

// RemoveOn is Remove taking its Handle as an argument.
//
// Deprecated: use the Remove method on Handle, which carries the body. This
// function remains so no caller is forced to move.
func RemoveOn[T Keyer](ctx context.Context, h Handle, v T, opts ...datastore.WriteOption) error {
	return h.Remove(ctx, v, opts...)
}

// writeOne is the shared body of StoreOn and InsertOn, which differ only in the
// driver method they send.
func writeOne[T EntityEncoder](
	ctx context.Context,
	h Handle,
	v T,
	opts []datastore.WriteOption,
	send func(*datastore.Client, context.Context, datastore.Entity, ...datastore.WriteOption) (datastore.Key, error),
) (datastore.Key, error) {
	c, ns, err := h.resolve()
	if err != nil {
		return datastore.Key{}, err
	}
	return send(c, ctx, withNamespace(ctx, ns, v.EncodeEntity()), opts...)
}

// withBaseVersion adds the optimistic-concurrency precondition when the value
// carries a version it was read at.
//
// It is appended rather than prepended, so a caller who supplies its own
// WithBaseVersion loses to the tag. The two cannot both be right, and the tag is
// the one the decoder filled.
//
// Insert takes none: it already fails if the key exists, and a precondition on
// an entity that must not exist yet says nothing.
func withBaseVersion(v any, opts []datastore.WriteOption) []datastore.WriteOption {
	versioner, ok := v.(Versioner)
	if !ok {
		return opts
	}
	version := versioner.EntityVersion()
	if version == 0 {
		return opts
	}
	out := make([]datastore.WriteOption, 0, len(opts)+1)
	out = append(out, opts...)
	return append(out, datastore.WithBaseVersion(version))
}

// withNamespace stamps the resolved namespace onto an encoded entity's key. An
// entity with no key at all is left alone; the driver rejects it, and saying so
// is the driver's job rather than this package's.
func withNamespace(ctx context.Context, resolve NamespaceResolver, e datastore.Entity) datastore.Entity {
	if resolve == nil || e.Key == nil {
		return e
	}
	key := applyNamespace(ctx, resolve, *e.Key)
	e.Key = &key
	return e
}

//go:build !tinygo

package firestorefixture_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shibukawa/tinygodriver/nosql/datastore"

	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinybind-go/internal/firestorefixture"
)

// TestHandleAndContextAgreeOnTheWire is the property the two forms exist to
// share, checked where it matters: the same entity written through a Handle is
// readable through a Context built from the same client, and the reverse.
func TestHandleAndContextAgreeOnTheWire(t *testing.T) {
	client, _ := newFakeDatastore(t)
	ctx := firestorebind.WithClient(t.Context(), client)
	handle := firestorebind.NewHandle(client)

	want := sample()
	if _, err := firestorebind.StoreOn(t.Context(), handle, want); err != nil {
		t.Fatalf("StoreOn: %v", err)
	}

	// Written through the parameter form, read through the Context form.
	got, err := firestorefixture.LoadReading(ctx, want.EntityKey())
	if err != nil {
		t.Fatalf("LoadReading: %v", err)
	}
	if got.ID != want.ID || got.Note != want.Note {
		t.Errorf("the Handle write is not what the Context read: got %+v", got)
	}

	// And back the other way.
	viaHandle, err := firestorebind.LoadOn[firestorefixture.Reading](t.Context(), handle, want.EntityKey())
	if err != nil {
		t.Fatalf("LoadOn: %v", err)
	}
	if viaHandle.ID != got.ID || viaHandle.Note != got.Note {
		t.Errorf("the two forms read different entities: %+v and %+v", viaHandle, got)
	}
}

// TestZeroHandleIsErrNoClient keeps the errors-not-panics rule at the new
// entry, including in the iterator, which reports it once as a failed batch
// already does.
func TestZeroHandleIsErrNoClient(t *testing.T) {
	var zero firestorebind.Handle

	if _, err := firestorebind.LoadOn[firestorefixture.Reading](t.Context(), zero, datastore.NameKey("Reading", "x")); !errors.Is(err, firestorebind.ErrNoClient) {
		t.Fatalf("LoadOn error = %v", err)
	}
	if err := firestorebind.RemoveKeysOn(t.Context(), zero, []datastore.Key{datastore.NameKey("Reading", "x")}); !errors.Is(err, firestorebind.ErrNoClient) {
		t.Fatalf("RemoveKeysOn error = %v", err)
	}
	if zero.Client() != nil {
		t.Fatal("the zero Handle carries a client")
	}

	calls := 0
	for _, err := range firestorebind.QueryOn[firestorefixture.Reading](t.Context(), zero, datastore.NewQuery("Reading")) {
		calls++
		if !errors.Is(err, firestorebind.ErrNoClient) {
			t.Fatalf("iterator error = %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("the iterator yielded %d times, want 1", calls)
	}
}

// TestKeyForOnPlacesAsKeyForDoes pins that the escape hatch keeps working in
// the parameter form: a key stamped through a Handle lands where the Context
// form would have put it.
func TestKeyForOnPlacesAsKeyForDoes(t *testing.T) {
	client, _ := newFakeDatastore(t)
	tenant := firestorebind.WithNamespace(func(context.Context) string { return "acme" })

	ctx := firestorebind.WithClient(t.Context(), client, tenant)
	handle := firestorebind.NewHandle(client, tenant)

	key := datastore.NameKey("Reading", "x")
	viaContext := firestorebind.KeyFor(ctx, key)
	viaHandle := firestorebind.KeyForOn(t.Context(), handle, key)
	if viaContext.Namespace != "acme" || viaHandle.Namespace != viaContext.Namespace {
		t.Fatalf("namespaces disagree: %q and %q", viaContext.Namespace, viaHandle.Namespace)
	}

	// A key that already names a namespace is not moved, in either form.
	placed := key.WithNamespace("explicit")
	if got := firestorebind.KeyForOn(t.Context(), handle, placed); got.Namespace != "explicit" {
		t.Fatalf("an explicitly placed key moved to %q", got.Namespace)
	}

	// A zero Handle returns the key unchanged, as a Context with no client does.
	var zero firestorebind.Handle
	if got := firestorebind.KeyForOn(t.Context(), zero, key); got.Namespace != "" {
		t.Fatalf("the zero Handle stamped %q", got.Namespace)
	}
}

// TestHandleMethodsAgreeWithTheOnForms pins that a Handle method and its
// deprecated On function are one operation on the wire: an entity written
// through the method is what the function reads back, and the reverse. The
// keyless entries are checked the same way, on the namespace a key is given.
func TestHandleMethodsAgreeWithTheOnForms(t *testing.T) {
	client, _ := newFakeDatastore(t)
	handle := firestorebind.NewHandle(client)
	ctx := t.Context()

	want := sample()
	if _, err := handle.Store(ctx, want); err != nil {
		t.Fatalf("Handle.Store: %v", err)
	}
	got, err := firestorebind.LoadOn[firestorefixture.Reading](ctx, handle, want.EntityKey())
	if err != nil {
		t.Fatalf("LoadOn: %v", err)
	}
	if got.ID != want.ID || got.Note != want.Note {
		t.Errorf("the method write is not what the function read: %+v", got)
	}

	second := want
	second.Note = "through the function"
	if _, err := firestorebind.StoreOn(ctx, handle, second); err != nil {
		t.Fatalf("StoreOn: %v", err)
	}
	viaMethod, err := handle.Load[firestorefixture.Reading](ctx, second.EntityKey())
	if err != nil {
		t.Fatalf("Handle.Load: %v", err)
	}
	if viaMethod.Note != second.Note {
		t.Errorf("the function write is not what the method read: %+v", viaMethod)
	}

	tenant := firestorebind.WithNamespace(func(context.Context) string { return "acme" })
	placed := firestorebind.NewHandle(client, tenant)
	key := datastore.NameKey("Reading", "x")
	if placed.KeyFor(ctx, key).Namespace != firestorebind.KeyForOn(ctx, placed, key).Namespace {
		t.Errorf("KeyFor and KeyForOn placed the key differently")
	}
}

// TestTransactionReadsAreMethods pins the first entry of
// decision:generic-method-migration: a read inside a transaction is a method
// on the Tx beside the writes, and the deprecated LoadTx reads the same entity.
func TestTransactionReadsAreMethods(t *testing.T) {
	ctx, _ := withFake(t)
	task := firestorefixture.Task{Number: 3, Title: "before"}
	if _, err := firestorefixture.InsertTask(ctx, task); err != nil {
		t.Fatalf("insert: %v", err)
	}

	err := firestorebind.Run(ctx, func(tx *firestorebind.Tx) error {
		got, err := tx.Load[firestorefixture.Task](ctx, task.EntityKey())
		if err != nil {
			return err
		}
		if got.Title != "before" {
			t.Errorf("Tx.Load read %+v", got)
		}
		got.Title = "after"
		tx.Store(got)
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	err = firestorebind.Run(ctx, func(tx *firestorebind.Tx) error {
		viaMethod, err := tx.Load[firestorefixture.Task](ctx, task.EntityKey())
		if err != nil {
			return err
		}
		viaFunc, err := firestorebind.LoadTx[firestorefixture.Task](ctx, tx, task.EntityKey())
		if err != nil {
			return err
		}
		if viaMethod.Title != "after" || viaFunc.Title != viaMethod.Title {
			t.Errorf("the two spellings read %+v and %+v", viaMethod, viaFunc)
		}
		many, _, _, err := tx.LoadAll[firestorefixture.Task](ctx, []datastore.Key{task.EntityKey()})
		if err != nil {
			return err
		}
		if len(many) != 1 || many[0].Title != "after" {
			t.Errorf("Tx.LoadAll read %+v", many)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

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
	if _, err := handle.Store(t.Context(), want); err != nil {
		t.Fatalf("Handle.Store: %v", err)
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
	viaHandle, err := handle.Load[firestorefixture.Reading](t.Context(), want.EntityKey())
	if err != nil {
		t.Fatalf("Handle.Load: %v", err)
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

	if _, err := zero.Load[firestorefixture.Reading](t.Context(), datastore.NameKey("Reading", "x")); !errors.Is(err, firestorebind.ErrNoClient) {
		t.Fatalf("Handle.Load error = %v", err)
	}
	if err := zero.RemoveKeys(t.Context(), []datastore.Key{datastore.NameKey("Reading", "x")}); !errors.Is(err, firestorebind.ErrNoClient) {
		t.Fatalf("Handle.RemoveKeys error = %v", err)
	}
	if zero.Client() != nil {
		t.Fatal("the zero Handle carries a client")
	}

	calls := 0
	for _, err := range zero.Query[firestorefixture.Reading](t.Context(), datastore.NewQuery("Reading")) {
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
	viaHandle := handle.KeyFor(t.Context(), key)
	if viaContext.Namespace != "acme" || viaHandle.Namespace != viaContext.Namespace {
		t.Fatalf("namespaces disagree: %q and %q", viaContext.Namespace, viaHandle.Namespace)
	}

	// A key that already names a namespace is not moved, in either form.
	placed := key.WithNamespace("explicit")
	if got := handle.KeyFor(t.Context(), placed); got.Namespace != "explicit" {
		t.Fatalf("an explicitly placed key moved to %q", got.Namespace)
	}

	// A zero Handle returns the key unchanged, as a Context with no client does.
	var zero firestorebind.Handle
	if got := zero.KeyFor(t.Context(), key); got.Namespace != "" {
		t.Fatalf("the zero Handle stamped %q", got.Namespace)
	}
}

// TestHandleMethodsRoundTripOnTheWire pins the Handle methods where it
// matters: an entity written through one is what another reads back, and the
// keyless KeyFor places a key where the Context form does.
func TestHandleMethodsRoundTripOnTheWire(t *testing.T) {
	client, _ := newFakeDatastore(t)
	handle := firestorebind.NewHandle(client)
	ctx := t.Context()

	want := sample()
	if _, err := handle.Store(ctx, want); err != nil {
		t.Fatalf("Handle.Store: %v", err)
	}
	got, err := handle.Load[firestorefixture.Reading](ctx, want.EntityKey())
	if err != nil {
		t.Fatalf("Handle.Load: %v", err)
	}
	if got.ID != want.ID || got.Note != want.Note {
		t.Errorf("the write is not what the read returned: %+v", got)
	}

	second := want
	second.Note = "through the function"
	if _, err := handle.Store(ctx, second); err != nil {
		t.Fatalf("Handle.Store: %v", err)
	}
	viaMethod, err := handle.Load[firestorefixture.Reading](ctx, second.EntityKey())
	if err != nil {
		t.Fatalf("Handle.Load: %v", err)
	}
	if viaMethod.Note != second.Note {
		t.Errorf("the second write is not what the read returned: %+v", viaMethod)
	}

	tenant := firestorebind.WithNamespace(func(context.Context) string { return "acme" })
	placed := firestorebind.NewHandle(client, tenant)
	key := datastore.NameKey("Reading", "x")
	if placed.KeyFor(ctx, key).Namespace != "acme" {
		t.Errorf("KeyFor placed the key in %q", placed.KeyFor(ctx, key).Namespace)
	}
}

// TestTransactionReadsAreMethods pins the first entry of
// decision:generic-method-migration: a read inside a transaction is a method
// on the Tx beside the writes, and reads what the writes committed.
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
		if viaMethod.Title != "after" {
			t.Errorf("Tx.Load read %+v after the commit", viaMethod)
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

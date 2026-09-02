//go:build !tinygo

package dynamofixture_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinybind-go/internal/dynamofixture"
)

// TestHandleMethodsAgreeWithTheOnFormsOnTheWire pins that a Handle method and
// its deprecated On function are one operation where it matters: an item
// written through the method is what the function reads back, and the reverse.
func TestHandleMethodsAgreeWithTheOnFormsOnTheWire(t *testing.T) {
	client, _ := newFakeDynamo(t)
	handle := dynamobind.NewHandle(client)
	ctx := t.Context()

	want := sample()
	if err := handle.Store(ctx, table, want); err != nil {
		t.Fatalf("Handle.Store: %v", err)
	}
	got, err := dynamobind.LoadOn[dynamofixture.Reading](ctx, handle, table, want.ItemKey())
	if err != nil {
		t.Fatalf("LoadOn: %v", err)
	}
	if got.Sensor != want.Sensor || got.Note != want.Note {
		t.Errorf("the method write is not what the function read: %+v", got)
	}

	second := want
	second.Note = "through the function"
	if err := dynamobind.StoreOn(ctx, handle, table, second); err != nil {
		t.Fatalf("StoreOn: %v", err)
	}
	viaMethod, err := handle.Load[dynamofixture.Reading](ctx, table, second.ItemKey())
	if err != nil {
		t.Fatalf("Handle.Load: %v", err)
	}
	if viaMethod.Note != second.Note {
		t.Errorf("the function write is not what the method read: %+v", viaMethod)
	}
}

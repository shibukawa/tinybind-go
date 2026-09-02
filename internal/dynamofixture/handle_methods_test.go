//go:build !tinygo

package dynamofixture_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinybind-go/internal/dynamofixture"
)

// TestHandleMethodsRoundTripOnTheWire pins the Handle methods where it
// matters: an item written through Store is what Load reads back.
func TestHandleMethodsRoundTripOnTheWire(t *testing.T) {
	client, _ := newFakeDynamo(t)
	handle := dynamobind.NewHandle(client)
	ctx := t.Context()

	want := sample()
	if err := handle.Store(ctx, table, want); err != nil {
		t.Fatalf("Handle.Store: %v", err)
	}
	got, err := handle.Load[dynamofixture.Reading](ctx, table, want.ItemKey())
	if err != nil {
		t.Fatalf("Handle.Load: %v", err)
	}
	if got.Sensor != want.Sensor || got.Note != want.Note {
		t.Errorf("the write is not what the read returned: %+v", got)
	}

	second := want
	second.Note = "through the function"
	if err := handle.Store(ctx, table, second); err != nil {
		t.Fatalf("Handle.Store: %v", err)
	}
	viaMethod, err := handle.Load[dynamofixture.Reading](ctx, table, second.ItemKey())
	if err != nil {
		t.Fatalf("Handle.Load: %v", err)
	}
	if viaMethod.Note != second.Note {
		t.Errorf("the second write is not what the read returned: %+v", viaMethod)
	}
}

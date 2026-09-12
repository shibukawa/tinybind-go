//go:build !tinygo

package dynamobind_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

func testClient(t *testing.T) *dynamodb.Client {
	t.Helper()
	client, err := newTestClient()
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestClientFromContext(t *testing.T) {
	if _, err := dynamobind.ClientFromContext(context.Background()); !errors.Is(err, dynamobind.ErrNoClient) {
		t.Fatalf("missing client error = %v", err)
	}

	client := testClient(t)
	ctx := dynamobind.WithClient(context.Background(), client)
	got, err := dynamobind.ClientFromContext(ctx)
	if err != nil {
		t.Fatalf("ClientFromContext: %v", err)
	}
	if got != client {
		t.Fatal("ClientFromContext returned another client")
	}

	// An item operation resolves the client alone, so a Context carrying no
	// prefix still serves one.
	second := testClient(t)
	got, err = dynamobind.ClientFromContext(dynamobind.WithClient(ctx, second))
	if err != nil {
		t.Fatalf("ClientFromContext: %v", err)
	}
	if got != second {
		t.Fatal("the inner client did not win")
	}
}

// TestTableFromContextDefaultsToTheDeclaredName pins the default: without a
// resolver the name is sent as written, which is what a deployment whose tables
// carry the declared names wants.
func TestTableFromContextDefaultsToTheDeclaredName(t *testing.T) {
	client := testClient(t)

	got, table, err := dynamobind.TableFromContext(dynamobind.WithClient(context.Background(), client), "readings")
	if err != nil {
		t.Fatalf("TableFromContext: %v", err)
	}
	if got != client || table != "readings" {
		t.Fatalf("table = %q", table)
	}

	// A nil resolver is the same as none, rather than a panic.
	ctx := dynamobind.WithClient(context.Background(), client, dynamobind.WithTableNames(nil))
	if _, table, err = dynamobind.TableFromContext(ctx, "readings"); err != nil || table != "readings" {
		t.Fatalf("table = %q, err = %v", table, err)
	}
}

// TestTableFromContextMapsNames covers the resolver contract: it receives both
// the request Context and the declared table name, and its result is used.
func TestTableFromContextMapsNames(t *testing.T) {
	client := testClient(t)
	type tenantKey struct{}
	resolve := func(ctx context.Context, declared string) string {
		tenant, _ := ctx.Value(tenantKey{}).(string)
		return tenant + "-" + declared
	}
	ctx := context.WithValue(context.Background(), tenantKey{}, "acme")
	ctx = dynamobind.WithClient(ctx, client, dynamobind.WithTableNames(resolve))
	gotClient, table, err := dynamobind.TableFromContext(ctx, "readings")
	if err != nil {
		t.Fatal(err)
	}
	if gotClient != client {
		t.Fatal("TableFromContext returned another client")
	}
	if table != "acme-readings" {
		t.Fatalf("table = %q", table)
	}
}

func TestTableFromContextWithoutAClient(t *testing.T) {
	_, _, err := dynamobind.TableFromContext(context.Background(), "readings")
	if !errors.Is(err, dynamobind.ErrNoClient) {
		t.Fatalf("missing client error = %v", err)
	}
	// A value under some other key resolves nothing: the client is what a
	// request needs.
	ctx := context.WithValue(context.Background(), struct{}{}, "staging-")
	if _, _, err := dynamobind.TableFromContext(ctx, "readings"); !errors.Is(err, dynamobind.ErrNoClient) {
		t.Fatalf("missing client error = %v", err)
	}
}

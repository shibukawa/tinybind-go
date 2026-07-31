//go:build !tinygo

package dynamobind_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

func testClient(t *testing.T) *dynamodb.Client {
	t.Helper()
	client, err := dynamodb.New(
		dynamodb.WithEndpoint("http://127.0.0.1:1"),
		dynamodb.WithRegion("ap-northeast-1"),
		dynamodb.WithCredentials(aws.Credentials{AccessKeyID: "id", SecretAccessKey: "secret"}),
	)
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

// TestTableFromContextMapsNames covers what the resolver exists for: the
// mapping is a function, so a prefix, a suffix and an unrelated name cost the
// same, and it sees the Context so it can vary per request.
func TestTableFromContextMapsNames(t *testing.T) {
	client := testClient(t)

	tests := []struct {
		name    string
		resolve dynamobind.TableResolver
		want    string
	}{
		{
			name:    "prefix",
			resolve: func(context.Context, string) string { return "staging-readings" },
			want:    "staging-readings",
		},
		{
			name:    "suffix, which a prefix could not express",
			resolve: func(_ context.Context, declared string) string { return declared + "-prod" },
			want:    "readings-prod",
		},
		{
			name:    "a name the IaC chose, sharing nothing with the declared one",
			resolve: func(context.Context, string) string { return "Readings-A1B2C3D4" },
			want:    "Readings-A1B2C3D4",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := dynamobind.WithClient(context.Background(), client, dynamobind.WithTableNames(test.resolve))
			_, table, err := dynamobind.TableFromContext(ctx, "readings")
			if err != nil {
				t.Fatal(err)
			}
			if table != test.want {
				t.Fatalf("table = %q, want %q", table, test.want)
			}
		})
	}

	// The Context reaches the resolver, which is why it is a parameter: a
	// per-tenant table is the same one function.
	type tenantKey struct{}
	byTenant := func(ctx context.Context, declared string) string {
		tenant, _ := ctx.Value(tenantKey{}).(string)
		return tenant + "-" + declared
	}
	ctx := dynamobind.WithClient(context.Background(), client, dynamobind.WithTableNames(byTenant))
	_, table, err := dynamobind.TableFromContext(context.WithValue(ctx, tenantKey{}, "acme"), "readings")
	if err != nil {
		t.Fatal(err)
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

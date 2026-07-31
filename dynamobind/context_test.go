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

// TestTableFromContextRequiresAPrefix pins the rule that separates this from
// the SQL executor: an unset prefix is an error, because reading the unprefixed
// table would answer with a normal empty page that no caller can tell apart
// from a table holding nothing.
func TestTableFromContextRequiresAPrefix(t *testing.T) {
	client := testClient(t)

	_, _, err := dynamobind.TableFromContext(dynamobind.WithClient(context.Background(), client), "readings")
	if !errors.Is(err, dynamobind.ErrNoTablePrefix) {
		t.Fatalf("unset prefix error = %v", err)
	}

	// Declaring the empty prefix is not the same as leaving it unset.
	ctx := dynamobind.WithClient(context.Background(), client, dynamobind.WithTablePrefix(""))
	got, table, err := dynamobind.TableFromContext(ctx, "readings")
	if err != nil {
		t.Fatalf("TableFromContext: %v", err)
	}
	if got != client || table != "readings" {
		t.Fatalf("table = %q", table)
	}

	ctx = dynamobind.WithClient(context.Background(), client, dynamobind.WithTablePrefix("staging-"))
	if _, table, err = dynamobind.TableFromContext(ctx, "readings"); err != nil || table != "staging-readings" {
		t.Fatalf("table = %q, err = %v", table, err)
	}
}

func TestTableFromContextWithoutAClient(t *testing.T) {
	_, _, err := dynamobind.TableFromContext(context.Background(), "readings")
	if !errors.Is(err, dynamobind.ErrNoClient) {
		t.Fatalf("missing client error = %v", err)
	}
	// A prefix on its own resolves nothing: the client is what a request needs.
	ctx := context.WithValue(context.Background(), struct{}{}, "staging-")
	if _, _, err := dynamobind.TableFromContext(ctx, "readings"); !errors.Is(err, dynamobind.ErrNoClient) {
		t.Fatalf("missing client error = %v", err)
	}
}

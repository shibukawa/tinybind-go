//go:build !tinygo

package dynamobind_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// decodable is the minimal item type the generic entries need. Nothing here
// decodes anything: every case reaches its error before a reply exists.
type decodable struct{}

func (*decodable) DecodeItem(dynamodb.Item) error { return nil }

// newTestClient builds the client both the tests and the benchmark use. It
// points at a port nothing listens on: every case here resolves a client
// without sending anything.
func newTestClient() (*dynamodb.Client, error) {
	return dynamodb.New(
		dynamodb.WithEndpoint("http://127.0.0.1:1"),
		dynamodb.WithRegion("ap-northeast-1"),
		dynamodb.WithCredentials(aws.Credentials{AccessKeyID: "id", SecretAccessKey: "secret"}),
	)
}

// TestHandleAndContextAgree is the property the two forms exist to share: the
// same client and the same options behave identically whether they were reached
// through a Context or passed as an argument.
func TestHandleAndContextAgree(t *testing.T) {
	client := testClient(t)
	names := dynamobind.WithTableNames(func(_ context.Context, declared string) string {
		return "staging-" + declared
	})

	ctx := dynamobind.WithClient(context.Background(), client, names)
	handle := dynamobind.NewHandle(client, names)

	fromContext, viaContext, err := dynamobind.TableFromContext(ctx, "readings")
	if err != nil {
		t.Fatalf("TableFromContext: %v", err)
	}
	fromHandle, viaHandle, err := handle.Table(context.Background(), "readings")
	if err != nil {
		t.Fatalf("Handle.Table: %v", err)
	}
	if fromContext != fromHandle {
		t.Fatal("the two forms resolved different clients")
	}
	if viaContext != viaHandle || viaHandle != "staging-readings" {
		t.Fatalf("table names disagree: %q and %q", viaContext, viaHandle)
	}
}

// TestZeroHandleIsErrNoClient keeps the errors-not-panics rule at the new
// entry: a Handle that was never built reports what a Context carrying no
// client reports.
func TestZeroHandleIsErrNoClient(t *testing.T) {
	var zero dynamobind.Handle
	if _, _, err := zero.Table(context.Background(), "readings"); !errors.Is(err, dynamobind.ErrNoClient) {
		t.Fatalf("zero Handle error = %v", err)
	}
	if zero.Client() != nil {
		t.Fatal("the zero Handle carries a client")
	}

	// The iterators report it once rather than panicking, which is what a
	// failed page already does.
	calls := 0
	for _, err := range dynamobind.ScanOn[decodable, *decodable](context.Background(), zero, "readings") {
		calls++
		if !errors.Is(err, dynamobind.ErrNoClient) {
			t.Fatalf("iterator error = %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("the iterator yielded %d times, want 1", calls)
	}
}

// TestHandleFromContextRoundTrips covers the lookup a framework does once per
// request: it hands back the Handle it stored, so every later call can take the
// parameter form and look nothing up.
func TestHandleFromContextRoundTrips(t *testing.T) {
	if _, err := dynamobind.HandleFromContext(context.Background()); !errors.Is(err, dynamobind.ErrNoClient) {
		t.Fatalf("missing handle error = %v", err)
	}

	client := testClient(t)
	handle := dynamobind.NewHandle(client, dynamobind.WithTableNames(
		func(_ context.Context, declared string) string { return declared + "-prod" },
	))
	got, err := dynamobind.HandleFromContext(dynamobind.WithHandle(context.Background(), handle))
	if err != nil {
		t.Fatalf("HandleFromContext: %v", err)
	}
	if got.Client() != client {
		t.Fatal("HandleFromContext returned another client")
	}
	_, table, err := got.Table(context.Background(), "readings")
	if err != nil || table != "readings-prod" {
		t.Fatalf("table = %q, err = %v", table, err)
	}
}

// TestHandleClientIsTheEscapeHatch pins that Handle.Client applies no naming,
// matching ClientFromContext, so the two escape hatches cannot disagree.
func TestHandleClientIsTheEscapeHatch(t *testing.T) {
	client := testClient(t)
	handle := dynamobind.NewHandle(client, dynamobind.WithTableNames(
		func(context.Context, string) string { return "never-used" },
	))
	if handle.Client() != client {
		t.Fatal("Handle.Client returned another client")
	}
}

// BenchmarkContextDepth measures what the framework bundle is for: a lookup
// walks the Context chain, so the cost of resolving a client grows with how many
// values were installed after it.
//
// The bundle form is the one-node case: a framework carrying every value it
// manages in one struct pays the depth-1 number no matter how many values that
// struct holds. The parameter form pays none of it.
func BenchmarkContextDepth(b *testing.B) {
	client, err := newTestClient()
	if err != nil {
		b.Fatalf("client: %v", err)
	}
	b.Cleanup(func() { _ = client.Close() })

	type filler struct{ n int }
	for _, depth := range []int{1, 5, 20} {
		ctx := dynamobind.WithClient(context.Background(), client)
		for i := 0; i < depth-1; i++ {
			ctx = context.WithValue(ctx, filler{i}, i)
		}

		b.Run("context/depth-"+strconv.Itoa(depth), func(b *testing.B) {
			for b.Loop() {
				if _, _, err := dynamobind.TableFromContext(ctx, "readings"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	// The parameter form for comparison: no walk and no assertion, whatever the
	// Context depth is.
	handle := dynamobind.NewHandle(client)
	b.Run("handle", func(b *testing.B) {
		ctx := context.Background()
		for b.Loop() {
			if _, _, err := handle.Table(ctx, "readings"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

//go:build !tinygo

// The tests here drive the driver over an httptest server. TinyGo has no
// net/http server, so the whole file is a host test; the codec tests next to it
// are not, and run on both.

package dynamofixture_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinybind-go/internal/dynamofixture"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

func TestStoreAndFetch(t *testing.T) {
	ctx, _ := newFakeContext(t)
	want := sample()

	if err := dynamofixture.Save(ctx, table, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := dynamofixture.Fetch(ctx, table, want.ItemKey())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.Sensor != want.Sensor || got.Site.City != want.Site.City {
		t.Fatalf("round trip through the wire: %+v", got)
	}
	if number, ok := got.Exact.AsNumber(); !ok || number != exact {
		t.Fatalf("38-digit number through JSON: %q", number)
	}
}

// TestFetchMissKeepsTheDriverSentinel is the rule that no helper may swallow a
// driver error.
func TestFetchMissKeepsTheDriverSentinel(t *testing.T) {
	ctx, _ := newFakeContext(t)

	_, err := dynamofixture.Fetch(ctx, table, dynamodb.Key{
		"sensor": dynamodb.S("absent"), "at": dynamodb.N(1),
	})
	if !errors.Is(err, dynamodb.ErrItemNotFound) {
		t.Fatalf("errors.Is lost ErrItemNotFound: %v", err)
	}
	var driverError *dynamodb.Error
	if !errors.As(err, &driverError) {
		t.Fatalf("errors.As lost *dynamodb.Error: %v", err)
	}
	if driverError.Op != "GetItem" {
		t.Fatalf("driver error lost its context: %+v", driverError)
	}
}

func TestReplaceAndRetireReturnTheOldItem(t *testing.T) {
	ctx, _ := newFakeContext(t)
	first := sample()

	old, existed, err := dynamofixture.Replace(ctx, table, first)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if existed {
		t.Fatalf("nothing was stored yet, got %+v", old)
	}

	second := first
	second.Note = "corrected"
	old, existed, err = dynamofixture.Replace(ctx, table, second)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if !existed || old.Note != "" {
		t.Fatalf("replaced item: existed=%v %+v", existed, old)
	}

	deleted, existed, err := dynamofixture.Retire(ctx, table, second)
	if err != nil {
		t.Fatalf("Retire: %v", err)
	}
	if !existed || deleted.Note != "corrected" {
		t.Fatalf("deleted item: existed=%v %+v", existed, deleted)
	}
	if _, err := dynamofixture.Fetch(ctx, table, second.ItemKey()); !errors.Is(err, dynamodb.ErrItemNotFound) {
		t.Fatalf("item survived Retire: %v", err)
	}
}

func TestDeleteUsesOnlyTheKey(t *testing.T) {
	ctx, _ := newFakeContext(t)
	stored := sample()
	if err := dynamofixture.Save(ctx, table, stored); err != nil {
		t.Fatal(err)
	}

	// Only the key fields are filled in: Remove must not need the rest.
	if err := dynamofixture.Delete(ctx, table, dynamofixture.Reading{
		Sensor: stored.Sensor, At: stored.At,
	}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := dynamofixture.Fetch(ctx, table, stored.ItemKey()); !errors.Is(err, dynamodb.ErrItemNotFound) {
		t.Fatalf("item survived Delete: %v", err)
	}
}

func TestQueryPageReportsItsContinuation(t *testing.T) {
	ctx, fake := newFakeContext(t)
	store(t, fake, 5)

	page, err := dynamofixture.Page(ctx, table, "sensor = :s")
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if len(page.Items) != 2 || page.Count != 2 || page.ScannedCount != 2 {
		t.Fatalf("page: %+v", page)
	}
	if !page.HasMore() {
		t.Fatal("a page with a continuation key must report more")
	}
	if fake.count("Query") != 1 {
		t.Fatalf("one page is one request, got %d", fake.count("Query"))
	}
}

func TestQueryIteratorWalksEveryPage(t *testing.T) {
	ctx, fake := newFakeContext(t)
	store(t, fake, 5)

	var seen []int64
	for reading, err := range dynamofixture.Each(ctx, table, "sensor = :s") {
		if err != nil {
			t.Fatalf("iterate: %v", err)
		}
		seen = append(seen, reading.At)
	}
	if len(seen) != 5 {
		t.Fatalf("iterated %d of 5: %v", len(seen), seen)
	}
	// Five items at two per page: three requests. The count is the cost the
	// iterator hides, so the test states it.
	if requests := fake.count("Query"); requests != 3 {
		t.Fatalf("expected 3 requests for 3 pages, got %d", requests)
	}
}

func TestQueryIteratorStopsWithoutAnotherRequest(t *testing.T) {
	ctx, fake := newFakeContext(t)
	store(t, fake, 5)

	for _, err := range dynamofixture.Each(ctx, table, "sensor = :s") {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if requests := fake.count("Query"); requests != 1 {
		t.Fatalf("a break after the first item cost %d requests", requests)
	}
}

func TestScanIteratorWalksTheTable(t *testing.T) {
	ctx, fake := newFakeContext(t)
	store(t, fake, 3)

	count := 0
	for _, err := range dynamofixture.Sweep(ctx, table) {
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		count++
	}
	if count != 3 {
		t.Fatalf("scanned %d of 3", count)
	}
	if requests := fake.count("Scan"); requests != 2 {
		t.Fatalf("expected 2 requests for 3 items at 2 per page, got %d", requests)
	}
}

func TestStoreAllChunksAtTheServiceLimit(t *testing.T) {
	ctx, fake := newFakeContext(t)

	readings := make([]dynamofixture.Reading, 30)
	for i := range readings {
		readings[i] = dynamofixture.Reading{Sensor: "s", At: int64(i)}
	}
	unprocessed, err := dynamofixture.SaveAll(ctx, table, readings)
	if err != nil {
		t.Fatalf("SaveAll: %v", err)
	}
	if len(unprocessed) != 0 {
		t.Fatalf("%d writes went unprocessed", len(unprocessed))
	}
	// 30 writes at 25 per request is two requests, which is the whole reason
	// chunking is in the runtime.
	if requests := fake.count("BatchWriteItem"); requests != 2 {
		t.Fatalf("expected 2 requests for 30 writes, got %d", requests)
	}
}

func TestStoreAllReturnsDeclinedWritesUnretried(t *testing.T) {
	ctx, fake := newFakeContext(t)
	fake.declineWrites = 3

	readings := []dynamofixture.Reading{
		{Sensor: "s", At: 1, Note: "one"},
		{Sensor: "s", At: 2, Note: "two"},
		{Sensor: "s", At: 3, Note: "three"},
		{Sensor: "s", At: 4, Note: "four"},
	}
	unprocessed, err := dynamofixture.SaveAll(ctx, table, readings)
	if err != nil {
		t.Fatalf("SaveAll: %v", err)
	}
	if len(unprocessed) != 3 {
		t.Fatalf("expected the 3 declined writes back, got %d", len(unprocessed))
	}
	for i, reading := range unprocessed {
		if reading.At != readings[i].At || reading.Note != readings[i].Note {
			t.Fatalf("declined write %d came back as %+v", i, reading)
		}
	}
	// One request, not two: nothing here retries what the service declined.
	if requests := fake.count("BatchWriteItem"); requests != 1 {
		t.Fatalf("StoreAll retried: %d requests", requests)
	}
}

func TestLoadAllChunksAndDecodes(t *testing.T) {
	ctx, fake := newFakeContext(t)
	store(t, fake, 120)

	keys := make([]dynamodb.Key, 120)
	for i := range keys {
		keys[i] = dynamodb.Key{"sensor": dynamodb.S("s"), "at": dynamodb.N(int64(i))}
	}
	items, unprocessed, err := dynamofixture.FetchAll(ctx, table, keys)
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if len(items) != 120 || len(unprocessed) != 0 {
		t.Fatalf("read %d items, %d unprocessed", len(items), len(unprocessed))
	}
	if requests := fake.count("BatchGetItem"); requests != 2 {
		t.Fatalf("expected 2 requests for 120 keys, got %d", requests)
	}
}

func TestUpdatePassesTheExpressionThrough(t *testing.T) {
	ctx, fake := newFakeContext(t)
	stored := sample()
	if err := dynamofixture.Save(ctx, table, stored); err != nil {
		t.Fatal(err)
	}
	err := dynamofixture.Correct(ctx, table, stored, "SET note = :n",
		dynamodb.WithExpressionValues(map[string]dynamodb.AttributeValue{":n": dynamodb.S("fixed")}))
	if err != nil {
		t.Fatalf("Correct: %v", err)
	}
	if fake.count("UpdateItem") != 1 {
		t.Fatal("UpdateItem was not called")
	}
}

// store fills the fake with n readings under one sensor.
func store(t *testing.T, fake *fakeDynamo, n int) {
	t.Helper()
	for i := range n {
		fake.put(dynamofixture.Reading{Sensor: "s", At: int64(i), Note: strconv.Itoa(i)}.EncodeItem())
	}
}

// TestDeclaredQueryRunsAgainstTheWire drives a generated query function end to
// end. Nothing in the call names an attribute, a placeholder or an expression.
func TestDeclaredQueryRunsAgainstTheWire(t *testing.T) {
	ctx, fake := newFakeContext(t)
	store(t, fake, 5)

	var seen []int64
	for reading, err := range dynamofixture.ReadingsSince(ctx, "s", 0) {
		if err != nil {
			t.Fatalf("ReadingsSince: %v", err)
		}
		seen = append(seen, reading.At)
	}
	if len(seen) != 5 {
		t.Fatalf("iterated %d of 5: %v", len(seen), seen)
	}

	page, err := dynamofixture.ReadingsBetween(ctx, "s", 0, 10)
	if err != nil {
		t.Fatalf("ReadingsBetween: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("page: %+v", page)
	}
}

// TestDeclaredQuerySendsAliasesAndValues proves the request carries the aliased
// expression and the bound values the declaration described.
func TestDeclaredQuerySendsAliasesAndValues(t *testing.T) {
	ctx, fake := newFakeContext(t)
	store(t, fake, 1)

	for _, err := range dynamofixture.ReadingsSince(ctx, "s", 3) {
		if err != nil {
			t.Fatal(err)
		}
	}
	request := fake.lastRequest()
	if request.KeyConditionExpression != "#k0 = :v0 AND #k1 > :v1" {
		t.Fatalf("key condition: %q", request.KeyConditionExpression)
	}
	if request.ExpressionAttributeNames["#k0"] != "sensor" || request.ExpressionAttributeNames["#k1"] != "at" {
		t.Fatalf("names: %v", request.ExpressionAttributeNames)
	}
	sensor, _ := request.ExpressionAttributeValues[":v0"].AsString()
	from, _ := request.ExpressionAttributeValues[":v1"].AsNumber()
	if sensor != "s" || from != "3" {
		t.Fatalf("values: sensor=%q from=%q", sensor, from)
	}
}

// TestDeployedPrefixReachesTheWire proves the declared table name is what the
// prefix is applied to, in both result shapes. The calls name neither the
// client nor the table.
func TestDeployedPrefixReachesTheWire(t *testing.T) {
	client, fake := newFakeDynamo(t)
	store(t, fake, 3)
	ctx := dynamobind.WithClient(context.Background(), client, dynamobind.WithTablePrefix("staging-"))

	var seen []int64
	for reading, err := range dynamofixture.ReadingsSince(ctx, "s", 0) {
		if err != nil {
			t.Fatalf("ReadingsSince: %v", err)
		}
		seen = append(seen, reading.At)
	}
	if len(seen) != 3 {
		t.Fatalf("iterated %d of 3: %v", len(seen), seen)
	}
	if name := fake.lastRequest().TableName; name != "staging-readings" {
		t.Fatalf("table name on the wire: %q", name)
	}

	page, err := dynamofixture.ReadingsBetween(ctx, "s", 0, 10)
	if err != nil {
		t.Fatalf("ReadingsBetween: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("page: %+v", page)
	}
	if name := fake.lastRequest().TableName; name != "staging-readings" {
		t.Fatalf("table name on the wire: %q", name)
	}

	// An item operation names its own table, so the prefix applies there too.
	if err := dynamofixture.Save(ctx, table, sample()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if name := fake.lastRequest().TableName; name != "staging-readings" {
		t.Fatalf("table name on the wire: %q", name)
	}
}

// TestUnresolvableContextReachesNothing proves every entry fails loudly rather
// than reading the unprefixed table. An iterator cannot return the error, so it
// yields it once and stops.
func TestUnresolvableContextReachesNothing(t *testing.T) {
	client, fake := newFakeDynamo(t)
	store(t, fake, 3)

	for _, unresolvable := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "no client", ctx: context.Background(), want: dynamobind.ErrNoClient},
		{
			name: "no prefix",
			ctx:  dynamobind.WithClient(context.Background(), client),
			want: dynamobind.ErrNoTablePrefix,
		},
	} {
		t.Run(unresolvable.name, func(t *testing.T) {
			before := fake.count("Query") + fake.count("GetItem") + fake.count("PutItem")

			yields := 0
			for reading, err := range dynamofixture.ReadingsSince(unresolvable.ctx, "s", 0) {
				yields++
				if !errors.Is(err, unresolvable.want) {
					t.Fatalf("iterator error = %v, want %v", err, unresolvable.want)
				}
				if reading.At != 0 || reading.Sensor != "" {
					t.Fatalf("a failed resolution yielded an item: %+v", reading)
				}
			}
			if yields != 1 {
				t.Fatalf("the iterator yielded %d times, want 1", yields)
			}

			if _, err := dynamofixture.ReadingsBetween(unresolvable.ctx, "s", 0, 10); !errors.Is(err, unresolvable.want) {
				t.Fatalf("page error = %v, want %v", err, unresolvable.want)
			}
			// The item operations resolve through the same door.
			if _, err := dynamofixture.Fetch(unresolvable.ctx, table, sample().ItemKey()); !errors.Is(err, unresolvable.want) {
				t.Fatalf("Fetch error = %v, want %v", err, unresolvable.want)
			}
			if err := dynamofixture.Save(unresolvable.ctx, table, sample()); !errors.Is(err, unresolvable.want) {
				t.Fatalf("Save error = %v, want %v", err, unresolvable.want)
			}
			if _, err := dynamofixture.SaveAll(unresolvable.ctx, table, []dynamofixture.Reading{sample()}); !errors.Is(err, unresolvable.want) {
				t.Fatalf("SaveAll error = %v, want %v", err, unresolvable.want)
			}
			if _, _, err := dynamofixture.FetchAll(unresolvable.ctx, table, []dynamodb.Key{sample().ItemKey()}); !errors.Is(err, unresolvable.want) {
				t.Fatalf("FetchAll error = %v, want %v", err, unresolvable.want)
			}

			if fake.count("Query")+fake.count("GetItem")+fake.count("PutItem") != before {
				t.Fatal("an unresolvable Context still reached the service")
			}
		})
	}
}

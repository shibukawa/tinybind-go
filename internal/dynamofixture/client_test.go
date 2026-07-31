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

	"github.com/shibukawa/tinybind-go/internal/dynamofixture"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

func TestStoreAndFetch(t *testing.T) {
	ctx := context.Background()
	client, _ := newFakeDynamo(t)
	want := sample()

	if err := dynamofixture.Save(ctx, client, table, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := dynamofixture.Fetch(ctx, client, table, want.ItemKey())
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
	ctx := context.Background()
	client, _ := newFakeDynamo(t)

	_, err := dynamofixture.Fetch(ctx, client, table, dynamodb.Key{
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
	ctx := context.Background()
	client, _ := newFakeDynamo(t)
	first := sample()

	old, existed, err := dynamofixture.Replace(ctx, client, table, first)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if existed {
		t.Fatalf("nothing was stored yet, got %+v", old)
	}

	second := first
	second.Note = "corrected"
	old, existed, err = dynamofixture.Replace(ctx, client, table, second)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if !existed || old.Note != "" {
		t.Fatalf("replaced item: existed=%v %+v", existed, old)
	}

	deleted, existed, err := dynamofixture.Retire(ctx, client, table, second)
	if err != nil {
		t.Fatalf("Retire: %v", err)
	}
	if !existed || deleted.Note != "corrected" {
		t.Fatalf("deleted item: existed=%v %+v", existed, deleted)
	}
	if _, err := dynamofixture.Fetch(ctx, client, table, second.ItemKey()); !errors.Is(err, dynamodb.ErrItemNotFound) {
		t.Fatalf("item survived Retire: %v", err)
	}
}

func TestDeleteUsesOnlyTheKey(t *testing.T) {
	ctx := context.Background()
	client, _ := newFakeDynamo(t)
	stored := sample()
	if err := dynamofixture.Save(ctx, client, table, stored); err != nil {
		t.Fatal(err)
	}

	// Only the key fields are filled in: Remove must not need the rest.
	if err := dynamofixture.Delete(ctx, client, table, dynamofixture.Reading{
		Sensor: stored.Sensor, At: stored.At,
	}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := dynamofixture.Fetch(ctx, client, table, stored.ItemKey()); !errors.Is(err, dynamodb.ErrItemNotFound) {
		t.Fatalf("item survived Delete: %v", err)
	}
}

func TestQueryPageReportsItsContinuation(t *testing.T) {
	ctx := context.Background()
	client, fake := newFakeDynamo(t)
	store(t, fake, 5)

	page, err := dynamofixture.Page(ctx, client, table, "sensor = :s")
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
	ctx := context.Background()
	client, fake := newFakeDynamo(t)
	store(t, fake, 5)

	var seen []int64
	for reading, err := range dynamofixture.Each(ctx, client, table, "sensor = :s") {
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
	ctx := context.Background()
	client, fake := newFakeDynamo(t)
	store(t, fake, 5)

	for _, err := range dynamofixture.Each(ctx, client, table, "sensor = :s") {
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
	ctx := context.Background()
	client, fake := newFakeDynamo(t)
	store(t, fake, 3)

	count := 0
	for _, err := range dynamofixture.Sweep(ctx, client, table) {
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
	ctx := context.Background()
	client, fake := newFakeDynamo(t)

	readings := make([]dynamofixture.Reading, 30)
	for i := range readings {
		readings[i] = dynamofixture.Reading{Sensor: "s", At: int64(i)}
	}
	unprocessed, err := dynamofixture.SaveAll(ctx, client, table, readings)
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
	ctx := context.Background()
	client, fake := newFakeDynamo(t)
	fake.declineWrites = 3

	readings := []dynamofixture.Reading{
		{Sensor: "s", At: 1, Note: "one"},
		{Sensor: "s", At: 2, Note: "two"},
		{Sensor: "s", At: 3, Note: "three"},
		{Sensor: "s", At: 4, Note: "four"},
	}
	unprocessed, err := dynamofixture.SaveAll(ctx, client, table, readings)
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
	ctx := context.Background()
	client, fake := newFakeDynamo(t)
	store(t, fake, 120)

	keys := make([]dynamodb.Key, 120)
	for i := range keys {
		keys[i] = dynamodb.Key{"sensor": dynamodb.S("s"), "at": dynamodb.N(int64(i))}
	}
	items, unprocessed, err := dynamofixture.FetchAll(ctx, client, table, keys)
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
	ctx := context.Background()
	client, fake := newFakeDynamo(t)
	stored := sample()
	if err := dynamofixture.Save(ctx, client, table, stored); err != nil {
		t.Fatal(err)
	}
	err := dynamofixture.Correct(ctx, client, table, stored, "SET note = :n",
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

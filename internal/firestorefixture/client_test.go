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

func withFake(t *testing.T) (context.Context, *fakeDatastore) {
	t.Helper()
	client, fake := newFakeDatastore(t)
	return firestorebind.WithClient(t.Context(), client), fake
}

func TestStoreAndLoadThroughTheDriver(t *testing.T) {
	ctx, _ := withFake(t)
	want := sample()

	key, err := firestorefixture.StoreReading(ctx, want)
	if err != nil {
		t.Fatalf("StoreReading: %v", err)
	}
	if key.Kind() != "Reading" {
		t.Errorf("returned key: got %v", key)
	}

	got, err := firestorefixture.LoadReading(ctx, want.EntityKey())
	if err != nil {
		t.Fatalf("LoadReading: %v", err)
	}
	if got.ID != want.ID || got.Note != want.Note || got.Count != want.Count {
		t.Errorf("round trip through the wire: got %+v", got)
	}
	if !got.At.Equal(want.At) {
		t.Errorf("At: got %v, want %v", got.At, want.At)
	}
}

// A miss stays the driver's sentinel rather than becoming a zero value, so it
// cannot be mistaken for an empty entity.
func TestLoadMissKeepsTheSentinel(t *testing.T) {
	ctx, _ := withFake(t)

	_, err := firestorefixture.LoadReading(ctx, datastore.NameKey("Reading", "absent"))
	if !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("got %v, want ErrNoSuchEntity", err)
	}
}

// The driver's typed error survives the binding, including the status string
// that is the only reliable discriminator.
func TestInsertConflictPassesThrough(t *testing.T) {
	ctx, _ := withFake(t)
	task := firestorefixture.Task{Number: 1, Title: "first"}

	if _, err := firestorefixture.InsertTask(ctx, task); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := firestorefixture.InsertTask(ctx, task)
	if !errors.Is(err, datastore.ErrAlreadyExists) {
		t.Fatalf("got %v, want ErrAlreadyExists", err)
	}
	var de *datastore.Error
	if !errors.As(err, &de) {
		t.Fatalf("errors.As to *datastore.Error failed for %v", err)
	}
	if de.Status != "ALREADY_EXISTS" {
		t.Errorf("status: got %q", de.Status)
	}
	// ALREADY_EXISTS and ABORTED share HTTP 409 and mean opposite things, so a
	// binding that reported this as retryable would retry a duplicate forever.
	if de.Retryable() {
		t.Error("a duplicate insert reported itself retryable")
	}
}

func TestNoClientIsAnErrorNotAPanic(t *testing.T) {
	_, err := firestorefixture.LoadReading(context.Background(), datastore.NameKey("Reading", "r-1"))
	if !errors.Is(err, firestorebind.ErrNoClient) {
		t.Fatalf("got %v, want ErrNoClient", err)
	}
}

// The iterator pages, and the request count it hides is real.
func TestQueryIteratesEveryBatch(t *testing.T) {
	ctx, fake := withFake(t)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		r := sample()
		r.ID = firestorefixture.SensorID(id)
		if _, err := firestorefixture.StoreReading(ctx, r); err != nil {
			t.Fatalf("store %s: %v", id, err)
		}
	}

	var seen []string
	for reading, err := range firestorefixture.QueryReadings(ctx, datastore.NewQuery("Reading")) {
		if err != nil {
			t.Fatalf("iterate: %v", err)
		}
		seen = append(seen, string(reading.ID))
	}
	if len(seen) != 5 {
		t.Fatalf("got %d readings, want 5: %v", len(seen), seen)
	}
	// Five entities at a page size of two is three requests. The iterator makes
	// that invisible to the caller, which is exactly why QueryPage stays public.
	if got := fake.countOf("runQuery"); got != 3 {
		t.Errorf("runQuery count: got %d, want 3", got)
	}
}

// An early break stops without issuing a further request.
func TestQueryBreakStopsRequesting(t *testing.T) {
	ctx, fake := withFake(t)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		r := sample()
		r.ID = firestorefixture.SensorID(id)
		if _, err := firestorefixture.StoreReading(ctx, r); err != nil {
			t.Fatalf("store %s: %v", id, err)
		}
	}

	for _, err := range firestorefixture.QueryReadings(ctx, datastore.NewQuery("Reading")) {
		if err != nil {
			t.Fatalf("iterate: %v", err)
		}
		break
	}
	if got := fake.countOf("runQuery"); got != 1 {
		t.Errorf("runQuery count after a break: got %d, want 1", got)
	}
}

// Found, missing and deferred are three different facts, and a missing key is
// not an absent value.
func TestLoadAllSeparatesMissingFromFound(t *testing.T) {
	ctx, _ := withFake(t)
	present := sample()
	if _, err := firestorefixture.StoreReading(ctx, present); err != nil {
		t.Fatalf("store: %v", err)
	}

	keys := []datastore.Key{
		present.EntityKey(),
		datastore.NameKey("Reading", "absent"),
	}
	values, missing, deferred, err := firestorefixture.LoadReadings(ctx, keys)
	if err != nil {
		t.Fatalf("LoadReadings: %v", err)
	}
	if len(values) != 1 || values[0].ID != present.ID {
		t.Errorf("values: got %+v", values)
	}
	if len(missing) != 1 || missing[0].Path[0].Name != "absent" {
		t.Errorf("missing: got %v", missing)
	}
	if len(deferred) != 0 {
		t.Errorf("deferred: got %v", deferred)
	}
}

// A batch write commits in as few requests as the size limit allows. Nothing
// here chunks by count, because Datastore publishes no count to chunk against.
func TestStoreAllCommitsInOneRequestWhenItFits(t *testing.T) {
	ctx, fake := withFake(t)
	readings := make([]firestorefixture.Reading, 0, 20)
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j",
		"k", "l", "m", "n", "o", "p", "q", "r", "s", "t"} {
		r := sample()
		r.ID = firestorefixture.SensorID(id)
		readings = append(readings, r)
	}

	keys, err := firestorefixture.StoreReadings(ctx, readings)
	if err != nil {
		t.Fatalf("StoreReadings: %v", err)
	}
	if len(keys) != len(readings) {
		t.Errorf("keys: got %d, want %d", len(keys), len(readings))
	}
	if got := fake.countOf("commit"); got != 1 {
		t.Errorf("commit count: got %d, want 1; twenty small entities fit one request", got)
	}
	fake.mu.Lock()
	size := fake.lastCommitSize
	fake.mu.Unlock()
	if size != len(readings) {
		t.Errorf("mutations in the commit: got %d, want %d", size, len(readings))
	}
}

// A read-modify-write is what a transaction is for, since nothing on this wire
// evaluates a predicate over a property.
func TestTransactionReadModifyWrite(t *testing.T) {
	ctx, fake := withFake(t)
	task := firestorefixture.Task{Number: 3, Title: "before"}
	if _, err := firestorefixture.InsertTask(ctx, task); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := firestorefixture.RenameInTransaction(ctx, task.EntityKey(), "after"); err != nil {
		t.Fatalf("RenameInTransaction: %v", err)
	}
	if got := fake.countOf("beginTransaction"); got != 1 {
		t.Errorf("beginTransaction count: got %d, want 1", got)
	}

	var got firestorefixture.Task
	loaded, err := firestorebind.Load[firestorefixture.Task](ctx, task.EntityKey())
	if err != nil {
		t.Fatalf("load after commit: %v", err)
	}
	got = loaded
	if got.Title != "after" {
		t.Errorf("title: got %q, want after", got.Title)
	}
	if got.Number != 3 {
		t.Errorf("number: got %d, want 3", got.Number)
	}
}

// Contention re-runs the whole closure rather than resending the commit, and
// the binding adds no second loop on top of that.
func TestTransactionRestartsOnAbort(t *testing.T) {
	ctx, fake := withFake(t)
	task := firestorefixture.Task{Number: 4, Title: "before"}
	if _, err := firestorefixture.InsertTask(ctx, task); err != nil {
		t.Fatalf("insert: %v", err)
	}

	fake.mu.Lock()
	fake.abortCommits = 1
	fake.mu.Unlock()

	if err := firestorefixture.RenameInTransaction(ctx, task.EntityKey(), "after"); err != nil {
		t.Fatalf("RenameInTransaction: %v", err)
	}
	if got := fake.countOf("beginTransaction"); got != 2 {
		t.Errorf("beginTransaction count: got %d, want 2; the closure re-runs, so the reads run again", got)
	}
	if got := fake.countOf("commit"); got != 3 {
		// One insert, one aborted commit, one that succeeded.
		t.Errorf("commit count: got %d, want 3", got)
	}
}

// A closure that returns an error writes nothing, because the mutations travel
// with the commit that never happens.
func TestTransactionErrorWritesNothing(t *testing.T) {
	ctx, fake := withFake(t)
	task := firestorefixture.Task{Number: 5, Title: "before"}
	if _, err := firestorefixture.InsertTask(ctx, task); err != nil {
		t.Fatalf("insert: %v", err)
	}
	before := fake.countOf("commit")

	sentinel := errors.New("decided against it")
	err := firestorebind.Run(ctx, func(tx *firestorebind.Tx) error {
		tx.Store(firestorefixture.Task{Number: 5, Title: "after"})
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v, want the closure's own error", err)
	}
	if got := fake.countOf("commit"); got != before {
		t.Errorf("commit count: got %d, want %d; an aborted closure commits nothing", got, before)
	}

	loaded, err := firestorebind.Load[firestorefixture.Task](ctx, task.EntityKey())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Title != "before" {
		t.Errorf("title: got %q, want before", loaded.Title)
	}
}

func TestRemoveDeletes(t *testing.T) {
	ctx, _ := withFake(t)
	r := sample()
	if _, err := firestorefixture.StoreReading(ctx, r); err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := firestorefixture.RemoveReading(ctx, r); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := firestorefixture.LoadReading(ctx, r.EntityKey()); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("after remove: got %v, want ErrNoSuchEntity", err)
	}
}

// An incomplete key cannot identify what to delete, and saying so here beats
// sending a request the server will reject.
func TestRemoveRejectsAnIncompleteKey(t *testing.T) {
	ctx, fake := withFake(t)
	before := fake.countOf("commit")

	err := firestorefixture.RemoveReading(ctx, firestorefixture.Reading{})
	if err == nil {
		t.Fatal("removing an entity with no identity succeeded")
	}
	if _, ok := firestorebind.AsError(err); !ok {
		t.Errorf("error is not a firestorebind Error: %v", err)
	}
	if got := fake.countOf("commit"); got != before {
		t.Errorf("a request was sent anyway: commits went from %d to %d", before, got)
	}
}

// The namespace is a request fact rather than a property of the type, so a
// generated key carries none and the runtime stamps it.
func TestNamespaceComesFromTheContext(t *testing.T) {
	client, _ := newFakeDatastore(t)
	ctx := firestorebind.WithClient(t.Context(), client,
		firestorebind.WithNamespace(func(context.Context) string { return "tenant-a" }))

	r := sample()
	if _, err := firestorefixture.StoreReading(ctx, r); err != nil {
		t.Fatalf("store: %v", err)
	}
	// The same key, unqualified, finds it again because the resolver applies to
	// the read as well as to the write.
	got, err := firestorefixture.LoadReading(ctx, r.EntityKey())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ID != r.ID {
		t.Errorf("got %+v", got)
	}

	// A second tenant is a second Context, and it does not see the first.
	other := firestorebind.WithClient(t.Context(), client,
		firestorebind.WithNamespace(func(context.Context) string { return "tenant-b" }))
	if _, err := firestorefixture.LoadReading(other, r.EntityKey()); !errors.Is(err, datastore.ErrNoSuchEntity) {
		t.Fatalf("tenant-b saw tenant-a's entity: %v", err)
	}
}

func TestCountUsesTheAggregationQuery(t *testing.T) {
	ctx, fake := withFake(t)
	for _, id := range []string{"a", "b", "c"} {
		r := sample()
		r.ID = firestorefixture.SensorID(id)
		if _, err := firestorefixture.StoreReading(ctx, r); err != nil {
			t.Fatalf("store %s: %v", id, err)
		}
	}

	n, err := firestorebind.Count(ctx, datastore.NewQuery("Reading"))
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Errorf("count: got %d, want 3", n)
	}
	if got := fake.countOf("runAggregationQuery"); got != 1 {
		t.Errorf("aggregation count: got %d, want 1", got)
	}
	// Counting must not have paged through the entities, which is the expensive
	// thing this exists to avoid.
	if got := fake.countOf("runQuery"); got != 0 {
		t.Errorf("runQuery count: got %d, want 0", got)
	}
}

// A version tag turns a later write into optimistic concurrency: the write
// applies only if the stored entity is still at the version that was read.
func TestVersionMakesTheWriteConditional(t *testing.T) {
	ctx, fake := withFake(t)
	r := sample()
	if _, err := firestorefixture.StoreReading(ctx, r); err != nil {
		t.Fatalf("store: %v", err)
	}

	// A value that was never read carries version zero and sends no
	// precondition, which is what an unconditional first write should do.
	if got := fake.lastBaseVersion(); got != "" {
		t.Errorf("baseVersion on an unread value: got %q, want none", got)
	}

	loaded, err := firestorefixture.LoadReading(ctx, r.EntityKey())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Ver == 0 {
		t.Fatal("the decoder filled no version; there is nothing to be conditional on")
	}
	if _, err := firestorefixture.StoreReading(ctx, loaded); err != nil {
		t.Fatalf("conditional store: %v", err)
	}
	if got := fake.lastBaseVersion(); got == "" {
		t.Error("a value read at a version wrote unconditionally")
	}
}

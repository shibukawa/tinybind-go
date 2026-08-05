//go:build !tinygo

package firestorefixture_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/tinygodriver/cloud/google"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// fakeDatastore is enough of the Datastore v1 API to exercise the binding: it
// speaks the same JSON wire shapes over HTTP, so the driver's own request
// building, response decoding and error mapping all run.
//
// It is not a Datastore implementation - there are no indexes, no consistency
// and no real contention - and nothing here should test Datastore semantics.
type fakeDatastore struct {
	mu sync.Mutex
	// entities is the store, keyed by the key's String form.
	entities map[string]json.RawMessage
	// order keeps insertion order, so query replies are deterministic.
	order []string
	// pageSize splits runQuery replies, so pagination is real.
	pageSize int
	// calls counts requests per RPC, which is how a test observes the request
	// count an iterator hides.
	calls map[string]int
	// abortCommits fails the first N commits with ABORTED, so the driver's
	// transaction restart is exercised.
	abortCommits int
	// lastCommitSize is how many mutations the most recent commit carried.
	lastCommitSize int
	// commitSizes records every commit's mutation count, so a test can see how
	// a batch was chunked.
	commitSizes []int
	// commitBytes records every commit's request body length, which is the
	// figure the chunking is supposed to keep under the request limit. Counting
	// mutations cannot show that: the whole point of the envelope is that the
	// body is larger than the mutations in it.
	commitBytes []int
	// baseVersion is the precondition on the most recent write mutation, empty
	// when none was sent.
	baseVersion string
	// inlineTxStarts counts the reads that opened a transaction through
	// readOptions.newTransaction, and singleUseCommits the commits that opened
	// one through singleUseTransaction. As of tinygodriver v1.1.7 these are how
	// a transaction begins; beginTransaction is no longer sent, so counting that
	// RPC no longer says whether anything ran transactionally.
	inlineTxStarts   int
	singleUseCommits int
	// kind and keysOnly are what the most recent query asked for, which is how
	// a test sees that a declaration reached the wire as written.
	kind     string
	keysOnly bool
	// filterOps are the composite operators of the most recent query, so a test
	// can see that a disjunction went out as one rather than being flattened
	// into a conjunction.
	filterOps []string
}

func (f *fakeDatastore) lastFilterOps() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.filterOps)
}

// compositeOps collects the op of every compositeFilter in a query, at any
// depth.
func compositeOps(raw json.RawMessage) []string {
	var node struct {
		CompositeFilter *struct {
			Op      string            `json:"op"`
			Filters []json.RawMessage `json:"filters"`
		} `json:"compositeFilter"`
	}
	if json.Unmarshal(raw, &node) != nil || node.CompositeFilter == nil {
		return nil
	}
	out := []string{node.CompositeFilter.Op}
	for _, child := range node.CompositeFilter.Filters {
		out = append(out, compositeOps(child)...)
	}
	return out
}

func (f *fakeDatastore) lastKind() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.kind
}

func (f *fakeDatastore) lastKeysOnly() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.keysOnly
}

func (f *fakeDatastore) lastBaseVersion() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.baseVersion
}

type wireKey struct {
	PartitionID *struct {
		ProjectID   string `json:"projectId"`
		NamespaceID string `json:"namespaceId"`
	} `json:"partitionId"`
	Path []struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"path"`
}

func (k wireKey) id() string {
	var b strings.Builder
	if k.PartitionID != nil && k.PartitionID.NamespaceID != "" {
		b.WriteString(k.PartitionID.NamespaceID)
		b.WriteString("|")
	}
	for _, e := range k.Path {
		b.WriteString(e.Kind)
		b.WriteString("/")
		if e.Name != "" {
			b.WriteString(e.Name)
		} else {
			b.WriteString(e.ID)
		}
		b.WriteString(":")
	}
	return b.String()
}

func (k wireKey) kind() string {
	if len(k.Path) == 0 {
		return ""
	}
	return k.Path[len(k.Path)-1].Kind
}

type wireEntity struct {
	Key        json.RawMessage            `json:"key"`
	Properties map[string]json.RawMessage `json:"properties"`
}

type wireMutation struct {
	Insert *wireEntity     `json:"insert"`
	Update *wireEntity     `json:"update"`
	Upsert *wireEntity     `json:"upsert"`
	Delete json.RawMessage `json:"delete"`
	// BaseVersion is the optimistic-concurrency precondition. The fake does not
	// enforce it - that is a Datastore semantic, not a wire shape - but it
	// records whether one was sent.
	BaseVersion string `json:"baseVersion"`
}

func newFakeDatastore(t *testing.T) (*datastore.Client, *fakeDatastore) {
	t.Helper()
	fake := &fakeDatastore{
		entities: map[string]json.RawMessage{},
		pageSize: 2,
		calls:    map[string]int{},
	}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(server.Close)

	client, err := datastore.New("test-project",
		datastore.WithEndpoint(server.URL),
		// The credential path is not what these tests are about, and the
		// emulator ignores the header too, so a static token stands in.
		datastore.WithTokenSource(google.StaticTokenSource(google.Token{
			Value:  "test-token",
			Expiry: time.Now().Add(time.Hour),
		})),
	)
	if err != nil {
		t.Fatalf("datastore.New: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, fake
}

func (f *fakeDatastore) countOf(rpc string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[rpc]
}

// transactionStarts is how many transactions were opened, however they were
// opened: explicitly, by a read that asked for a new one, or by a commit that
// carried its own.
func (f *fakeDatastore) transactionStarts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls["beginTransaction"] + f.inlineTxStarts + f.singleUseCommits
}

// startsTransaction reports whether readOptions asked for a new transaction,
// and records it. The reply then has to carry the handle back, because that is
// the only place the caller can learn it.
func (f *fakeDatastore) startsTransaction(body map[string]json.RawMessage) bool {
	var options struct {
		NewTransaction json.RawMessage `json:"newTransaction"`
	}
	_ = json.Unmarshal(body["readOptions"], &options)
	if options.NewTransaction == nil {
		return false
	}
	f.inlineTxStarts++
	return true
}

func (f *fakeDatastore) serve(w http.ResponseWriter, r *http.Request) {
	rpc := r.URL.Path[strings.LastIndex(r.URL.Path, ":")+1:]
	f.mu.Lock()
	f.calls[rpc]++
	f.mu.Unlock()

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		f.fail(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}
	// Every key on the wire carries the partition that says which project,
	// database and namespace it belongs to. A Key inside a program carries only
	// the path, so the partition is attached at encode time, and a keyValue that
	// arrives without one was built by a path that skipped that step.
	//
	// This is asserted here rather than in one test because it is a property of
	// every request: the driver shipped with it holding at exactly one nesting
	// level, and a stored reference went out naming no project for as long as
	// that lasted. Checking the shape of the request catches the class wherever
	// it comes back.
	//
	// wireKey declares partitionId first, so a keyValue whose object opens with
	// "path" is exactly one that never got a partition.
	if bytes.Contains(raw, []byte(`"keyValue":{"path"`)) {
		f.fail(w, http.StatusBadRequest, "INVALID_ARGUMENT",
			"a keyValue reached the wire with no partitionId: "+string(raw))
		return
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		f.fail(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	switch rpc {
	case "lookup":
		f.lookup(w, body)
	case "commit":
		f.mu.Lock()
		f.commitBytes = append(f.commitBytes, len(raw))
		f.mu.Unlock()
		f.commit(w, body)
	case "runQuery":
		f.runQuery(w, body)
	case "runAggregationQuery":
		f.runAggregationQuery(w, body)
	case "beginTransaction":
		writeJSON(w, map[string]any{"transaction": "tx-handle"})
	case "rollback":
		writeJSON(w, map[string]any{})
	default:
		f.fail(w, http.StatusNotFound, "NOT_FOUND", "no such rpc "+rpc)
	}
}

func (f *fakeDatastore) lookup(w http.ResponseWriter, body map[string]json.RawMessage) {
	var keys []json.RawMessage
	_ = json.Unmarshal(body["keys"], &keys)

	f.mu.Lock()
	defer f.mu.Unlock()
	var found, missing []map[string]any
	for _, raw := range keys {
		var k wireKey
		_ = json.Unmarshal(raw, &k)
		if entity, ok := f.entities[k.id()]; ok {
			found = append(found, map[string]any{"entity": entity, "version": "1"})
		} else {
			missing = append(missing, map[string]any{"entity": map[string]any{"key": raw}, "version": "0"})
		}
	}
	reply := map[string]any{"found": found, "missing": missing}
	if f.startsTransaction(body) {
		reply["transaction"] = "tx-handle"
	}
	writeJSON(w, reply)
}

func (f *fakeDatastore) commit(w http.ResponseWriter, body map[string]json.RawMessage) {
	var mutations []wireMutation
	_ = json.Unmarshal(body["mutations"], &mutations)

	f.mu.Lock()
	defer f.mu.Unlock()
	// A commit that carries its own transaction is one the closure never read
	// in: there was no read to fold the begin into, so the commit does it.
	if body["singleUseTransaction"] != nil {
		f.singleUseCommits++
	}
	f.lastCommitSize = len(mutations)
	f.commitSizes = append(f.commitSizes, len(mutations))
	if f.abortCommits > 0 {
		f.abortCommits--
		f.fail(w, http.StatusConflict, "ABORTED", "too much contention")
		return
	}

	f.baseVersion = ""
	results := make([]map[string]any, 0, len(mutations))
	for _, m := range mutations {
		if m.BaseVersion != "" {
			f.baseVersion = m.BaseVersion
		}
		if m.Delete != nil {
			var k wireKey
			_ = json.Unmarshal(m.Delete, &k)
			delete(f.entities, k.id())
			f.dropOrder(k.id())
			results = append(results, map[string]any{"version": "2"})
			continue
		}
		entity := m.Upsert
		if entity == nil {
			entity = m.Insert
		}
		if entity == nil {
			entity = m.Update
		}
		if entity == nil {
			f.fail(w, http.StatusBadRequest, "INVALID_ARGUMENT", "mutation names no verb")
			return
		}
		var k wireKey
		_ = json.Unmarshal(entity.Key, &k)
		id := k.id()
		if _, exists := f.entities[id]; exists && m.Insert != nil {
			f.fail(w, http.StatusConflict, "ALREADY_EXISTS", "entity already exists")
			return
		}
		raw, _ := json.Marshal(entity)
		if _, exists := f.entities[id]; !exists {
			f.order = append(f.order, id)
		}
		f.entities[id] = raw
		results = append(results, map[string]any{"key": entity.Key, "version": "2"})
	}
	writeJSON(w, map[string]any{"mutationResults": results, "indexUpdates": len(mutations)})
}

func (f *fakeDatastore) dropOrder(id string) {
	for i, existing := range f.order {
		if existing == id {
			f.order = append(f.order[:i], f.order[i+1:]...)
			return
		}
	}
}

// matching returns the stored entities of one kind, in insertion order. The fake
// filters on kind alone: a real filter needs an index, which is exactly what
// this cannot model.
func (f *fakeDatastore) matching(kind string) []json.RawMessage {
	var out []json.RawMessage
	for _, id := range f.order {
		raw := f.entities[id]
		var e wireEntity
		_ = json.Unmarshal(raw, &e)
		var k wireKey
		_ = json.Unmarshal(e.Key, &k)
		if kind == "" || k.kind() == kind {
			out = append(out, raw)
		}
	}
	return out
}

func (f *fakeDatastore) runQuery(w http.ResponseWriter, body map[string]json.RawMessage) {
	var query struct {
		Kind []struct {
			Name string `json:"name"`
		} `json:"kind"`
		Projection []struct {
			Property struct {
				Name string `json:"name"`
			} `json:"property"`
		} `json:"projection"`
		Filter      json.RawMessage `json:"filter"`
		StartCursor string          `json:"startCursor"`
	}
	_ = json.Unmarshal(body["query"], &query)
	kind := ""
	if len(query.Kind) > 0 {
		kind = query.Kind[0].Name
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.kind = kind
	// A keys-only query is a projection of the __key__ pseudo-property, which
	// is how it travels on this wire.
	f.keysOnly = len(query.Projection) == 1 && query.Projection[0].Property.Name == "__key__"
	f.filterOps = compositeOps(query.Filter)
	all := f.matching(kind)
	start := 0
	if query.StartCursor != "" {
		// The cursor is the decimal offset, which is all this fake needs it to
		// be. A real cursor is opaque and the caller never reads it either.
		for i, c := range all {
			_ = c
			if query.StartCursor == cursorAt(i) {
				start = i
				break
			}
		}
	}
	end := min(start+f.pageSize, len(all))
	results := make([]map[string]any, 0, end-start)
	for _, raw := range all[start:end] {
		results = append(results, map[string]any{"entity": raw, "version": "1"})
	}
	more := "NO_MORE_RESULTS"
	if end < len(all) {
		more = "NOT_FINISHED"
	}
	reply := map[string]any{"batch": map[string]any{
		"entityResults": results,
		"endCursor":     cursorAt(end),
		"moreResults":   more,
	}}
	if f.startsTransaction(body) {
		reply["transaction"] = "tx-handle"
	}
	writeJSON(w, reply)
}

func cursorAt(i int) string {
	return "cursor-" + string(rune('a'+i))
}

func (f *fakeDatastore) runAggregationQuery(w http.ResponseWriter, body map[string]json.RawMessage) {
	var agg struct {
		NestedQuery struct {
			Kind []struct {
				Name string `json:"name"`
			} `json:"kind"`
		} `json:"nestedQuery"`
	}
	_ = json.Unmarshal(body["aggregationQuery"], &agg)
	kind := ""
	if len(agg.NestedQuery.Kind) > 0 {
		kind = agg.NestedQuery.Kind[0].Name
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	count := len(f.matching(kind))
	writeJSON(w, map[string]any{"batch": map[string]any{
		"aggregationResults": []map[string]any{
			// The alias is "count", which is what the driver asks for and what
			// it reads back; a reply keyed on anything else is not this reply.
			{"aggregateProperties": map[string]any{"count": map[string]any{
				"integerValue": itoa(count),
			}}},
		},
	}})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func (f *fakeDatastore) fail(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": status, "message": message, "status": code},
	})
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

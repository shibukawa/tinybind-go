//go:build !tinygo

package dynamofixture_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// fakeDynamo is enough of DynamoDB to exercise the binding: it speaks the same
// JSON wire shapes over HTTP, so the driver's own request building, signing and
// response decoding all run. It is not a DynamoDB implementation - there are no
// conditions, no capacity and no consistency - and nothing here should test
// DynamoDB semantics.
type fakeDynamo struct {
	mu    sync.Mutex
	items []dynamodb.Item
	// pageSize splits Query and Scan replies, so pagination is real.
	pageSize int
	// declineWrites drops the first N batch writes into UnprocessedItems.
	declineWrites int
	// calls counts requests per operation, which is how a test observes the
	// request count an iterator hides.
	calls map[string]int
}

type fakeRequest struct {
	TableName         string          `json:"TableName"`
	Key               dynamodb.Key    `json:"Key"`
	Item              dynamodb.Item   `json:"Item"`
	ReturnValues      string          `json:"ReturnValues"`
	UpdateExpression  string          `json:"UpdateExpression"`
	ExclusiveStartKey dynamodb.Key    `json:"ExclusiveStartKey"`
	RequestItems      json.RawMessage `json:"RequestItems"`
}

func newFakeDynamo(t *testing.T) (*dynamodb.Client, *fakeDynamo) {
	t.Helper()
	fake := &fakeDynamo{pageSize: 2, calls: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(server.Close)

	client, err := dynamodb.New(
		dynamodb.WithEndpoint(server.URL),
		dynamodb.WithRegion("ap-northeast-1"),
		dynamodb.WithCredentials(aws.Credentials{AccessKeyID: "id", SecretAccessKey: "secret"}),
	)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, fake
}

func (f *fakeDynamo) count(op string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[op]
}

func (f *fakeDynamo) put(item dynamodb.Item) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putLocked(item)
}

func (f *fakeDynamo) putLocked(item dynamodb.Item) dynamodb.Item {
	key := itemKey(item)
	for i, existing := range f.items {
		if itemKey(existing) == key {
			old := f.items[i]
			f.items[i] = item
			return old
		}
	}
	f.items = append(f.items, item)
	return nil
}

func (f *fakeDynamo) deleteLocked(key dynamodb.Key) dynamodb.Item {
	want := itemKey(key)
	for i, existing := range f.items {
		if itemKey(existing) == want {
			old := f.items[i]
			f.items = append(f.items[:i], f.items[i+1:]...)
			return old
		}
	}
	return nil
}

// itemKey identifies an item by its primary key attributes. The fixture's table
// is keyed by sensor and at, which is all this needs to know.
func itemKey(item dynamodb.Item) string {
	sensor, _ := item["sensor"].AsString()
	at, _ := item["at"].AsNumber()
	return sensor + "\x00" + at
}

func (f *fakeDynamo) serve(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	op := target[strings.LastIndex(target, ".")+1:]

	var req fakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFakeError(w, http.StatusBadRequest, "ValidationException", err.Error())
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[op]++

	switch op {
	case "GetItem":
		f.getItem(w, req)
	case "PutItem":
		f.putItem(w, req)
	case "DeleteItem":
		f.deleteItem(w, req)
	case "UpdateItem":
		writeFakeJSON(w, map[string]any{})
	case "Query", "Scan":
		f.page(w, req)
	case "BatchWriteItem":
		f.batchWrite(w, req)
	case "BatchGetItem":
		f.batchGet(w, req)
	default:
		writeFakeError(w, http.StatusBadRequest, "UnknownOperationException", op)
	}
}

func (f *fakeDynamo) getItem(w http.ResponseWriter, req fakeRequest) {
	want := itemKey(req.Key)
	for _, item := range f.items {
		if itemKey(item) == want {
			writeFakeJSON(w, map[string]any{"Item": item})
			return
		}
	}
	// A miss is a 200 with no Item member, which is what makes the driver's
	// ErrItemNotFound worth having.
	writeFakeJSON(w, map[string]any{})
}

func (f *fakeDynamo) putItem(w http.ResponseWriter, req fakeRequest) {
	old := f.putLocked(req.Item)
	reply := map[string]any{}
	if req.ReturnValues == "ALL_OLD" && old != nil {
		reply["Attributes"] = old
	}
	writeFakeJSON(w, reply)
}

func (f *fakeDynamo) deleteItem(w http.ResponseWriter, req fakeRequest) {
	old := f.deleteLocked(req.Key)
	reply := map[string]any{}
	if req.ReturnValues == "ALL_OLD" && old != nil {
		reply["Attributes"] = old
	}
	writeFakeJSON(w, reply)
}

// page returns pageSize items at a time, continuing from ExclusiveStartKey.
func (f *fakeDynamo) page(w http.ResponseWriter, req fakeRequest) {
	start := 0
	if len(req.ExclusiveStartKey) > 0 {
		want := itemKey(req.ExclusiveStartKey)
		for i, item := range f.items {
			if itemKey(item) == want {
				start = i + 1
				break
			}
		}
	}
	end := min(start+f.pageSize, len(f.items))
	page := f.items[start:end]
	reply := map[string]any{
		"Items":        page,
		"Count":        len(page),
		"ScannedCount": len(page),
	}
	if end < len(f.items) && len(page) > 0 {
		last := page[len(page)-1]
		reply["LastEvaluatedKey"] = dynamodb.Key{"sensor": last["sensor"], "at": last["at"]}
	}
	writeFakeJSON(w, reply)
}

func (f *fakeDynamo) batchWrite(w http.ResponseWriter, req fakeRequest) {
	var request map[string][]dynamodb.WriteRequest
	if err := json.Unmarshal(req.RequestItems, &request); err != nil {
		writeFakeError(w, http.StatusBadRequest, "ValidationException", err.Error())
		return
	}
	unprocessed := map[string][]dynamodb.WriteRequest{}
	for table, writes := range request {
		for _, write := range writes {
			if f.declineWrites > 0 {
				f.declineWrites--
				unprocessed[table] = append(unprocessed[table], write)
				continue
			}
			switch {
			case write.Put != nil:
				f.putLocked(write.Put)
			case write.Delete != nil:
				f.deleteLocked(write.Delete)
			}
		}
	}
	writeFakeJSON(w, map[string]any{"UnprocessedItems": unprocessed})
}

func (f *fakeDynamo) batchGet(w http.ResponseWriter, req fakeRequest) {
	var request map[string]struct {
		Keys []dynamodb.Key `json:"Keys"`
	}
	if err := json.Unmarshal(req.RequestItems, &request); err != nil {
		writeFakeError(w, http.StatusBadRequest, "ValidationException", err.Error())
		return
	}
	responses := map[string][]dynamodb.Item{}
	for table, tableRequest := range request {
		for _, key := range tableRequest.Keys {
			want := itemKey(key)
			for _, item := range f.items {
				if itemKey(item) == want {
					responses[table] = append(responses[table], item)
					break
				}
			}
		}
	}
	writeFakeJSON(w, map[string]any{"Responses": responses, "UnprocessedKeys": map[string]any{}})
}

func writeFakeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	_ = json.NewEncoder(w).Encode(body)
}

func writeFakeError(w http.ResponseWriter, status int, kind, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"__type": kind, "message": message})
}

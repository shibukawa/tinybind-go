package htmlupdate_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// A fragment's static half derives from the template rather than from a request,
// which makes it the one response in this package that is not per user — and the
// only one a shared cache may hold. A client that says it can walk one gets the
// values instead of the markup, and asks for the tree separately.

func sequenceRequest(t *testing.T, address string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	request.Header.Set("X-Tinybind-Render", "sequence")
	if address != "" {
		request.Header.Set("X-Tinybind-Sequence-Address", address)
	}
	recorder := httptest.NewRecorder()
	if !options.Sequence(recorder, request) {
		t.Fatal("Sequence did not answer the request")
	}
	return recorder.Result()
}

// The two halves reach the client by different routes and reassemble into what
// the render wrote. Without the round trip the split is not a split.
func TestValuesAndFetchedSequenceRebuildTheFragment(t *testing.T) {
	response, body := fetchDeltaWithSequences(t, "/search?q=go&section=Docs")
	if got := response.StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d", got)
	}
	split := 0
	for _, operation := range body.Operations {
		// Values replace markup only where they are smaller: a fragment of two
		// elements is cheaper as markup, because the address is per-operation
		// overhead and there is almost no static text to save. So an operation
		// carries one half or the other, never both.
		if operation.Seq == "" {
			if operation.HTML == "" {
				t.Fatalf("%s carried neither markup nor values", operation.ID)
			}
			continue
		}
		if operation.HTML != "" {
			t.Fatalf("%s carried markup as well as values", operation.ID)
		}
		split++
		tree := fetchSequence(t, operation.Seq)
		rebuilt, err := tree.Reassemble(operation.Values)
		if err != nil {
			t.Fatalf("%s: %v", operation.ID, err)
		}
		if !strings.Contains(rebuilt, `data-tb-id="`+operation.ID+`"`) {
			t.Fatalf("%s reassembled to %q", operation.ID, rebuilt)
		}
	}
	if split == 0 {
		t.Fatal("no fragment travelled as values, so the round trip proved nothing")
	}
}

// Without the header nothing changes, so a client that knows nothing about
// sequences gets exactly the response it got before they existed.
func TestWithoutTheHeaderTheMarkupTravels(t *testing.T) {
	_, body := fetchDelta(t, "/search?q=go&section=Docs", deltaManifest())
	for _, operation := range body.Operations {
		if operation.Seq != "" || len(operation.Values) != 0 {
			t.Fatalf("%s carried values to a client that asked for none", operation.ID)
		}
		if operation.HTML == "" {
			t.Fatalf("%s carried neither markup nor values", operation.ID)
		}
	}
}

// A sequence is content-addressed, so it may be cached forever and shared. It is
// also the one response here that survives a build change, because the address
// digests the tree rather than naming a version.
func TestSequenceIsPublicAndImmutable(t *testing.T) {
	_, body := fetchDeltaWithSequences(t, "/search?q=go&section=Docs")
	response := sequenceRequest(t, body.Operations[0].Seq)
	if got := response.Header.Get("Cache-Control"); got != htmlupdate.DefaultSequenceCacheControl {
		t.Fatalf("Cache-Control = %q", got)
	}
	if !strings.Contains(bodyOf(response), `"addr":"`+body.Operations[0].Seq+`"`) {
		t.Fatal("the tree does not name the address it was asked for")
	}
}

// An address this process has never rendered is answered not-found, and a client
// falls back to asking for markup. A sequence is an optimisation over something
// still available, never a thing a screen depends on.
func TestUnknownSequenceIsNotFound(t *testing.T) {
	if code := sequenceRequest(t, "nosuchaddress").StatusCode; code != http.StatusNotFound {
		t.Fatalf("status = %d", code)
	}
	if code := sequenceRequest(t, "").StatusCode; code != http.StatusBadRequest {
		t.Fatalf("an address-less request = %d", code)
	}
}

func bodyOf(response *http.Response) string {
	raw, _ := io.ReadAll(response.Body)
	return string(raw)
}

func fetchSequence(t *testing.T, address string) *htmlbind.Sequence {
	t.Helper()
	var wire struct {
		Address string          `json:"addr"`
		Nodes   json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(bodyOf(sequenceRequest(t, address))), &wire); err != nil {
		t.Fatalf("sequence body is not JSON: %v", err)
	}
	if wire.Address != address {
		t.Fatalf("served %q for %q", wire.Address, address)
	}
	// The tree a client walks is the one this process holds; going through the
	// registry here keeps the test about the wire rather than about re-parsing.
	tree, ok := htmlbind.LookupSequence(address)
	if !ok {
		t.Fatalf("no sequence under %q", address)
	}
	return tree
}

package htmlupdate_test

import (
	"context"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// toastPayload stands in for a generated encoder.
type toastPayload struct{ Text string }

func (p toastPayload) AppendJSON(dst []byte) []byte {
	dst = append(dst, `{"text":"`...)
	dst = append(dst, p.Text...)
	return append(dst, '"', '}')
}

// signalPlan is livePlan with a source that emits a signal between its two
// deliveries, which is the whole feature end to end: an application yields one
// in the error slot and a client reads a record it can dispatch.
var signalPlan = &htmlbind.Plan[liveParams]{
	Boundary: &htmlbind.Boundary[liveParams]{
		ComponentID: "Feed@v1",
		Attr:        "data-tb-id",
		Input:       func(liveParams) string { return delta.CanonString("feed") },
	},
	HasAwaitBlock: true,
	HasLiveBlock:  true,
	Ops: []htmlbind.Op[liveParams]{
		liveOps.Static("<section"),
		liveOps.BoundaryAttr(),
		liveOps.Static(">"),
		htmlbind.Live(
			func(ctx context.Context, p liveParams) []htmlbind.LiveBinding[string] {
				return []htmlbind.LiveBinding[string]{
					func(deliver func(func(*string), error) bool) error {
						for index, value := range p.Values {
							if index == 1 {
								signal := htmlbind.NewSignal("app.toast", toastPayload{Text: "saved"})
								if !deliver(nil, signal) {
									return nil
								}
							}
							if !deliver(func(scope *string) { *scope = value }, nil) {
								return nil
							}
						}
						return nil
					},
				}
			},
			func(liveParams) string { return "" },
			func(_ liveParams, err htmlbind.AsyncError) htmlbind.AsyncError { return err },
			[]htmlbind.Op[string]{htmlbind.Builder[string]{}.Text(func(value string) string { return value })},
			[]htmlbind.Op[liveParams]{liveOps.Static("pending")},
			nil,
		),
		liveOps.Static("</section>"),
	},
}

func TestLiveStreamCarriesASignalRecord(t *testing.T) {
	_, records := requestMode(t, liveServer(signalPlan, "one", "two"), "live")

	var signals, operations int
	var name string
	var data map[string]any
	for _, item := range records {
		switch item["r"] {
		case "signal":
			signals++
			name, _ = item["name"].(string)
			data, _ = item["data"].(map[string]any)
		case "op", "await":
			operations++
		}
	}
	if signals != 1 {
		t.Fatalf("signal records = %d, want 1:\n%v", signals, records)
	}
	if name != "app.toast" {
		t.Errorf("name = %q, want %q", name, "app.toast")
	}
	if data["text"] != "saved" {
		t.Errorf("data = %v, want the encoded payload", data)
	}
	if operations == 0 {
		t.Errorf("no boundary records:\n%v", records)
	}
}

func TestSignalDoesNotTerminateTheLiveStream(t *testing.T) {
	// The unmigrated loop returns on the first non-nil error, which would end
	// the response with no terminator and make the client read truncation. The
	// terminator being present is what says the classification is in place all
	// the way out.
	_, records := requestMode(t, liveServer(signalPlan, "one", "two"), "live")
	if len(records) == 0 {
		t.Fatal("no records")
	}
	last := records[len(records)-1]
	if last["r"] != "end" {
		t.Fatalf("last record = %v, want the terminator: a stream ending without one reads as truncated", last)
	}
	if last["reason"] != "done" {
		t.Errorf("reason = %v, want %q: every source ended, so the client stops rather than reconnecting", last["reason"], "done")
	}
}

func TestSignalRecordNamesNoBoundary(t *testing.T) {
	// A signal is dispatched, not applied. Carrying an id or a validator would
	// invite a client to treat it as an operation on a region.
	_, records := requestMode(t, liveServer(signalPlan, "one", "two"), "live")
	for _, item := range records {
		if item["r"] != "signal" {
			continue
		}
		for _, field := range []string{"id", "frame", "children", "parent", "html", "kind"} {
			if _, ok := item[field]; ok {
				t.Errorf("signal record carries %q: %v", field, item)
			}
		}
	}
}

func TestPageWithNoSignalIsUnchanged(t *testing.T) {
	_, records := requestMode(t, liveServer(livePlan, "one", "two"), "live")
	for _, item := range records {
		if item["r"] == "signal" {
			t.Errorf("a page emitting no signal produced one: %v", item)
		}
	}
}

package htmlupdate_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// A children operation says a region's own markup is unchanged and its nested
// boundaries are now these, in this order. It is how a list says a row was
// appended without re-sending the list, and it carries no markup at all — which
// is what every stream path got wrong, because each decided what to write by
// looking at the markup instead of at the kind.

type listParams struct {
	ID   string
	Rows []rowParams
}

type rowParams struct {
	ID   string
	Text string
}

var (
	listOps = htmlbind.Builder[listParams]{}
	rowOps  = htmlbind.Builder[rowParams]{}
)

var rowPlan = &htmlbind.Plan[rowParams]{
	Boundary: &htmlbind.Boundary[rowParams]{
		ComponentID: "Row@v1", Attr: "data-tb-id",
		Instance: func(p rowParams) string { return p.ID },
		Input:    func(p rowParams) string { return delta.CanonString(p.Text) },
	},
	Ops: []htmlbind.Op[rowParams]{
		rowOps.Static("<li"), rowOps.BoundaryAttr(), rowOps.Static(">"),
		rowOps.Text(func(p rowParams) string { return p.Text }),
		rowOps.Static("</li>"),
	},
}

var listPlan = &htmlbind.Plan[listParams]{
	Boundary: &htmlbind.Boundary[listParams]{
		ComponentID: "List@v1", Attr: "data-tb-id",
		Instance: func(p listParams) string { return p.ID },
		Input:    func(listParams) string { return "" },
	},
	Ops: []htmlbind.Op[listParams]{
		listOps.Static("<ul"), listOps.BoundaryAttr(), listOps.Static(">"),
		htmlbind.For(
			func(p listParams) []rowParams { return p.Rows },
			func(_ listParams, item rowParams, _ int) rowParams { return item },
			[]htmlbind.Op[rowParams]{
				rowOps.Component(func(p rowParams) htmlbind.Fragment { return htmlbind.Bind(rowPlan, p) }),
			}),
		listOps.Static("</ul>"),
	},
}

func rowsUpTo(n int) []rowParams {
	rows := make([]rowParams, n)
	for i := range rows {
		rows[i] = rowParams{ID: "row-" + strconv.Itoa(i), Text: "line " + strconv.Itoa(i)}
	}
	return rows
}

// streamRecord is one line of the record stream.
type streamRecord struct {
	Record     string   `json:"r"`
	Kind       string   `json:"kind"`
	ID         string   `json:"id"`
	HTML       string   `json:"html"`
	Boundaries []string `json:"boundaries"`
	Seq        string   `json:"seq"`
	Values     []string `json:"values"`
	Frame      string   `json:"frame"`
	Children   string   `json:"children"`
	Parent     string   `json:"parent"`
}

func childRecords(t *testing.T, body string) []streamRecord {
	t.Helper()
	var records []streamRecord
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if line == "" {
			continue
		}
		var record streamRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("record %q is not JSON: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func findRecord(records []streamRecord, id string) (streamRecord, bool) {
	for _, record := range records {
		if record.Record == "op" && record.ID == id {
			return record, true
		}
	}
	return streamRecord{}, false
}

// listRequest renders three rows, then four, and returns the second response's
// records. Appending is the ordinary event on a live list.
//
// Both requests go through the entry under test, and the second returns what the
// first reported — which is what a client does, and the only way the validators
// are the ones this server computed. Building the known manifest another way
// seeds its digests differently and makes everything look changed.
func listRequest(t *testing.T, mode string, serve func(http.ResponseWriter, *http.Request, listParams) error) []streamRecord {
	t.Helper()
	ask := func(known delta.Manifest, rows int) []streamRecord {
		request := httptest.NewRequest(http.MethodGet, "/feed", nil)
		request.Header.Set("X-Tinybind-Render", mode)
		request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
		if len(known.Instances) > 0 {
			request.Header.Set("X-Tinybind-Manifest", htmlupdate.EncodeManifest(known))
		}
		recorder := httptest.NewRecorder()
		if err := serve(recorder, request, listParams{ID: "the-list", Rows: rowsUpTo(rows)}); err != nil {
			t.Fatal(err)
		}
		return childRecords(t, recorder.Body.String())
	}
	var known delta.Manifest
	for _, record := range ask(delta.Manifest{}, 3) {
		if record.Record == "op" {
			known.Instances = append(known.Instances, delta.Instance{
				ID: record.ID, FrameValidator: record.Frame,
				ChildrenValidator: record.Children, ParentID: record.Parent,
			})
		}
	}
	return ask(known, 4)
}

// The reported defect: on the streamed navigation path the list's record arrived
// in the unchanged shape, so nothing said where the new row goes and a client
// could only fall back to a full reload.
func TestStreamedNavigationCarriesTheChildrenOperation(t *testing.T) {
	records := listRequest(t, "navigation", func(w http.ResponseWriter, r *http.Request, p listParams) error {
		leaf := htmlbind.Bind(listPlan, p)
		htmlupdate.ApplyTo(options.StreamHeaders(r, nil, leaf), w)
		return options.RenderStreamAsync(r.Context(), w, r, nil, leaf)
	})
	list, ok := findRecord(records, "the-list")
	if !ok {
		t.Fatalf("the list said nothing: %+v", records)
	}
	if list.Kind != delta.OpChildren {
		t.Fatalf("list record kind = %q, want %q: %+v", list.Kind, delta.OpChildren, list)
	}
	if strings.Join(list.Boundaries, ",") != "row-0,row-1,row-2,row-3" {
		t.Fatalf("boundaries = %v", list.Boundaries)
	}
	if list.HTML != "" {
		t.Fatalf("a children operation carries markup: %q", list.HTML)
	}
	if _, sent := findRecord(records, "row-3"); !sent {
		t.Fatalf("the new row was not sent: %+v", records)
	}
}

// Worse than the reported face: on the live path the record fell into the branch
// that drops an unchanged boundary, so an appended row never appeared at all.
func TestLiveDeliveryCarriesTheChildrenOperation(t *testing.T) {
	records := listRequest(t, "live", func(w http.ResponseWriter, r *http.Request, p listParams) error {
		leaf := htmlbind.Bind(listPlan, p)
		htmlupdate.ApplyTo(options.LiveHeaders(r, nil, leaf), w)
		return options.RenderLiveStream(r.Context(), w, r, nil, leaf)
	})
	list, ok := findRecord(records, "the-list")
	if !ok {
		t.Fatalf("the list said nothing, so the appended row has nowhere to go: %+v", records)
	}
	if list.Kind != delta.OpChildren {
		t.Fatalf("list record kind = %q, want %q", list.Kind, delta.OpChildren)
	}
	if strings.Join(list.Boundaries, ",") != "row-0,row-1,row-2,row-3" {
		t.Fatalf("boundaries = %v", list.Boundaries)
	}
}

// Worse still: the buffered-render streamed-write path wrote every operation as a
// replace, so a children operation became a replace with no markup — which empties
// the region rather than reordering it.
func TestStreamedRenderDoesNotEmptyTheList(t *testing.T) {
	records := listRequest(t, "navigation", func(w http.ResponseWriter, r *http.Request, p listParams) error {
		leaf := htmlbind.Bind(listPlan, p)
		htmlupdate.ApplyTo(options.StreamHeaders(r, nil, leaf), w)
		return options.RenderStream(w, r, nil, leaf)
	})
	list, ok := findRecord(records, "the-list")
	if !ok {
		t.Fatalf("the list said nothing: %+v", records)
	}
	if list.Kind == delta.OpReplace && list.HTML == "" {
		t.Fatalf("the list was replaced with nothing, which empties it: %+v", list)
	}
	if list.Kind != delta.OpChildren {
		t.Fatalf("list record kind = %q, want %q", list.Kind, delta.OpChildren)
	}
}

// A manifest entry has three fields beside its id, and a client rebuilding one
// from a stream must be able to return all three. The children digest arrived
// first; without the parent, a removal cannot be attributed to the boundary that
// would report the survivors, so a shrinking list falls back to replacing the
// outermost boundary — expensive in exactly the case the children operation
// exists to make cheap.
func TestStreamRecordsCarryTheWholeManifestEntry(t *testing.T) {
	records := listRequest(t, "navigation", func(w http.ResponseWriter, r *http.Request, p listParams) error {
		leaf := htmlbind.Bind(listPlan, p)
		htmlupdate.ApplyTo(options.StreamHeaders(r, nil, leaf), w)
		return options.RenderStreamAsync(r.Context(), w, r, nil, leaf)
	})
	row, ok := findRecord(records, "row-3")
	if !ok {
		t.Fatalf("the new row was not sent: %+v", records)
	}
	if row.Parent != "the-list" {
		t.Fatalf("row record names parent %q, want the-list: %+v", row.Parent, row)
	}
	list, _ := findRecord(records, "the-list")
	if list.Children == "" {
		t.Fatalf("the list record carries no children validator: %+v", list)
	}
}

// A shrinking list is what the parent field buys. Rebuilt from a stream, the
// client's manifest attributes the removal to the list, so the response is the
// list's new order rather than a replacement of the outermost boundary.
func TestAShrinkingListStaysAChildrenOperation(t *testing.T) {
	ask := func(known delta.Manifest, rows int) []streamRecord {
		request := httptest.NewRequest(http.MethodGet, "/feed", nil)
		request.Header.Set("X-Tinybind-Render", "navigation")
		request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
		if len(known.Instances) > 0 {
			request.Header.Set("X-Tinybind-Manifest", htmlupdate.EncodeManifest(known))
		}
		recorder := httptest.NewRecorder()
		leaf := htmlbind.Bind(listPlan, listParams{ID: "the-list", Rows: rowsUpTo(rows)})
		htmlupdate.ApplyTo(options.StreamHeaders(request, nil, leaf), recorder)
		err := options.RenderStreamAsync(request.Context(), recorder, request, nil, leaf)
		if err != nil {
			t.Fatal(err)
		}
		return childRecords(t, recorder.Body.String())
	}
	var known delta.Manifest
	for _, record := range ask(delta.Manifest{}, 3) {
		if record.Record == "op" {
			known.Instances = append(known.Instances, delta.Instance{
				ID: record.ID, FrameValidator: record.Frame,
				ChildrenValidator: record.Children, ParentID: record.Parent,
			})
		}
	}
	list, ok := findRecord(ask(known, 2), "the-list")
	if !ok {
		t.Fatal("the list said nothing about the removal")
	}
	if list.Kind != delta.OpChildren {
		t.Fatalf("a removal fell back to %q; the parent field is what keeps it a children operation", list.Kind)
	}
	if strings.Join(list.Boundaries, ",") != "row-0,row-1" {
		t.Fatalf("boundaries = %v", list.Boundaries)
	}
}

// The values-or-markup choice is per fragment, and it was applied on the
// buffered path and not on the streamed one — which is the path every navigation
// goes through. A list row is the shape that inverts: two elements cost more as
// an address plus their values than as the markup itself.
func TestTheStreamedPathAlsoSendsWhicheverIsSmaller(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/feed", nil)
	request.Header.Set("X-Tinybind-Render", "navigation")
	request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
	request.Header.Set("X-Tinybind-Sequences", "1")
	recorder := httptest.NewRecorder()
	leaf := htmlbind.Bind(listPlan, listParams{ID: "the-list", Rows: rowsUpTo(40)})
	htmlupdate.ApplyTo(options.StreamHeaders(request, nil, leaf), recorder)
	if err := options.RenderStreamAsync(request.Context(), recorder, request, nil, leaf); err != nil {
		t.Fatal(err)
	}
	records := childRecords(t, recorder.Body.String())

	row, ok := findRecord(records, "row-0")
	if !ok {
		t.Fatalf("the row was not sent: %+v", records)
	}
	if row.Seq != "" {
		values := len(row.Seq)
		for _, value := range row.Values {
			values += len(value)
		}
		t.Fatalf("a %d-byte row travelled as %d bytes of address and values", len(row.HTML), values)
	}
	// The list is the opposite shape — forty hole frames, almost all static — so
	// the same rule has to send it the other way, or the test would pass with the
	// choice hard-wired to markup.
	list, ok := findRecord(records, "the-list")
	if !ok {
		t.Fatalf("the list was not sent: %+v", records)
	}
	if list.Seq == "" {
		t.Fatalf("a list of forty holes travelled as markup: %q", list.HTML)
	}
}

package httpbind

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// foreignResult stands for a result type declared in a package this build never
// analyzed: it carries its own encoder and registers no writer. Before the
// interface path, Write answered a missing-writer error for exactly this shape,
// which is the failure a generated typed-action wrapper would have hit on its
// first call.
type foreignResult struct{ ID int }

func (f foreignResult) AppendJSONTo(dst []byte) []byte {
	dst = append(dst, `{"id":`...)
	dst = jsonbind.AppendInt(dst, int64(f.ID))
	return append(dst, '}')
}

type unwritableResult struct{ ID int }

func TestWriteReachesATypeCarryingItsOwnEncoder(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := Write(w, r, foreignResult{ID: 3}); err != nil {
		t.Fatal(err)
	}
	// WriteJSONBytes terminates the document, as it does for every generated
	// writer, so the interface path produces the same framing.
	if got := w.Body.String(); got != "{\"id\":3}\n" {
		t.Fatalf("body %q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type %q", got)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestWriteStatusReachesATypeCarryingItsOwnEncoder(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := WriteStatus(w, r, http.StatusCreated, foreignResult{ID: 4}); err != nil {
		t.Fatal(err)
	}
	if got := w.Body.String(); got != `{"id":4}` {
		t.Fatalf("body %s", got)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d", w.Code)
	}
}

// A type carrying neither a registration nor a method keeps the error it had.
func TestWriteStillFailsForATypeCarryingNothing(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := Write(w, r, unwritableResult{ID: 5}); err == nil {
		t.Fatal("want a missing-writer error")
	}
}

// A registered writer still serves a type carrying no method, so an existing
// project's response path is unchanged.
func TestWriteStillUsesARegisteredWriter(t *testing.T) {
	RegisterWrite[unwritableResult](func(w http.ResponseWriter, _ *http.Request, v unwritableResult) error {
		return WriteJSONBytes(w, http.StatusOK, []byte(`"registered"`))
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := Write(w, r, unwritableResult{ID: 6}); err != nil {
		t.Fatal(err)
	}
	if got := w.Body.String(); got != "\"registered\"\n" {
		t.Fatalf("body %q", got)
	}
}

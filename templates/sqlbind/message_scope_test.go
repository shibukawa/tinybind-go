package sqlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// TestMessageReferenceIsHTMLOnly states the scope in the one place it can be
// enforced. The shared parser recognizes `{t id}` in every format because the
// recognizer lives in the body grammar, so each format that does not resolve
// one has to refuse it by name rather than by Go type.
//
// See .knowledge concept:template-message-surface scope.
func TestMessageReferenceIsHTMLOnly(t *testing.T) {
	source := "record R { id: int }\n\nexport statement S(id: int): sql.one<R> {\nSELECT id FROM t WHERE id = {t title}\n}\n"
	_, err := sqlbind.Generate("q.tb.sql", []byte(source), sqlbind.GenerateOptions{})
	if err == nil {
		t.Fatal("a message reference generated in a SQL template")
	}
	if !strings.Contains(err.Error(), "only available in HTML templates") {
		t.Fatalf("error = %v, want the scope stated", err)
	}
	if !strings.Contains(err.Error(), "{t title}") {
		t.Fatalf("error = %v, want it to name the reference", err)
	}
	// The position must be the reference, not the top of the file.
	if strings.Contains(err.Error(), ":1:1:") {
		t.Fatalf("error = %v, want the reference's own position", err)
	}
}

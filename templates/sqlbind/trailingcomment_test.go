package sqlbind_test

import (
	"testing"
)

// Formatting must preserve blank lines between trailing comments and between
// leading comments, including when line comments precede the blank.
func TestFormatKeepsTrailingCommentSpacing(t *testing.T) {
	t.Parallel()
	head := "package queries\n\nexport statement F(id: int): sql.one<E> {\n  SELECT id\n  FROM e\n  WHERE id = {id}\n}\n\n"
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"adjacent lines stay adjacent", head + "// tail one\n// tail two\n"},
		{"a deliberate blank survives", head + "// group one\n\n// group two\n"},
		{"block comments keep their blank", head + "/* one */\n\n/* two */\n"},
		{"a leading blank run is unchanged", "package queries\n\n// group one\n\n// group two\nexport statement F(id: int): sql.one<E> {\n  SELECT id\n  FROM e\n  WHERE id = {id}\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSource(t, tc.source)
			if got != tc.source {
				t.Errorf("format is not a fixed point\n got: %q\nwant: %q", got, tc.source)
			}
			if again := formatSource(t, got); again != got {
				t.Errorf("not idempotent\nfirst: %q\nsecond: %q", got, again)
			}
		})
	}
}

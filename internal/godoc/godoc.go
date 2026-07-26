// Package godoc extracts documentation text from Go doc comments so host-side
// analysis can carry it into generated artifacts such as OpenAPI descriptions.
//
// Doc text is kept verbatim (comment markers removed): the Go source is the
// single source of truth, so no rewording or prefix trimming happens here.
package godoc

import (
	"go/ast"
	"strings"
)

// deprecatedMarker is the godoc convention for deprecation notices.
const deprecatedMarker = "Deprecated:"

// Text returns the plain text of the first non-empty comment group.
// Nil groups are skipped, so callers can pass fallbacks in priority order
// (for example a TypeSpec doc followed by its GenDecl doc).
func Text(groups ...*ast.CommentGroup) string {
	for _, group := range groups {
		if group == nil {
			continue
		}
		if text := strings.TrimSpace(group.Text()); text != "" {
			return text
		}
	}
	return ""
}

// Split separates doc into its first sentence and the remaining text. The
// first sentence maps to an OpenAPI summary, the remainder to a description.
// Either result may be empty.
func Split(doc string) (summary, description string) {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return "", ""
	}
	cut := sentenceEnd(doc)
	return strings.TrimSpace(doc[:cut]), strings.TrimSpace(doc[cut:])
}

// Deprecated reports whether doc carries a godoc "Deprecated:" paragraph.
func Deprecated(doc string) bool {
	for _, paragraph := range strings.Split(doc, "\n\n") {
		if strings.HasPrefix(strings.TrimSpace(paragraph), deprecatedMarker) {
			return true
		}
	}
	return false
}

// sentenceEnd returns the index just past the first sentence of doc. A sentence
// ends at a period followed by whitespace or end of text; a paragraph break
// ends it too, so single-line docs without punctuation still work.
func sentenceEnd(doc string) int {
	for i := 0; i < len(doc); i++ {
		switch doc[i] {
		case '.':
			if i+1 == len(doc) {
				return len(doc)
			}
			if next := doc[i+1]; next == ' ' || next == '\t' || next == '\n' {
				return i + 1
			}
		case '\n':
			if i+1 < len(doc) && doc[i+1] == '\n' {
				return i
			}
		}
	}
	return len(doc)
}

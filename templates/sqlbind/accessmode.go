package sqlbind

import "strings"

// Access mode derivation per rule:sql-statement-access-mode. A statement is
// read-only only when its top-level verb provably reads. Every other outcome,
// including a body this scanner cannot resolve, is a write, because
// misclassification then costs at most a reader connection instead of sending a
// write to a read-only executor.

// readVerbs open a statement that only reads.
var readVerbs = map[string]bool{"SELECT": true, "VALUES": true, "TABLE": true}

// isReadOnly reports whether the statement body only reads.
func isReadOnly(nodes []Node) bool {
	tokens, ok := statementTokens(nodes)
	if !ok {
		return false
	}
	if hasTopLevelLock(tokens) {
		return false
	}
	if tokens[0].text == "WITH" {
		cteVerbs, tail, ok := splitWith(tokens)
		if !ok {
			return false
		}
		for _, verb := range cteVerbs {
			if mutatingVerbs[verb] {
				return false
			}
		}
		return readVerbs[tokens[tail].text]
	}
	return readVerbs[tokens[0].text]
}

// topLevelVerb returns the verb that opens the statement, resolving a WITH
// statement to the verb of its tail. It is empty when the verb cannot be
// determined, including when the leading text is conditional.
func topLevelVerb(nodes []Node) string {
	tokens, ok := statementTokens(nodes)
	if !ok {
		return ""
	}
	if tokens[0].text != "WITH" {
		return tokens[0].text
	}
	_, tail, ok := splitWith(tokens)
	if !ok {
		return ""
	}
	return tokens[tail].text
}

// statementTokens tokenizes a statement body for whole-statement inspection.
// ok is false when the body cannot be resolved, which includes a body whose
// first content is conditional: the leading verb would then depend on a runtime
// branch, so no single verb describes the statement.
func statementTokens(nodes []Node) ([]sqlToken, bool) {
	if !startsWithText(nodes) {
		return nil, false
	}
	tokens, ok := scanSQLTokens(accessSQL(nodes))
	if !ok || len(tokens) == 0 || !tokens[0].word {
		return nil, false
	}
	return tokens, true
}

// startsWithText reports whether the first content of the body is literal text.
func startsWithText(nodes []Node) bool {
	for _, node := range nodes {
		text, ok := node.(*TextNode)
		if !ok {
			return false
		}
		if strings.TrimSpace(text.Text) != "" {
			return true
		}
	}
	return false
}

// accessSQL joins the literal text of every branch with a space. Both branches
// of an if are included: a locking clause emitted on any path makes the
// statement a write, and the separator keeps adjacent fragments from fusing
// into one token.
func accessSQL(nodes []Node) string {
	var parts []string
	var walk func([]Node)
	walk = func(items []Node) {
		for _, node := range items {
			switch n := node.(type) {
			case *TextNode:
				parts = append(parts, n.Text)
			case *IfNode:
				walk(n.Then)
				walk(n.Else)
			}
		}
	}
	walk(nodes)
	return strings.Join(parts, " ")
}

// hasTopLevelLock reports a row-locking clause at the statement's own nesting
// level. FOR inside a subquery belongs to that subquery, not to this statement.
func hasTopLevelLock(tokens []sqlToken) bool {
	for i, token := range tokens {
		if !token.word || token.depth != 0 || token.text != "FOR" {
			continue
		}
		if i+1 >= len(tokens) || !tokens[i+1].word {
			continue
		}
		switch tokens[i+1].text {
		case "UPDATE", "SHARE", "NO", "KEY":
			return true
		}
	}
	return false
}

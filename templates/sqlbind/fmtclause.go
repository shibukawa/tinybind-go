package sqlbind

// Clause recognition for rule:sql-template-layout. A keyword opens a line only
// at the nesting level of the statement being laid out, which is the same test
// rule:sql-top-level-keyword-scan already applies; a subquery's WHERE therefore
// indents under its own SELECT instead of aligning with the outer one.

type clauseKind struct {
	// comma marks a clause whose items are separated by commas.
	comma bool
	// boolean marks a clause whose items are separated by AND and OR.
	boolean bool
	// indented marks a clause that continues the one above it, which is what an
	// ON does under its JOIN.
	indented bool
}

// joinLeadWords may precede JOIN in a join clause opener.
var joinLeadWords = map[string]bool{
	"INNER": true, "LEFT": true, "RIGHT": true, "FULL": true,
	"CROSS": true, "NATURAL": true, "OUTER": true, "LATERAL": true,
}

// matchClauseHead reports the length of the clause opener starting at i.
func matchClauseHead(els []element, i, depth int) (int, clauseKind, bool) {
	if i >= len(els) || !els[i].isAtom() || els[i].atom.kind != atomWord || els[i].atom.depth != depth {
		return 0, clauseKind{}, false
	}
	word := func(offset int) string {
		at := i + offset
		if at >= len(els) || !els[at].isAtom() || els[at].atom.depth != depth {
			return ""
		}
		return els[at].word()
	}
	switch first := els[i].word(); first {
	case "WITH":
		if word(1) == "RECURSIVE" {
			return 2, clauseKind{comma: true}, true
		}
		return 1, clauseKind{comma: true}, true
	case "SELECT":
		switch word(1) {
		case "DISTINCT", "ALL":
			return 2, clauseKind{comma: true}, true
		}
		return 1, clauseKind{comma: true}, true
	case "FROM", "SET", "RETURNING", "VALUES", "WINDOW", "USING":
		return 1, clauseKind{comma: true}, true
	case "WHERE", "HAVING", "QUALIFY":
		return 1, clauseKind{boolean: true}, true
	case "GROUP", "ORDER", "PARTITION":
		if word(1) == "BY" {
			return 2, clauseKind{comma: true}, true
		}
		return 0, clauseKind{}, false
	case "LIMIT", "OFFSET", "FETCH":
		return 1, clauseKind{}, true
	case "INSERT":
		if word(1) == "INTO" {
			return 2, clauseKind{}, true
		}
		return 1, clauseKind{}, true
	case "DELETE":
		if word(1) == "FROM" {
			return 2, clauseKind{}, true
		}
		return 1, clauseKind{}, true
	case "UPDATE", "MERGE", "TRUNCATE":
		return 1, clauseKind{}, true
	case "UNION", "INTERSECT", "EXCEPT":
		switch word(1) {
		case "ALL", "DISTINCT":
			return 2, clauseKind{}, true
		}
		return 1, clauseKind{}, true
	case "ON":
		// ON CONFLICT opens its own clause; a bare ON is the condition of the
		// join above it, so it indents rather than aligning.
		if word(1) == "CONFLICT" {
			return 2, clauseKind{}, true
		}
		return 1, clauseKind{boolean: true, indented: true}, true
	case "FOR":
		switch word(1) {
		case "UPDATE", "SHARE", "KEY", "NO":
			return 2, clauseKind{}, true
		}
		return 0, clauseKind{}, false
	case "JOIN":
		return 1, clauseKind{}, true
	default:
		if !joinLeadWords[first] {
			return 0, clauseKind{}, false
		}
		// A join opener is a run of lead words ending in JOIN. Anything else is
		// an ordinary identifier that happens to share the spelling.
		for n := 1; n < 4; n++ {
			switch word(n) {
			case "JOIN":
				return n + 1, clauseKind{}, true
			case "":
				return 0, clauseKind{}, false
			default:
				if !joinLeadWords[word(n)] {
					return 0, clauseKind{}, false
				}
			}
		}
		return 0, clauseKind{}, false
	}
}

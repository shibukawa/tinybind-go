package sqlbind

import (
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// INSERT column and value agreement, the check rule:sql-predicate-group-elision
// left open when comma groups landed. An INSERT column list and its VALUES tuple
// are two independent comma groups, so guarding a column with one condition and
// its value with another renders a column count that disagrees with its value
// count. The counts are a property of the template on every branch path, so the
// disagreement is decided here rather than by the database.

// insertPathState is one branch path's progress through the two lists. delta is
// the columns counted so far minus the values counted so far, and pending marks an
// item that has content but has not yet been closed by a comma or by the list's
// own parenthesis.
//
// decided is what makes the check usable: the same condition guarding a column and
// its value is one runtime question, not two. A path records which way each
// condition it has passed was resolved, so re-entering that condition follows the
// branch the path already committed to. Without it the union of two independent
// merges reports the recommended shape as a disagreement.
type insertPathState struct {
	delta   int
	pending bool
	// decided is an immutable, order-independent set of condition-to-branch
	// commitments, spelled as a string so the state stays a comparable map key.
	decided string
}

// conditionKey identifies a condition across the two lists. syntax.ExprString
// reinserts parentheses from precedence rather than remembering them, so two
// spellings of one condition render identically.
func conditionKey(e Expr) string { return syntax.ExprString(e) }

// decide returns the state with cond committed to taken, or reports that the path
// already committed to the other branch and cannot enter this one.
func (s insertPathState) decide(cond string, taken bool) (insertPathState, bool) {
	entry := "\x00" + cond + "\x01"
	if taken {
		entry += "t"
	} else {
		entry += "f"
	}
	other := "\x00" + cond + "\x01"
	if taken {
		other += "f"
	} else {
		other += "t"
	}
	if strings.Contains(s.decided, other) {
		return s, false
	}
	if strings.Contains(s.decided, entry) {
		return s, true
	}
	s.decided += entry
	return s, true
}

// insertPaths is the set of states reachable on some branch path. Every path ends
// in exactly one state, so requiring delta zero over the set requires it over the
// paths.
type insertPaths map[insertPathState]bool

// insertPathLimit bounds the state set. Independent conditions multiply paths, and
// a template past this many distinct states is not one this check can describe, so
// it is left unchecked rather than reported on partial information.
const insertPathLimit = 4096

// insertList says which of the two lists the walk is inside, which is the sign the
// items it counts carry.
type insertList int

const (
	insertOutside insertList = iota
	insertColumnList
	insertValueList
)

// checkInsertItemAgreement rejects an INSERT whose column count and value count can
// disagree on some branch path.
func (c *compiler) checkInsertItemAgreement(d *TemplateDecl, body []Node) error {
	if topLevelVerb(body) != "INSERT" {
		return nil
	}
	w := &insertWalker{c: c}
	paths := insertPaths{insertPathState{}: true}
	paths, _, ok := w.walk(body, paths, insertWalkState{})
	if !ok || !w.sawBoth {
		// An unresolvable body, a second VALUES tuple, or a form with only one of
		// the two lists. Nothing here is a disagreement this check can name.
		return nil
	}
	for state := range paths {
		if state.delta != 0 {
			return c.error(d.Pos, "INSERT column count and value count disagree on some branch; "+
				"guard a column and its value with the same condition")
		}
	}
	return nil
}

// insertWalkState is the scanning position, shared by every path because the list
// structure does not depend on a branch.
type insertWalkState struct {
	depth int
	list  insertList
	// itemDepth is the parenthesis depth of the open list's own items.
	itemDepth int
	// afterInto marks that INSERT INTO has passed, so the next parenthesis at that
	// depth is the column list.
	afterInto bool
	// afterValues marks that VALUES has passed, so the next parenthesis is a tuple.
	afterValues bool
}

type insertWalker struct {
	c *compiler
	// sawBoth records that a column list and a value list were both found and both
	// closed, which is the only shape the counts can be compared in.
	sawColumns bool
	sawValues  bool
	sawBoth    bool
}

// walk folds the node tree into the set of states its paths reach. ok is false for
// a body this check cannot describe.
func (w *insertWalker) walk(nodes []Node, paths insertPaths, state insertWalkState) (insertPaths, insertWalkState, bool) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			var ok bool
			paths, state, ok = w.text(n, paths, state)
			if !ok {
				return paths, state, false
			}
		case *ExpressionNode:
			if body, found := w.c.predicateBody(n); found && !w.c.alwaysEmits(body) {
				// A fragment that may emit nothing makes the item's content a
				// runtime question this walk cannot answer.
				return paths, state, false
			}
			paths = markContent(paths, state)
		case *RelationNode:
			paths = markContent(paths, state)
		case *IfNode:
			cond := conditionKey(n.Condition)
			// Only the paths that have not already ruled this condition out enter
			// each branch, which is what keeps one condition guarding both lists a
			// single question rather than two.
			takePaths, skipPaths := insertPaths{}, insertPaths{}
			for s := range paths {
				if next, ok := s.decide(cond, true); ok {
					takePaths[next] = true
				}
				if next, ok := s.decide(cond, false); ok {
					skipPaths[next] = true
				}
			}
			thenPaths, thenState, ok := w.walk(n.Then, takePaths, state)
			if !ok {
				return paths, state, false
			}
			elsePaths, elseState, ok := w.walk(n.Else, skipPaths, state)
			if !ok {
				return paths, state, false
			}
			// A branch that leaves the walk in a different list, or at a different
			// nesting, is already refused by the group planner; here it only means
			// the counts cannot be compared.
			if thenState != elseState {
				return paths, state, false
			}
			merged := insertPaths{}
			for s := range thenPaths {
				merged[s] = true
			}
			for s := range elsePaths {
				merged[s] = true
			}
			if len(merged) > insertPathLimit {
				return paths, state, false
			}
			paths, state = merged, thenState
		case *ValNode, *CheckNode:
			// Neither writes SQL, so neither is an item nor part of one.
		default:
			return paths, state, false
		}
	}
	return paths, state, true
}

// text advances every path across one text node.
func (w *insertWalker) text(n *TextNode, paths insertPaths, state insertWalkState) (insertPaths, insertWalkState, bool) {
	// literals: an item whose whole content is a literal — VALUES (…, 'bid', …) —
	// would otherwise scan as an item with nothing in it and go uncounted, which
	// reports a matched INSERT as a disagreement.
	lexer := sqlLexer{depth: state.depth, literals: true}
	tokens, ok := lexer.scan(n.Text)
	if !ok {
		return paths, state, false
	}
	for _, token := range tokens {
		switch {
		case token.text == "(":
			switch {
			case state.list == insertOutside && state.afterInto && !w.sawColumns:
				state.list, state.itemDepth = insertColumnList, token.depth+1
				state.afterInto = false
				w.sawColumns = true
			case state.list == insertOutside && state.afterValues:
				if w.sawValues {
					// A second tuple. requirement:sql-template-v1 has no bulk
					// insert, and summing tuples is not what this check means.
					return paths, state, false
				}
				state.list, state.itemDepth = insertValueList, token.depth+1
				state.afterValues = false
				w.sawValues = true
			case state.list != insertOutside:
				// A parenthesis inside an item, such as a function call, so its
				// content belongs to the item that holds it.
				paths = markContent(paths, state)
			}
		case token.text == ")":
			if state.list != insertOutside && token.depth+1 == state.itemDepth {
				paths = closeItem(paths, state)
				if state.list == insertValueList && w.sawColumns {
					w.sawBoth = true
				}
				state.list, state.itemDepth = insertOutside, 0
			} else if state.list != insertOutside {
				paths = markContent(paths, state)
			}
		case token.text == ",":
			if state.list != insertOutside && token.depth == state.itemDepth {
				paths = closeItem(paths, state)
			} else if state.list != insertOutside {
				paths = markContent(paths, state)
			}
		case token.word && state.list == insertOutside:
			switch token.text {
			case "INTO":
				state.afterInto = true
			case "VALUES":
				state.afterValues = true
			}
		default:
			if state.list != insertOutside {
				paths = markContent(paths, state)
			}
		}
	}
	state.depth = lexer.depth
	return paths, state, true
}

// markContent records that the current item of the open list has content.
func markContent(paths insertPaths, state insertWalkState) insertPaths {
	if state.list == insertOutside {
		return paths
	}
	out := insertPaths{}
	for s := range paths {
		s.pending = true
		out[s] = true
	}
	return out
}

// closeItem counts a pending item into the open list's side of the difference.
func closeItem(paths insertPaths, state insertWalkState) insertPaths {
	sign := 0
	switch state.list {
	case insertColumnList:
		sign = 1
	case insertValueList:
		sign = -1
	default:
		return paths
	}
	out := insertPaths{}
	for s := range paths {
		if s.pending {
			s.delta += sign
			s.pending = false
		}
		out[s] = true
	}
	return out
}

func clonePaths(paths insertPaths) insertPaths {
	out := insertPaths{}
	for s := range paths {
		out[s] = true
	}
	return out
}

package sqlbind

// Static mutation safety per rule:sql-static-mutation-safety. Whether a clause
// can come out empty is a property of the template, not of runtime data, so the
// proof runs here and no guard is emitted into generated code.

// clauseState is one branch path's progress through a top-level clause.
type clauseState uint8

const (
	clauseAbsent      clauseState = 1 << iota // the keyword has not appeared
	clauseEmpty                               // the keyword appeared, nothing followed yet
	clauseFilled                              // the keyword appeared and content followed
	clauseClosedEmpty                         // a later clause began while this one was still empty
	clauseClosedFilled
)

// clauseStates is the set of states reachable on some branch path. Every path
// ends in exactly one state, so a universal property over the set is a
// universal property over the paths.
type clauseStates uint8

// whereTerminators are the top-level keywords that end a WHERE clause. Without
// them a predicate emitted only inside an if would look proven by whatever
// followed the clause.
var whereTerminators = map[string]bool{
	"RETURNING": true, "ORDER": true, "LIMIT": true, "OFFSET": true, "FETCH": true,
	"GROUP": true, "HAVING": true, "WINDOW": true, "FOR": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
}

// setTerminators are the top-level keywords that end an UPDATE SET list.
var setTerminators = map[string]bool{"FROM": true, "WHERE": true, "RETURNING": true}

// checkMutationSafety rejects an UPDATE or DELETE whose WHERE clause, or whose
// SET list, can come out empty on some branch path.
func (c *compiler) checkMutationSafety(d *TemplateDecl, body []Node) error {
	info := c.statements[d.Name]
	if info.cardinality == "predicate" || info.cardinality == "relation" {
		return nil
	}
	verb := topLevelVerb(body)
	if verb != "UPDATE" && verb != "DELETE" {
		return nil
	}
	if verb == "UPDATE" && !c.proveClause(body, "SET", setTerminators, info.plan) {
		return c.error(d.Pos, "UPDATE statements require a SET list that is non-empty on every branch")
	}
	if !c.proveClause(body, "WHERE", whereTerminators, info.plan) {
		return c.error(d.Pos, "UPDATE and DELETE statements require a WHERE clause that is non-empty on every branch")
	}
	return nil
}

// proveClause reports whether every branch path opens the named top-level clause
// and puts at least one token or bound value in it. plan may be nil, which is a
// body with nothing elidable in it and so nothing the builder can withhold.
func (c *compiler) proveClause(nodes []Node, keyword string, terminators map[string]bool, plan *groupPlan) bool {
	states, _, ok := c.walkClause(nodes, keyword, terminators, clauseStates(clauseAbsent), 0, plan)
	if !ok {
		return false
	}
	return states&^clauseStates(clauseFilled|clauseClosedFilled) == 0
}

// walkClause folds the node tree into the set of clause states its paths reach.
// The two branches of an if both start from the incoming set and their results
// are merged, so a predicate emitted on one branch only is never proof.
func (c *compiler) walkClause(nodes []Node, keyword string, terminators map[string]bool, in clauseStates, depth int, plan *groupPlan) (clauseStates, int, bool) {
	states := in
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			lexer := sqlLexer{depth: depth}
			tokens, ok := lexer.scan(n.Text)
			if !ok {
				return states, depth, false
			}
			var withheld map[int]bool
			if plan != nil {
				withheld = plan.withheld[n]
			}
			for _, token := range tokens {
				// A joiner, a grouping parenthesis, and a closer are text the
				// builder may withhold, so none of them fills the clause. The
				// clause keyword itself is not in the set: it still opens.
				if withheld[token.start] {
					continue
				}
				states = applyToken(states, token, keyword, terminators)
			}
			depth = lexer.depth
		case *ExpressionNode:
			// A sql.predicate call is proof only when that predicate is itself
			// non-empty on every path; otherwise it may contribute nothing.
			if body, ok := c.predicateBody(n); ok && !c.alwaysEmits(body) {
				states |= applyContent(states)
				continue
			}
			states = applyContent(states)
		case *RelationNode:
			states = applyContent(states)
		case *IfNode:
			thenStates, thenDepth, ok := c.walkClause(n.Then, keyword, terminators, states, depth, plan)
			if !ok {
				return states, depth, false
			}
			elseStates, _, ok := c.walkClause(n.Else, keyword, terminators, states, depth, plan)
			if !ok {
				return states, depth, false
			}
			// Branches that close a different number of groups are not valid
			// SQL on both paths; following the then branch keeps the scan
			// deterministic.
			states, depth = thenStates|elseStates, thenDepth
		}
	}
	return states, depth, true
}

// applyToken advances every state in the set by one token.
func applyToken(states clauseStates, token sqlToken, keyword string, terminators map[string]bool) clauseStates {
	if token.depth != 0 || !token.word {
		return applyContent(states)
	}
	if token.text == keyword {
		// Every path in the set has now opened the clause.
		return clauseStates(clauseEmpty)
	}
	if terminators[token.text] {
		return closeStates(states)
	}
	return applyContent(states)
}

// applyContent records that something was emitted. Content before the clause
// opens says nothing about the clause.
func applyContent(states clauseStates) clauseStates {
	if states&clauseStates(clauseEmpty) != 0 {
		states = states&^clauseStates(clauseEmpty) | clauseStates(clauseFilled)
	}
	return states
}

// closeStates records that a later top-level clause began, freezing whatever
// this clause had collected.
func closeStates(states clauseStates) clauseStates {
	if states&clauseStates(clauseEmpty) != 0 {
		states = states&^clauseStates(clauseEmpty) | clauseStates(clauseClosedEmpty)
	}
	if states&clauseStates(clauseFilled) != 0 {
		states = states&^clauseStates(clauseFilled) | clauseStates(clauseClosedFilled)
	}
	return states
}

// alwaysEmits reports whether every branch path of nodes emits at least one SQL
// token or bound value.
func (c *compiler) alwaysEmits(nodes []Node) bool {
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			if tokens, ok := scanSQLTokens(n.Text); ok && len(tokens) > 0 {
				return true
			}
		case *ExpressionNode:
			if body, ok := c.predicateBody(n); ok {
				if c.alwaysEmits(body) {
					return true
				}
				continue
			}
			return true
		case *RelationNode:
			return true
		case *IfNode:
			if len(n.Else) > 0 && c.alwaysEmits(n.Then) && c.alwaysEmits(n.Else) {
				return true
			}
		}
	}
	return false
}

// predicateBody returns the body of the sql.predicate this expression calls.
func (c *compiler) predicateBody(node *ExpressionNode) ([]Node, bool) {
	if t, ok := c.exprTypes[node.Expression]; !ok || t.kind != kindPredicate {
		return nil, false
	}
	call, ok := node.Expression.(*CallExpr)
	if !ok {
		return nil, false
	}
	callee, ok := call.Callee.(*IdentifierExpr)
	if !ok {
		return nil, false
	}
	info, ok := c.statements[callee.Name]
	if !ok {
		return nil, false
	}
	body, ok := info.decl.Body.([]Node)
	if !ok {
		return nil, false
	}
	return body, true
}

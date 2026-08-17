package sqlbind

// Predicate group planning per rule:sql-predicate-group-elision. A clause
// keyword, a grouping parenthesis, and a joiner are written the moment a fragment
// inside them actually emits, so a group that stays empty was never opened and
// nothing has to be taken back.
//
// The plan is computed once and read twice: the emitter turns each chunk into a
// Builder call, and rule:sql-static-mutation-safety reads the kinds so withheld
// text counts as filling nothing.

// chunkKind says which Builder call a slice of a text node becomes.
type chunkKind uint8

const (
	// chunkText is ordinary SQL, written as it stands.
	chunkText chunkKind = iota
	// chunkClauseOpen is a boolean clause keyword that opens a group.
	chunkClauseOpen
	// chunkParenOpen is a grouping parenthesis that opens a group.
	chunkParenOpen
	// chunkJoiner is an AND or OR between two items of one group.
	chunkJoiner
	// chunkClose ends the innermost group. Its text is the closing parenthesis,
	// or empty for a clause group, which ends where the next clause begins.
	chunkClose
)

// sqlChunk is one slice of a text node together with the call it becomes. The
// slices of a text node partition it exactly, so no byte is lost or duplicated.
type sqlChunk struct {
	text string
	kind chunkKind
	// fills reports that the text writes something other than whitespace, which
	// is what requires an Item call before it. A whitespace-only run is not an
	// item: the scanner does not tokenize it and alwaysEmits does not count it.
	fills bool
}

// groupPlan is the result of planning one statement body. A nil plan means the
// body has nothing elidable in it and keeps the emission it has always had.
type groupPlan struct {
	// chunks slices each text node into the calls it becomes.
	chunks map[*TextNode][]sqlChunk
	// withheld indexes, by text node and token offset, the tokens whose text the
	// builder may withhold. rule:sql-static-mutation-safety counts them as
	// filling nothing; a clause opener is absent because it still opens.
	withheld map[*TextNode]map[int]bool
	// selfFilling names the predicate calls that fill the caller's group
	// themselves. A predicate that may emit nothing must not be preceded by an
	// Item call, or an empty one would open the group it was meant to leave shut.
	selfFilling map[*ExpressionNode]bool
	// branchCloses counts the groups each branch of an if opened and must close
	// at its own end, so a clause opened inside a condition does not leak past it.
	branchCloses map[*IfNode][2]int
	// trailingCloses counts the groups still open where the body ends, which a
	// clause running to the last token leaves behind.
	trailingCloses int
}

// booleanClauseOpeners are the clauses whose items AND and OR separate. A bare ON
// is one: fmtclause.go already classifies it boolean, so the opener needed here is
// already computed. ON CONFLICT is not, and its second word excludes it.
var booleanClauseOpeners = map[string]bool{
	"WHERE": true, "HAVING": true, "QUALIFY": true, "ON": true,
}

// clauseBoundaryWords end a boolean clause group where they appear at its own
// depth. They are the line-starting keywords of rule:sql-template-layout, which is
// the same boundary the layout pass already draws.
var clauseBoundaryWords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "GROUP": true, "HAVING": true,
	"WINDOW": true, "ORDER": true, "LIMIT": true, "OFFSET": true, "FETCH": true,
	"FOR": true, "RETURNING": true, "UNION": true, "INTERSECT": true,
	"EXCEPT": true, "QUALIFY": true, "SET": true, "VALUES": true, "USING": true,
	"ON": true, "JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true,
	"FULL": true, "CROSS": true, "NATURAL": true, "OUTER": true, "LATERAL": true,
	"INSERT": true, "UPDATE": true, "DELETE": true, "MERGE": true, "WITH": true,
	"INTO": true, "CONFLICT": true, "DO": true,
}

// groupingParenPredecessors are the tokens after which a parenthesis groups a
// boolean expression rather than opening data. rule:sql-template-layout already
// draws this line for layout: a value list, function argument list, or IN list is
// data, not a statement. Any other word makes it a call or list paren, which is
// what keeps "IN (...)" and "x = (SELECT ...)" from opening a group.
var groupingParenPredecessors = map[string]bool{
	"WHERE": true, "HAVING": true, "QUALIFY": true, "ON": true,
	"AND": true, "OR": true, "NOT": true, "(": true,
}

// valuePrev stands in for a bound value or a fragment where a token would be. It
// is not a grouping-paren predecessor, so a parenthesis after a value opens data.
const valuePrev = "\x00value"

// groupFrame is one open group during planning.
type groupFrame struct {
	// itemDepth is the parenthesis depth this group's own items sit at. A clause
	// group's items share the keyword's depth; a parenthesis group's items sit one
	// level in. An AND or OR is this group's joiner only at this depth.
	itemDepth int
	paren     bool
	// openedHere marks a frame pushed inside the branch body being walked, so it
	// closes at that branch's end.
	openedHere bool
}

// groupState is the scanning position. It crosses text nodes, and each branch of
// an if gets a copy so both paths start from the same place.
type groupState struct {
	depth int
	stack []groupFrame
	// prev is the last token scanned, which is what tells a grouping parenthesis
	// from a call paren.
	prev    string
	hasPrev bool
	// between are the depths holding an unclosed BETWEEN, whose closing AND is a
	// fixed part of a two-operand form rather than a joiner.
	between []int
	// caseDepth counts open CASE expressions. CASE opens a boolean region that is
	// neither a clause nor a parenthesis, so it has no frame to attach to and its
	// AND and OR stay ordinary text.
	caseDepth int
}

func (s groupState) clone() groupState {
	out := s
	out.stack = append([]groupFrame(nil), s.stack...)
	out.between = append([]int(nil), s.between...)
	return out
}

func (s *groupState) top() *groupFrame {
	if len(s.stack) == 0 {
		return nil
	}
	return &s.stack[len(s.stack)-1]
}

func (s *groupState) pop() {
	s.stack = s.stack[:len(s.stack)-1]
}

func (s *groupState) betweenAt(depth int) bool {
	for _, d := range s.between {
		if d == depth {
			return true
		}
	}
	return false
}

func (s *groupState) clearBetween(depth int) {
	for i, d := range s.between {
		if d == depth {
			s.between = append(s.between[:i:i], s.between[i+1:]...)
			return
		}
	}
}

// sameBetween reports whether two paths left the same BETWEEN forms open. They
// differ exactly when one branch closes a BETWEEN the other leaves open, which is
// the "BETWEEN {lo} {if hi}AND {hi}{/if}" shape: already broken on the false path,
// and reported rather than carried further.
func sameBetween(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for _, d := range a {
		found := false
		for _, e := range b {
			if d == e {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// planner walks a statement body and fills a groupPlan.
type planner struct {
	c    *compiler
	plan *groupPlan
}

// planGroups plans one statement body, or returns nil when no group is needed. A
// body with nothing elidable in it cannot leave an operator dangling, so it keeps
// the emission it has always had and nothing about its output moves.
func (c *compiler) planGroups(body []Node) (*groupPlan, error) {
	if !c.containsElidable(body) {
		return nil, nil
	}
	p := &planner{c: c, plan: &groupPlan{
		chunks:       map[*TextNode][]sqlChunk{},
		withheld:     map[*TextNode]map[int]bool{},
		selfFilling:  map[*ExpressionNode]bool{},
		branchCloses: map[*IfNode][2]int{},
	}}
	state, err := p.walk(body, groupState{})
	if err != nil {
		return nil, err
	}
	p.plan.trailingCloses = len(state.stack)
	return p.plan, nil
}

// containsElidable reports whether some node can emit nothing, which is what lets
// a neighbouring operator dangle. An if whose branches both emit cannot, and
// neither can a value expression or a relation.
func (c *compiler) containsElidable(nodes []Node) bool {
	for _, node := range nodes {
		switch n := node.(type) {
		case *ExpressionNode:
			if body, ok := c.predicateBody(n); ok && !c.alwaysEmits(body) {
				return true
			}
		case *IfNode:
			if !c.alwaysEmits([]Node{n}) {
				return true
			}
			if c.containsElidable(n.Then) || c.containsElidable(n.Else) {
				return true
			}
		}
	}
	return false
}

// walk scans nodes in source order, slicing each text node as it goes.
func (p *planner) walk(nodes []Node, state groupState) (groupState, error) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			state = p.text(n, state)
		case *ExpressionNode:
			if body, ok := p.c.predicateBody(n); ok && !p.c.alwaysEmits(body) {
				// The callee writes into this same Builder and calls Item where its
				// own fragments emit, so the group fills only if the callee does.
				p.plan.selfFilling[n] = true
				continue
			}
			state.prev, state.hasPrev = valuePrev, true
		case *RelationNode:
			state.prev, state.hasPrev = valuePrev, true
		case *IfNode:
			thenState, thenCloses, err := p.branch(n.Then, state)
			if err != nil {
				return state, err
			}
			elseState, elseCloses, err := p.branch(n.Else, state)
			if err != nil {
				return state, err
			}
			if thenState.depth != elseState.depth {
				return state, p.c.error(n.Pos, "a conditional branch must close every parenthesis it opens; these branches leave different nesting")
			}
			if !sameBetween(thenState.between, elseState.between) {
				return state, p.c.error(n.Pos, "the AND that closes BETWEEN is part of that form, not a conditional separator; put the whole BETWEEN inside the condition")
			}
			p.plan.branchCloses[n] = [2]int{thenCloses, elseCloses}
			state = thenState
		case *ValNode, *CheckNode:
			// Neither writes SQL, so neither opens a group nor fills one.
		}
	}
	return state, nil
}

// branch walks one branch body and reports how many groups it left open, which is
// how many closes the emitter owes at the branch's end. Closing them there keeps a
// clause opened inside a condition from leaking past it, and leaves both paths of
// an if with the same stack.
func (p *planner) branch(nodes []Node, state groupState) (groupState, int, error) {
	inner := state.clone()
	for i := range inner.stack {
		inner.stack[i].openedHere = false
	}
	out, err := p.walk(nodes, inner)
	if err != nil {
		return out, 0, err
	}
	closes := 0
	for top := out.top(); top != nil && top.openedHere; top = out.top() {
		out.pop()
		closes++
	}
	return out, closes, nil
}

// text slices one text node into chunks and advances the state across it.
func (p *planner) text(n *TextNode, state groupState) groupState {
	lexer := sqlLexer{depth: state.depth}
	tokens, ok := lexer.scan(n.Text)
	if !ok {
		// An unterminated construct is an unresolvable body everywhere else in
		// this package; here it means the node is written exactly as it stands.
		p.plan.chunks[n] = []sqlChunk{{text: n.Text, fills: fillsText(n.Text)}}
		return state
	}
	var chunks []sqlChunk
	withheld := map[int]bool{}
	cursor := 0
	// emit slices from the cursor through end, so the whitespace before a token
	// travels with that token and an elided chunk takes its own spacing with it.
	emit := func(end int, kind chunkKind) {
		slice := n.Text[cursor:end]
		cursor = end
		chunks = append(chunks, sqlChunk{text: slice, kind: kind, fills: fillsText(slice)})
	}
	closeGroup := func() {
		chunks = append(chunks, sqlChunk{kind: chunkClose})
	}
	// prune closes every group whose items sit deeper than depth, which is where a
	// closing parenthesis took them out of scope.
	prune := func(depth int) {
		for top := state.top(); top != nil && top.itemDepth > depth; top = state.top() {
			state.pop()
			closeGroup()
		}
	}
	for i, token := range tokens {
		// next is the following token's text, which is what separates a join's ON
		// from ON CONFLICT. It is empty at the end of the node; a boolean clause
		// keyword is the last token of a statement only in a body that emits no
		// predicate at all, where opening a group changes nothing.
		next := ""
		if i+1 < len(tokens) {
			next = tokens[i+1].text
		}
		switch {
		case token.text == ")":
			// A subquery's own clause frame sits above the parenthesis frame and
			// closes first; the parenthesis frame is the one this token closes.
			for top := state.top(); top != nil && top.itemDepth >= token.depth+1 &&
				!(top.paren && top.itemDepth == token.depth+1); top = state.top() {
				state.pop()
				closeGroup()
			}
			if top := state.top(); top != nil && top.paren && top.itemDepth == token.depth+1 {
				state.pop()
				withheld[token.start] = true
				emit(token.end, chunkClose)
			} else {
				emit(token.end, chunkText)
			}
			prune(token.depth)
		case token.text == "(":
			prune(token.depth)
			top := state.top()
			grouping := top != nil && top.itemDepth == token.depth && state.caseDepth == 0 &&
				(!state.hasPrev || groupingParenPredecessors[state.prev])
			if grouping {
				state.stack = append(state.stack, groupFrame{itemDepth: token.depth + 1, paren: true, openedHere: true})
				withheld[token.start] = true
				emit(token.end, chunkParenOpen)
			} else {
				emit(token.end, chunkText)
			}
		case !token.word:
			// A comma. Comma-separated clauses are not in scope, so it stays text.
			prune(token.depth)
			emit(token.end, chunkText)
		default:
			prune(token.depth)
			p.word(&state, token, next, withheld, emit, closeGroup)
		}
		state.prev, state.hasPrev = token.text, true
	}
	if cursor < len(n.Text) {
		// The tail travels with the chunk before it, so an elided joiner takes the
		// space that followed it and a surviving one keeps it.
		tail := n.Text[cursor:]
		if len(chunks) > 0 {
			last := &chunks[len(chunks)-1]
			last.text += tail
			last.fills = last.fills || fillsText(tail)
		} else {
			chunks = append(chunks, sqlChunk{text: tail, fills: fillsText(tail)})
		}
	}
	p.plan.chunks[n] = chunks
	p.plan.withheld[n] = withheld
	state.depth = lexer.depth
	return state
}

// word classifies one word token. next is the token following it, which is what
// tells a join's ON from ON CONFLICT.
func (p *planner) word(state *groupState, token sqlToken, next string, withheld map[int]bool, emit func(int, chunkKind), closeGroup func()) {
	switch token.text {
	case "CASE":
		state.caseDepth++
		emit(token.end, chunkText)
		return
	case "END":
		if state.caseDepth > 0 {
			state.caseDepth--
		}
		emit(token.end, chunkText)
		return
	case "BETWEEN":
		state.between = append(state.between, token.depth)
		emit(token.end, chunkText)
		return
	case "AND", "OR":
		top := state.top()
		joins := top != nil && top.itemDepth == token.depth && state.caseDepth == 0
		if token.text == "AND" && state.betweenAt(token.depth) {
			// The AND closing BETWEEN belongs to that two-operand form. One bit of
			// state settles it exactly, which is grammar rather than a heuristic.
			state.clearBetween(token.depth)
			joins = false
		}
		if joins {
			withheld[token.start] = true
			emit(token.end, chunkJoiner)
			return
		}
		emit(token.end, chunkText)
		return
	}
	// ON CONFLICT is a conflict action rather than a join predicate, so its ON
	// opens no group. Without this the ON frame would be closed by CONFLICT with
	// nothing having filled it, and the keyword would vanish with the group.
	opens := booleanClauseOpeners[token.text] && !(token.text == "ON" && next == "CONFLICT")
	// A clause group ends where the next clause begins, and a boolean clause
	// keyword opens one of its own.
	if clauseBoundaryWords[token.text] {
		for top := state.top(); top != nil && !top.paren && top.itemDepth == token.depth; top = state.top() {
			state.pop()
			closeGroup()
		}
	}
	if opens && state.caseDepth == 0 {
		state.stack = append(state.stack, groupFrame{itemDepth: token.depth, openedHere: true})
		emit(token.end, chunkClauseOpen)
		return
	}
	emit(token.end, chunkText)
}

// fillsText reports whether text writes anything but whitespace, which is what
// makes it an item of its group.
func fillsText(text string) bool {
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case ' ', '\t', '\r', '\n':
		default:
			return true
		}
	}
	return false
}

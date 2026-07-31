package sqlbind

import "strings"

// SQL scanning per rule:sql-top-level-keyword-scan. A clause keyword counts only
// at the analyzed statement's own nesting level, and a keyword spelled inside a
// literal, a quoted identifier, or a comment is not syntax at all.

// sqlToken is one significant token of emitted SQL. Literals, quoted
// identifiers, and comments never become tokens.
type sqlToken struct {
	text  string // uppercased for words; "(", ")", or "," for the punctuation we track
	word  bool
	depth int // parenthesis nesting; a "(" and its matching ")" share one depth
	start int // byte offsets into the scanned text, for callers that slice it back
	end   int
}

// sqlLexer carries parenthesis depth across the text nodes of one statement.
// Quotes and comments never span nodes: the template parser skips each of them
// whole while looking for embedded expressions, so an expression cannot appear
// inside one and the text around it cannot continue one.
type sqlLexer struct {
	depth int
}

// scan tokenizes one run of SQL text. ok is false for an unterminated construct,
// which every caller treats as an unresolvable body rather than scanning a
// partially understood statement.
func (l *sqlLexer) scan(sql string) (tokens []sqlToken, ok bool) {
	for i := 0; i < len(sql); {
		switch ch := sql[i]; {
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			i++
		case ch == '-' && i+1 < len(sql) && sql[i+1] == '-':
			end := strings.IndexByte(sql[i:], '\n')
			if end < 0 {
				return tokens, true
			}
			i += end + 1
		case ch == '/' && i+1 < len(sql) && sql[i+1] == '*':
			next, done := skipBlockComment(sql, i)
			if !done {
				return nil, false
			}
			i = next
		case ch == '\'' || ch == '"' || ch == '`':
			next, done := skipQuoted(sql, i, ch)
			if !done {
				return nil, false
			}
			i = next
		case ch == '$':
			if next, done, dollar := skipDollarQuoted(sql, i); dollar {
				if !done {
					return nil, false
				}
				i = next
				continue
			}
			i += wordLength(sql, i)
		case ch == '(':
			tokens = append(tokens, sqlToken{text: "(", depth: l.depth, start: i, end: i + 1})
			l.depth++
			i++
		case ch == ')':
			if l.depth > 0 {
				l.depth--
			}
			tokens = append(tokens, sqlToken{text: ")", depth: l.depth, start: i, end: i + 1})
			i++
		case ch == ',':
			tokens = append(tokens, sqlToken{text: ",", depth: l.depth, start: i, end: i + 1})
			i++
		case isWordByte(ch):
			n := wordLength(sql, i)
			tokens = append(tokens, sqlToken{text: strings.ToUpper(sql[i : i+n]), word: true, depth: l.depth, start: i, end: i + n})
			i += n
		default:
			i++
		}
	}
	return tokens, true
}

// scanSQLTokens tokenizes a complete run of SQL starting at nesting level zero.
func scanSQLTokens(sql string) ([]sqlToken, bool) {
	var lexer sqlLexer
	return lexer.scan(sql)
}

// skipBlockComment skips a /* */ comment. PostgreSQL nests them, so the scanner
// counts openings rather than stopping at the first close.
func skipBlockComment(sql string, i int) (int, bool) {
	level := 0
	for i < len(sql) {
		switch {
		case strings.HasPrefix(sql[i:], "/*"):
			level++
			i += 2
		case strings.HasPrefix(sql[i:], "*/"):
			level--
			i += 2
			if level == 0 {
				return i, true
			}
		default:
			i++
		}
	}
	return i, false
}

// skipQuoted skips a single-quoted literal, a double-quoted identifier, or a
// backtick-quoted identifier. A doubled quote is an escaped quote, not a close.
func skipQuoted(sql string, i int, quote byte) (int, bool) {
	for i++; i < len(sql); i++ {
		if sql[i] == '\\' && quote == '\'' && i+1 < len(sql) {
			i++
			continue
		}
		if sql[i] != quote {
			continue
		}
		if i+1 < len(sql) && sql[i+1] == quote {
			i++
			continue
		}
		return i + 1, true
	}
	return i, false
}

// skipDollarQuoted skips a $tag$ ... $tag$ literal. dollar is false when the $
// does not open one, such as an identifier containing $.
func skipDollarQuoted(sql string, i int) (next int, done, dollar bool) {
	end := strings.IndexByte(sql[i+1:], '$')
	if end < 0 {
		return i, false, false
	}
	tag := sql[i : i+end+2]
	for at, ch := range []byte(tag[1 : len(tag)-1]) {
		if !isWordByte(ch) || ch == '$' || at == 0 && ch >= '0' && ch <= '9' {
			return i, false, false
		}
	}
	rest := strings.Index(sql[i+len(tag):], tag)
	if rest < 0 {
		return i, false, true
	}
	return i + len(tag) + rest + len(tag), true, true
}

func wordLength(sql string, i int) int {
	n := 0
	for i+n < len(sql) && isWordByte(sql[i+n]) {
		n++
	}
	if n == 0 {
		n = 1
	}
	return n
}

func isWordByte(ch byte) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '$'
}

// mutatingVerbs open a statement, or a CTE body, that writes.
var mutatingVerbs = map[string]bool{"INSERT": true, "UPDATE": true, "DELETE": true, "MERGE": true}

// splitWith walks the CTE list of a WITH statement. It returns the leading verb
// of every CTE body and the index of the first token of the tail, which is the
// statement the outer analysis is actually about.
func splitWith(tokens []sqlToken) (cteVerbs []string, tail int, ok bool) {
	i := 1
	if i < len(tokens) && tokens[i].word && tokens[i].text == "RECURSIVE" {
		i++
	}
	for {
		if i >= len(tokens) || !tokens[i].word {
			return nil, 0, false
		}
		i++ // CTE name
		if i < len(tokens) && tokens[i].text == "(" {
			if i = matchParen(tokens, i); i < 0 {
				return nil, 0, false
			}
			i++ // past the column list
		}
		if i >= len(tokens) || !tokens[i].word || tokens[i].text != "AS" {
			return nil, 0, false
		}
		i++
		if i < len(tokens) && tokens[i].word && tokens[i].text == "NOT" {
			i++
		}
		if i < len(tokens) && tokens[i].word && tokens[i].text == "MATERIALIZED" {
			i++
		}
		if i >= len(tokens) || tokens[i].text != "(" {
			return nil, 0, false
		}
		verb := firstWord(tokens, i+1)
		if verb == "" {
			return nil, 0, false
		}
		cteVerbs = append(cteVerbs, verb)
		if i = matchParen(tokens, i); i < 0 {
			return nil, 0, false
		}
		i++
		if i < len(tokens) && tokens[i].text == "," {
			i++
			continue
		}
		break
	}
	if i >= len(tokens) || !tokens[i].word {
		return nil, 0, false
	}
	return cteVerbs, i, true
}

// matchParen returns the index of the ")" closing the "(" at open.
func matchParen(tokens []sqlToken, open int) int {
	for i := open + 1; i < len(tokens); i++ {
		if tokens[i].text == ")" && tokens[i].depth == tokens[open].depth {
			return i
		}
	}
	return -1
}

// firstWord returns the first word token at or after from.
func firstWord(tokens []sqlToken, from int) string {
	for i := from; i < len(tokens); i++ {
		if tokens[i].word {
			return tokens[i].text
		}
	}
	return ""
}

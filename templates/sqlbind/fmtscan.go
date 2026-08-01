package sqlbind

import (
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// Layout scanning for rule:sql-template-layout. The analysis scanner in
// sqlscan.go drops literals, quoted identifiers, and comments because no check
// needs them; a layout pass needs every byte to have a place, so this one emits
// the skipped regions as opaque atoms instead. Both walk the same skip helpers,
// so the two agree on where a literal ends.

type atomKind int

const (
	atomWord    atomKind = iota // an identifier or keyword
	atomPunct                   // ( ) , ;
	atomSymbol                  // any other operator or punctuation run
	atomOpaque                  // a literal or quoted identifier, never inspected
	atomComment                 // a comment, which owns the rest of its line when it is a line comment
	atomNode                    // an embedded expression or control node
)

// atom is one indivisible unit of a statement body.
type atom struct {
	kind atomKind
	text string
	// upper is the uppercased text of a word atom, so keyword matching never
	// rewrites what is printed.
	upper string
	// depth is parenthesis nesting; a "(" and its matching ")" share one depth.
	depth int
	// spaced reports whitespace between this atom and the previous one. Layout
	// owns line breaks; spacing within a line stays as the author wrote it, so
	// t.id, count(*), and a = 1 all survive.
	spaced bool
	// lineComment marks a comment that ends at a newline, so nothing may follow
	// it on its line.
	lineComment bool
	// node is the embedded node an atomNode stands for.
	node syntax.Node
}

// scanAtoms tokenizes one run of SQL text. ok is false for an unterminated
// construct, which makes the whole body unformattable rather than guessed at.
func (l *sqlLexer) scanAtoms(sql string) (atoms []atom, ok bool) {
	spaced := false
	add := func(a atom) {
		a.spaced = spaced
		spaced = false
		atoms = append(atoms, a)
	}
	for i := 0; i < len(sql); {
		switch ch := sql[i]; {
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			spaced = true
			i++
		case ch == '-' && i+1 < len(sql) && sql[i+1] == '-':
			end := strings.IndexByte(sql[i:], '\n')
			if end < 0 {
				add(atom{kind: atomComment, text: sql[i:], depth: l.depth, lineComment: true})
				return atoms, true
			}
			add(atom{kind: atomComment, text: strings.TrimRight(sql[i:i+end], " \t\r"), depth: l.depth, lineComment: true})
			i += end + 1
			spaced = true
		case ch == '/' && i+1 < len(sql) && sql[i+1] == '*':
			next, done := skipBlockComment(sql, i)
			if !done {
				return nil, false
			}
			add(atom{kind: atomComment, text: sql[i:next], depth: l.depth})
			i = next
		case ch == '\'' || ch == '"' || ch == '`':
			next, done := skipQuoted(sql, i, ch)
			if !done {
				return nil, false
			}
			add(atom{kind: atomOpaque, text: sql[i:next], depth: l.depth})
			i = next
		case ch == '$':
			if next, done, dollar := skipDollarQuoted(sql, i); dollar {
				if !done {
					return nil, false
				}
				add(atom{kind: atomOpaque, text: sql[i:next], depth: l.depth})
				i = next
				continue
			}
			n := wordLength(sql, i)
			add(atom{kind: atomWord, text: sql[i : i+n], upper: strings.ToUpper(sql[i : i+n]), depth: l.depth})
			i += n
		case ch == '(':
			add(atom{kind: atomPunct, text: "(", depth: l.depth})
			l.depth++
			i++
		case ch == ')':
			if l.depth > 0 {
				l.depth--
			}
			add(atom{kind: atomPunct, text: ")", depth: l.depth})
			i++
		case ch == ',' || ch == ';':
			add(atom{kind: atomPunct, text: string(ch), depth: l.depth})
			i++
		case isWordByte(ch):
			n := wordLength(sql, i)
			add(atom{kind: atomWord, text: sql[i : i+n], upper: strings.ToUpper(sql[i : i+n]), depth: l.depth})
			i += n
		default:
			n := symbolLength(sql, i)
			add(atom{kind: atomSymbol, text: sql[i : i+n], depth: l.depth})
			i += n
		}
	}
	return atoms, true
}

// symbolLength measures a run of operator bytes, so <= and || stay one atom.
func symbolLength(sql string, i int) int {
	n := 0
	for i+n < len(sql) && isSymbolByte(sql[i+n]) {
		n++
	}
	if n == 0 {
		n = 1
	}
	return n
}

func isSymbolByte(ch byte) bool {
	switch ch {
	case '=', '<', '>', '!', '|', '+', '*', '/', '%', '^', '~', '&', '@', '#', '-', ':', '.':
		return true
	}
	return false
}

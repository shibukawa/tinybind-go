package sqlbind

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// Statement is the low-level result of a generated SQL component: the compiled
// SQL text plus its bound arguments, with no database handle attached.
type Statement struct {
	SQL  string
	Args []any
}

// Execer is the minimal executor a generated sql.exec component needs. It is
// satisfied by *sql.DB, *sql.Conn, and *sql.Tx.
type Execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// Rows is the row cursor generated code and the sqlbind scanners consume. It is
// satisfied by *sql.Rows unchanged; a non-database/sql backend such as pgx
// returns its own implementation (wrapping Close to add the error return, and
// deriving Columns from its field descriptions). Columns is required by ForEach,
// which builds column-indexed Row maps; generated statement bodies use only
// Next, Scan, Err, and Close.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
	Columns() ([]string, error)
}

// Querier is the minimal executor a generated row-returning component needs. It
// is satisfied by *sql.DB, *sql.Conn, and *sql.Tx. Its QueryContext must return
// the concrete *sql.Rows because those types do, and Go interface satisfaction
// is exact; a backend outside database/sql cannot construct a *sql.Rows, so it
// additionally implements RowsQuerier and embeds UnimplementedQuerier.
type Querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// RowsQuerier is the driver-agnostic query surface a non-database/sql executor
// implements alongside Querier. Query prefers it over Querier.QueryContext, the
// same way io.Copy prefers io.ReaderFrom: the optional interface upgrades the
// backend without forking the generated code path or the executor parameter
// type.
type RowsQuerier interface {
	QueryRows(ctx context.Context, query string, args ...any) (Rows, error)
}

// Query executes a row-returning statement through db. A backend implementing
// RowsQuerier is queried through it; otherwise db is a database/sql handle and
// its *sql.Rows is returned as Rows. Generated row-returning statements call
// this instead of db.QueryContext directly, so one generated body serves both
// kinds of backend.
func Query(ctx context.Context, db Querier, query string, args ...any) (Rows, error) {
	if rq, ok := db.(RowsQuerier); ok {
		return rq.QueryRows(ctx, query, args...)
	}
	return db.QueryContext(ctx, query, args...)
}

// UnimplementedQuerier satisfies Querier's QueryContext with an error, for
// embedding in an executor whose real query path is QueryRows. Query never
// reaches it when RowsQuerier is implemented; it exists so a custom backend can
// satisfy Querier without being able to construct a *sql.Rows.
type UnimplementedQuerier struct{}

// QueryContext always fails: a backend embedding UnimplementedQuerier queries
// through RowsQuerier. Reaching this method means the executor was used where
// sqlbind.Query cannot dispatch, such as pre-Rows generated code.
func (UnimplementedQuerier) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("sqlbind: this executor implements QueryRows, not QueryContext; regenerate code that calls QueryContext directly")
}

// PlaceholderStyle selects the bind-placeholder syntax of the target dialect.
// Generated code passes the style chosen at generation time, so one Builder
// implementation serves every dialect.
type PlaceholderStyle int

const (
	// Dollar emits PostgreSQL-style $1, $2, ... placeholders.
	Dollar PlaceholderStyle = iota
	// Question emits ?-style placeholders.
	Question
)

// ErrEmptyValueList reports that an expanded value list had no elements, which
// would produce invalid SQL such as "IN ()".
var ErrEmptyValueList = errors.New("tinybind SQL: an expanded value list cannot be empty")

// Builder accumulates SQL text and its bound arguments. Generated statement
// builders write into it; template authors never construct placeholders
// themselves.
type Builder struct {
	strings.Builder
	style PlaceholderStyle
	args  []any
	// groups are the open predicate groups of rule:sql-predicate-group-elision,
	// outermost first. A group's opening text is withheld until something inside
	// it emits, so a group that stays empty was never opened and nothing has to
	// be taken back.
	groups []group
}

// group is one open clause or grouping parenthesis whose opening text has not
// necessarily reached the output yet.
type group struct {
	// opener is the whitespace run preceding the clause keyword or parenthesis,
	// through that token. Keeping the leading whitespace here leaves no double
	// space when the group survives and no missing space when it vanishes.
	opener string
	// written records that opener reached the output, which is also what makes
	// the group non-empty: only Item writes an opener.
	written bool
	// joiner is a separator recorded but not yet written. It is written by the
	// next Item in this group and dropped if none follows, which is what makes a
	// trailing operator vanish.
	joiner  string
	pending bool
}

// OpenGroup begins a group whose opener is withheld. Nothing is written.
func (b *Builder) OpenGroup(opener string) {
	b.groups = append(b.groups, group{opener: opener})
}

// Joiner records the separator between two items of the innermost open group. It
// is dropped when the group has written no item yet, which is what makes a
// leading operator vanish along with the condition it was meant to join.
func (b *Builder) Joiner(separator string) {
	if len(b.groups) == 0 {
		b.WriteString(separator)
		return
	}
	top := &b.groups[len(b.groups)-1]
	if !top.written {
		return
	}
	top.joiner, top.pending = separator, true
}

// Space writes a run of whitespace that separates two items rather than being one.
// It travels with the separator it follows, so an elided joiner takes its spacing
// with it, and it is dropped in a group that has written no item, where it would
// otherwise be spacing in front of nothing. The scanner does not tokenize
// whitespace and alwaysEmits does not count it, so it can never fill a group.
func (b *Builder) Space(text string) {
	if len(b.groups) == 0 {
		b.WriteString(text)
		return
	}
	top := &b.groups[len(b.groups)-1]
	switch {
	case !top.written:
		return
	case top.pending:
		top.joiner += text
	default:
		b.WriteString(text)
	}
}

// Item is called immediately before every fragment that writes a token or binds
// a value. It opens whatever groups are still withheld, outermost first, so that
// each opener is preceded by the separator attaching it to its own parent.
func (b *Builder) Item() {
	// Everything inside an unwritten group is unwritten too, so scanning outward
	// to the first unwritten frame finds where the openers start.
	start := len(b.groups)
	for start > 0 && !b.groups[start-1].written {
		start--
	}
	for i := start; i < len(b.groups); i++ {
		if i > 0 {
			b.flushJoiner(i - 1)
		}
		b.WriteString(b.groups[i].opener)
		b.groups[i].written = true
	}
	if len(b.groups) > 0 {
		b.flushJoiner(len(b.groups) - 1)
	}
}

// flushJoiner writes the separator group i has been holding, if any.
func (b *Builder) flushJoiner(i int) {
	if !b.groups[i].pending {
		return
	}
	b.WriteString(b.groups[i].joiner)
	b.groups[i].joiner, b.groups[i].pending = "", false
}

// CloseGroup ends the innermost group, writing closer only if the group was
// opened. A clause group passes an empty closer: it ends at the keyword that
// starts the next clause, which belongs outside it. A separator the group was
// holding is dropped, because nothing followed it.
func (b *Builder) CloseGroup(closer string) {
	if len(b.groups) == 0 {
		b.WriteString(closer)
		return
	}
	written := b.groups[len(b.groups)-1].written
	b.groups = b.groups[:len(b.groups)-1]
	if written {
		b.WriteString(closer)
	}
}

// NewBuilder returns a Builder emitting placeholders in the given style.
func NewBuilder(style PlaceholderStyle) Builder {
	return Builder{style: style}
}

// Arg binds one value and writes its placeholder. Appending the argument and
// emitting its placeholder is a single operation, so numbering always matches
// argument order.
func (b *Builder) Arg(value any) {
	value = bindValue(value)
	if b.style == Question {
		b.WriteByte('?')
		b.args = append(b.args, value)
		return
	}
	b.args = append(b.args, value)
	b.WriteByte('$')
	b.WriteString(strconv.Itoa(len(b.args)))
}

// bindValue converts a value database/sql cannot carry into one it can. Only
// url.URL needs this: driver.DefaultParameterConverter rejects a struct, and
// url.URL implements neither driver.Valuer nor a text form Scan can reverse.
// Handling it here also covers optional parameters and AppendValues.
func bindValue(value any) any {
	switch v := value.(type) {
	case url.URL:
		return v.String()
	case *url.URL:
		if v == nil {
			return nil
		}
		return v.String()
	}
	return value
}

// Statement returns the accumulated SQL and arguments.
func (b *Builder) Statement() Statement {
	return Statement{SQL: b.String(), Args: b.args}
}

// AppendValues expands a slice into a comma-separated placeholder list. It is a
// package function because a Go method cannot introduce its own type parameter.
func AppendValues[T any](b *Builder, values []T) error {
	if len(values) == 0 {
		return ErrEmptyValueList
	}
	for i, value := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		b.Arg(value)
	}
	return nil
}

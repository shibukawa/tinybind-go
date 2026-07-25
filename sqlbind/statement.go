package sqlbind

import (
	"context"
	"database/sql"
	"errors"
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

// Querier is the minimal executor a generated row-returning component needs. It
// is satisfied by *sql.DB, *sql.Conn, and *sql.Tx.
type Querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
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
}

// NewBuilder returns a Builder emitting placeholders in the given style.
func NewBuilder(style PlaceholderStyle) Builder {
	return Builder{style: style}
}

// Arg binds one value and writes its placeholder. Appending the argument and
// emitting its placeholder is a single operation, so numbering always matches
// argument order.
func (b *Builder) Arg(value any) {
	if b.style == Question {
		b.WriteByte('?')
		b.args = append(b.args, value)
		return
	}
	b.args = append(b.args, value)
	b.WriteByte('$')
	b.WriteString(strconv.Itoa(len(b.args)))
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

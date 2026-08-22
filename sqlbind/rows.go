package sqlbind

import (
	"fmt"
	"strconv"
)

// Row is a SQL row indexed by result column name.
type Row map[string]any

// ForEach scans rows without retaining the full result.
func ForEach(rows Rows, fn func(Row) error) error {
	if rows == nil {
		return fmt.Errorf("sqlbind: nil Rows")
	}
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	values := make([]any, len(cols))
	dest := make([]any, len(cols))
	for i := range values {
		dest[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		row := make(Row, len(cols))
		for i, col := range cols {
			row[col] = values[i]
		}
		if err := fn(row); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Key returns a stable grouping key. present is false for SQL NULL.
func Key(row Row, column string) (key string, present bool, err error) {
	v, ok := row[column]
	if !ok {
		return "", false, fmt.Errorf("sqlbind: SQL result has no column %q", column)
	}
	if v == nil {
		return "", false, nil
	}
	switch k := v.(type) {
	case []byte:
		return string(k), true, nil
	case int64:
		return strconv.FormatInt(k, 10), true, nil
	case int:
		return strconv.Itoa(k), true, nil
	case float64:
		return strconv.FormatFloat(k, 'g', -1, 64), true, nil
	}
	return fmt.Sprint(v), true, nil
}

// RequiredKey is Key with NULL rejected for a root object.
func RequiredKey(row Row, column string) (string, error) {
	k, present, err := Key(row, column)
	if err != nil {
		return "", err
	}
	if !present {
		return "", fmt.Errorf("sqlbind: NULL root group key %q", column)
	}
	return k, nil
}

func value(row Row, column string) (any, error) {
	v, ok := row[column]
	if !ok {
		return nil, fmt.Errorf("sqlbind: SQL result has no column %q", column)
	}
	return v, nil
}

func String(row Row, column string) (string, error) {
	v, err := value(row, column)
	if err != nil || v == nil {
		return "", err
	}
	if b, ok := v.([]byte); ok {
		return string(b), nil
	}
	return fmt.Sprint(v), nil
}
func Int(row Row, column string) (int, error) {
	v, err := value(row, column)
	if err != nil || v == nil {
		return 0, err
	}
	switch n := v.(type) {
	case int64:
		return int(n), nil
	case int:
		return n, nil
	case float64:
		return int(n), nil
	}
	return strconv.Atoi(text(v))
}
func Int64(row Row, column string) (int64, error) {
	v, err := value(row, column)
	if err != nil || v == nil {
		return 0, err
	}
	switch n := v.(type) {
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case float64:
		return int64(n), nil
	}
	return strconv.ParseInt(text(v), 10, 64)
}

// Uint64 scans a column as uint64, refusing a negative value rather than
// wrapping it into a large positive one.
//
// This is the one unsigned scanner. The narrower widths are read through it
// and range-checked by the generated code against bounds it knows at
// generation, which is what [jsonbind.Parser.Uint64] does on the document
// side and keeps four more scanners out of the runtime.
func Uint64(row Row, column string) (uint64, error) {
	v, err := value(row, column)
	if err != nil || v == nil {
		return 0, err
	}
	switch n := v.(type) {
	case uint64:
		return n, nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("sqlbind: column %q: %d is negative", column, n)
		}
		return uint64(n), nil
	case int:
		if n < 0 {
			return 0, fmt.Errorf("sqlbind: column %q: %d is negative", column, n)
		}
		return uint64(n), nil
	case float64:
		if n < 0 {
			return 0, fmt.Errorf("sqlbind: column %q: %v is negative", column, n)
		}
		return uint64(n), nil
	}
	return strconv.ParseUint(text(v), 10, 64)
}

// SignedN scans a column as a signed integer of the given width, reporting a
// value the width cannot hold rather than truncating it. It is what generated
// code calls for int8, int16 and int32; int and int64 keep [Int] and [Int64].
func SignedN(row Row, column string, bits int) (int64, error) {
	n, err := Int64(row, column)
	if err != nil {
		return 0, err
	}
	if bits < 64 {
		lo, hi := int64(-1)<<(bits-1), int64(1)<<(bits-1)-1
		if n < lo || n > hi {
			return 0, fmt.Errorf("sqlbind: column %q: %d does not fit in int%d", column, n, bits)
		}
	}
	return n, nil
}

// UnsignedN is the [SignedN] twin. Passing 64 checks nothing beyond what
// [Uint64] already refuses, which is a negative value.
func UnsignedN(row Row, column string, bits int) (uint64, error) {
	n, err := Uint64(row, column)
	if err != nil {
		return 0, err
	}
	if bits < 64 && n > uint64(1)<<bits-1 {
		return 0, fmt.Errorf("sqlbind: column %q: %d does not fit in uint%d", column, n, bits)
	}
	return n, nil
}

func Bool(row Row, column string) (bool, error) {
	v, e := value(row, column)
	if e != nil || v == nil {
		return false, e
	}
	if b, ok := v.(bool); ok {
		return b, nil
	}
	return strconv.ParseBool(text(v))
}
func Float64(row Row, column string) (float64, error) {
	v, e := value(row, column)
	if e != nil || v == nil {
		return 0, e
	}
	if n, ok := v.(float64); ok {
		return n, nil
	}
	return strconv.ParseFloat(text(v), 64)
}
func text(v any) string {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(v)
}

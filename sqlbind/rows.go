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
	for rows.Next() {
		values := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range values {
			dest[i] = &values[i]
		}
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
	if b, ok := v.([]byte); ok {
		return string(b), true, nil
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

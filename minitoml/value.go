// Package minitoml parses a restricted TOML subset into a flat intermediate key/value form.
package minitoml

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Kind classifies an intermediate value.
type Kind int

const (
	// KindString is a string scalar.
	KindString Kind = iota
	// KindBool is a boolean scalar.
	KindBool
	// KindInt is an integer scalar.
	KindInt
	// KindFloat is a floating-point scalar.
	KindFloat
	// KindArray is an array of primitive scalars.
	KindArray
)

// Value is one intermediate scalar or primitive array.
type Value struct {
	Kind  Kind
	Str   string
	Bool  bool
	Int   int64
	Float float64
	Array []Value
}

// String returns a display form of the value.
func (v Value) String() string {
	switch v.Kind {
	case KindString:
		return v.Str
	case KindBool:
		return strconv.FormatBool(v.Bool)
	case KindInt:
		return strconv.FormatInt(v.Int, 10)
	case KindFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64)
	case KindArray:
		parts := make([]string, len(v.Array))
		for i, e := range v.Array {
			parts[i] = e.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return ""
	}
}

// AsString coerces a scalar value to string.
func (v Value) AsString() (string, error) {
	switch v.Kind {
	case KindString:
		return v.Str, nil
	case KindBool:
		return strconv.FormatBool(v.Bool), nil
	case KindInt:
		return strconv.FormatInt(v.Int, 10), nil
	case KindFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64), nil
	case KindArray:
		return "", fmt.Errorf("minitoml: expected string scalar, got array")
	default:
		return "", fmt.Errorf("minitoml: unknown value kind %d", v.Kind)
	}
}

// AsBool coerces a scalar value to bool.
func (v Value) AsBool() (bool, error) {
	switch v.Kind {
	case KindBool:
		return v.Bool, nil
	case KindString:
		switch strings.ToLower(v.Str) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return false, fmt.Errorf("minitoml: cannot convert string %q to bool", v.Str)
		}
	default:
		return false, fmt.Errorf("minitoml: expected bool, got %s", v.KindName())
	}
}

// AsInt coerces a scalar value to int64.
func (v Value) AsInt() (int64, error) {
	switch v.Kind {
	case KindInt:
		return v.Int, nil
	case KindString:
		n, err := strconv.ParseInt(v.Str, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("minitoml: cannot convert string %q to int: %w", v.Str, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("minitoml: expected int, got %s", v.KindName())
	}
}

// AsStringSlice returns a primitive array as []string.
func (v Value) AsStringSlice() ([]string, error) {
	if v.Kind != KindArray {
		return nil, fmt.Errorf("minitoml: expected array, got %s", v.KindName())
	}
	out := make([]string, len(v.Array))
	for i, e := range v.Array {
		if e.Kind == KindArray {
			return nil, fmt.Errorf("minitoml: nested arrays are not allowed")
		}
		s, err := e.AsString()
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

// KindName returns a human-readable kind name.
func (v Value) KindName() string {
	switch v.Kind {
	case KindString:
		return "string"
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindArray:
		return "array"
	default:
		return "unknown"
	}
}

// Document is a flat map of dotted hierarchical keys to intermediate values.
type Document struct {
	entries map[string]Value
}

// NewDocument returns an empty intermediate document.
func NewDocument() Document {
	return Document{entries: make(map[string]Value)}
}

// Get returns the value for a dotted key path.
func (d Document) Get(key string) (Value, bool) {
	if d.entries == nil {
		return Value{}, false
	}
	v, ok := d.entries[key]
	return v, ok
}

// Set stores a value under a dotted key path (used by the parser and tests).
func (d *Document) Set(key string, v Value) {
	if d.entries == nil {
		d.entries = make(map[string]Value)
	}
	d.entries[key] = v
}

// Len returns the number of keys.
func (d Document) Len() int {
	return len(d.entries)
}

// Keys returns sorted dotted key paths.
func (d Document) Keys() []string {
	keys := make([]string, 0, len(d.entries))
	for k := range d.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Clone returns a shallow copy of the document map.
func (d Document) Clone() Document {
	out := NewDocument()
	for k, v := range d.entries {
		out.entries[k] = v
	}
	return out
}

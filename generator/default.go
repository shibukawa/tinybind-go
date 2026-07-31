package generator

import (
	"fmt"
)

// DefaultRule is the parsed default tag of a field (codegen only).
// A default is not a constraint: it never rejects a value, it only fills in one
// that never arrived, which is why it lives outside the check tag.
type DefaultRule struct {
	Value string
	Set   bool
}

// ParseDefaultTag parses a default tag value for a field of the given Go kind.
// Callers pass only tags that are actually present, so default:"" stays
// distinguishable from a missing tag. Unsupported kinds and values that cannot
// be converted to the field type fail here.
func ParseDefaultTag(raw, kind string) (DefaultRule, error) {
	d := DefaultRule{Value: raw, Set: true}
	switch kind {
	case "file", KindRestAny, KindRestRaw, KindStruct, KindSlice, KindMap:
		return DefaultRule{}, fmt.Errorf("default: only scalar fields support defaults, not %s", kind)
	}
	if _, err := defaultGoLiteral(kind, raw); err != nil {
		return DefaultRule{}, fmt.Errorf("default: invalid value for %s: %w", kind, err)
	}
	return d, nil
}

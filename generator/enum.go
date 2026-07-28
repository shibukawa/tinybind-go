package generator

import (
	"fmt"
	"strings"
)

// EnumRule is the parsed enum tag of a field (codegen only). Unlike a default,
// an enum can reject a value, so it counts as validation; it lives outside the
// check tag only because config structs already spell it this way.
type EnumRule struct {
	Values []string
	Set    bool
}

// ParseEnumTag parses an enum tag value for a field of the given Go kind.
// Values are comma-separated, which means a value cannot contain a comma — the
// same limit the check tag had, where commas separated rules.
func ParseEnumTag(raw, kind string) (EnumRule, error) {
	switch kind {
	case "file", KindRestAny, KindRestRaw, KindStruct, KindSlice, KindMap:
		return EnumRule{}, fmt.Errorf("enum: only scalar fields support enums, not %s", kind)
	}
	if raw == "" {
		return EnumRule{}, fmt.Errorf("enum: empty enum")
	}
	e := EnumRule{Set: true}
	for _, v := range strings.Split(raw, ",") {
		v = strings.TrimSpace(v)
		if v == "" {
			return EnumRule{}, fmt.Errorf("enum: empty enum value in %q", raw)
		}
		if _, err := defaultGoLiteral(kind, v); err != nil {
			return EnumRule{}, fmt.Errorf("enum: invalid value %q for %s: %w", v, kind, err)
		}
		e.Values = append(e.Values, v)
	}
	return e, nil
}

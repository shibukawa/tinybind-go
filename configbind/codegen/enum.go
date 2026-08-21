package codegen

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/tinybind-go/minitoml/codegen"
)

// enumChoices splits an enum tag into the values it lists. It only splits and
// trims: checkEnumTags is what rejects a tag this returns nothing usable for,
// and it runs before any caller here reads the result.
func enumChoices(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, choice := range strings.Split(raw, ",") {
		if choice = strings.TrimSpace(choice); choice != "" {
			out = append(out, choice)
		}
	}
	return out
}

// checkEnumTags rejects every enum tag the generated check could not honor, on
// leaf fields, on nested struct leaves, and on array-of-tables element fields
// alike. An element field is included on purpose: unlike dependon and falsy it
// needs no stable key of its own, only the value the element overlay holds.
//
// A struct or an array carries its own rejection in checkStructFieldTags, which
// words it by where the tag sits; this walk steps over them so that message is
// not preempted by one about value kinds.
func checkEnumTags(owner string, fields []Field) error {
	for _, field := range fields {
		name := field.GoName
		if owner != "" {
			name = owner + "." + field.GoName
		}
		switch field.Kind {
		case FieldStruct, FieldStructSlice:
			if err := checkEnumTags(name, field.Nested); err != nil {
				return err
			}
			continue
		}
		if err := checkEnumTag(name, field); err != nil {
			return err
		}
	}
	return nil
}

// checkEnumTag rejects one field's tag. Everything it can catch is caught here
// rather than at load: a choice that cannot be parsed, a list that repeats one,
// and a default or falsy value the field's own allowlist does not contain. The
// last two are the typos with no runtime symptom — an unlisted default is simply
// applied, and an unlisted falsy silently disables the emptiness test that the
// keys depending on this one ride on.
func checkEnumTag(name string, field Field) error {
	if field.Enum == "" {
		return nil
	}
	switch field.Kind {
	case FieldString, FieldInt, FieldDuration, FieldStringSlice:
	case FieldBool:
		return fmt.Errorf("field %s: enum applies to string, int, duration, and []string fields; a bool already holds only true and false", name)
	default:
		return fmt.Errorf("field %s: enum applies to string, int, duration, and []string fields only", name)
	}
	choices := enumChoices(field.Enum)
	if len(choices) != len(strings.Split(field.Enum, ",")) {
		return fmt.Errorf("field %s: enum %q names an empty choice", name, field.Enum)
	}
	literals, err := enumLiterals(name, field, choices)
	if err != nil {
		return err
	}
	seen := make(map[string]string, len(literals))
	for i, literal := range literals {
		if first, ok := seen[literal]; ok {
			return fmt.Errorf("field %s: enum names %q twice", name, first)
		}
		seen[literal] = choices[i]
	}
	// A []string has no single value, so neither a default nor a falsy choice
	// reaches one; both are already inert on that kind.
	if field.Kind == FieldStringSlice {
		return nil
	}
	for _, tag := range []struct {
		name  string
		value string
	}{
		{"default", field.Default},
		{"falsy", field.Falsy},
	} {
		if tag.value == "" {
			continue
		}
		literal, err := enumLiteral(name, field, tag.value)
		if err != nil {
			// The tag's own collector reports an unparsable value with the message
			// written for it; this walk only rates membership.
			continue
		}
		if _, ok := seen[literal]; !ok {
			return fmt.Errorf("field %s: %s %q is not one of the enum choices %q", name, tag.name, tag.value, field.Enum)
		}
	}
	return nil
}

// enumLiterals renders a field's choices as the Go literals its generated check
// compares against. An int or duration choice is rendered from the parsed value
// rather than from the text, so "08080" and "8080" are one choice of an int
// field and "60s" and "1m" are one choice of a duration.
func enumLiterals(name string, field Field, choices []string) ([]string, error) {
	out := make([]string, 0, len(choices))
	for _, choice := range choices {
		literal, err := enumLiteral(name, field, choice)
		if err != nil {
			return nil, err
		}
		out = append(out, literal)
	}
	return out, nil
}

func enumLiteral(name string, field Field, value string) (string, error) {
	switch field.Kind {
	case FieldInt:
		spec, err := codegen.IntFieldSpecOf(field.GoType)
		if err != nil {
			return "", fmt.Errorf("field %s: %w", name, err)
		}
		literal, err := spec.ParseLiteral(value)
		if err != nil {
			return "", fmt.Errorf("field %s: enum value %q is not a valid %s: %w", name, value, spec.Name, err)
		}
		return literal, nil
	case FieldDuration:
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return "", fmt.Errorf("field %s: enum value %q is not a duration: %w", name, value, err)
		}
		return strconv.FormatInt(int64(parsed), 10), nil
	default:
		return strconv.Quote(value), nil
	}
}

// emitEnumCheck writes the allowlist test for one field, or nothing when the
// field carries no enum tag. subject is the Go expression holding the value to
// test, which is the parsed one so that a choice is matched by value rather than
// by spelling; raw is the expression a rejection quotes, which is always the
// text the source supplied.
func emitEnumCheck(b *bytes.Buffer, scope applyScope, indent, full string, field Field, subject, raw string) error {
	choices := enumChoices(field.Enum)
	if len(choices) == 0 {
		return nil
	}
	literals, err := enumLiterals(field.GoName, field, choices)
	if err != nil {
		return err
	}
	written := strings.Join(choices, ", ")
	cases := strings.Join(literals, ", ")
	// A duration case list is nanoseconds, which no reader recognises, and an int
	// list may have been renormalised. Both keep the tag's own spelling beside the
	// literals; a quoted string list already spells it.
	trailer := ""
	if (field.Kind == FieldDuration || field.Kind == FieldInt) && cases != written {
		trailer = " // " + written
	}
	fmt.Fprintf(b, "%sswitch %s {\n", indent, subject)
	fmt.Fprintf(b, "%scase %s:%s\n", indent, cases, trailer)
	fmt.Fprintf(b, "%sdefault:\n", indent)
	// diagKey carries one [%d] verb per enclosing array of tables, so its verbs are
	// kept; a choice is arbitrary tag text, so its own are escaped and the whole
	// format string is quoted rather than pasted into the emitted literal.
	format := fmt.Sprintf("configbind: %s: %%q must be one of: %s",
		scope.diagKey(full), strings.ReplaceAll(written, "%", "%%"))
	fmt.Fprintf(b, "%s\treturn fmt.Errorf(%s%s, %s)\n", indent, strconv.Quote(format), scope.diagArgs(), raw)
	fmt.Fprintf(b, "%s}\n", indent)
	return nil
}

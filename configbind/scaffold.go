package configbind

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/tinybind-go/cliparser"
)

// ScaffoldKind is the value kind needed to render a configuration example.
type ScaffoldKind uint8

const (
	ScaffoldString ScaffoldKind = iota
	ScaffoldBool
	ScaffoldInt
	ScaffoldDuration
	ScaffoldStringSlice
	// ScaffoldTableArray is an array of tables rendered as a [[key]] block.
	ScaffoldTableArray
)

// ScaffoldField is generated metadata for one leaf configuration field, or for
// one array of tables and its element fields.
type ScaffoldField struct {
	Key     string
	Kind    ScaffoldKind
	Default string
	Opt     string
	Env     string
	Help    string
	// Enum is the allowlist the field accepts, already split and trimmed at
	// generation time. It is rendered as a comment beside the example value so a
	// reader sees the choices where the typo would otherwise be made; the value
	// check itself lives in the generated apply code.
	Enum []string
	// Nested holds the element fields when Kind is ScaffoldTableArray. Their
	// keys are relative to Key.
	Nested []ScaffoldField
}

// ScaffoldTOML renders all registered definitions as one deterministic TOML scaffold.
func ScaffoldTOML() (string, error) {
	entries, docs, err := scaffoldEntries()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	currentPrefix := ""
	// Arrays of tables are held back: TOML reads every key after a [[header]] as
	// part of that element, so the prefix's own keys must come first.
	var tableArrays []scaffoldEntry
	flushTableArrays := func() error {
		for _, entry := range tableArrays {
			if err := writeScaffoldTableArray(&b, entry.fullKey, entry.field); err != nil {
				return fmt.Errorf("configbind: scaffold %s: %w", entry.fullKey, err)
			}
		}
		tableArrays = nil
		return nil
	}
	for _, entry := range entries {
		if entry.definition.Prefix != currentPrefix {
			if err := flushTableArrays(); err != nil {
				return "", err
			}
			if currentPrefix != "" {
				b.WriteByte('\n')
			}
			currentPrefix = entry.definition.Prefix
			writeScaffoldHelp(&b, docs[currentPrefix])
			fmt.Fprintf(&b, "[%s]\n", currentPrefix)
		}
		if entry.field.Kind == ScaffoldTableArray {
			tableArrays = append(tableArrays, entry)
			continue
		}
		writeScaffoldHelp(&b, entry.field.Help)
		writeScaffoldChoices(&b, entry.field.Enum)
		value, err := scaffoldValue(entry.field, true)
		if err != nil {
			return "", fmt.Errorf("configbind: scaffold %s: %w", entry.fullKey, err)
		}
		fmt.Fprintf(&b, "%s = %s\n", entry.field.Key, value)
	}
	if err := flushTableArrays(); err != nil {
		return "", err
	}
	return b.String(), nil
}

// writeScaffoldTableArray renders one [[key]] block holding a single example
// element, then the element's own arrays of tables under it.
func writeScaffoldTableArray(b *strings.Builder, fullKey string, field ScaffoldField) error {
	b.WriteByte('\n')
	writeScaffoldHelp(b, field.Help)
	fmt.Fprintf(b, "[[%s]]\n", fullKey)
	var nestedArrays []ScaffoldField
	for _, nested := range field.Nested {
		if !validScaffoldKeyPath(nested.Key) {
			return fmt.Errorf("scaffold field key %q is not a bare TOML key path", nested.Key)
		}
		if nested.Kind == ScaffoldTableArray {
			nestedArrays = append(nestedArrays, nested)
			continue
		}
		writeScaffoldHelp(b, nested.Help)
		writeScaffoldChoices(b, nested.Enum)
		value, err := scaffoldValue(nested, true)
		if err != nil {
			return fmt.Errorf("%s: %w", nested.Key, err)
		}
		fmt.Fprintf(b, "%s = %s\n", nested.Key, value)
	}
	for _, nested := range nestedArrays {
		if err := writeScaffoldTableArray(b, fullKey+"."+nested.Key, nested); err != nil {
			return err
		}
	}
	return nil
}

// ScaffoldEnv renders all registered Bind fragments as one deterministic .env scaffold.
// Struct docs are omitted: env output is sorted globally by variable name, so a
// per-definition comment has no stable position.
func ScaffoldEnv() (string, error) {
	entries, _, err := scaffoldEntries()
	if err != nil {
		return "", err
	}
	type envEntry struct {
		name  string
		entry scaffoldEntry
	}
	envs := make([]envEntry, 0, len(entries))
	seen := map[string]string{}
	for _, entry := range entries {
		if entry.field.Kind == ScaffoldTableArray {
			// A repeated table has no environment variable form.
			continue
		}
		def, err := cliparser.DefFromField(cliparser.FieldMeta{
			Prefix: entry.definition.Prefix,
			Key:    entry.field.Key,
			Opt:    entry.field.Opt,
			Env:    entry.field.Env,
			Help:   entry.field.Help,
		})
		if err != nil {
			return "", fmt.Errorf("configbind: scaffold %s: %w", entry.fullKey, err)
		}
		if def.Env == "-" {
			continue
		}
		name := def.Env
		if name == "" && len(def.Longs) > 0 {
			name = EnvName(def.Longs[0])
		}
		if previous, ok := seen[name]; ok {
			return "", fmt.Errorf("configbind: duplicate scaffold environment variable %q for %q and %q", name, previous, entry.fullKey)
		}
		seen[name] = entry.fullKey
		envs = append(envs, envEntry{name: name, entry: entry})
	}
	sortFunc(envs, func(a, b envEntry) bool { return a.name < b.name })

	var b strings.Builder
	for _, item := range envs {
		writeScaffoldHelp(&b, item.entry.field.Help)
		writeScaffoldChoices(&b, item.entry.field.Enum)
		value, err := scaffoldValue(item.entry.field, false)
		if err != nil {
			return "", fmt.Errorf("configbind: scaffold %s: %w", item.entry.fullKey, err)
		}
		fmt.Fprintf(&b, "%s=%s\n", item.name, value)
	}
	return b.String(), nil
}

// WriteScaffoldTOML writes the combined TOML scaffold to w.
func WriteScaffoldTOML(w io.Writer) error {
	text, err := ScaffoldTOML()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}

// WriteScaffoldEnv writes the combined .env scaffold to w.
func WriteScaffoldEnv(w io.Writer) error {
	text, err := ScaffoldEnv()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}

type scaffoldEntry struct {
	definition Definition
	field      ScaffoldField
	fullKey    string
}

// scaffoldEntries returns the flattened leaf fields plus the doc text to render
// above each prefix table. When several definitions share a prefix the doc comes
// from the first one in (prefix, TypeName) order.
func scaffoldEntries() ([]scaffoldEntry, map[string]string, error) {
	definitionsMu.RLock()
	registered := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		definition.Scaffold = append([]ScaffoldField(nil), definition.Scaffold...)
		registered = append(registered, definition)
	}
	definitionsMu.RUnlock()

	sortFunc(registered, func(a, b Definition) bool {
		if a.Prefix != b.Prefix {
			return a.Prefix < b.Prefix
		}
		return a.TypeName < b.TypeName
	})
	seenKeys := map[string]string{}
	docs := map[string]string{}
	var entries []scaffoldEntry
	for _, definition := range registered {
		if !validScaffoldKeyPath(definition.Prefix) {
			return nil, nil, fmt.Errorf("configbind: scaffold prefix %q is not a bare TOML key path", definition.Prefix)
		}
		if _, ok := docs[definition.Prefix]; !ok && definition.Doc != "" {
			docs[definition.Prefix] = definition.Doc
		}
		for _, field := range definition.Scaffold {
			if !validScaffoldKeyPath(field.Key) {
				return nil, nil, fmt.Errorf("configbind: scaffold field key %q is not a bare TOML key path", field.Key)
			}
			fullKey := definition.Prefix + "." + field.Key
			if previous, ok := seenKeys[fullKey]; ok {
				return nil, nil, fmt.Errorf("configbind: duplicate scaffold key %q in definitions %q and %q", fullKey, previous, definition.TypeName)
			}
			seenKeys[fullKey] = definition.TypeName
			entries = append(entries, scaffoldEntry{definition: definition, field: field, fullKey: fullKey})
		}
	}
	// registered is already sorted by (prefix, TypeName) and each definition's
	// scaffold fields are generated in struct declaration order, so entries are
	// grouped by table and ordered by declaration without a further sort. Table
	// order stays independent of package init order.
	return entries, docs, nil
}

func scaffoldValue(field ScaffoldField, toml bool) (string, error) {
	switch field.Kind {
	case ScaffoldString:
		if toml {
			return quoteTOMLString(field.Default), nil
		}
		return strconv.Quote(field.Default), nil
	case ScaffoldBool:
		if field.Default == "" {
			return "false", nil
		}
		value, err := strconv.ParseBool(field.Default)
		if err != nil {
			return "", fmt.Errorf("invalid bool default %q", field.Default)
		}
		return strconv.FormatBool(value), nil
	case ScaffoldInt:
		if field.Default == "" {
			return "0", nil
		}
		value, err := strconv.ParseInt(field.Default, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid int default %q", field.Default)
		}
		return strconv.FormatInt(value, 10), nil
	case ScaffoldDuration:
		// A duration is always a quoted string; a bare number has no unit.
		value := time.Duration(0)
		if field.Default != "" {
			parsed, err := time.ParseDuration(field.Default)
			if err != nil {
				return "", fmt.Errorf("invalid duration default %q", field.Default)
			}
			value = parsed
		}
		if toml {
			return quoteTOMLString(value.String()), nil
		}
		return strconv.Quote(value.String()), nil
	case ScaffoldStringSlice:
		if toml {
			return "[]", nil
		}
		return strconv.Quote(""), nil
	case ScaffoldTableArray:
		return "", fmt.Errorf("an array of tables has no inline value form")
	default:
		return "", fmt.Errorf("unsupported field kind %d", field.Kind)
	}
}

func writeScaffoldHelp(b *strings.Builder, help string) {
	for _, line := range strings.Split(strings.TrimSpace(help), "\n") {
		if line != "" {
			fmt.Fprintf(b, "# %s\n", strings.TrimSpace(line))
		}
	}
}

// writeScaffoldChoices renders a field's allowlist as its own comment line under
// any help text. It is a line of its own rather than a suffix on the help so a
// multi-line help comment keeps the choices at a fixed position.
func writeScaffoldChoices(b *strings.Builder, enum []string) {
	if len(enum) == 0 {
		return
	}
	fmt.Fprintf(b, "# %s\n", enumNote(enum))
}

// enumNote is the one spelling of an allowlist shared by the scaffolds and the
// subcommand usage text.
func enumNote(enum []string) string {
	return "one of: " + strings.Join(enum, ", ")
}

func quoteTOMLString(value string) string {
	var b bytes.Buffer
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString("\\b")
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\f':
			b.WriteString("\\f")
		case '\r':
			b.WriteString("\\r")
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, "\\u%04X", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func validScaffoldKeyPath(path string) bool {
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			return false
		}
		for _, r := range part {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
				(r >= '0' && r <= '9') || r == '_' || r == '-') {
				return false
			}
		}
	}
	return true
}

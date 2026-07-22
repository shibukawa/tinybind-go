package configbind

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/shibukawa/tinybind-go/cliparser"
)

// ScaffoldKind is the value kind needed to render a configuration example.
type ScaffoldKind uint8

const (
	ScaffoldString ScaffoldKind = iota
	ScaffoldBool
	ScaffoldInt
	ScaffoldStringSlice
)

// ScaffoldField is generated metadata for one leaf configuration field.
type ScaffoldField struct {
	Key     string
	Kind    ScaffoldKind
	Default string
	Opt     string
	Env     string
	Help    string
}

// ScaffoldFragment is the generated scaffold metadata for one Bind type and prefix.
// ID must be stable and package-qualified.
type ScaffoldFragment struct {
	ID     string
	Prefix string
	Fields []ScaffoldField
}

var (
	scaffoldMu        sync.RWMutex
	scaffoldFragments []ScaffoldFragment
)

// RegisterScaffold registers generated metadata for later aggregate rendering.
func RegisterScaffold(fragment ScaffoldFragment) {
	fragment.Fields = append([]ScaffoldField(nil), fragment.Fields...)
	scaffoldMu.Lock()
	scaffoldFragments = append(scaffoldFragments, fragment)
	scaffoldMu.Unlock()
}

// ScaffoldTOML renders all registered Bind fragments as one deterministic TOML scaffold.
func ScaffoldTOML() (string, error) {
	entries, err := scaffoldEntries()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	currentPrefix := ""
	for _, entry := range entries {
		if entry.fragment.Prefix != currentPrefix {
			if currentPrefix != "" {
				b.WriteByte('\n')
			}
			currentPrefix = entry.fragment.Prefix
			fmt.Fprintf(&b, "[%s]\n", currentPrefix)
		}
		writeScaffoldHelp(&b, entry.field.Help)
		value, err := scaffoldValue(entry.field, true)
		if err != nil {
			return "", fmt.Errorf("configbind: scaffold %s: %w", entry.fullKey, err)
		}
		fmt.Fprintf(&b, "%s = %s\n", entry.field.Key, value)
	}
	return b.String(), nil
}

// ScaffoldEnv renders all registered Bind fragments as one deterministic .env scaffold.
func ScaffoldEnv() (string, error) {
	entries, err := scaffoldEntries()
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
		def, err := cliparser.DefFromField(cliparser.FieldMeta{
			Prefix: entry.fragment.Prefix,
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
	sort.Slice(envs, func(i, j int) bool { return envs[i].name < envs[j].name })

	var b strings.Builder
	for _, item := range envs {
		writeScaffoldHelp(&b, item.entry.field.Help)
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

// ResetScaffolds clears generated scaffold registrations. It is intended for tests.
func ResetScaffolds() {
	scaffoldMu.Lock()
	scaffoldFragments = nil
	scaffoldMu.Unlock()
}

type scaffoldEntry struct {
	fragment ScaffoldFragment
	field    ScaffoldField
	fullKey  string
}

func scaffoldEntries() ([]scaffoldEntry, error) {
	scaffoldMu.RLock()
	fragments := make([]ScaffoldFragment, len(scaffoldFragments))
	for i, fragment := range scaffoldFragments {
		fragments[i] = fragment
		fragments[i].Fields = append([]ScaffoldField(nil), fragment.Fields...)
	}
	scaffoldMu.RUnlock()

	sort.Slice(fragments, func(i, j int) bool {
		if fragments[i].Prefix != fragments[j].Prefix {
			return fragments[i].Prefix < fragments[j].Prefix
		}
		return fragments[i].ID < fragments[j].ID
	})
	seenIDs := map[string]ScaffoldFragment{}
	seenKeys := map[string]string{}
	var entries []scaffoldEntry
	for _, fragment := range fragments {
		if fragment.ID == "" || fragment.Prefix == "" {
			return nil, fmt.Errorf("configbind: scaffold fragment requires ID and Prefix")
		}
		if !validScaffoldKeyPath(fragment.Prefix) {
			return nil, fmt.Errorf("configbind: scaffold prefix %q is not a bare TOML key path", fragment.Prefix)
		}
		if previous, ok := seenIDs[fragment.ID]; ok {
			if equalScaffoldFragment(previous, fragment) {
				continue
			}
			return nil, fmt.Errorf("configbind: conflicting scaffold fragment ID %q", fragment.ID)
		}
		seenIDs[fragment.ID] = fragment
		for _, field := range fragment.Fields {
			if !validScaffoldKeyPath(field.Key) {
				return nil, fmt.Errorf("configbind: scaffold field key %q is not a bare TOML key path", field.Key)
			}
			fullKey := fragment.Prefix + "." + field.Key
			if previous, ok := seenKeys[fullKey]; ok {
				return nil, fmt.Errorf("configbind: duplicate scaffold key %q in fragments %q and %q", fullKey, previous, fragment.ID)
			}
			seenKeys[fullKey] = fragment.ID
			entries = append(entries, scaffoldEntry{fragment: fragment, field: field, fullKey: fullKey})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].fragment.Prefix != entries[j].fragment.Prefix {
			return entries[i].fragment.Prefix < entries[j].fragment.Prefix
		}
		return entries[i].field.Key < entries[j].field.Key
	})
	return entries, nil
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
	case ScaffoldStringSlice:
		if toml {
			return "[]", nil
		}
		return strconv.Quote(""), nil
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

func equalScaffoldFragment(a, b ScaffoldFragment) bool {
	if a.ID != b.ID || a.Prefix != b.Prefix || len(a.Fields) != len(b.Fields) {
		return false
	}
	for i := range a.Fields {
		if a.Fields[i] != b.Fields[i] {
			return false
		}
	}
	return true
}

// Package cliparser parses argv into a map of stable config keys using precomputed flag definitions.
// Code generators build Def values at generate time; runtime parsing does not use reflection.
package cliparser

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind classifies how a flag consumes argv values.
type Kind int

const (
	// KindString is a single string value flag (--flag value or --flag=value).
	KindString Kind = iota
	// KindBool is a boolean flag (--flag, --flag=true|false).
	KindBool
	// KindArray accumulates repeated flag occurrences into a multi-value list.
	KindArray
)

// Def is one CLI flag definition mapping long/short names to a stable config key.
// Aligns with data:cli-flag-def / decision:cli-flag-naming.
type Def struct {
	// ConfigKey is the stable overlay key (e.g. "webserver.port").
	ConfigKey string
	// Longs are long flag names without leading dashes (e.g. "webserver-port" or "port").
	Longs []string
	// Shorts are optional single-letter short names without leading dashes (e.g. "p").
	Shorts []string
	// Help is optional usage text.
	Help string
	// Kind selects value parsing behavior.
	Kind Kind
	// UsesOptOverride is true when names came from an opt-style override.
	UsesOptOverride bool
}

// FieldMeta is codegen-facing field metadata used to build Def values without runtime tags.
type FieldMeta struct {
	// Prefix is the Bind prefix / table name (e.g. "webserver").
	Prefix string
	// Key is the relative field key under the prefix (e.g. "port" or "tls.enabled").
	Key string
	// Opt is an optional override of the form "long" or "long,short" (no dashes).
	// When set, only those names are registered; default --prefix-key is suppressed.
	Opt string
	// Help is optional CLI help text.
	Help string
	// Kind defaults to KindString when zero value is left as KindString.
	Kind Kind
}

// ConfigKeyPath builds the stable config key "prefix.key".
func ConfigKeyPath(prefix, key string) string {
	prefix = strings.TrimSpace(prefix)
	key = strings.TrimSpace(key)
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + "." + key
}

// DefaultLongName builds the default long flag name without dashes: "{prefix}-{key}"
// with '.' in key replaced by '-'.
func DefaultLongName(prefix, key string) string {
	prefix = strings.TrimSpace(prefix)
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, ".", "-")
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + "-" + key
}

// DefFromField builds a Def from FieldMeta using decision:cli-flag-naming rules.
func DefFromField(m FieldMeta) (Def, error) {
	if strings.TrimSpace(m.Prefix) == "" && strings.TrimSpace(m.Key) == "" {
		return Def{}, fmt.Errorf("cliparser: FieldMeta requires Prefix and/or Key")
	}
	if strings.TrimSpace(m.Key) == "" {
		return Def{}, fmt.Errorf("cliparser: FieldMeta.Key is required")
	}
	cfgKey := ConfigKeyPath(m.Prefix, m.Key)
	d := Def{
		ConfigKey: cfgKey,
		Help:      m.Help,
		Kind:      m.Kind,
	}
	opt := strings.TrimSpace(m.Opt)
	if opt == "" {
		d.Longs = []string{DefaultLongName(m.Prefix, m.Key)}
		d.UsesOptOverride = false
		return d, nil
	}
	longs, shorts, err := parseOpt(opt)
	if err != nil {
		return Def{}, err
	}
	d.Longs = longs
	d.Shorts = shorts
	d.UsesOptOverride = true
	return d, nil
}

// BuildDefs builds flag definitions for a slice of field metadata (codegen helper entry point).
func BuildDefs(fields []FieldMeta) ([]Def, error) {
	out := make([]Def, 0, len(fields))
	seenLong := make(map[string]string)  // long -> config key
	seenShort := make(map[string]string) // short -> config key
	for _, f := range fields {
		d, err := DefFromField(f)
		if err != nil {
			return nil, err
		}
		for _, ln := range d.Longs {
			if prev, ok := seenLong[ln]; ok {
				return nil, fmt.Errorf("cliparser: duplicate long flag %q for %q and %q", ln, prev, d.ConfigKey)
			}
			seenLong[ln] = d.ConfigKey
		}
		for _, sh := range d.Shorts {
			if prev, ok := seenShort[sh]; ok {
				return nil, fmt.Errorf("cliparser: duplicate short flag %q for %q and %q", sh, prev, d.ConfigKey)
			}
			seenShort[sh] = d.ConfigKey
		}
		out = append(out, d)
	}
	return out, nil
}

func parseOpt(opt string) (longs []string, shorts []string, err error) {
	parts := strings.Split(opt, ",")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return nil, nil, fmt.Errorf("cliparser: opt %q must include a long name", opt)
	}
	if len(parts) > 2 {
		return nil, nil, fmt.Errorf("cliparser: opt %q has too many comma-separated parts", opt)
	}
	long := strings.TrimSpace(parts[0])
	long = strings.TrimPrefix(long, "--")
	if long == "" || strings.ContainsAny(long, " \t") {
		return nil, nil, fmt.Errorf("cliparser: invalid opt long name in %q", opt)
	}
	longs = []string{long}
	if len(parts) == 2 {
		short := strings.TrimSpace(parts[1])
		short = strings.TrimPrefix(short, "-")
		if short == "" {
			return nil, nil, fmt.Errorf("cliparser: empty short in opt %q", opt)
		}
		r, size := utf8.DecodeRuneInString(short)
		if size != len(short) || r == utf8.RuneError || !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return nil, nil, fmt.Errorf("cliparser: short flag must be a single letter or digit in opt %q", opt)
		}
		shorts = []string{short}
	}
	return longs, shorts, nil
}

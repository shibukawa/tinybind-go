package cliparser

import (
	"fmt"
	"strings"
)

// Result is the outcome of parsing argv against flag definitions.
type Result struct {
	// Values maps stable config keys to a single string (last write wins for KindString/KindBool).
	Values map[string]string
	// Multi maps stable config keys to accumulated values for KindArray flags.
	Multi map[string][]string
	// Rest is non-flag arguments after flags (or after "--").
	Rest []string
}

// Parse matches argv against defs and returns stable config key → raw values.
// Only flags that appear in argv are present in Values/Multi; missing flags are absent.
// Runtime uses only the provided Def list—no reflection.
func Parse(args []string, defs []Def) (Result, error) {
	res := Result{
		Values: make(map[string]string),
		Multi:  make(map[string][]string),
	}
	byLong := make(map[string]Def, len(defs))
	byShort := make(map[string]Def)
	for _, d := range defs {
		if d.ConfigKey == "" {
			return Result{}, fmt.Errorf("cliparser: Def with empty ConfigKey")
		}
		if len(d.Longs) == 0 && len(d.Shorts) == 0 {
			return Result{}, fmt.Errorf("cliparser: Def %q has no flag names", d.ConfigKey)
		}
		for _, ln := range d.Longs {
			ln = strings.TrimPrefix(strings.TrimSpace(ln), "--")
			if ln == "" {
				return Result{}, fmt.Errorf("cliparser: empty long flag for %q", d.ConfigKey)
			}
			if prev, ok := byLong[ln]; ok {
				return Result{}, fmt.Errorf("cliparser: duplicate long %q (%q and %q)", ln, prev.ConfigKey, d.ConfigKey)
			}
			byLong[ln] = d
		}
		for _, sh := range d.Shorts {
			sh = strings.TrimPrefix(strings.TrimSpace(sh), "-")
			if len(sh) != 1 {
				return Result{}, fmt.Errorf("cliparser: short flag %q for %q must be one character", sh, d.ConfigKey)
			}
			if prev, ok := byShort[sh]; ok {
				return Result{}, fmt.Errorf("cliparser: duplicate short %q (%q and %q)", sh, prev.ConfigKey, d.ConfigKey)
			}
			byShort[sh] = d
		}
	}

	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			res.Rest = append(res.Rest, args[i+1:]...)
			return res, nil
		}
		if strings.HasPrefix(a, "--") {
			name := a[2:]
			var inline string
			hasInline := false
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				inline = name[eq+1:]
				name = name[:eq]
				hasInline = true
			}
			d, ok := byLong[name]
			if !ok {
				return Result{}, fmt.Errorf("cliparser: unknown flag --%s", name)
			}
			val, next, err := takeValue(args, i, d, hasInline, inline)
			if err != nil {
				return Result{}, err
			}
			setResult(&res, d, val)
			i = next
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			body := a[1:]
			if body == "" {
				return Result{}, fmt.Errorf("cliparser: invalid flag %q", a)
			}
			sh := body[:1]
			d, ok := byShort[sh]
			if !ok {
				return Result{}, fmt.Errorf("cliparser: unknown flag -%s", sh)
			}
			rest := body[1:]
			if rest != "" {
				if d.Kind == KindBool {
					v, err := parseBoolToken(rest)
					if err != nil {
						return Result{}, err
					}
					setResult(&res, d, v)
					i++
					continue
				}
				setResult(&res, d, rest)
				i++
				continue
			}
			val, next, err := takeValue(args, i, d, false, "")
			if err != nil {
				return Result{}, err
			}
			setResult(&res, d, val)
			i = next
			continue
		}
		res.Rest = append(res.Rest, args[i:]...)
		return res, nil
	}
	return res, nil
}

func takeValue(args []string, i int, d Def, hasInline bool, inline string) (string, int, error) {
	if d.Kind == KindBool {
		if hasInline {
			v, err := parseBoolToken(inline)
			if err != nil {
				return "", 0, err
			}
			return v, i + 1, nil
		}
		return "true", i + 1, nil
	}
	if hasInline {
		return inline, i + 1, nil
	}
	if i+1 >= len(args) {
		return "", 0, fmt.Errorf("cliparser: flag %s requires a value", flagLabel(d))
	}
	next := args[i+1]
	if isFlagToken(next) {
		return "", 0, fmt.Errorf("cliparser: flag %s requires a value", flagLabel(d))
	}
	return next, i + 2, nil
}

func isFlagToken(s string) bool {
	if s == "-" {
		return false
	}
	if !strings.HasPrefix(s, "-") {
		return false
	}
	// negative number is a value, not a flag
	if len(s) > 1 {
		c := s[1]
		if c >= '0' && c <= '9' {
			return false
		}
		if c == '.' && len(s) > 2 && s[2] >= '0' && s[2] <= '9' {
			return false
		}
	}
	return true
}

func parseBoolToken(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "y":
		return "true", nil
	case "false", "0", "no", "n":
		return "false", nil
	default:
		return "", fmt.Errorf("cliparser: invalid bool value %q", s)
	}
}

func flagLabel(d Def) string {
	if len(d.Longs) > 0 {
		return "--" + d.Longs[0]
	}
	if len(d.Shorts) > 0 {
		return "-" + d.Shorts[0]
	}
	return d.ConfigKey
}

func setResult(res *Result, d Def, val string) {
	if d.Kind == KindArray {
		res.Multi[d.ConfigKey] = append(res.Multi[d.ConfigKey], val)
		res.Values[d.ConfigKey] = strings.Join(res.Multi[d.ConfigKey], ",")
		return
	}
	if d.Kind == KindBool {
		if v, err := parseBoolToken(val); err == nil {
			val = v
		}
	}
	res.Values[d.ConfigKey] = val
}

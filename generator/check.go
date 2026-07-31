package generator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CheckRules is the structured form of a field's check tag (codegen only).
type CheckRules struct {
	Required bool
	Min      *float64
	Max      *float64
	MinLen   *int
	MaxLen   *int
	Len      *int
	Pattern  string
	Email    bool
	UUID     bool
	Date     bool
	Time     bool
	DateTime bool
}

// HasValidation reports whether the check rules can reject a bound value. Every
// check rule can; defaults live in DefaultRule precisely because they cannot.
// Enums can too, but are their own tag: see FieldPlan.HasValidation.
func (c CheckRules) HasValidation() bool {
	return c.Required ||
		c.Min != nil || c.Max != nil ||
		c.MinLen != nil || c.MaxLen != nil || c.Len != nil ||
		c.Pattern != "" ||
		c.Email || c.UUID || c.Date || c.Time || c.DateTime
}

// ParseCheckTag parses a check tag value for a field of the given Go kind.
// Invalid syntax, unknown rules, type mismatches, and invalid patterns fail here.
func ParseCheckTag(raw, kind string) (CheckRules, error) {
	var c CheckRules
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return c, nil
	}

	rest := raw
	if i := indexPatternToken(raw); i >= 0 {
		// pattern= must be last token; value may contain commas
		c.Pattern = raw[i+len("pattern="):]
		if c.Pattern == "" {
			return c, fmt.Errorf("check: empty pattern")
		}
		if _, err := regexp.Compile(c.Pattern); err != nil {
			return c, fmt.Errorf("check: invalid pattern: %w", err)
		}
		rest = strings.TrimSpace(strings.TrimSuffix(raw[:i], ","))
	}

	if rest != "" {
		for _, tok := range strings.Split(rest, ",") {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			if err := applyCheckToken(&c, tok); err != nil {
				return CheckRules{}, err
			}
		}
	}

	if err := validateCheckAgainstKind(c, kind); err != nil {
		return CheckRules{}, err
	}
	return c, nil
}

func indexPatternToken(s string) int {
	if strings.HasPrefix(s, "pattern=") {
		return 0
	}
	if i := strings.Index(s, ",pattern="); i >= 0 {
		return i + 1 // point at 'p'
	}
	return -1
}

func applyCheckToken(c *CheckRules, tok string) error {
	if key, val, ok := strings.Cut(tok, "="); ok {
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "min":
			n, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf("check: invalid min %q", val)
			}
			c.Min = &n
		case "max":
			n, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf("check: invalid max %q", val)
			}
			c.Max = &n
		case "minlen":
			n, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("check: invalid minlen %q", val)
			}
			c.MinLen = &n
		case "maxlen":
			n, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("check: invalid maxlen %q", val)
			}
			c.MaxLen = &n
		case "len":
			n, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("check: invalid len %q", val)
			}
			c.Len = &n
		case "enum":
			return fmt.Errorf("check: enum is not a check rule; use the struct tag enum:%q instead", strings.ReplaceAll(val, "|", ","))
		case "default":
			return fmt.Errorf("check: default is not a check rule; use the struct tag default:%q instead", val)
		case "pattern":
			return fmt.Errorf("check: pattern= must be the last rule")
		default:
			return fmt.Errorf("check: unknown rule %q", key)
		}
		return nil
	}

	switch tok {
	case "required":
		c.Required = true
	case "email":
		c.Email = true
	case "uuid":
		c.UUID = true
	case "date":
		c.Date = true
	case "time":
		c.Time = true
	case "datetime":
		c.DateTime = true
	default:
		return fmt.Errorf("check: unknown rule %q", tok)
	}
	return nil
}

func validateCheckAgainstKind(c CheckRules, kind string) error {
	isNum := kind == "int" || kind == "int64" || kind == "float64"
	isString := kind == "string"
	isFile := kind == "file"
	isRest := kind == KindRestAny || kind == KindRestRaw
	isComposite := kind == KindStruct || kind == KindSlice || kind == KindMap
	if isFile || isRest || isComposite {
		// Nested/file/rest content rules deferred; only required is allowed for now.
		if c.Min != nil || c.Max != nil || c.MinLen != nil || c.MaxLen != nil || c.Len != nil ||
			c.Pattern != "" ||
			c.Email || c.UUID || c.Date || c.Time || c.DateTime {
			what := "file"
			if isRest {
				what = "rest map"
			} else if isComposite {
				what = "nested " + kind
			}
			return fmt.Errorf("check: only required is supported for %s fields in v1", what)
		}
		return nil
	}
	if c.Min != nil || c.Max != nil {
		if !isNum {
			return fmt.Errorf("check: min/max only apply to numeric types, not %s", kind)
		}
	}
	if c.MinLen != nil || c.MaxLen != nil || c.Len != nil {
		if !isString {
			return fmt.Errorf("check: minlen/maxlen/len only apply to string, not %s", kind)
		}
	}
	if c.Email || c.UUID || c.Date || c.Time || c.DateTime || c.Pattern != "" {
		if !isString {
			return fmt.Errorf("check: format/pattern rules only apply to string, not %s", kind)
		}
	}
	return nil
}

// defaultGoLiteral returns a Go source literal for a default/enum value of kind.
func defaultGoLiteral(kind, val string) (string, error) {
	switch kind {
	case "string":
		return strconv.Quote(val), nil
	case "int":
		n, err := strconv.Atoi(val)
		if err != nil {
			return "", err
		}
		return strconv.Itoa(n), nil
	case "int64":
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("int64(%d)", n), nil
	case "bool":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return "", err
		}
		return strconv.FormatBool(b), nil
	case "float64":
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(n, 'g', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported kind %s", kind)
	}
}

// checkLocation maps a field source to a validation Field location string.
func checkLocation(src FieldSource) string {
	switch src {
	case SourceInput:
		return "input"
	case SourceQuery:
		return "query"
	case SourcePayload:
		return "payload"
	case SourcePath:
		return "path"
	case SourceHeader:
		return "header"
	case SourceCookie:
		return "cookie"
	case SourceMethod:
		return "method"
	default:
		return string(src)
	}
}

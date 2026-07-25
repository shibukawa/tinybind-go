package htmlbind

import (
	"fmt"
	"strings"
)

// styleScope records the generation-time renaming applied to one component's
// style block. Class names are renamed so two components can declare the same
// name, and keyframes names follow because they are referenced only from within
// CSS. Names shared with the outside world, such as font families and custom
// properties, stay global.
type styleScope struct {
	suffix    string
	classes   map[string]string
	keyframes map[string]string
}

// className returns the emitted name for a class used in component markup.
// A class the style block does not declare passes through, so utility classes
// from an external framework keep working.
func (s *styleScope) className(name string) string {
	if s == nil {
		return name
	}
	if renamed, ok := s.classes[name]; ok {
		return renamed
	}
	return name
}

// rewriteCSS scopes a component style block, returning the rewritten CSS and
// the renaming that markup must follow.
func rewriteCSS(source, suffix string) (string, *styleScope, error) {
	scope := &styleScope{suffix: suffix, classes: map[string]string{}, keyframes: map[string]string{}}
	collectKeyframes(source, scope)
	var out strings.Builder
	if err := rewriteRules(source, scope, &out); err != nil {
		return "", nil, err
	}
	return out.String(), scope, nil
}

// collectKeyframes records every keyframes name declared anywhere in the block
// so references can be rewritten even when they appear before the definition.
func collectKeyframes(source string, scope *styleScope) {
	rest := source
	for {
		index := strings.Index(rest, "@")
		if index < 0 {
			return
		}
		rest = rest[index:]
		prelude, _, remainder, ok := splitBlock(rest)
		if !ok {
			return
		}
		if name, isKeyframes := keyframesName(prelude); isKeyframes {
			scope.keyframes[name] = name + "_" + scope.suffix
		}
		rest = remainder
	}
}

func rewriteRules(source string, scope *styleScope, out *strings.Builder) error {
	rest := source
	for {
		trimmed := strings.TrimLeft(rest, " \t\r\n")
		out.WriteString(rest[:len(rest)-len(trimmed)])
		rest = trimmed
		if rest == "" {
			return nil
		}
		if strings.HasPrefix(rest, "/*") {
			end := strings.Index(rest, "*/")
			if end < 0 {
				return fmt.Errorf("unterminated CSS comment")
			}
			out.WriteString(rest[:end+2])
			rest = rest[end+2:]
			continue
		}
		prelude, body, remainder, ok := splitBlock(rest)
		if !ok {
			// A statement at-rule such as @import ends at a semicolon.
			if end := strings.IndexByte(rest, ';'); end >= 0 {
				out.WriteString(rest[:end+1])
				rest = rest[end+1:]
				continue
			}
			return fmt.Errorf("unterminated CSS rule")
		}
		if err := rewriteBlock(prelude, body, scope, out); err != nil {
			return err
		}
		rest = remainder
	}
}

func rewriteBlock(prelude, body string, scope *styleScope, out *strings.Builder) error {
	trimmed := strings.TrimSpace(prelude)
	if strings.HasPrefix(trimmed, "@") {
		if name, isKeyframes := keyframesName(trimmed); isKeyframes {
			out.WriteString(strings.Replace(prelude, name, scope.keyframes[name], 1))
			out.WriteString("{")
			out.WriteString(rewriteDeclarations(body, scope))
			out.WriteString("}")
			return nil
		}
		if isConditionalAtRule(trimmed) {
			out.WriteString(prelude)
			out.WriteString("{")
			if err := rewriteRules(body, scope, out); err != nil {
				return err
			}
			out.WriteString("}")
			return nil
		}
		out.WriteString(prelude)
		out.WriteString("{")
		out.WriteString(body)
		out.WriteString("}")
		return nil
	}
	selector, err := rewriteSelectorList(prelude, scope)
	if err != nil {
		return err
	}
	out.WriteString(selector)
	out.WriteString("{")
	out.WriteString(rewriteDeclarations(body, scope))
	out.WriteString("}")
	return nil
}

// rewriteSelectorList scopes each selector by renaming its classes. A selector
// carrying no class cannot be scoped, so it is rejected instead of leaking to
// every page that loads the stylesheet.
func rewriteSelectorList(list string, scope *styleScope) (string, error) {
	parts := splitTopLevel(list, ',')
	for i, part := range parts {
		rewritten, classes, global := rewriteSelector(part, scope)
		if classes == 0 && !global {
			return "", fmt.Errorf("selector %q has no class to scope; add a class or wrap it in :global()", strings.TrimSpace(part))
		}
		parts[i] = rewritten
	}
	return strings.Join(parts, ","), nil
}

func rewriteSelector(selector string, scope *styleScope) (string, int, bool) {
	var out strings.Builder
	classes := 0
	global := false
	for i := 0; i < len(selector); {
		if strings.HasPrefix(selector[i:], ":global(") {
			depth := 0
			start := i
			for i < len(selector) {
				if selector[i] == '(' {
					depth++
				} else if selector[i] == ')' {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				i++
			}
			inner := selector[start+len(":global(") : i-1]
			out.WriteString(inner)
			global = true
			continue
		}
		if selector[i] == '.' {
			name, width := readCSSIdent(selector[i+1:])
			if width > 0 {
				renamed, ok := scope.classes[name]
				if !ok {
					renamed = name + "_" + scope.suffix
					scope.classes[name] = renamed
				}
				out.WriteString("." + renamed)
				classes++
				i += 1 + width
				continue
			}
		}
		out.WriteByte(selector[i])
		i++
	}
	return out.String(), classes, global
}

// rewriteDeclarations renames keyframes references so a scoped animation binds
// to the scoped definition. Other declaration values are left untouched.
func rewriteDeclarations(body string, scope *styleScope) string {
	if len(scope.keyframes) == 0 {
		return body
	}
	parts := splitTopLevel(body, ';')
	for i, part := range parts {
		name, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(name) {
		case "animation", "animation-name":
			parts[i] = name + ":" + renameKeyframeRefs(value, scope)
		}
	}
	return strings.Join(parts, ";")
}

func renameKeyframeRefs(value string, scope *styleScope) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		name, width := readCSSIdent(value[i:])
		if width == 0 {
			out.WriteByte(value[i])
			i++
			continue
		}
		if renamed, ok := scope.keyframes[name]; ok {
			out.WriteString(renamed)
		} else {
			out.WriteString(name)
		}
		i += width
	}
	return out.String()
}

// splitBlock separates the prelude and brace-balanced body of the first rule in
// source, returning the remainder after it.
func splitBlock(source string) (prelude, body, rest string, ok bool) {
	open := -1
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case ';':
			if open < 0 {
				return "", "", "", false
			}
		case '{':
			open = i
		}
		if open >= 0 {
			break
		}
	}
	if open < 0 {
		return "", "", "", false
	}
	depth := 0
	for i := open; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[:open], source[open+1 : i], source[i+1:], true
			}
		}
	}
	return "", "", "", false
}

// splitTopLevel splits on sep outside parentheses, brackets, and strings.
func splitTopLevel(value string, sep byte) []string {
	var parts []string
	depth := 0
	quote := byte(0)
	start := 0
	for i := 0; i < len(value); i++ {
		c := value[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, value[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, value[start:])
}

func readCSSIdent(value string) (string, int) {
	i := 0
	for i < len(value) {
		c := value[i]
		if c == '-' || c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (i > 0 && c >= '0' && c <= '9') {
			i++
			continue
		}
		break
	}
	return value[:i], i
}

func keyframesName(prelude string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(prelude))
	if len(fields) < 2 {
		return "", false
	}
	rule := strings.ToLower(fields[0])
	if rule != "@keyframes" && !strings.HasSuffix(rule, "-keyframes") {
		return "", false
	}
	return fields[1], true
}

func isConditionalAtRule(prelude string) bool {
	fields := strings.Fields(strings.TrimSpace(prelude))
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "@media", "@supports", "@container", "@layer":
		return true
	}
	return false
}

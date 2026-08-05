package generator

import (
	"fmt"
	"go/token"
	"unicode"
)

// resolverAlias is the import alias generated NoSQL query code gives a framework
// resolver package. It matches the spelling the SQL emitter already uses, and it
// is deliberately unwritable by hand so it cannot collide with a package the
// declaration's own types come from.
const resolverAlias = "_tinybindresolver"

// checkHandleResolver rejects a resolver that could not be called from generated
// code, naming the option so the error says which one to fix.
//
// It is checked at generation time rather than left to the Go compiler, because
// the failure would otherwise surface as an unbuildable generated file whose
// cause is one setting in a config the reader is not looking at.
func checkHandleResolver(option string, resolver *SymbolPattern) error {
	if resolver == nil {
		return nil
	}
	if resolver.PackagePath == "" {
		return fmt.Errorf("%s names no package path", option)
	}
	if !token.IsIdentifier(resolver.Name) || !unicode.IsUpper([]rune(resolver.Name)[0]) {
		return fmt.Errorf("%s %q.%q is not an exported function name", option, resolver.PackagePath, resolver.Name)
	}
	return nil
}

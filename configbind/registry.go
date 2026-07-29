package configbind

import (
	"fmt"
	"sync"

	"github.com/shibukawa/tinybind-go/cliparser"
)

// ApplyFunc applies an overlay onto a destination pointer without reflection.
type ApplyFunc func(dst any, o *Overlay) error

// Definition describes one generated Bind target and its scaffold fields.
type Definition struct {
	// TypeName is the package-qualified Go type identity used for diagnostics.
	TypeName string
	// Prefix is the configuration key prefix passed to Bind.
	Prefix string
	// Doc is the config struct's godoc text, rendered above the scaffold table.
	Doc string
	// KnownKeys lists stable config keys for env and provenance.
	KnownKeys []string
	// FlagMetas builds cliparser defs for this type's fields.
	FlagMetas []cliparser.FieldMeta
	// Defaults maps stable keys to default raw strings applied when absent.
	Defaults map[string]string
	// DependsOn maps a stable key to every parent it answers to: its own
	// dependon tag plus the tags of the structs it sits under. One empty parent
	// is enough to hide the key from provenance output; nothing else.
	DependsOn map[string][]string
	// Falsy maps a stable key to the choice from its falsy tag. The key resolves
	// to that value when nothing sets it and it has no default, and the value
	// counts as empty when other keys depend on this one.
	Falsy map[string]string
	// Secrets maps a stable key to its secret tag: hide, mask, or show. A key
	// with no entry follows the key-name policy in displayValue.
	Secrets map[string]string
	// Apply writes overlay values into *T (dst must be *T).
	Apply ApplyFunc
	// Scaffold contains the leaf fields used to render example configuration.
	Scaffold []ScaffoldField
}

var (
	definitionsMu sync.RWMutex
	definitions   = map[bindingKey]Definition{}
)

type typeMarker[T any] struct{}
type bindingKey struct {
	typeID any
	prefix string
}

func typeKey[T any]() any { return typeMarker[T]{} }

// Register installs one generated definition.
func Register[T any](definition Definition) {
	if definition.TypeName == "" || definition.Prefix == "" || definition.Apply == nil {
		panic("configbind: Register requires TypeName, Prefix, and Apply")
	}
	definition.KnownKeys = append([]string(nil), definition.KnownKeys...)
	definition.FlagMetas = append([]cliparser.FieldMeta(nil), definition.FlagMetas...)
	definition.Scaffold = append([]ScaffoldField(nil), definition.Scaffold...)
	definitionsMu.Lock()
	key := bindingKey{typeID: typeKey[T](), prefix: definition.Prefix}
	if _, exists := definitions[key]; exists {
		definitionsMu.Unlock()
		panic(fmt.Sprintf("configbind: duplicate definition for %s with prefix %q", definition.TypeName, definition.Prefix))
	}
	definitions[key] = definition
	definitionsMu.Unlock()
}

func definitionFor[T any](prefix string) (Definition, bool) {
	definitionsMu.RLock()
	defer definitionsMu.RUnlock()
	definition, ok := definitions[bindingKey{typeID: typeKey[T](), prefix: prefix}]
	return definition, ok
}

func snapshotDefinitions() []Definition {
	definitionsMu.RLock()
	defer definitionsMu.RUnlock()
	out := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, definition)
	}
	return out
}

// target is one Bind registration awaiting Load.
type target struct {
	prefix   string
	typeName string
	dst      any
	meta     Definition
}

var (
	targetsMu sync.Mutex
	targets   []target
)

// Bind allocates *T, registers it for the next Load, and returns the pointer.
// Code generation must Register[T] before Bind is used.
func Bind[T any](prefix string) *T {
	meta, ok := definitionFor[T](prefix)
	if !ok {
		panic(fmt.Sprintf("configbind: type/prefix not registered; run go generate (Bind[%T](%q))", *new(T), prefix))
	}
	dst := new(T)
	targetsMu.Lock()
	targets = append(targets, target{
		prefix:   prefix,
		typeName: meta.TypeName,
		dst:      dst,
		meta:     meta,
	})
	targetsMu.Unlock()
	return dst
}

// ResetDefinitions clears generated definitions. It is intended for tests.
func ResetDefinitions() {
	definitionsMu.Lock()
	definitions = map[bindingKey]Definition{}
	definitionsMu.Unlock()
	resetSubcommandDefinitions()
}

// ResetTargets clears Bind registrations (tests only).
func ResetTargets() {
	targetsMu.Lock()
	targets = nil
	targetsMu.Unlock()
	resetSubcommandTargets()
}

func snapshotTargets() []target {
	targetsMu.Lock()
	defer targetsMu.Unlock()
	out := make([]target, len(targets))
	copy(out, targets)
	return out
}

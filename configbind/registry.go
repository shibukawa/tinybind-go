package configbind

import (
	"fmt"
	"sync"

	"github.com/shibukawa/tinybind-go/cliparser"
)

// ApplyFunc applies an overlay onto a destination pointer without reflection.
type ApplyFunc func(dst any, o *Overlay) error

// Meta describes generated key tables and flags for one Bind target type.
type Meta struct {
	// TypeName is the package-qualified Go type identity used for diagnostics.
	TypeName string
	// KnownKeys lists stable config keys for env and provenance.
	KnownKeys []string
	// FlagMetas builds cliparser defs for this type's fields.
	FlagMetas []cliparser.FieldMeta
	// Defaults maps stable keys to default raw strings applied when absent.
	Defaults map[string]string
	// Apply writes overlay values into *T (dst must be *T).
	Apply ApplyFunc
}

var (
	metaMu   sync.RWMutex
	metaByTy = map[string]Meta{} // TypeName -> Meta
)

// RegisterMeta registers legacy metadata by a caller-supplied identity.
// New generated code uses RegisterBinding so package and prefix identity cannot collide.
func RegisterMeta(m Meta) {
	if m.TypeName == "" || m.Apply == nil {
		panic("configbind: RegisterMeta requires TypeName and Apply")
	}
	metaMu.Lock()
	metaByTy[m.TypeName] = m
	metaMu.Unlock()
}

func lookupMeta(typeName string) (Meta, bool) {
	metaMu.RLock()
	defer metaMu.RUnlock()
	m, ok := metaByTy[typeName]
	return m, ok
}

// Typed metadata avoids collisions between same-named types in different packages.
var (
	typeMu       sync.RWMutex
	typeMetas    = map[any]Meta{}
	bindingMetas = map[bindingKey]Meta{}
)

type typeMarker[T any] struct{}
type bindingKey struct {
	typeID any
	prefix string
}

func typeKey[T any]() any { return typeMarker[T]{} }

// RegisterType associates T with prefix-independent legacy metadata.
// New generated code uses RegisterBinding.
func RegisterType[T any](typeName string, m Meta) {
	m.TypeName = typeName
	RegisterMeta(m)
	typeMu.Lock()
	typeMetas[typeKey[T]()] = m
	typeMu.Unlock()
}

// RegisterBinding associates T and one generated Bind prefix with metadata.
// Unlike RegisterType, it supports using the same type with multiple prefixes.
func RegisterBinding[T any](prefix, typeName string, m Meta) {
	if prefix == "" {
		panic("configbind: RegisterBinding requires prefix")
	}
	m.TypeName = typeName
	typeMu.Lock()
	bindingMetas[bindingKey{typeID: typeKey[T](), prefix: prefix}] = m
	typeMu.Unlock()
}

func metaForBinding[T any](prefix string) (Meta, bool) {
	typeMu.RLock()
	defer typeMu.RUnlock()
	if m, ok := bindingMetas[bindingKey{typeID: typeKey[T](), prefix: prefix}]; ok {
		return m, true
	}
	m, ok := typeMetas[typeKey[T]()]
	return m, ok
}

// target is one Bind registration awaiting Load.
type target struct {
	prefix   string
	typeName string
	dst      any
	meta     Meta
}

var (
	targetsMu sync.Mutex
	targets   []target
)

// Bind allocates *T, registers it for the next Load, and returns the pointer.
// Code generation must RegisterBinding[T] before Bind is used.
func Bind[T any](prefix string) *T {
	meta, ok := metaForBinding[T](prefix)
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

// ResetTargets clears Bind registrations (tests only).
func ResetTargets() {
	targetsMu.Lock()
	targets = nil
	targetsMu.Unlock()
}

func snapshotTargets() []target {
	targetsMu.Lock()
	defer targetsMu.Unlock()
	out := make([]target, len(targets))
	copy(out, targets)
	return out
}

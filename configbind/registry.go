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
	// TypeName is the Go type name used for registration (e.g. "WebServerConfig").
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

// RegisterMeta registers generated apply and key metadata for a type name.
// Called from generated init functions.
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

// typeNames maps a zero pointer of T to its registered type name via BindType registration.
var (
	typeMu    sync.RWMutex
	typeNames = map[any]string{} // (*T)(nil) identity via named marker
)

type typeMarker[T any] struct{}

func typeKey[T any]() any { return typeMarker[T]{} }

// RegisterType associates a Go type parameter T with its generated type name and meta.
func RegisterType[T any](typeName string, m Meta) {
	m.TypeName = typeName
	RegisterMeta(m)
	typeMu.Lock()
	typeNames[typeKey[T]()] = typeName
	typeMu.Unlock()
}

func typeNameOf[T any]() (string, bool) {
	typeMu.RLock()
	defer typeMu.RUnlock()
	n, ok := typeNames[typeKey[T]()]
	return n, ok
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
// Code generation must RegisterType[T] before Bind is used.
func Bind[T any](prefix string) *T {
	name, ok := typeNameOf[T]()
	if !ok {
		panic(fmt.Sprintf("configbind: type not registered; run go generate (Bind[%T])", *new(T)))
	}
	meta, ok := lookupMeta(name)
	if !ok {
		panic(fmt.Sprintf("configbind: missing meta for %s", name))
	}
	dst := new(T)
	targetsMu.Lock()
	targets = append(targets, target{
		prefix:   prefix,
		typeName: name,
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

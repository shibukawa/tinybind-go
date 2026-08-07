//go:build !tinygo

package sqlbind

import (
	"fmt"
	"reflect"
	"sync"
)

func typeKey[T any]() reflect.Type { return reflect.TypeFor[T]() }

type scanRowsFunc func(Rows) (any, error)

var scanners sync.Map

// RegisterScanRows registers a generated SQL tree scanner for T.
func RegisterScanRows[T any](fn func(Rows) ([]T, error)) {
	scanners.Store(typeKey[T](), scanRowsFunc(func(rows Rows) (any, error) { return fn(rows) }))
}

func lookupScanner(t any) (scanRowsFunc, bool) {
	v, ok := scanners.Load(t)
	if !ok {
		return nil, false
	}
	return v.(scanRowsFunc), true
}

func missingScannerError(t interface{ String() string }) error {
	return fmt.Errorf("sqlbind: no SQL scanner registered for %s", t.String())
}

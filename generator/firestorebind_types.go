package generator

import (
	"fmt"
	"go/types"
)

// resolve maps one Go type onto the property value it is stored as, and rejects
// what Datastore cannot express.
func (c *firestoreCollector) resolve(t types.Type, typeName, fieldName string) (FirestoreType, error) {
	goType := c.goString(t)
	fail := func(format string, args ...any) (FirestoreType, error) {
		return FirestoreType{}, fmt.Errorf("firestorebind: %s.%s: "+format, append([]any{typeName, fieldName}, args...)...)
	}

	if isDatastoreValue(t) {
		return FirestoreType{Kind: FirestoreRaw, Go: goType}, nil
	}
	if isDatastoreKey(t) {
		return FirestoreType{Kind: FirestoreKeyRef, Go: goType}, nil
	}
	if isDatastoreLatLng(t) {
		return FirestoreType{Kind: FirestoreGeo, Go: goType}, nil
	}
	if isTimeTime(t) {
		return FirestoreType{Kind: FirestoreTime, Go: goType}, nil
	}

	switch underlying := t.Underlying().(type) {
	case *types.Basic:
		if reason, wide := tooWideForInt64(underlying); wide {
			return fail("%s", reason)
		}
		kind, bits, ok := basicFirestoreKind(underlying)
		if !ok {
			return fail("%s has no Datastore property type", goType)
		}
		return FirestoreType{Kind: kind, Go: goType, Bits: bits}, nil

	case *types.Slice:
		if isByteSlice(underlying) {
			return FirestoreType{Kind: FirestoreBlob, Go: goType}, nil
		}
		elem, err := c.resolve(underlying.Elem(), typeName, fieldName)
		if err != nil {
			return FirestoreType{}, err
		}
		if elem.Kind == FirestoreArray {
			return fail("an array of arrays is not a Datastore value")
		}
		return FirestoreType{Kind: FirestoreArray, Go: goType, Elem: &elem}, nil

	case *types.Map:
		// Datastore has no map type, so a map would have to become a nested
		// entity whose property names come from run-time data. Names coming
		// from anywhere but a tag is the one thing this codec exists to
		// prevent, and the driver's own mapper refuses maps for the same
		// reason.
		return fail("a map has no Datastore property type; its property names would come from run-time data. Use a nested struct, or datastore.Value where the names really are dynamic")

	case *types.Pointer:
		elem, err := c.resolve(underlying.Elem(), typeName, fieldName)
		if err != nil {
			return FirestoreType{}, err
		}
		if elem.Kind == FirestorePointer {
			return fail("a pointer to a pointer has no property form")
		}
		return FirestoreType{Kind: FirestorePointer, Go: goType, Elem: &elem}, nil

	case *types.Struct:
		named, ok := types.Unalias(t).(*types.Named)
		if !ok || named.Obj() == nil {
			return fail("an anonymous struct field has no generated codec; declare a named type")
		}
		if named.Obj().Pkg() == nil || c.pkg.Types == nil || named.Obj().Pkg().Path() != c.pkg.Types.Path() {
			return fail("nested type %s is declared in another package, where no codec can be generated for it", goType)
		}
		return FirestoreType{Kind: FirestoreStruct, Go: goType, Struct: named.Obj().Name()}, nil

	default:
		return fail("%s has no Datastore property type", goType)
	}
}

// goString writes a type as the generated file must spell it: unqualified for
// this package, package-qualified for anything else.
func (c *firestoreCollector) goString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if c.pkg != nil && c.pkg.Types != nil && p == c.pkg.Types {
			return ""
		}
		return p.Name()
	})
}

// basicFirestoreKind reports the value kind of a basic type and the width its
// text form is parsed at.
//
// Integer and Double are separate kinds rather than one number, because
// Datastore stores and orders them separately: a value written as one and read
// as the other would stop matching the filter it was written for.
func basicFirestoreKind(b *types.Basic) (FirestoreKind, int, bool) {
	switch b.Kind() {
	case types.String:
		return FirestoreString, 0, true
	case types.Bool:
		return FirestoreBool, 0, true
	case types.Int:
		return FirestoreInt, 0, true
	case types.Int8:
		return FirestoreInt, 8, true
	case types.Int16:
		return FirestoreInt, 16, true
	case types.Int32:
		return FirestoreInt, 32, true
	case types.Int64:
		return FirestoreInt, 64, true
	case types.Uint8:
		return FirestoreUint, 8, true
	case types.Uint16:
		return FirestoreUint, 16, true
	case types.Uint32:
		return FirestoreUint, 32, true
	// uint, uint64 and uintptr never reach here: tooWideForInt64 rejects them
	// before this mapping is consulted.
	case types.Float32:
		return FirestoreDouble, 32, true
	case types.Float64:
		return FirestoreDouble, 64, true
	default:
		return "", 0, false
	}
}

// tooWideForInt64 reports the unsigned kinds whose range exceeds int64, which is
// the whole of Datastore's integer type.
//
// This is where the two NoSQL backends genuinely differ: a DynamoDB number is
// arbitrary-precision text, so data:dynamodb-attribute-mapping takes every
// unsigned kind, while proto3 integerValue is an int64 wearing a string. The
// driver refuses to marshal a wider one at all, so a field that accepted it
// would compile, store small values for months, and fail on the first large
// one. Rejecting the field is the loud version of the same fact.
func tooWideForInt64(b *types.Basic) (string, bool) {
	switch b.Kind() {
	case types.Uint64:
		return "uint64 exceeds the int64 a Datastore integer holds; use int64, or a string property when the value really is that wide", true
	case types.Uint:
		return "uint is 64 bits on the platforms this targets, and exceeds the int64 a Datastore integer holds; use int64 or a narrower unsigned type", true
	case types.Uintptr:
		return "uintptr has no Datastore property type", true
	}
	return "", false
}

func isDatastoreValue(t types.Type) bool { return isDatastoreNamed(t, "Value") }
func isDatastoreKey(t types.Type) bool   { return isDatastoreNamed(t, "Key") }
func isDatastoreLatLng(t types.Type) bool {
	return isDatastoreNamed(t, "LatLng")
}

func isDatastoreNamed(t types.Type, name string) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == datastoreImportPath && named.Obj().Name() == name
}

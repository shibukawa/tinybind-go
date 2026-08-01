package generator

import (
	"fmt"
	"go/types"
)

// resolve maps one Go type onto the attribute it is stored as, and rejects what
// DynamoDB cannot express. Rejecting is the point: the driver's reflection path
// reads an unsupported field as nothing in particular, and a generator can say
// which field and why instead.
func (c *dynamoCollector) resolve(t types.Type, typeName, fieldName string, set DynamoKind, unixTime bool) (DynamoType, error) {
	goType := c.goString(t)
	fail := func(format string, args ...any) (DynamoType, error) {
		return DynamoType{}, fmt.Errorf("dynamobind: %s.%s: "+format, append([]any{typeName, fieldName}, args...)...)
	}

	if isDynamoAttributeValue(t) {
		if set != "" || unixTime {
			return fail("a %s field takes no tag option", goType)
		}
		return DynamoType{Kind: DynamoRaw, Go: goType}, nil
	}
	if isTimeTime(t) {
		if set != "" {
			return fail("a time.Time field is not a set")
		}
		if unixTime {
			return DynamoType{Kind: DynamoUnixTime, Go: goType}, nil
		}
		return DynamoType{Kind: DynamoTime, Go: goType}, nil
	}
	if unixTime {
		return fail("unixtime applies to time.Time, not %s", goType)
	}

	switch underlying := t.Underlying().(type) {
	case *types.Basic:
		if set != "" {
			return fail("%s is not a slice, so it cannot be a set", goType)
		}
		kind, bits, ok := basicDynamoKind(underlying)
		if !ok {
			return fail("%s has no DynamoDB attribute type", goType)
		}
		return DynamoType{Kind: kind, Go: goType, Bits: bits}, nil

	case *types.Slice:
		if isByteSlice(underlying) {
			if set != "" {
				return fail("%s is binary, not a set of binaries", goType)
			}
			return DynamoType{Kind: DynamoBytes, Go: goType}, nil
		}
		elem, err := c.resolve(underlying.Elem(), typeName, fieldName, "", false)
		if err != nil {
			return DynamoType{}, err
		}
		if set != "" {
			if err := checkSetElem(set, elem); err != nil {
				return fail("%s: %w", goType, err)
			}
			return DynamoType{Kind: set, Go: goType, Elem: &elem}, nil
		}
		return DynamoType{Kind: DynamoList, Go: goType, Elem: &elem}, nil

	case *types.Map:
		if set != "" {
			return fail("%s is a map, not a set", goType)
		}
		if key, ok := underlying.Key().Underlying().(*types.Basic); !ok || key.Kind() != types.String {
			return fail("a map attribute needs a string key, and %s does not have one", goType)
		}
		elem, err := c.resolve(underlying.Elem(), typeName, fieldName, "", false)
		if err != nil {
			return DynamoType{}, err
		}
		return DynamoType{Kind: DynamoMap, Go: goType, Elem: &elem, MapKey: c.goString(underlying.Key())}, nil

	case *types.Pointer:
		if set != "" {
			return fail("%s is a pointer, not a set", goType)
		}
		elem, err := c.resolve(underlying.Elem(), typeName, fieldName, "", false)
		if err != nil {
			return DynamoType{}, err
		}
		if elem.Kind == DynamoPointer {
			return fail("a pointer to a pointer has no attribute form")
		}
		return DynamoType{Kind: DynamoPointer, Go: goType, Elem: &elem}, nil

	case *types.Struct:
		if set != "" {
			return fail("%s is a struct, not a set", goType)
		}
		named, ok := types.Unalias(t).(*types.Named)
		if !ok || named.Obj() == nil {
			return fail("an anonymous struct field has no generated codec; declare a named type")
		}
		if named.Obj().Pkg() == nil || c.pkg.Types == nil || named.Obj().Pkg().Path() != c.pkg.Types.Path() {
			return fail("nested type %s is declared in another package, where no codec can be generated for it", goType)
		}
		return DynamoType{Kind: DynamoStruct, Go: goType, Struct: named.Obj().Name()}, nil

	default:
		return fail("%s has no DynamoDB attribute type", goType)
	}
}

// goString writes a type as the generated file must spell it: unqualified for
// this package, package-qualified for anything else.
func (c *dynamoCollector) goString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if c.pkg != nil && c.pkg.Types != nil && p == c.pkg.Types {
			return ""
		}
		return p.Name()
	})
}

// basicDynamoKind reports the attribute kind of a basic type and the width its
// text form is parsed at. Width 0 means the platform-sized int or uint, which is
// what strconv reads a 0 bit size as.
func basicDynamoKind(b *types.Basic) (DynamoKind, int, bool) {
	switch b.Kind() {
	case types.String:
		return DynamoString, 0, true
	case types.Bool:
		return DynamoBool, 0, true
	case types.Int:
		return DynamoInt, 0, true
	case types.Int8:
		return DynamoInt, 8, true
	case types.Int16:
		return DynamoInt, 16, true
	case types.Int32:
		return DynamoInt, 32, true
	case types.Int64:
		return DynamoInt, 64, true
	case types.Uint, types.Uintptr:
		return DynamoUint, 0, true
	case types.Uint8:
		return DynamoUint, 8, true
	case types.Uint16:
		return DynamoUint, 16, true
	case types.Uint32:
		return DynamoUint, 32, true
	case types.Uint64:
		return DynamoUint, 64, true
	case types.Float32:
		return DynamoFloat, 32, true
	case types.Float64:
		return DynamoFloat, 64, true
	default:
		// Complex, unsafe.Pointer and the untyped kinds land here.
		return "", 0, false
	}
}

func checkSetElem(set DynamoKind, elem DynamoType) error {
	switch set {
	case DynamoStringSet:
		if elem.Kind != DynamoString {
			return fmt.Errorf("stringset needs string elements, got %s", elem.Go)
		}
	case DynamoNumberSet:
		switch elem.Kind {
		case DynamoInt, DynamoUint, DynamoFloat:
		default:
			return fmt.Errorf("numberset needs numeric elements, got %s", elem.Go)
		}
	case DynamoBinarySet:
		if elem.Kind != DynamoBytes {
			return fmt.Errorf("binaryset needs []byte elements, got %s", elem.Go)
		}
	}
	return nil
}

func isByteSlice(s *types.Slice) bool {
	basic, ok := s.Elem().Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Uint8
}

func isTimeTime(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "time" && named.Obj().Name() == "Time"
}

func isDynamoAttributeValue(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == dynamodbImportPath && named.Obj().Name() == "AttributeValue"
}

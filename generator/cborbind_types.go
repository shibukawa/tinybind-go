package generator

import (
	"fmt"
	"go/types"
)

// CborProfile is which of the driver's two named profiles a codec is generated
// for. It is part of the contract rather than a generator setting: the emitted
// code pins it, so a wire codec will not read a world message.
type CborProfile string

const (
	// CborWire is the frozen realtime format: a fixed-length array, no field
	// names, and no room for a field the schema does not know.
	CborWire CborProfile = "wire"
	// CborWorld is the evolvable one: a map with deterministic key order, whose
	// decoder skips a field it does not know.
	CborWorld CborProfile = "world"
)

// CborKind is how one Go type reaches CBOR.
type CborKind string

const (
	CborUint  CborKind = "uint"
	CborInt   CborKind = "int"
	CborBool  CborKind = "bool"
	CborText  CborKind = "text"
	CborBytes CborKind = "bytes"
	// CborStruct is a struct this run also plans, reached through the function
	// emitted for it rather than through any interface.
	CborStruct CborKind = "struct"
	CborSlice  CborKind = "slice"
	// CborSelf is a type carrying its own AppendCBORTo or DecodeCBORFrom.
	//
	// It covers two cases the generator need not tell apart: a type from a
	// package this analysis cannot walk, and a type declared right here whose
	// author wrote the methods deliberately. The second is how a fixed-point
	// scale reaches the wire -- one declared type per scale, each converting in
	// its own methods -- so this kind is checked before every other.
	CborSelf CborKind = "self"
)

// CborType describes one Go type in CBOR terms.
type CborType struct {
	Kind CborKind
	// Go is the type as the generated package must spell it.
	Go string
	// Bits is the width an integer is read at: 8, 16, 32 or 64, and never the
	// platform width, which is refused. Reading at the field's own width turns
	// a value it cannot hold into an error instead of a silent wrap.
	Bits int
	// Named is the declared type when it is not the underlying spelling, so
	// generated code can convert in both directions.
	Named string
	// Struct is the named struct of a nested planned type.
	Struct string
	// Elem is a slice's element.
	Elem *CborType
	// ElemIdentity is the identity field of a slice's struct element, and
	// ElemIdentityGo the Go type of that field. Together they are what makes a
	// collection diffable element by element rather than carried whole.
	ElemIdentity   string
	ElemIdentityGo string
	// CanAppend and CanDecode record which halves a CborSelf type carries. One
	// half is enough to be admitted; which is required follows from the
	// directions the declaration named.
	CanAppend bool
	CanDecode bool
}

// resolve maps one Go type onto what it encodes as, and refuses what cannot
// encode deterministically.
//
// Refusing is the point. Every rejection below is a value that would encode,
// round trip, and then differ between two runs or two targets -- which is a
// desync found in production rather than a build error found at the keystroke
// that caused it.
func (c *cborCollector) resolve(t types.Type, typeName, fieldName string) (CborType, error) {
	goType := c.goString(t)
	fail := func(format string, args ...any) (CborType, error) {
		return CborType{}, fmt.Errorf("cborbind: %s.%s: "+format, append([]any{typeName, fieldName}, args...)...)
	}

	// A type carrying its own codec wins over everything below, including a
	// plan this run would otherwise have made for it. Generating a codec for a
	// type whose author wrote an encoder, and then using the generated one,
	// silently produces bytes the author did not intend.
	if canAppend, canDecode := c.selfCodec(t); canAppend || canDecode {
		return CborType{Kind: CborSelf, Go: goType, CanAppend: canAppend, CanDecode: canDecode}, nil
	}
	if isTimeTime(t) {
		return fail("time.Time is not a function of the tick, so it cannot encode deterministically; carry the tick, or a duration in ticks, instead")
	}

	named := ""
	if n, ok := t.(*types.Named); ok {
		if obj := n.Obj(); obj != nil {
			named = obj.Name()
		}
	}

	switch underlying := t.Underlying().(type) {
	case *types.Basic:
		kind, bits, ok := basicCborKind(underlying)
		if !ok {
			// The float rejection names what the type is underneath, because a
			// declared name looks innocent and a diagnostic naming only it
			// sends the author looking in the wrong place.
			if isFloatBasic(underlying) {
				if named != "" && named != underlying.Name() {
					return fail("%s is %s underneath, and a float cannot encode deterministically: it varies with the target's rounding and with fused multiply-add; declare a fixed-point type carrying its own AppendCBORTo instead", named, underlying.Name())
				}
				return fail("%s cannot encode deterministically: it varies with the target's rounding and with fused multiply-add; declare a fixed-point type carrying its own AppendCBORTo instead", goType)
			}
			if underlying.Kind() == types.Int || underlying.Kind() == types.Uint {
				// int is 64 bits on a host target and 32 on wasm, so the range
				// a field accepts would depend on where the binary ran. Both
				// ends of a protocol have to agree about that, and only a
				// declared width makes them.
				return fail("%s is a platform-width %s, which is 64 bits on a host target and 32 on wasm, so the two ends of a protocol would disagree about what fits; declare the width, as %s32 or %s64",
					goType, underlying.Name(), underlying.Name(), underlying.Name())
			}
			return fail("%s has no CBOR encoding", goType)
		}
		if named != "" && named == underlying.Name() {
			named = ""
		}
		return CborType{Kind: kind, Go: goType, Bits: bits, Named: named}, nil

	case *types.Slice:
		if isByteSlice(underlying) {
			return CborType{Kind: CborBytes, Go: goType, Named: named}, nil
		}
		elem, err := c.resolve(underlying.Elem(), typeName, fieldName)
		if err != nil {
			return CborType{}, err
		}
		out := CborType{Kind: CborSlice, Go: goType, Elem: &elem, Named: named}
		// The element's plan exists by now, since resolving a struct collects
		// it. Reading the identity here rather than at emission means the
		// decision to diff a collection or carry it whole is made once, in the
		// plan, where the report of it can also be written.
		if elem.Kind == CborStruct {
			if plan, ok := c.plans[cborKey{elem.Struct, c.profile}]; ok && plan.IdentityField != "" {
				out.ElemIdentity = plan.IdentityField
				for _, f := range plan.Fields {
					if f.Name == plan.IdentityField {
						out.ElemIdentityGo = f.Type.Go
					}
				}
			}
		}
		return out, nil

	case *types.Struct:
		structName := named
		if structName == "" {
			return fail("an anonymous struct has no name to generate a codec for; declare it as a named type")
		}
		if err := c.collect(structName); err != nil {
			return CborType{}, err
		}
		return CborType{Kind: CborStruct, Go: goType, Struct: structName}, nil

	case *types.Map:
		return fail("a Go map cannot be traversed deterministically, because Go randomizes iteration order; declare an ordered slice, or give the type its own AppendCBORTo that walks its keys in sorted order")

	case *types.Interface:
		return fail("an interface value is not reproducible from a snapshot, because what it holds is not part of the declared shape; declare the concrete type")

	case *types.Pointer:
		return fail("a pointer is not reproducible from a snapshot, because identity and aliasing are not part of the bytes; declare the value, or an index into a slice")

	case *types.Chan, *types.Signature:
		return fail("%s is not state, so it has no encoding", goType)

	default:
		return fail("%s has no CBOR encoding", goType)
	}
}

// basicCborKind reports the kind of a basic type and the width it is read at.
func basicCborKind(b *types.Basic) (CborKind, int, bool) {
	switch b.Kind() {
	case types.Bool:
		return CborBool, 0, true
	case types.String:
		return CborText, 0, true
	case types.Int8:
		return CborInt, 8, true
	case types.Int16:
		return CborInt, 16, true
	case types.Int32:
		return CborInt, 32, true
	case types.Int64:
		return CborInt, 64, true
	case types.Uint8:
		return CborUint, 8, true
	case types.Uint16:
		return CborUint, 16, true
	case types.Uint32:
		return CborUint, 32, true
	case types.Uint64:
		return CborUint, 64, true
	default:
		return "", 0, false
	}
}

func isFloatBasic(b *types.Basic) bool {
	return b.Info()&(types.IsFloat|types.IsComplex) != 0
}

// goString writes a type as the generated file must spell it: unqualified for
// this package, package-qualified for any other.
func (c *cborCollector) goString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if c.pkg != nil && c.pkg.Types != nil && p == c.pkg.Types {
			return ""
		}
		return p.Name()
	})
}

// selfCodec reports which halves of the driver's codec contract a type carries.
//
// The check is structural rather than against a named interface, so a package
// need not import cborbind merely to have its types admitted; and it ignores a
// method a previous run of this generator wrote, since a codec regenerated over
// its own output would otherwise find the methods it emitted last time and
// conclude the type encodes itself.
func (c *cborCollector) selfCodec(t types.Type) (canAppend, canDecode bool) {
	ptr := types.NewPointer(t)
	for _, set := range []*types.MethodSet{types.NewMethodSet(t), types.NewMethodSet(ptr)} {
		for i := 0; i < set.Len(); i++ {
			fn, ok := set.At(i).Obj().(*types.Func)
			if !ok || c.emittedHere(fn) {
				continue
			}
			signature, ok := fn.Type().(*types.Signature)
			if !ok {
				continue
			}
			switch fn.Name() {
			case "AppendCBORTo":
				if isAppendCBORSignature(signature) {
					canAppend = true
				}
			case "DecodeCBORFrom":
				if isDecodeCBORSignature(signature) {
					canDecode = true
				}
			}
		}
	}
	return canAppend, canDecode
}

// emittedHere reports whether a method was written by a previous generation run
// into one of this package's generated files.
func (c *cborCollector) emittedHere(fn *types.Func) bool {
	if len(c.generated) == 0 || c.pkg == nil || c.pkg.Fset == nil {
		return false
	}
	handle := c.pkg.Fset.File(fn.Pos())
	if handle == nil {
		return false
	}
	return c.generated[handle.Name()]
}

// isAppendCBORSignature matches AppendCBORTo(dst []byte) []byte.
func isAppendCBORSignature(s *types.Signature) bool {
	if s.Params().Len() != 1 || s.Results().Len() != 1 || s.Variadic() {
		return false
	}
	return isByteSliceType(s.Params().At(0).Type()) && isByteSliceType(s.Results().At(0).Type())
}

// isDecodeCBORSignature matches DecodeCBORFrom(data []byte) error.
func isDecodeCBORSignature(s *types.Signature) bool {
	if s.Params().Len() != 1 || s.Results().Len() != 1 || s.Variadic() {
		return false
	}
	if !isByteSliceType(s.Params().At(0).Type()) {
		return false
	}
	named, ok := s.Results().At(0).Type().(*types.Named)
	return ok && named.Obj() != nil && named.Obj().Name() == "error" && named.Obj().Pkg() == nil
}

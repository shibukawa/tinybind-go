package generator

import (
	"bytes"
	"fmt"
)

// The value reader below is the one place the decoder's shape is written:
// a scalar is read by its parser method, a struct decoded into the address
// it will live at, and a collection built in place around a recursive read
// of its element. The recursion is what carries a [][]float64 or a []*Node,
// which the flat slice and map emitters could not: each level names its
// locals by depth, so a loop inside a loop shadows nothing above it.

// readOpts is what differs between the two walks that use the reader.
type readOpts struct {
	// nilOnNull assigns nil to a slice or map member that arrives as null,
	// which is the binder's rule; the codec leaves the destination alone.
	nilOnNull bool
	// types is the plan's struct index, for sizing a slice's first
	// allocation by its element.
	types map[string]TypePlan
}

// sliceCap is the capacity a slice's first allocation gets: eight elements
// for the small ones, and fewer as the element grows, so the reservation
// stays around a quarter of a kilobyte whatever the element is.
func sliceCap(f FieldPlan, types map[string]TypePlan) int {
	size := estimatedSize(f.ElemPlan(), types, 0)
	if size <= 0 {
		return 8
	}
	c := 256 / size
	if c > 8 {
		return 8
	}
	if c < 1 {
		return 1
	}
	return c
}

// estimatedSize is the rough size in bytes of one value of f's kind on a
// 64-bit target, enough to tell a two-word element from a hundred-byte
// struct; it is not a layout computation.
func estimatedSize(f FieldPlan, types map[string]TypePlan, depth int) int {
	switch f.Kind {
	case "string", KindAny:
		return 16
	case "bool", "int8", "uint8":
		return 1
	case "int16", "uint16":
		return 2
	case "int32", "uint32", "float32":
		return 4
	case KindSlice, KindBytes:
		return 24
	case KindArray:
		return 8 * estimatedSize(f.ElemPlan(), types, depth+1)
	case KindStruct:
		t, ok := types[f.TypeName]
		if !ok || depth > 3 {
			return 64
		}
		size := 0
		for _, field := range t.Fields {
			size += estimatedSize(field, types, depth+1)
		}
		return size
	default:
		return 8
	}
}

// emitReadInto writes the code that reads the next value into dest. member
// is the document name errors carry, and depth numbers the locals.
func emitReadInto(b *bytes.Buffer, f FieldPlan, prefix, dest, member string, depth int, o readOpts) {
	n := func(base string) string { return fmt.Sprintf("%s%d", base, depth) }
	fail := func(what, cause string) string {
		return fmt.Sprintf("return out, jsonbind.FieldError(%q, %q, %s)", member, what, cause)
	}
	switch f.Kind {
	case "string", "int", "int64", "bool", "float64":
		v := n("v")
		fmt.Fprintf(b, "%s%s, err := p.%s()\n", prefix, v, parserMethod(f.Kind))
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, fail("invalid "+f.Kind, "err"), prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write(v))
	case sizedIntCase(f.Kind):
		_, unsigned, _ := intKindBits(f.Kind)
		method := "Int64"
		if unsigned {
			method = "Uint64"
		}
		v := n("v")
		fmt.Fprintf(b, "%s%s, err := p.%s()\n", prefix, v, method)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, fail("invalid "+f.Kind, "err"), prefix)
		if lo, hi, needed := intKindBounds(f.Kind); needed {
			fmt.Fprintf(b, "%sif %s < %s || %s > %s {\n%s\t%s\n%s}\n", prefix, v, lo, v, hi, prefix, fail("invalid "+f.Kind, "jsonbind.ErrIntegerRange"), prefix)
		}
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write(f.Kind+"("+v+")"))
	case KindAny:
		v := n("v")
		fmt.Fprintf(b, "%s%s, err := p.Any()\n", prefix, v)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, fail("invalid value", "err"), prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, v)
	case KindStruct:
		fmt.Fprintf(b, "%sif _, err := decode%sJSON(p, &%s); err != nil {\n%s\treturn out, err\n%s}\n", prefix, f.TypeName, dest, prefix, prefix)
	case KindForeign:
		// A foreign type cannot join the parser walk, because its decoder
		// takes a complete document. RawValue lifts the sub-document out so
		// it can be handed one, which is the second scan of that region the
		// decode half costs at depth.
		raw := n("raw")
		fmt.Fprintf(b, "%s%s, err := p.RawValue()\n", prefix, raw)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, fail("invalid object", "err"), prefix)
		fmt.Fprintf(b, "%sif err := %s.DecodeJSONFrom(%s); err != nil {\n%s\treturn out, err\n%s}\n", prefix, dest, raw, prefix, prefix)
	case KindBytes:
		emitStreamBytes(b, f, prefix, dest, member)
	case "file":
		// A file never arrives inside a JSON document.
		fmt.Fprintf(b, "%sif err := p.SkipValue(); err != nil {\n%s\treturn out, err\n%s}\n", prefix, prefix, prefix)
	case KindPtr:
		elem := f.ElemPlan()
		fmt.Fprintf(b, "%sif p.IsNull() {\n%s\t%s = nil\n%s} else {\n", prefix, prefix, dest, prefix)
		in := prefix + "\t"
		// The pointee is decoded where it lives. An existing pointee is
		// written over, the way a nested struct value is.
		fmt.Fprintf(b, "%sif %s == nil {\n%s\t%s = new(%s)\n%s}\n", in, dest, in, dest, elem.GoType(), in)
		if elem.Kind == KindStruct {
			fmt.Fprintf(b, "%sif _, err := decode%sJSON(p, %s); err != nil {\n%s\treturn out, err\n%s}\n", in, elem.TypeName, dest, in, in)
		} else {
			emitReadInto(b, elem, in, "(*"+dest+")", member, depth+1, o)
		}
		fmt.Fprintf(b, "%s}\n", prefix)
	case KindSlice:
		fmt.Fprintf(b, "%s{\n", prefix)
		in := prefix + "\t"
		emitArrayOpenAt(b, in, member, n("null"))
		if o.nilOnNull {
			fmt.Fprintf(b, "%sif %s {\n%s\t%s = nil\n%s} else {\n", in, n("null"), in, dest, in)
		} else {
			fmt.Fprintf(b, "%sif !%s {\n", in, n("null"))
		}
		body := in + "\t"
		v, i := n("v"), n("i")
		fmt.Fprintf(b, "%s%s := %s{}\n", body, v, f.GoType())
		fmt.Fprintf(b, "%sfor %s := 0; ; %s++ {\n", body, i, i)
		loop := body + "\t"
		emitArrayNextAt(b, loop, member, i, n("more"), n("fast"))
		// Sizing on the first element keeps an empty array allocation-free
		// while giving a populated one a single growth step in the common
		// case. The capacity is chosen from the element's size, so a slice
		// of wide structs, which is usually one or two elements long, does
		// not reserve eight of them: a decode of a document full of such
		// slices measured a third more bytes than it needed to.
		fmt.Fprintf(b, "%sif %s == 0 {\n%s\t%s = make(%s, 0, %d)\n%s}\n", loop, i, loop, v, f.GoType(), sliceCap(f, o.types), loop)
		emitSliceElement(b, f, loop, v, member, depth, o)
		fmt.Fprintf(b, "%s}\n", body)
		fmt.Fprintf(b, "%s%s = %s\n", body, dest, v)
		fmt.Fprintf(b, "%s}\n", in)
		fmt.Fprintf(b, "%s}\n", prefix)
	case KindArray:
		fmt.Fprintf(b, "%s{\n", prefix)
		in := prefix + "\t"
		emitArrayOpenAt(b, in, member, n("null"))
		fmt.Fprintf(b, "%sif !%s {\n", in, n("null"))
		body := in + "\t"
		filled, i := n("filled"), n("i")
		fmt.Fprintf(b, "%s%s := 0\n", body, filled)
		fmt.Fprintf(b, "%sfor %s := 0; ; %s++ {\n", body, i, i)
		loop := body + "\t"
		emitArrayNextAt(b, loop, member, i, n("more"), n("fast"))
		fmt.Fprintf(b, "%sif %s >= len(%s) {\n%s\treturn out, jsonbind.ArrayTooLong(%q, len(%s))\n%s}\n", loop, i, dest, loop, member, dest, loop)
		cell := dest + "[" + i + "]"
		if elem := f.ElemPlan(); elem.Kind == KindStruct || f.Elem != nil {
			// A struct or nested collection is decoded in place, so the cell
			// is cleared first; a scalar is assigned whole below.
			fmt.Fprintf(b, "%s%s = %s\n", loop, cell, zeroLiteral(f))
		}
		emitElementInto(b, f, loop, cell, member, depth, o)
		fmt.Fprintf(b, "%s%s = %s + 1\n", loop, filled, i)
		fmt.Fprintf(b, "%s}\n", body)
		fmt.Fprintf(b, "%sclear(%s[%s:])\n", body, dest, filled)
		fmt.Fprintf(b, "%s}\n", in)
		fmt.Fprintf(b, "%s}\n", prefix)
	case KindMap:
		fmt.Fprintf(b, "%s{\n", prefix)
		in := prefix + "\t"
		null := n("null")
		fmt.Fprintf(b, "%s%s := false\n", in, null)
		fmt.Fprintf(b, "%sif !p.ObjectStartFast() {\n", in)
		fmt.Fprintf(b, "%s\tvar err error\n%s\t%s, err = p.ObjectStart()\n", in, in, null)
		fmt.Fprintf(b, "%s\tif err != nil {\n%s\t\t%s\n%s\t}\n", in, in, fail("invalid map", "err"), in)
		fmt.Fprintf(b, "%s}\n", in)
		if o.nilOnNull {
			fmt.Fprintf(b, "%sif %s {\n%s\t%s = nil\n%s} else {\n", in, null, in, dest, in)
		} else {
			fmt.Fprintf(b, "%sif !%s {\n", in, null)
		}
		body := in + "\t"
		v, k, key, name := n("v"), n("n"), n("key"), n("name")
		fmt.Fprintf(b, "%s%s := %s{}\n", body, v, f.GoType())
		fmt.Fprintf(b, "%sfor %s := 0; ; %s++ {\n", body, k, k)
		loop := body + "\t"
		fmt.Fprintf(b, "%s%s, ok, err := p.ObjectKey(%s)\n", loop, key, k)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", loop, loop, fail("invalid map", "err"), loop)
		fmt.Fprintf(b, "%sif !ok {\n%s\tbreak\n%s}\n", loop, loop, loop)
		// The key aliases parser scratch that the element read below may
		// reuse, so it is copied before the value is read; a key seen before
		// comes back from the parser's cache rather than being copied again.
		fmt.Fprintf(b, "%s%s := p.Intern(%s)\n", loop, name, key)
		// A map value has no address, so the element is read into a local
		// and stored: a scalar is declared by its read, a struct or a nested
		// collection is declared here and filled in place.
		e := n("e")
		if elem := f.ElemPlan(); elem.Kind == KindStruct || f.Elem != nil {
			fmt.Fprintf(b, "%svar %s %s\n", loop, e, f.ElemGoType())
			emitElementInto(b, f, loop, e, member, depth, o)
		} else {
			emitScalarElement(b, f, loop, e, member, depth)
		}
		fmt.Fprintf(b, "%s%s[%s] = %s\n", loop, v, name, e)
		fmt.Fprintf(b, "%s}\n", body)
		fmt.Fprintf(b, "%s%s = %s\n", body, dest, v)
		fmt.Fprintf(b, "%s}\n", in)
		fmt.Fprintf(b, "%s}\n", prefix)
	}
}

// emitSliceElement appends one element to the slice named v: a zero value
// first, then a read into the slot append made, so a struct or a nested
// collection is built in place.
func emitSliceElement(b *bytes.Buffer, f FieldPlan, loop, v, member string, depth int, o readOpts) {
	elem := f.ElemPlan()
	if elem.Kind == KindStruct || f.Elem != nil {
		fmt.Fprintf(b, "%s%s = append(%s, %s)\n", loop, v, v, zeroLiteral(f))
		emitElementInto(b, f, loop, v+"[len("+v+")-1]", member, depth, o)
		return
	}
	e := fmt.Sprintf("e%d", depth)
	emitScalarElement(b, f, loop, e, member, depth)
	fmt.Fprintf(b, "%s%s = append(%s, %s)\n", loop, v, v, e)
}

// emitElementInto reads one element into cell, which is addressable.
func emitElementInto(b *bytes.Buffer, f FieldPlan, loop, cell, member string, depth int, o readOpts) {
	elem := f.ElemPlan()
	switch {
	case elem.Kind == KindStruct:
		fmt.Fprintf(b, "%sif _, err := decode%sJSON(p, &%s); err != nil {\n%s\treturn out, err\n%s}\n", loop, elem.TypeName, cell, loop, loop)
	case f.Elem != nil:
		emitReadInto(b, elem, loop, cell, member, depth+1, o)
	default:
		e := fmt.Sprintf("e%d", depth)
		emitScalarElement(b, f, loop, e, member, depth)
		fmt.Fprintf(b, "%s%s = %s\n", loop, cell, e)
	}
}

// emitScalarElement reads one scalar or any element into a local named e,
// under the annotation rules ParseSlice applied: the member name and the
// kind that failed. A sized integer is range-checked against its width, and
// a named element is converted to its declared type.
func emitScalarElement(b *bytes.Buffer, f FieldPlan, prefix, e, member string, depth int) {
	fail := func(cause string) string {
		return fmt.Sprintf("return out, jsonbind.FieldError(%q, \"invalid %s\", %s)", member, f.ElemKind, cause)
	}
	w := fmt.Sprintf("w%d", depth)
	switch {
	case isSizedIntKind(f.ElemKind):
		_, unsigned, _ := intKindBits(f.ElemKind)
		method := "Int64"
		if unsigned {
			method = "Uint64"
		}
		fmt.Fprintf(b, "%s%s, err := p.%s()\n", prefix, w, method)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, fail("err"), prefix)
		if lo, hi, needed := intKindBounds(f.ElemKind); needed {
			fmt.Fprintf(b, "%sif %s < %s || %s > %s {\n%s\t%s\n%s}\n", prefix, w, lo, w, hi, prefix, fail("jsonbind.ErrIntegerRange"), prefix)
		}
		fmt.Fprintf(b, "%s%s := %s(%s)\n", prefix, e, f.ElemGoType(), w)
	case f.ElemNamed != "":
		fmt.Fprintf(b, "%s%s, err := p.%s()\n", prefix, w, parserMethod(f.ElemKind))
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, fail("err"), prefix)
		fmt.Fprintf(b, "%s%s := %s(%s)\n", prefix, e, f.ElemNamed, w)
	default:
		fmt.Fprintf(b, "%s%s, err := p.%s()\n", prefix, e, parserMethod(f.ElemKind))
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, fail("err"), prefix)
	}
}

// emitArrayOpenAt writes the fast-or-full array start, leaving null set.
func emitArrayOpenAt(b *bytes.Buffer, in, member, null string) {
	fmt.Fprintf(b, "%s%s := false\n", in, null)
	fmt.Fprintf(b, "%sif !p.ArrayStartFast() {\n", in)
	fmt.Fprintf(b, "%s\tvar err error\n%s\t%s, err = p.ArrayStart()\n", in, in, null)
	fmt.Fprintf(b, "%s\tif err != nil {\n%s\t\treturn out, jsonbind.FieldError(%q, \"invalid array\", err)\n%s\t}\n", in, in, member, in)
	fmt.Fprintf(b, "%s}\n", in)
}

// emitArrayNextAt writes the fast-or-full step to the next element,
// breaking the loop at the closing bracket.
func emitArrayNextAt(b *bytes.Buffer, loop, member, i, more, fast string) {
	fmt.Fprintf(b, "%s%s, %s := p.ArrayNextFast(%s)\n", loop, more, fast, i)
	fmt.Fprintf(b, "%sif !%s {\n", loop, fast)
	fmt.Fprintf(b, "%s\tvar err error\n%s\t%s, err = p.ArrayNext(%s)\n", loop, loop, more, i)
	fmt.Fprintf(b, "%s\tif err != nil {\n%s\t\treturn out, jsonbind.FieldError(%q, \"invalid array\", err)\n%s\t}\n", loop, loop, member, loop)
	fmt.Fprintf(b, "%s}\n", loop)
	fmt.Fprintf(b, "%sif !%s {\n%s\tbreak\n%s}\n", loop, more, loop, loop)
}

// zeroLiteral spells the zero value of f's element: nil for a pointer, a
// slice or a map, and the empty composite literal for a struct or an array.
func zeroLiteral(f FieldPlan) string {
	switch f.ElemPlan().Kind {
	case KindPtr, KindSlice, KindMap, KindAny:
		return "nil"
	}
	return f.ElemGoType() + "{}"
}

package generator

import (
	"fmt"
	"strings"
)

// The delta surface is a named delta type per struct, the diff that fills one,
// the apply that puts it back, and the delta's own codec.
//
// The shape is data:cbor-state-delta: a presence mask followed by the changed
// values. Both ends have already agreed on CBORProtocolVersion, so a field is
// named by its bit rather than by a path, and a change one field deep costs the
// mask chain that addresses it and nothing else.

// deltaFields is the fields that take a mask bit, in bit order.
//
// The identity is not among them. An entity whose identity changed is a
// different entity, so the identity is carried by the collection that holds the
// element rather than by the delta of it.
func deltaFields(item CborTypePlan) []CborFieldPlan {
	out := make([]CborFieldPlan, 0, len(item.Fields))
	for _, f := range item.Fields {
		if !f.Identity {
			out = append(out, f)
		}
	}
	return out
}

func cborDeltaTypeName(name string) string     { return name + "Delta" }
func cborListDeltaTypeName(name string) string { return name + "ListDelta" }
func cborPatchTypeName(name string) string     { return name + "Patch" }

// emitDelta writes everything the delta surface needs for one type.
func (e *cborEmitter) emitDelta(item CborTypePlan) {
	fields := deltaFields(item)
	if len(fields) > 64 {
		return
	}
	e.deltaType(item, fields)
	e.diffFunc(item, fields)
	e.applyFunc(item, fields)
	e.deltaAppendFunc(item, fields)
	e.deltaDecodeFunc(item, fields)
	for _, f := range fields {
		if f.Type.Kind == CborSlice && f.Type.ElemIdentity != "" {
			e.listDelta(f.Type)
		}
	}
	if item.Declared {
		e.deltaEntryPoints(item)
	}
}

// deltaType declares the delta a diff fills.
//
// It is a named struct rather than a generic document because a caller retains
// and reuses it across ticks, which is what makes the steady state
// allocation-free, and because applying one type's delta to another does not
// compile where a generic document would have failed on a value that already
// crossed the network.
func (e *cborEmitter) deltaType(item CborTypePlan, fields []CborFieldPlan) {
	b := &e.body
	name := cborDeltaTypeName(item.Name)
	fmt.Fprintf(b, "// %s is what changed between two %s values.\n//\n"+
		"// Present names the fields carried: bit n is set when field n of the list\n"+
		"// below changed, and every other field holds nothing meaningful.\n",
		name, item.Name)
	fmt.Fprintf(b, "type %s struct {\n\tPresent uint64\n", name)
	for i, f := range fields {
		fmt.Fprintf(b, "\t// bit %d\n\t%s %s\n", i, f.Name, e.deltaFieldType(f.Type))
	}
	b.WriteString("}\n\n")
}

// deltaFieldType is how one field is carried in a delta.
func (e *cborEmitter) deltaFieldType(t CborType) string {
	switch {
	case t.Kind == CborStruct:
		return cborDeltaTypeName(t.Struct)
	case t.Kind == CborSlice && t.ElemIdentity != "":
		return cborListDeltaTypeName(t.Elem.Struct)
	default:
		return t.Go
	}
}

// diffFunc reports what changed, into a delta the caller owns.
func (e *cborEmitter) diffFunc(item CborTypePlan, fields []CborFieldPlan) {
	b := &e.body
	fmt.Fprintf(b, "// diff%sInto fills d with what changed between baseline and current,\n"+
		"// and reports whether anything did.\n", item.Name)
	fmt.Fprintf(b, "func diff%sInto(d *%s, baseline, current %s) bool {\n",
		item.Name, cborDeltaTypeName(item.Name), item.Name)
	b.WriteString("\td.Present = 0\n")
	for i, f := range fields {
		bit := fmt.Sprintf("1 << %d", i)
		switch {
		case f.Type.Kind == CborStruct:
			fmt.Fprintf(b, "\tif diff%sInto(&d.%s, baseline.%s, current.%s) {\n\t\td.Present |= %s\n\t}\n",
				f.Type.Struct, f.Name, f.Name, f.Name, bit)
		case f.Type.Kind == CborSlice && f.Type.ElemIdentity != "":
			fmt.Fprintf(b, "\tif diff%sListInto(&d.%s, baseline.%s, current.%s) {\n\t\td.Present |= %s\n\t}\n",
				f.Type.Elem.Struct, f.Name, f.Name, f.Name, bit)
		default:
			fmt.Fprintf(b, "\tif %s {\n\t\td.Present |= %s\n\t\td.%s = current.%s\n\t}\n",
				e.notEqual("baseline."+f.Name, "current."+f.Name, f.Type), bit, f.Name, f.Name)
		}
	}
	b.WriteString("\treturn d.Present != 0\n}\n\n")
}

// notEqual writes the test for a value carried whole.
func (e *cborEmitter) notEqual(a, bExpr string, t CborType) string {
	switch t.Kind {
	case CborBytes:
		// Comparing the string conversions is the one byte-slice comparison Go
		// promises not to allocate for, and it needs no import.
		return fmt.Sprintf("string(%s) != string(%s)", a, bExpr)
	case CborSlice:
		return fmt.Sprintf("!%s(%s, %s)", e.equalSliceHelper(t), a, bExpr)
	default:
		return fmt.Sprintf("%s != %s", a, bExpr)
	}
}

// equalSliceHelper emits, once, the element-wise comparison a collection
// carried whole needs, and returns its name.
func (e *cborEmitter) equalSliceHelper(t CborType) string {
	name := "cborEqual" + cborShapeName(t)
	if _, done := e.helpers[name]; done {
		return name
	}
	e.helpers[name] = "" // reserve, so a self-reaching shape terminates
	var b strings.Builder
	fmt.Fprintf(&b, "// %s reports whether two %s hold the same values.\n", name, t.Go)
	fmt.Fprintf(&b, "func %s(a, b %s) bool {\n\tif len(a) != len(b) {\n\t\treturn false\n\t}\n", name, t.Go)
	b.WriteString("\tfor i := range a {\n")
	if t.Elem.Kind == CborStruct {
		fmt.Fprintf(&b, "\t\tif !%s(a[i], b[i]) {\n\t\t\treturn false\n\t\t}\n", e.equalStructHelper(*t.Elem))
	} else {
		fmt.Fprintf(&b, "\t\tif %s {\n\t\t\treturn false\n\t\t}\n", e.notEqual("a[i]", "b[i]", *t.Elem))
	}
	b.WriteString("\t}\n\treturn true\n}\n\n")
	e.helpers[name] = b.String()
	return name
}

// equalStructHelper emits, once, a field-by-field comparison of a struct.
func (e *cborEmitter) equalStructHelper(t CborType) string {
	name := "cborEqual" + t.Struct
	if _, done := e.helpers[name]; done {
		return name
	}
	e.helpers[name] = ""
	item := e.index[t.Struct]
	var b strings.Builder
	fmt.Fprintf(&b, "// %s reports whether two %s hold the same values.\n", name, t.Struct)
	fmt.Fprintf(&b, "func %s(a, b %s) bool {\n", name, t.Struct)
	for _, f := range item.Fields {
		if f.Type.Kind == CborStruct {
			fmt.Fprintf(&b, "\tif !%s(a.%s, b.%s) {\n\t\treturn false\n\t}\n",
				e.equalStructHelper(f.Type), f.Name, f.Name)
			continue
		}
		fmt.Fprintf(&b, "\tif %s {\n\t\treturn false\n\t}\n",
			e.notEqual("a."+f.Name, "b."+f.Name, f.Type))
	}
	b.WriteString("\treturn true\n}\n\n")
	e.helpers[name] = b.String()
	return name
}

// cborShapeName names a type for a helper, so two structurally identical
// shapes share one function and two different ones never collide.
func cborShapeName(t CborType) string {
	switch t.Kind {
	case CborSlice:
		return "Slice" + cborShapeName(*t.Elem)
	case CborStruct:
		return t.Struct
	case CborSelf:
		return cborIdentifierOf(t.Go)
	default:
		return strings.ToUpper(string(t.Kind[0])) + string(t.Kind[1:])
	}
}

// cborIdentifierOf turns a Go type as spelled into something usable in a name.
func cborIdentifierOf(goType string) string {
	var b strings.Builder
	for _, r := range goType {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

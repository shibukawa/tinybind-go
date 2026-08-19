package generator

import (
	"fmt"
)

// applyFunc puts a delta back onto a value.
//
// It returns an error because a collection inside it can: a delta naming an
// entity the baseline does not hold means the receiver is not holding the
// baseline the sender diffed against, and that is recoverable only by saying so.
func (e *cborEmitter) applyFunc(item CborTypePlan, fields []CborFieldPlan) {
	b := &e.body
	fmt.Fprintf(b, "// apply%sDelta puts d back onto v. A bit Present does not name leaves\n"+
		"// that field as it stands.\n", item.Name)
	fmt.Fprintf(b, "func apply%sDelta(v *%s, d %s) error {\n",
		item.Name, item.Name, cborDeltaTypeName(item.Name))
	for i, f := range fields {
		fmt.Fprintf(b, "\tif d.Present&(1<<%d) != 0 {\n", i)
		switch {
		case f.Type.Kind == CborStruct:
			fmt.Fprintf(b, "\t\tif err := apply%sDelta(&v.%s, d.%s); err != nil {\n\t\t\treturn err\n\t\t}\n",
				f.Type.Struct, f.Name, f.Name)
		case f.Type.Kind == CborSlice && f.Type.ElemIdentity != "":
			fmt.Fprintf(b, "\t\tif err := apply%sListDelta(&v.%s, d.%s); err != nil {\n\t\t\treturn err\n\t\t}\n",
				f.Type.Elem.Struct, f.Name, f.Name)
		default:
			fmt.Fprintf(b, "\t\tv.%s = d.%s\n", f.Name, f.Name)
		}
		b.WriteString("\t}\n")
	}
	b.WriteString("\treturn nil\n}\n\n")
}

// deltaAppendFunc writes the delta as a presence mask followed by the values
// that mask names.
//
// A field the mask does not name costs one bit and no bytes, which is what
// makes a tick that moved three fields a few bytes rather than a snapshot.
func (e *cborEmitter) deltaAppendFunc(item CborTypePlan, fields []CborFieldPlan) {
	b := &e.body
	name := cborDeltaTypeName(item.Name)
	fmt.Fprintf(b, "// append%sDeltaCBOR appends d as one CBOR item: the mask, then the\n"+
		"// values it names, in bit order.\n", item.Name)
	fmt.Fprintf(b, "func append%sDeltaCBOR(dst []byte, v %s) []byte {\n", item.Name, name)
	fmt.Fprintf(b, "\tn := 1\n\tfor bit := 0; bit < %d; bit++ {\n"+
		"\t\tif v.Present&(1<<uint(bit)) != 0 {\n\t\t\tn++\n\t\t}\n\t}\n", len(fields))
	b.WriteString("\tdst = cbor.AppendArrayHeader(dst, n)\n\tdst = cbor.AppendUint(dst, v.Present)\n")
	for i, f := range fields {
		fmt.Fprintf(b, "\tif v.Present&(1<<%d) != 0 {\n", i)
		switch {
		case f.Type.Kind == CborStruct:
			fmt.Fprintf(b, "\t\tdst = append%sDeltaCBOR(dst, v.%s)\n", f.Type.Struct, f.Name)
		case f.Type.Kind == CborSlice && f.Type.ElemIdentity != "":
			fmt.Fprintf(b, "\t\tdst = append%sListDeltaCBOR(dst, v.%s)\n", f.Type.Elem.Struct, f.Name)
		default:
			cborAppendValue(b, "v."+f.Name, f.Type, item.Profile, 2, 0)
		}
		b.WriteString("\t}\n")
	}
	b.WriteString("\treturn dst\n}\n\n")
}

// deltaDecodeFunc reads a delta back.
//
// A bit this schema does not know is skipped rather than refused: a CBOR item
// is self-delimiting, so a reader meeting an unknown bit skips exactly one item
// and stays aligned. That is what makes appending a field to a type something an
// older reader survives.
func (e *cborEmitter) deltaDecodeFunc(item CborTypePlan, fields []CborFieldPlan) {
	b := &e.body
	name := cborDeltaTypeName(item.Name)
	fmt.Fprintf(b, "// decode%sDeltaCBOR reads one delta into v.\n", item.Name)
	fmt.Fprintf(b, "func decode%sDeltaCBOR(r *cbor.Reader, v *%s) error {\n", item.Name, name)
	b.WriteString("\tn, indefinite, err := r.ReadArrayHeader()\n\tif err != nil {\n\t\treturn err\n\t}\n")
	fmt.Fprintf(b, "\tif indefinite || n < 1 {\n\t\treturn %s\n\t}\n", cborError(name))
	b.WriteString("\tpresent, err := r.ReadUint64()\n\tif err != nil {\n\t\treturn err\n\t}\n")
	b.WriteString("\tv.Present = present\n\tread := 1\n")
	for i, f := range fields {
		fmt.Fprintf(b, "\tif present&(1<<%d) != 0 {\n", i)
		switch {
		case f.Type.Kind == CborStruct:
			fmt.Fprintf(b, "\t\tif err := decode%sDeltaCBOR(r, &v.%s); err != nil {\n\t\t\treturn err\n\t\t}\n",
				f.Type.Struct, f.Name)
		case f.Type.Kind == CborSlice && f.Type.ElemIdentity != "":
			fmt.Fprintf(b, "\t\tif err := decode%sListDeltaCBOR(r, &v.%s); err != nil {\n\t\t\treturn err\n\t\t}\n",
				f.Type.Elem.Struct, f.Name)
		default:
			cborDecodeValue(b, "v."+f.Name, f.Type, item.Profile, name+"."+f.Name, 2, 0)
		}
		b.WriteString("\t\tread++\n\t}\n")
	}
	b.WriteString("\t// A bit this schema does not know: one item per remaining slot.\n")
	b.WriteString("\tfor ; read < n; read++ {\n\t\tif err := r.Skip(); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n")
	b.WriteString("\treturn nil\n}\n\n")
}

// deltaEntryPoints publishes the delta surface of a declared type.
//
// The delta carries the driver's own codec interfaces, so it is handed to
// anything that takes a cbor.Appender with no second surface to learn.
func (e *cborEmitter) deltaEntryPoints(item CborTypePlan) {
	b := &e.body
	name := cborDeltaTypeName(item.Name)
	profileVar := "cborWireProfile"
	if item.Profile == CborWorld {
		profileVar, e.world = "cborWorldProfile", true
	} else {
		e.wire = true
	}

	fmt.Fprintf(b, "// Diff%s reports what changed between baseline and current.\n//\n"+
		"// Use Diff%sInto in a loop that runs per tick: it fills a delta the caller\n"+
		"// retains, and reuses its collection capacity, so the steady state allocates\n"+
		"// nothing.\n", item.Name, item.Name)
	fmt.Fprintf(b, "func Diff%s(baseline, current %s) %s {\n\tvar d %s\n\tdiff%sInto(&d, baseline, current)\n\treturn d\n}\n\n",
		item.Name, item.Name, name, name, item.Name)

	fmt.Fprintf(b, "// Diff%sInto is Diff%s into a delta the caller owns and reuses.\n",
		item.Name, item.Name)
	fmt.Fprintf(b, "func Diff%sInto(d *%s, baseline, current %s) {\n\tdiff%sInto(d, baseline, current)\n}\n\n",
		item.Name, name, item.Name, item.Name)

	fmt.Fprintf(b, "// Apply%sDelta puts d back onto v.\n//\n"+
		"// It reports an error when the delta names an entity v does not hold, which\n"+
		"// means v is not the baseline the delta was diffed against.\n", item.Name)
	fmt.Fprintf(b, "func Apply%sDelta(v *%s, d %s) error {\n\treturn apply%sDelta(v, d)\n}\n\n",
		item.Name, item.Name, name, item.Name)

	fmt.Fprintf(b, "// AppendCBORTo appends the delta as one CBOR item, satisfying cbor.Appender.\n")
	fmt.Fprintf(b, "func (v %s) AppendCBORTo(dst []byte) []byte {\n\treturn append%sDeltaCBOR(dst, v)\n}\n\n",
		name, item.Name)

	fmt.Fprintf(b, "// DecodeCBORFrom reads one delta into v, satisfying cbor.Decodable.\n")
	fmt.Fprintf(b, "func (v *%s) DecodeCBORFrom(data []byte) error {\n", name)
	fmt.Fprintf(b, "\tr := %s.ReaderOver(data)\n", profileVar)
	fmt.Fprintf(b, "\tif err := decode%sDeltaCBOR(&r, v); err != nil {\n\t\treturn err\n\t}\n", item.Name)
	b.WriteString("\tif !r.Done() {\n\t\treturn cbor.ErrExtraneousData\n\t}\n\treturn nil\n}\n\n")
}

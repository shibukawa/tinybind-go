package generator

import (
	"bytes"
	"fmt"
	"strings"
)

// decodeFunc emits the decoder.
//
// It reads from the Reader the caller owns rather than making one, so a nested
// struct is read in the same walk as its parent and the steady state of a tick
// loop allocates nothing. Only the published method below makes a Reader, and
// it makes it on the stack.
func (e *cborEmitter) decodeFunc(item CborTypePlan) {
	b := &e.body
	name := cborDecodeFuncName(item)
	fmt.Fprintf(b, "// %s decodes one %s-profile CBOR item into v.\n", name, item.Profile)
	if item.Profile == CborWire {
		fmt.Fprintf(b, "//\n// The array must be exactly %d long. Under the wire profile a field the\n"+
			"// schema does not know cannot exist, so a count that disagrees is refused\n"+
			"// rather than read past: the two ends are running different protocols.\n", len(item.Fields))
	} else {
		b.WriteString("//\n// A key this schema does not know is skipped, which is the tolerance the\n" +
			"// world profile exists to provide. A key it knows and the message omits\n" +
			"// leaves that field zero, so decoding into a reused value cannot inherit a\n" +
			"// field from the message before it.\n")
	}
	fmt.Fprintf(b, "func %s(r *cbor.Reader, v *%s) error {\n", name, item.Name)

	if item.Profile == CborWire {
		fmt.Fprintf(b, "\tn, indefinite, err := r.ReadArrayHeader()\n\tif err != nil {\n\t\treturn err\n\t}\n")
		fmt.Fprintf(b, "\tif indefinite || n != %d {\n\t\treturn %s\n\t}\n", len(item.Fields), cborError(item.Name))
		for _, field := range item.Fields {
			cborDecodeValue(b, "v."+field.Name, field.Type, item.Profile, item.Name+"."+field.Name, 1, 0)
		}
		b.WriteString("\treturn nil\n}\n\n")
		return
	}

	b.WriteString("\tpairs, indefinite, err := r.ReadMapHeader()\n\tif err != nil {\n\t\treturn err\n\t}\n")
	fmt.Fprintf(b, "\tif indefinite {\n\t\treturn %s\n\t}\n", cborError(item.Name))
	fmt.Fprintf(b, "\t*v = %s{}\n", item.Name)
	b.WriteString("\tfor range pairs {\n")
	if item.IntKeys {
		b.WriteString("\t\tkey, err := r.ReadUint64()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")
		b.WriteString("\t\tswitch key {\n")
	} else {
		// ReadTextBytes borrows the key from the input, where ReadText would
		// copy it into a string. Switching on the conversion of a byte slice
		// is the one place Go promises not to allocate for it.
		b.WriteString("\t\tkey, err := r.ReadTextBytes()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")
		b.WriteString("\t\tswitch string(key) {\n")
	}
	for _, field := range item.Fields {
		if item.IntKeys {
			fmt.Fprintf(b, "\t\tcase %d:\n", field.IntKey)
		} else {
			fmt.Fprintf(b, "\t\tcase %q:\n", field.Key)
		}
		cborDecodeValue(b, "v."+field.Name, field.Type, item.Profile, item.Name+"."+field.Name, 3, 0)
	}
	b.WriteString("\t\tdefault:\n\t\t\tif err := r.Skip(); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n")
	b.WriteString("\t\t}\n\t}\n\treturn nil\n}\n\n")
}

// cborError writes the refusal a shape disagreement produces. It names the byte
// offset and the route to it, which is what tells a protocol mismatch apart
// from a corrupt frame.
func cborError(path string) string {
	return fmt.Sprintf("&cbor.Error{Offset: int64(r.Offset()), Path: %q, Err: cbor.ErrUnexpectedToken}", path)
}

// decodeValue reads one value into target.
func cborDecodeValue(b *bytes.Buffer, target string, t CborType, profile CborProfile, path string, indent, depth int) {
	tab := strings.Repeat("\t", indent)
	inner := strings.Repeat("\t", indent+1)
	switch t.Kind {
	case CborUint, CborInt, CborBool, CborText:
		read, primitive := cborReadCall(t)
		fmt.Fprintf(b, "%s{\n", tab)
		fmt.Fprintf(b, "%sx, err := r.%s()\n", inner, read)
		fmt.Fprintf(b, "%sif err != nil {\n%s\treturn err\n%s}\n", inner, inner, inner)
		fmt.Fprintf(b, "%s%s = %s\n", inner, target, cborConvert(t.Go, primitive, "x"))
		fmt.Fprintf(b, "%s}\n", tab)

	case CborBytes:
		fmt.Fprintf(b, "%s{\n", tab)
		fmt.Fprintf(b, "%sx, err := r.ReadBytes()\n", inner)
		fmt.Fprintf(b, "%sif err != nil {\n%s\treturn err\n%s}\n", inner, inner, inner)
		fmt.Fprintf(b, "%s// ReadBytes borrows from the input, so the value is copied into whatever\n"+
			"%s// capacity the field already holds: it must outlive the buffer it was read\n"+
			"%s// from, and reusing the capacity keeps the steady state allocation-free.\n", inner, inner, inner)
		fmt.Fprintf(b, "%s%s = append(%s[:0], x...)\n", inner, target, target)
		fmt.Fprintf(b, "%s}\n", tab)

	case CborSelf:
		// ReadRaw borrows one complete sub-item from the input rather than
		// copying the message, so the cost of reaching a foreign type at depth
		// is the sub-item and not the walk.
		fmt.Fprintf(b, "%s{\n", tab)
		fmt.Fprintf(b, "%sraw, err := r.ReadRaw()\n", inner)
		fmt.Fprintf(b, "%sif err != nil {\n%s\treturn err\n%s}\n", inner, inner, inner)
		fmt.Fprintf(b, "%sif err := %s.DecodeCBORFrom(raw); err != nil {\n%s\treturn err\n%s}\n",
			inner, target, inner, inner)
		fmt.Fprintf(b, "%s}\n", tab)

	case CborStruct:
		fmt.Fprintf(b, "%sif err := %s(r, &%s); err != nil {\n%s\treturn err\n%s}\n", tab,
			cborDecodeFuncName(CborTypePlan{Name: t.Struct, Profile: profile}), target, tab, tab)

	case CborSlice:
		n := fmt.Sprintf("n%d", depth)
		i := fmt.Sprintf("i%d", depth)
		fmt.Fprintf(b, "%s{\n", tab)
		fmt.Fprintf(b, "%s%s, indefinite, err := r.ReadArrayHeader()\n", inner, n)
		fmt.Fprintf(b, "%sif err != nil {\n%s\treturn err\n%s}\n", inner, inner, inner)
		fmt.Fprintf(b, "%sif indefinite {\n%s\treturn %s\n%s}\n", inner, inner, cborError(path), inner)
		fmt.Fprintf(b, "%s// The existing capacity is reused where it fits, so a value decoded into\n"+
			"%s// repeatedly stops allocating after the first message.\n", inner, inner)
		fmt.Fprintf(b, "%sif cap(%s) >= %s {\n%s\t%s = %s[:%s]\n%s} else {\n%s\t%s = make(%s, %s)\n%s}\n",
			inner, target, n, inner, target, target, n, inner, inner, target, t.Go, n, inner)
		fmt.Fprintf(b, "%sfor %s := 0; %s < %s; %s++ {\n", inner, i, i, n, i)
		cborDecodeValue(b, fmt.Sprintf("%s[%s]", target, i), *t.Elem, profile, path, indent+2, depth+1)
		fmt.Fprintf(b, "%s}\n", inner)
		fmt.Fprintf(b, "%s}\n", tab)
	}
}

// cborReadCall is the width-enforcing read for a kind, and the Go type it
// answers. Reading at the field's own width is what turns a value too wide for
// it into an error rather than a silent truncation.
func cborReadCall(t CborType) (call, primitive string) {
	switch t.Kind {
	case CborUint:
		return fmt.Sprintf("ReadUint%d", t.Bits), fmt.Sprintf("uint%d", t.Bits)
	case CborInt:
		return fmt.Sprintf("ReadInt%d", t.Bits), fmt.Sprintf("int%d", t.Bits)
	case CborBool:
		return "ReadBool", "bool"
	default:
		return "ReadText", "string"
	}
}

// cborConvert writes the conversion a declared type needs and nothing where the
// field is already the type the read answers.
func cborConvert(goType, primitive, expr string) string {
	if goType == primitive {
		return expr
	}
	return fmt.Sprintf("%s(%s)", goType, expr)
}

// decodeMethod publishes the decoder as the driver's cbor.Decodable.
//
// The Reader is made here and nowhere below, on the stack, from the profile
// this file pins. Everything deeper reads from it, so one message is one
// Reader however deep its schema goes.
func (e *cborEmitter) decodeMethod(item CborTypePlan) {
	b := &e.body
	profileVar := "cborWireProfile"
	if item.Profile == CborWorld {
		profileVar, e.world = "cborWorldProfile", true
	} else {
		e.wire = true
	}
	fmt.Fprintf(b, "// DecodeCBORFrom decodes one %s-profile CBOR item into v, satisfying\n"+
		"// cbor.Decodable. data holds exactly one item and nothing after it.\n", item.Profile)
	fmt.Fprintf(b, "func (v *%s) DecodeCBORFrom(data []byte) error {\n", item.Name)
	fmt.Fprintf(b, "\tr := %s.ReaderOver(data)\n", profileVar)
	fmt.Fprintf(b, "\tif err := %s(&r, v); err != nil {\n\t\treturn err\n\t}\n", cborDecodeFuncName(item))
	b.WriteString("\tif !r.Done() {\n\t\treturn cbor.ErrExtraneousData\n\t}\n\treturn nil\n}\n\n")
}

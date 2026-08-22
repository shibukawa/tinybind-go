package generator

import (
	"bytes"
	"fmt"
	"sort"
)

// cborbindRuntimeImportPath is the package whose entry points asked for these
// codecs, and whose ErrShape the emitted decoders return.
const cborbindRuntimeImportPath = "github.com/shibukawa/tinybind-go/cborbind"

// CBOR codec emission for requirement:cbor-codec-generation.
//
// A call to a cborbind entry point is the ask, and its name says which
// container the codec is for. Two shapes are emitted from one field plan:
//
//   - Array: a fixed-length array in declaration order. Member names are not
//     on the wire, so it is the smaller of the two and both ends have to be
//     rebuilt together when a field is added.
//   - Map: text keys in RFC 8949 bytewise order of the encoded key, with an
//     unknown key skipped on decode, so the two ends can ship separately.
//
// Member names come from the json tag, through the same jsonMembers the JSON
// and HTTP codecs use. A struct therefore spells its wire names once and the
// three codecs cannot disagree about them.
//
// There is no profile: what a codec can encode is a property of these
// emitters, and what it does encode is a property of the struct, per
// decision:cbor-shape-is-the-only-axis.

// cborShape is one container the codecs are generated for.
type cborShape struct {
	// suffix keys the emitted function names, so a struct reached from both
	// shapes gets one function per shape rather than one that serves neither.
	suffix string
	// method is the interface method the shape publishes on the type.
	appendMethod string
	decodeMethod string
	// encodeUsage and decodeUsage are the bits a call site set.
	encodeUsage Usage
	decodeUsage Usage
	sortedKeys  bool
	asMap       bool
}

var cborShapes = []cborShape{
	{
		suffix: "CBORArray", appendMethod: "AppendCBORInArrayTo", decodeMethod: "DecodeCBORInArrayFrom",
		encodeUsage: UsageCBORArrayEncode, decodeUsage: UsageCBORArrayDecode,
	},
	{
		suffix: "CBORMap", appendMethod: "AppendCBORInMapTo", decodeMethod: "DecodeCBORInMapFrom",
		encodeUsage: UsageCBORMapEncode, decodeUsage: UsageCBORMapDecode,
		sortedKeys: true, asMap: true,
	},
}

// checkCBORBindTypes refuses a type whose fields a CBOR codec cannot carry,
// naming the field rather than emitting a codec that silently drops it.
func checkCBORBindTypes(emitted []TypePlan) error {
	for _, t := range emitted {
		if t.Usage&UsageCBOR == 0 {
			continue
		}
		for _, f := range t.Fields {
			if f.IsRest() {
				return fmt.Errorf("cborbind: %s.%s: a payload:\"*\" rest map has no CBOR mapping", t.Name, f.Name)
			}
			if !isDocumentField(f) {
				continue
			}
			if f.Kind == KindForeign {
				return fmt.Errorf("cborbind: %s.%s: the field type carries its own JSON codec and no CBOR one", t.Name, f.Name)
			}
			if f.Kind == "file" {
				return fmt.Errorf("cborbind: %s.%s: an uploaded file has no CBOR encoding; mark it json:\"-\"", t.Name, f.Name)
			}
		}
	}
	return nil
}

// cborBindMembers is the member list a codec carries, in the order it writes
// them: the JSON member set, bytewise-ordered over the encoded key for the map
// shape and left in declaration order for the array shape, which is what makes
// the array positional.
func cborBindMembers(t TypePlan, shape cborShape) []FieldPlan {
	var out []FieldPlan
	for _, f := range jsonMembers(t) {
		if f.Kind == "file" {
			continue
		}
		out = append(out, f)
	}
	if shape.sortedKeys {
		sort.SliceStable(out, func(i, j int) bool {
			return bytes.Compare(cborEncodedTextKey(jsonMemberName(out[i])), cborEncodedTextKey(jsonMemberName(out[j]))) < 0
		})
	}
	return out
}

// emitCBORBindCodecs writes every CBOR codec the plan asked for, plus the
// methods that make the entry points compile against the type.
func emitCBORBindCodecs(b *bytes.Buffer, emitted []TypePlan, types map[string]TypePlan) {
	for _, t := range emitted {
		if t.Usage&UsageCBOR == 0 {
			continue
		}
		for _, shape := range cborShapes {
			if t.Usage&shape.encodeUsage != 0 {
				emitCBORBindEncode(b, t, types, shape)
			}
			if t.Usage&shape.decodeUsage != 0 {
				emitCBORBindDecode(b, t, types, shape)
			}
		}
		emitCBORBindMethods(b, t)
	}
}

// emitCBORBindEncode writes append<T><Shape>, which appends one container and
// returns the extended slice.
func emitCBORBindEncode(b *bytes.Buffer, t TypePlan, types map[string]TypePlan, shape cborShape) {
	members := cborBindMembers(t, shape)
	fmt.Fprintf(b, "func append%s%s(dst []byte, v %s) []byte {\n", t.Name, shape.suffix, t.Name)
	if shape.asMap {
		fmt.Fprintf(b, "\tdst = cbor.AppendMapHeader(dst, %d)\n", len(members))
	} else {
		fmt.Fprintf(b, "\tdst = cbor.AppendArrayHeader(dst, %d)\n", len(members))
	}
	for _, f := range members {
		if shape.asMap {
			fmt.Fprintf(b, "\tdst = cbor.AppendText(dst, %q)\n", jsonMemberName(f))
		}
		emitCBORAppendValue(b, f, "\t", "v."+f.Name, shape.suffix)
	}
	b.WriteString("\treturn dst\n}\n\n")
}

// emitCBORBindDecode writes decode<T><Shape> over a Reader the caller owns, so
// a nested struct joins its parent's walk with no second scan.
func emitCBORBindDecode(b *bytes.Buffer, t TypePlan, types map[string]TypePlan, shape cborShape) {
	members := cborBindMembers(t, shape)
	fmt.Fprintf(b, "func decode%s%s(cr *cbor.Reader) (%s, error) {\n", t.Name, shape.suffix, t.Name)
	fmt.Fprintf(b, "\tvar out %s\n", t.Name)
	if shape.asMap {
		b.WriteString("\tpairs, indefinite, err := cr.ReadMapHeader()\n")
		b.WriteString("\tif err != nil {\n\t\treturn out, err\n\t}\n")
		b.WriteString("\tif indefinite {\n\t\treturn out, cborbind.ErrShape\n\t}\n")
		b.WriteString("\tfor i := 0; i < pairs; i++ {\n")
		b.WriteString("\t\tkey, err := cr.ReadTextBytes()\n")
		b.WriteString("\t\tif err != nil {\n\t\t\treturn out, err\n\t\t}\n")
		b.WriteString("\t\tswitch string(key) {\n")
		for _, f := range members {
			fmt.Fprintf(b, "\t\tcase %q:\n", jsonMemberName(f))
			emitCBORReadValue(b, f, "\t\t\t", "out."+f.Name, shape.suffix, cborPlainErrRet)
		}
		// An unknown key is skipped rather than refused: that tolerance is the
		// whole reason to pick the map shape.
		b.WriteString("\t\tdefault:\n\t\t\tif err := cr.Skip(); err != nil {\n\t\t\t\treturn out, err\n\t\t\t}\n")
		b.WriteString("\t\t}\n\t}\n")
	} else {
		// The length is part of the contract, so a different one is refused
		// rather than read as far as it goes -- the array shape has no way to
		// tell a renamed field from a reordered one.
		b.WriteString("\tn, indefinite, err := cr.ReadArrayHeader()\n")
		b.WriteString("\tif err != nil {\n\t\treturn out, err\n\t}\n")
		fmt.Fprintf(b, "\tif indefinite || n != %d {\n\t\treturn out, cborbind.ErrShape\n\t}\n", len(members))
		for _, f := range members {
			b.WriteString("\t{\n")
			emitCBORReadValue(b, f, "\t\t", "out."+f.Name, shape.suffix, cborPlainErrRet)
			b.WriteString("\t}\n")
		}
	}
	b.WriteString("\treturn out, nil\n}\n\n")
}

// emitCBORBindMethods publishes the codecs as the methods the entry points
// constrain on, and as the driver's own pair when there is one shape to
// delegate to.
func emitCBORBindMethods(b *bytes.Buffer, t TypePlan) {
	var appended, decoded []cborShape
	for _, shape := range cborShapes {
		if t.Usage&shape.encodeUsage != 0 {
			appended = append(appended, shape)
			fmt.Fprintf(b, "// %s appends v as CBOR, for cborbind.%s.\n", shape.appendMethod, shape.appendMethod)
			fmt.Fprintf(b, "func (v %s) %s(dst []byte) []byte {\n", t.Name, shape.appendMethod)
			fmt.Fprintf(b, "\treturn append%s%s(dst, v)\n}\n\n", t.Name, shape.suffix)
		}
		if t.Usage&shape.decodeUsage != 0 {
			decoded = append(decoded, shape)
			fmt.Fprintf(b, "// %s decodes one CBOR value into v, for cborbind.%s.\n", shape.decodeMethod, shape.decodeMethod)
			fmt.Fprintf(b, "func (v *%s) %s(data []byte) error {\n", t.Name, shape.decodeMethod)
			b.WriteString("\tcr := cbor.ReaderOver(data, cbor.DecoderOptions{})\n")
			fmt.Fprintf(b, "\tgot, err := decode%s%s(&cr)\n", t.Name, shape.suffix)
			b.WriteString("\tif err != nil {\n\t\treturn err\n\t}\n\t*v = got\n\treturn nil\n}\n\n")
		}
	}
	// The driver's pair delegates, so a consumer holding a cbor.Appender
	// reaches the type. A type with both shapes gets neither: there is no
	// unambiguous target, and leaving the ambiguity visible in the type beats
	// resolving it by declaration order.
	if len(appended) == 1 {
		fmt.Fprintf(b, "// AppendCBORTo satisfies cbor.Appender through the one shape %s declares.\n", t.Name)
		fmt.Fprintf(b, "func (v %s) AppendCBORTo(dst []byte) []byte {\n\treturn v.%s(dst)\n}\n\n", t.Name, appended[0].appendMethod)
	}
	if len(decoded) == 1 {
		fmt.Fprintf(b, "// DecodeCBORFrom satisfies cbor.Decodable through the one shape %s declares.\n", t.Name)
		fmt.Fprintf(b, "func (v *%s) DecodeCBORFrom(data []byte) error {\n\treturn v.%s(data)\n}\n\n", t.Name, decoded[0].decodeMethod)
	}
}

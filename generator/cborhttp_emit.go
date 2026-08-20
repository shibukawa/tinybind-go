package generator

import (
	"bytes"
	"fmt"
	"sort"
)

// cborImportPath is the driver's CBOR layer, the one package the emitted
// codecs call into.
const cborImportPath = "github.com/shibukawa/tinygodriver/encoding/cbor"

// CBOR HTTP emission: the application/cbor twins of the JSON mapping emitters,
// produced only when Options.EnableCBORHTTP is set. The codecs are generated
// from the same TypePlan the JSON codecs use, so both formats agree about wire
// names, payload membership, and nesting — a member is spelled the same in a
// JSON document and a CBOR map.
//
// The generated container is a map with text keys in struct field order (or
// RFC 8949 bytewise order under CBORHTTPProfile.RequireSortedKeys), unknown
// keys are skipped on decode, and every limit at the read site defers to the
// one body cap httpbind.MaxCBORBodyBytes enforces. The subset is this mode's
// own, tuned through Options.CBORHTTPProfile; no application profile is named.

// checkCBORHTTPTypes refuses a type whose fields the CBOR HTTP codecs cannot
// carry, naming the field and the reason rather than emitting a document that
// silently drops it.
func checkCBORHTTPTypes(emitted []TypePlan, profile CBORHTTPProfile) error {
	for _, t := range emitted {
		decodes := t.Usage&(UsageBind|UsageDecodeJSON) != 0
		encodes := t.Usage&(UsageWrite|UsageEncodeJSON) != 0
		if !decodes && !encodes {
			continue
		}
		for _, f := range t.Fields {
			if f.IsRest() {
				return fmt.Errorf("cborhttp: %s.%s: a payload:\"*\" rest map has no CBOR mapping; EnableCBORHTTP cannot cover this type", t.Name, f.Name)
			}
			if !isDocumentField(f) {
				if f.Kind == "file" && encodes && !f.JSONSkip {
					return fmt.Errorf("cborhttp: %s.%s: an uploaded file has no CBOR response encoding; mark it json:\"-\" or leave EnableCBORHTTP off", t.Name, f.Name)
				}
				continue
			}
			if f.Kind == KindForeign {
				return fmt.Errorf("cborhttp: %s.%s: the field type carries its own JSON codec and no CBOR one", t.Name, f.Name)
			}
			if profile.RejectFloats && (f.Kind == "float64" || f.ElemKind == "float64") {
				return fmt.Errorf("cborhttp: %s.%s: CBORHTTPProfile.RejectFloats refuses a float64 field", t.Name, f.Name)
			}
			if profile.RequireSortedKeys && f.Kind == KindMap && encodes {
				return fmt.Errorf("cborhttp: %s.%s: a runtime map cannot promise the sorted key order CBORHTTPProfile.RequireSortedKeys demands", t.Name, f.Name)
			}
		}
	}
	return nil
}

// cborHTTPMembers is the member list a CBOR map carries, in the order it is
// emitted: the JSON member set minus uploaded files, sorted bytewise over the
// encoded key when the profile demands it and left in declaration order
// otherwise, matching what the JSON encoder does.
func cborHTTPMembers(t TypePlan, profile CBORHTTPProfile) []FieldPlan {
	var out []FieldPlan
	for _, f := range jsonMembers(t) {
		if f.Kind == "file" {
			continue
		}
		out = append(out, f)
	}
	if profile.RequireSortedKeys {
		sort.SliceStable(out, func(i, j int) bool {
			return bytes.Compare(cborEncodedTextKey(jsonMemberName(out[i])), cborEncodedTextKey(jsonMemberName(out[j]))) < 0
		})
	}
	return out
}

// cborEncodedTextKey is the CBOR encoding of a text-string key, which is what
// RFC 8949 section 4.2.1 bytewise order compares. Computing it here is what
// lets the order be settled at generation time.
func cborEncodedTextKey(s string) []byte {
	n := len(s)
	var head []byte
	switch {
	case n < 24:
		head = []byte{0x60 | byte(n)}
	case n < 256:
		head = []byte{0x78, byte(n)}
	default:
		head = []byte{0x79, byte(n >> 8), byte(n)}
	}
	return append(head, s...)
}

// emitCBORHTTPEncode writes append<T>CBORHTTP, the CBOR twin of append<T>JSON.
//
// Every member is written unconditionally: omitempty and omitzero are JSON
// member semantics, and emitting all members is what keeps the map header
// count a generation-time constant.
func emitCBORHTTPEncode(b *bytes.Buffer, t TypePlan, types map[string]TypePlan, profile CBORHTTPProfile) {
	members := cborHTTPMembers(t, profile)
	fmt.Fprintf(b, "func append%sCBORHTTP(dst []byte, v %s) []byte {\n", t.Name, t.Name)
	fmt.Fprintf(b, "\tdst = cbor.AppendMapHeader(dst, %d)\n", len(members))
	for _, f := range members {
		fmt.Fprintf(b, "\tdst = cbor.AppendText(dst, %q)\n", jsonMemberName(f))
		emitCBORAppendValue(b, f, "\t", "v."+f.Name)
	}
	b.WriteString("\treturn dst\n}\n\n")
	_ = types
}

// emitCBORAppendValue appends one field value to dst, mirroring
// emitAppendValue arm for arm.
func emitCBORAppendValue(b *bytes.Buffer, f FieldPlan, prefix, src string) {
	switch f.Kind {
	case "string":
		fmt.Fprintf(b, "%sdst = cbor.AppendText(dst, %s)\n", prefix, f.Read(src))
	case "int":
		fmt.Fprintf(b, "%sdst = cbor.AppendInt(dst, int64(%s))\n", prefix, src)
	case "int64":
		fmt.Fprintf(b, "%sdst = cbor.AppendInt(dst, %s)\n", prefix, f.Read(src))
	case "bool":
		fmt.Fprintf(b, "%sdst = cbor.AppendBool(dst, %s)\n", prefix, f.Read(src))
	case "float64":
		fmt.Fprintf(b, "%sdst = cbor.AppendFloat(dst, %s)\n", prefix, f.Read(src))
	case KindStruct:
		fmt.Fprintf(b, "%sdst = append%sCBORHTTP(dst, %s)\n", prefix, f.TypeName, src)
	case KindSlice:
		fmt.Fprintf(b, "%sdst = cbor.AppendArrayHeader(dst, len(%s))\n", prefix, src)
		fmt.Fprintf(b, "%sfor i := range %s {\n", prefix, src)
		emitCBORAppendValue(b, FieldPlan{Kind: f.ElemKind, TypeName: f.TypeName}, prefix+"\t", src+"[i]")
		fmt.Fprintf(b, "%s}\n", prefix)
	case KindMap:
		fmt.Fprintf(b, "%sdst = cbor.AppendMapHeader(dst, len(%s))\n", prefix, src)
		fmt.Fprintf(b, "%sfor _, k := range jsonbind.SortedKeys(%s) {\n", prefix, src)
		fmt.Fprintf(b, "%s\tdst = cbor.AppendText(dst, k)\n", prefix)
		emitCBORAppendValue(b, FieldPlan{Kind: f.ElemKind, TypeName: f.TypeName}, prefix+"\t", src+"[k]")
		fmt.Fprintf(b, "%s}\n", prefix)
	default:
		fmt.Fprintf(b, "%sdst = cbor.AppendNull(dst)\n", prefix)
	}
}

// emitCBORHTTPDecode writes decode<T>CBORHTTP, the CBOR twin of
// decode<T>JSON: one map read off a Reader the caller owns, unknown keys
// skipped, so a nested struct at any depth joins its parent's walk without a
// second scan.
func emitCBORHTTPDecode(b *bytes.Buffer, t TypePlan, types map[string]TypePlan, profile CBORHTTPProfile) {
	fmt.Fprintf(b, "func decode%sCBORHTTP(cr *cbor.Reader) (%s, error) {\n", t.Name, t.Name)
	fmt.Fprintf(b, "\tvar out %s\n", t.Name)
	b.WriteString("\tpairs, indefinite, err := cr.ReadMapHeader()\n")
	b.WriteString("\tif err != nil {\n\t\treturn out, err\n\t}\n")
	b.WriteString("\tif indefinite {\n\t\treturn out, cbor.ErrUnexpectedToken\n\t}\n")
	b.WriteString("\tfor i := 0; i < pairs; i++ {\n")
	b.WriteString("\t\tkey, err := cr.ReadTextBytes()\n")
	b.WriteString("\t\tif err != nil {\n\t\t\treturn out, err\n\t\t}\n")
	b.WriteString("\t\tswitch string(key) {\n")
	for _, f := range t.Fields {
		if !isDocumentField(f) || f.IsRest() {
			continue
		}
		fmt.Fprintf(b, "\t\tcase %q:\n", jsonMemberName(f))
		emitCBORReadValue(b, f, "\t\t\t", "out."+f.Name, cborPlainErrRet)
	}
	// No named skip cases: a rest map is refused under this mode, so the
	// default arm's Skip already consumes every member no case stores.
	b.WriteString("\t\tdefault:\n")
	b.WriteString("\t\t\tif err := cr.Skip(); err != nil {\n\t\t\t\treturn out, err\n\t\t\t}\n")
	b.WriteString("\t\t}\n\t}\n")
	b.WriteString("\treturn out, nil\n}\n\n")
	_ = types
	_ = profile
}

// cborErrRet renders the error return for one failed member read. The two walk
// shapes differ only here: the standalone decoder hands the driver error up,
// and the binder wraps it as the 400 a bad payload member already produces on
// the JSON path.
type cborErrRet func(f FieldPlan, what string) string

func cborPlainErrRet(FieldPlan, string) string { return "return out, err" }

func cborBindErrRet(f FieldPlan, what string) string {
	return fmt.Sprintf("return out, httpbind.BindError(%q, \"payload\", %q)", f.Wire, "invalid "+what)
}

// emitCBORReadValue reads one CBOR item into dest.
func emitCBORReadValue(b *bytes.Buffer, f FieldPlan, prefix, dest string, errRet cborErrRet) {
	switch f.Kind {
	case "string":
		fmt.Fprintf(b, "%sv, err := cr.ReadText()\n", prefix)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "string"), prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write("v"))
	case "int":
		fmt.Fprintf(b, "%sv, err := cr.ReadInt()\n", prefix)
		fmt.Fprintf(b, "%sif err != nil || int64(int(v)) != v {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "int"), prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write("int(v)"))
	case "int64":
		fmt.Fprintf(b, "%sv, err := cr.ReadInt()\n", prefix)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "int64"), prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write("v"))
	case "bool":
		fmt.Fprintf(b, "%sv, err := cr.ReadBool()\n", prefix)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "bool"), prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write("v"))
	case "float64":
		fmt.Fprintf(b, "%sv, err := cr.ReadFloat()\n", prefix)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "float64"), prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write("v"))
	case KindStruct:
		fmt.Fprintf(b, "%sv, err := decode%sCBORHTTP(cr)\n", prefix, f.TypeName)
		fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "object"), prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write("v"))
	case KindSlice:
		if !supportedElemKind(f.ElemKind) {
			fmt.Fprintf(b, "%sif err := cr.Skip(); err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "value"), prefix)
			return
		}
		fmt.Fprintf(b, "%sn, indef, err := cr.ReadArrayHeader()\n", prefix)
		fmt.Fprintf(b, "%sif err != nil || indef {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "array"), prefix)
		fmt.Fprintf(b, "%sslice := make([]%s, 0, n)\n", prefix, cborElemGoType(f))
		fmt.Fprintf(b, "%sfor j := 0; j < n; j++ {\n", prefix)
		emitCBORReadElem(b, f, prefix+"\t", "slice = append(slice, %s)", errRet)
		fmt.Fprintf(b, "%s}\n", prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write("slice"))
	case KindMap:
		if !supportedElemKind(f.ElemKind) {
			fmt.Fprintf(b, "%sif err := cr.Skip(); err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "value"), prefix)
			return
		}
		fmt.Fprintf(b, "%sn, indef, err := cr.ReadMapHeader()\n", prefix)
		fmt.Fprintf(b, "%sif err != nil || indef {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "map"), prefix)
		fmt.Fprintf(b, "%sm := make(map[string]%s, n)\n", prefix, cborElemGoType(f))
		fmt.Fprintf(b, "%sfor j := 0; j < n; j++ {\n", prefix)
		fmt.Fprintf(b, "%s\tmk, err := cr.ReadText()\n", prefix)
		fmt.Fprintf(b, "%s\tif err != nil {\n%s\t\t%s\n%s\t}\n", prefix, prefix, errRet(f, "map"), prefix)
		emitCBORReadElem(b, f, prefix+"\t", "m[mk] = %s", errRet)
		fmt.Fprintf(b, "%s}\n", prefix)
		fmt.Fprintf(b, "%s%s = %s\n", prefix, dest, f.Write("m"))
	default:
		fmt.Fprintf(b, "%sif err := cr.Skip(); err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "value"), prefix)
	}
}

// emitCBORReadElem reads one slice or map element and stores it through the
// assign format, which receives the element expression.
func emitCBORReadElem(b *bytes.Buffer, f FieldPlan, prefix string, assign string, errRet cborErrRet) {
	switch f.ElemKind {
	case "string":
		fmt.Fprintf(b, "%sev, err := cr.ReadText()\n", prefix)
	case "int":
		fmt.Fprintf(b, "%sev64, err := cr.ReadInt()\n", prefix)
		fmt.Fprintf(b, "%sif err != nil || int64(int(ev64)) != ev64 {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, "int"), prefix)
		fmt.Fprintf(b, "%sev := int(ev64)\n", prefix)
		fmt.Fprintf(b, prefix+assign+"\n", "ev")
		return
	case "int64":
		fmt.Fprintf(b, "%sev, err := cr.ReadInt()\n", prefix)
	case "bool":
		fmt.Fprintf(b, "%sev, err := cr.ReadBool()\n", prefix)
	case "float64":
		fmt.Fprintf(b, "%sev, err := cr.ReadFloat()\n", prefix)
	case KindStruct:
		fmt.Fprintf(b, "%sev, err := decode%sCBORHTTP(cr)\n", prefix, f.TypeName)
	}
	fmt.Fprintf(b, "%sif err != nil {\n%s\t%s\n%s}\n", prefix, prefix, errRet(f, f.ElemKind), prefix)
	fmt.Fprintf(b, prefix+assign+"\n", "ev")
}

// cborElemGoType is the Go element type a decoded slice or map is made of.
func cborElemGoType(f FieldPlan) string {
	if f.ElemKind == KindStruct {
		return f.TypeName
	}
	return f.ElemKind
}

// emitCBORPayloadWalk writes the binder's CBOR arm: one pass over the body
// map that fills payload fields and their presence flags, before the JSON and
// form arms run and find no body of their kind. Field precedence is untouched
// — a query value still overrides a body member for an input field, because
// the query arms run after this walk and overwrite what it stored.
func emitCBORPayloadWalk(b *bytes.Buffer, t TypePlan, types map[string]TypePlan) {
	b.WriteString("\tif httpbind.IsCBORRequest(r) {\n")
	b.WriteString("\t\tif err := readBody(); err != nil {\n\t\t\treturn out, err\n\t\t}\n")
	b.WriteString("\t}\n")
	b.WriteString("\tif len(cborBody) > 0 {\n")
	b.WriteString("\t\tcrv := cbor.ReaderOver(cborBody, cborHTTPReadOptions(len(cborBody)))\n")
	b.WriteString("\t\tcr := &crv\n")
	b.WriteString("\t\tpairs, indefinite, err := cr.ReadMapHeader()\n")
	b.WriteString("\t\tif err != nil || indefinite {\n\t\t\treturn out, httpbind.BindError(\"body\", \"payload\", \"invalid cbor body\")\n\t\t}\n")
	b.WriteString("\t\tfor i := 0; i < pairs; i++ {\n")
	b.WriteString("\t\t\tkey, err := cr.ReadTextBytes()\n")
	b.WriteString("\t\t\tif err != nil {\n\t\t\t\treturn out, httpbind.BindError(\"body\", \"payload\", \"invalid cbor body\")\n\t\t\t}\n")
	b.WriteString("\t\t\tswitch string(key) {\n")
	for _, f := range t.Fields {
		if !isDocumentField(f) || f.IsRest() {
			continue
		}
		fmt.Fprintf(b, "\t\t\tcase %q:\n", jsonMemberName(f))
		if f.NeedsPresence() {
			fmt.Fprintf(b, "\t\t\t\tpresent%s = true\n", f.Name)
		}
		emitCBORReadValue(b, f, "\t\t\t\t", "out."+f.Name, cborBindErrRet)
	}
	b.WriteString("\t\t\tdefault:\n")
	b.WriteString("\t\t\t\tif err := cr.Skip(); err != nil {\n\t\t\t\t\treturn out, httpbind.BindError(\"body\", \"payload\", \"invalid cbor body\")\n\t\t\t\t}\n")
	b.WriteString("\t\t\t}\n\t\t}\n")
	b.WriteString("\t\tif !cr.Done() {\n\t\t\treturn out, httpbind.BindError(\"body\", \"payload\", \"invalid cbor body\")\n\t\t}\n")
	b.WriteString("\t}\n")
	_ = types
}

// emitCBORHTTPReadOptionsHelper writes the one limit set every generated read
// site passes. Every limit defers to the body length, because the read that
// produced the bytes already enforced httpbind.MaxCBORBodyBytes and a second,
// smaller ceiling here would refuse bodies the deployment allowed. The nesting
// bound stays the driver's stack safety net.
func emitCBORHTTPReadOptionsHelper(b *bytes.Buffer, profile CBORHTTPProfile) {
	b.WriteString("func cborHTTPReadOptions(n int) cbor.DecoderOptions {\n")
	b.WriteString("\treturn cbor.DecoderOptions{\n")
	b.WriteString("\t\tMaxInputBytes:      int64(n),\n")
	b.WriteString("\t\tMaxContainerItems:  n,\n")
	b.WriteString("\t\tMaxStringBytes:     n,\n")
	b.WriteString("\t\tMaxRawMessageBytes: n,\n")
	if profile.RejectFloats {
		b.WriteString("\t\tRejectFloats:       true,\n")
	}
	b.WriteString("\t}\n}\n\n")
}

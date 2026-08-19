package generator

import (
	"fmt"
	"strings"
)

// listDelta writes the surface an identified collection needs: the delta type,
// the merge that fills it, and the apply that puts it back.
//
// Without rule:cbor-entity-identity none of this exists and the collection is
// carried whole, because nothing distinguishes an entity that changed from one
// that was replaced.
func (e *cborEmitter) listDelta(t CborType) {
	elem := t.Elem.Struct
	name := cborListDeltaTypeName(elem)
	if _, done := e.helpers[name]; done {
		return
	}
	e.helpers[name] = ""
	id, idGo := t.ElemIdentity, t.ElemIdentityGo
	var b strings.Builder

	fmt.Fprintf(&b, "// %s is what changed in a collection of %s, keyed by %s.\n//\n"+
		"// Present names the groups carried: bit 0 Set, bit 1 Removed, bit 2 Patched.\n"+
		"// The ordinary tick carries Patched alone, so an unchanged group costs a bit\n"+
		"// rather than an empty array.\n", name, elem, id)
	fmt.Fprintf(&b, "type %s struct {\n\tPresent uint64\n", name)
	fmt.Fprintf(&b, "\t// Set carries an entity that arrived, or one replaced outright.\n\tSet []%s\n", elem)
	fmt.Fprintf(&b, "\t// Removed carries the identity of an entity that left.\n\tRemoved []%s\n", idGo)
	fmt.Fprintf(&b, "\t// Patched carries an entity that changed, as its identity and its delta.\n\tPatched []%s\n",
		cborPatchTypeName(elem))
	b.WriteString("\t// index is scratch the diff reuses across ticks, so a steady state\n" +
		"\t// allocates nothing after the first call. It is never encoded and never\n" +
		"\t// ranged: every loop below walks a slice, which is what keeps the output\n" +
		"\t// the same on two runs.\n")
	fmt.Fprintf(&b, "\tindex map[%s]int32\n}\n\n", idGo)

	fmt.Fprintf(&b, "// %s is one changed %s: its identity, and what changed in it.\n",
		cborPatchTypeName(elem), elem)
	fmt.Fprintf(&b, "type %s struct {\n\t%s %s\n\tDelta %s\n}\n\n",
		cborPatchTypeName(elem), id, idGo, cborDeltaTypeName(elem))

	// --- diff ---
	fmt.Fprintf(&b, "// diff%sListInto fills d with what changed between two collections of %s,\n"+
		"// and reports whether anything did.\n//\n"+
		"// The order of the two slices is not compared: an entity is found by its\n"+
		"// identity, so a collection the game keeps in spawn order diffs the same as\n"+
		"// one it keeps sorted.\n", elem, elem)
	fmt.Fprintf(&b, "func diff%sListInto(d *%s, baseline, current []%s) bool {\n", elem, name, elem)
	b.WriteString("\td.Present = 0\n\td.Set = d.Set[:0]\n\td.Removed = d.Removed[:0]\n\td.Patched = d.Patched[:0]\n")
	fmt.Fprintf(&b, "\tif d.index == nil {\n\t\td.index = make(map[%s]int32, len(baseline))\n\t} else {\n\t\tclear(d.index)\n\t}\n", idGo)
	fmt.Fprintf(&b, "\tfor i := range baseline {\n\t\td.index[baseline[i].%s] = int32(i)\n\t}\n", id)
	b.WriteString("\tfor j := range current {\n")
	fmt.Fprintf(&b, "\t\ti, ok := d.index[current[j].%s]\n", id)
	b.WriteString("\t\tif !ok {\n\t\t\td.Set = append(d.Set, current[j])\n\t\t\tcontinue\n\t\t}\n")
	b.WriteString("\t\t// Marked seen, so the sweep below reports only what did not arrive.\n")
	fmt.Fprintf(&b, "\t\td.index[current[j].%s] = -1\n", id)
	b.WriteString("\t\t// The slot is grown into rather than appended to, so the delta nested\n" +
		"\t\t// inside it -- and the scratch index inside that -- is the one the last\n" +
		"\t\t// tick used. Appending a fresh value here would allocate a new index map\n" +
		"\t\t// per changed entity per tick, which is the whole steady state.\n")
	b.WriteString("\t\tif len(d.Patched) < cap(d.Patched) {\n\t\t\td.Patched = d.Patched[:len(d.Patched)+1]\n" +
		"\t\t} else {\n")
	fmt.Fprintf(&b, "\t\t\td.Patched = append(d.Patched, %s{})\n\t\t}\n", cborPatchTypeName(elem))
	b.WriteString("\t\tp := &d.Patched[len(d.Patched)-1]\n")
	fmt.Fprintf(&b, "\t\tp.%s = current[j].%s\n", id, id)
	fmt.Fprintf(&b, "\t\tif !diff%sInto(&p.Delta, baseline[i], current[j]) {\n"+
		"\t\t\td.Patched = d.Patched[:len(d.Patched)-1]\n\t\t}\n\t}\n", elem)
	fmt.Fprintf(&b, "\tfor i := range baseline {\n\t\tif d.index[baseline[i].%s] >= 0 {\n"+
		"\t\t\td.Removed = append(d.Removed, baseline[i].%s)\n\t\t}\n\t}\n", id, id)
	b.WriteString("\tif len(d.Set) > 0 {\n\t\td.Present |= 1 << 0\n\t}\n")
	b.WriteString("\tif len(d.Removed) > 0 {\n\t\td.Present |= 1 << 1\n\t}\n")
	b.WriteString("\tif len(d.Patched) > 0 {\n\t\td.Present |= 1 << 2\n\t}\n")
	b.WriteString("\treturn d.Present != 0\n}\n\n")

	// --- lookup and sort ---
	fmt.Fprintf(&b, "// cborIndexOf%s finds an entity by identity, and reports -1 for one the\n"+
		"// collection does not hold.\n", elem)
	fmt.Fprintf(&b, "func cborIndexOf%s(s []%s, id %s) int {\n\tfor i := range s {\n"+
		"\t\tif s[i].%s == id {\n\t\t\treturn i\n\t\t}\n\t}\n\treturn -1\n}\n\n", elem, elem, idGo, id)

	fmt.Fprintf(&b, "// cborSort%s puts a collection in identity order.\n//\n"+
		"// An insertion sort, because a collection is nearly sorted already after a\n"+
		"// tick that changed a few entities, and because it allocates nothing and\n"+
		"// pulls in no import on a wasm target.\n", elem)
	fmt.Fprintf(&b, "func cborSort%s(s []%s) {\n\tfor i := 1; i < len(s); i++ {\n"+
		"\t\tfor j := i; j > 0 && s[j].%s < s[j-1].%s; j-- {\n"+
		"\t\t\ts[j], s[j-1] = s[j-1], s[j]\n\t\t}\n\t}\n}\n\n", elem, elem, id, id)

	// --- apply ---
	fmt.Fprintf(&b, "// apply%sListDelta puts a collection delta back.\n//\n"+
		"// Removals first, then patches, then arrivals: a patch names an entity the\n"+
		"// baseline held, and applying it after an arrival of the same identity would\n"+
		"// patch the wrong value. An identity a patch or a removal names and the\n"+
		"// collection does not hold means the baseline is not the one the sender\n"+
		"// diffed against, which is reported rather than ignored.\n", elem)
	fmt.Fprintf(&b, "func apply%sListDelta(v *[]%s, d %s) error {\n", elem, elem, name)
	b.WriteString("\tif d.Present&(1<<1) != 0 {\n\t\tfor _, id := range d.Removed {\n")
	fmt.Fprintf(&b, "\t\t\tk := cborIndexOf%s(*v, id)\n", elem)
	fmt.Fprintf(&b, "\t\t\tif k < 0 {\n\t\t\t\treturn %s\n\t\t\t}\n", cborBaselineError(elem, "removes"))
	b.WriteString("\t\t\t*v = append((*v)[:k], (*v)[k+1:]...)\n\t\t}\n\t}\n")
	b.WriteString("\tif d.Present&(1<<2) != 0 {\n\t\tfor i := range d.Patched {\n")
	fmt.Fprintf(&b, "\t\t\tk := cborIndexOf%s(*v, d.Patched[i].%s)\n", elem, id)
	fmt.Fprintf(&b, "\t\t\tif k < 0 {\n\t\t\t\treturn %s\n\t\t\t}\n", cborBaselineError(elem, "patches"))
	fmt.Fprintf(&b, "\t\t\tif err := apply%sDelta(&(*v)[k], d.Patched[i].Delta); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t}\n\t}\n", elem)
	b.WriteString("\tif d.Present&(1<<0) != 0 {\n\t\tfor i := range d.Set {\n")
	fmt.Fprintf(&b, "\t\t\tk := cborIndexOf%s(*v, d.Set[i].%s)\n", elem, id)
	b.WriteString("\t\t\tif k < 0 {\n\t\t\t\t*v = append(*v, d.Set[i])\n\t\t\t} else {\n\t\t\t\t(*v)[k] = d.Set[i]\n\t\t\t}\n\t\t}\n\t}\n")
	b.WriteString("\t// Identity order, so a receiver fed deltas and a sender holding the same\n" +
		"\t// entities encode to the same bytes, which is what a replay compares.\n")
	fmt.Fprintf(&b, "\tcborSort%s(*v)\n\treturn nil\n}\n\n", elem)

	// --- codec ---
	fmt.Fprintf(&b, "// append%sListDeltaCBOR appends a collection delta.\n//\n"+
		"// Only the groups Present names are written, and Patched alternates identity\n"+
		"// and payload in one array rather than pairing them: a byte per element, and\n"+
		"// a nesting level per hierarchy level.\n", elem)
	fmt.Fprintf(&b, "func append%sListDeltaCBOR(dst []byte, v %s) []byte {\n", elem, name)
	b.WriteString("\tn := 1\n\tfor bit := 0; bit < 3; bit++ {\n\t\tif v.Present&(1<<uint(bit)) != 0 {\n\t\t\tn++\n\t\t}\n\t}\n")
	b.WriteString("\tdst = cbor.AppendArrayHeader(dst, n)\n\tdst = cbor.AppendUint(dst, v.Present)\n")
	b.WriteString("\tif v.Present&(1<<0) != 0 {\n\t\tdst = cbor.AppendArrayHeader(dst, len(v.Set))\n\t\tfor i := range v.Set {\n")
	fmt.Fprintf(&b, "\t\t\tdst = %s(dst, v.Set[i])\n\t\t}\n\t}\n",
		cborAppendFuncName(CborTypePlan{Name: elem, Profile: e.profileOf(elem)}))
	b.WriteString("\tif v.Present&(1<<1) != 0 {\n\t\tdst = cbor.AppendArrayHeader(dst, len(v.Removed))\n\t\tfor i := range v.Removed {\n")
	fmt.Fprintf(&b, "\t\t\tdst = %s\n\t\t}\n\t}\n", cborAppendIdentity("v.Removed[i]", t))
	b.WriteString("\tif v.Present&(1<<2) != 0 {\n\t\tdst = cbor.AppendArrayHeader(dst, len(v.Patched)*2)\n\t\tfor i := range v.Patched {\n")
	fmt.Fprintf(&b, "\t\t\tdst = %s\n", cborAppendIdentity("v.Patched[i]."+id, t))
	fmt.Fprintf(&b, "\t\t\tdst = append%sDeltaCBOR(dst, v.Patched[i].Delta)\n\t\t}\n\t}\n", elem)
	b.WriteString("\treturn dst\n}\n\n")

	fmt.Fprintf(&b, "// decode%sListDeltaCBOR reads a collection delta.\n", elem)
	fmt.Fprintf(&b, "func decode%sListDeltaCBOR(r *cbor.Reader, v *%s) error {\n", elem, name)
	b.WriteString("\tn, indefinite, err := r.ReadArrayHeader()\n\tif err != nil {\n\t\treturn err\n\t}\n")
	fmt.Fprintf(&b, "\tif indefinite || n < 1 {\n\t\treturn %s\n\t}\n", cborError(name))
	b.WriteString("\tpresent, err := r.ReadUint64()\n\tif err != nil {\n\t\treturn err\n\t}\n")
	b.WriteString("\tv.Present = present\n\tv.Set = v.Set[:0]\n\tv.Removed = v.Removed[:0]\n\tv.Patched = v.Patched[:0]\n\tread := 1\n")

	b.WriteString("\tif present&(1<<0) != 0 {\n\t\tcount, indefinite, err := r.ReadArrayHeader()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")
	fmt.Fprintf(&b, "\t\tif indefinite {\n\t\t\treturn %s\n\t\t}\n", cborError(name))
	fmt.Fprintf(&b, "\t\tfor k := 0; k < count; k++ {\n\t\t\tvar x %s\n", elem)
	fmt.Fprintf(&b, "\t\t\tif err := %s(r, &x); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n",
		cborDecodeFuncName(CborTypePlan{Name: elem, Profile: e.profileOf(elem)}))
	b.WriteString("\t\t\tv.Set = append(v.Set, x)\n\t\t}\n\t\tread++\n\t}\n")

	b.WriteString("\tif present&(1<<1) != 0 {\n\t\tcount, indefinite, err := r.ReadArrayHeader()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")
	fmt.Fprintf(&b, "\t\tif indefinite {\n\t\t\treturn %s\n\t\t}\n", cborError(name))
	b.WriteString("\t\tfor k := 0; k < count; k++ {\n")
	fmt.Fprintf(&b, "%s", cborReadIdentity("\t\t\t", "id", t))
	b.WriteString("\t\t\tv.Removed = append(v.Removed, id)\n\t\t}\n\t\tread++\n\t}\n")

	b.WriteString("\tif present&(1<<2) != 0 {\n\t\tcount, indefinite, err := r.ReadArrayHeader()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")
	fmt.Fprintf(&b, "\t\tif indefinite || count%%2 != 0 {\n\t\t\treturn %s\n\t\t}\n", cborError(name))
	b.WriteString("\t\tfor k := 0; k < count; k += 2 {\n")
	fmt.Fprintf(&b, "%s", cborReadIdentity("\t\t\t", "id", t))
	fmt.Fprintf(&b, "\t\t\tvar p %s\n", cborPatchTypeName(elem))
	fmt.Fprintf(&b, "\t\t\tp.%s = id\n", id)
	fmt.Fprintf(&b, "\t\t\tif err := decode%sDeltaCBOR(r, &p.Delta); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n", elem)
	b.WriteString("\t\t\tv.Patched = append(v.Patched, p)\n\t\t}\n\t\tread++\n\t}\n")

	b.WriteString("\t// A group this schema does not know: one item per remaining slot, which\n" +
		"\t// is what keeps an older reader aligned when a group was appended.\n")
	b.WriteString("\tfor ; read < n; read++ {\n\t\tif err := r.Skip(); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\treturn nil\n}\n\n")

	e.helpers[name] = b.String()
}

// profileOf is the profile a nested type's codec was generated under.
func (e *cborEmitter) profileOf(name string) CborProfile {
	if item, ok := e.index[name]; ok {
		return item.Profile
	}
	return CborWire
}

// cborAppendIdentity writes an identity value, which is an integer or a string.
func cborAppendIdentity(expr string, t CborType) string {
	if strings.HasPrefix(t.ElemIdentityGo, "string") || t.ElemIdentityGo == "string" {
		return fmt.Sprintf("cbor.AppendText(dst, string(%s))", expr)
	}
	if strings.HasPrefix(t.ElemIdentityGo, "int") {
		return fmt.Sprintf("cbor.AppendInt(dst, int64(%s))", expr)
	}
	return fmt.Sprintf("cbor.AppendUint(dst, uint64(%s))", expr)
}

// cborReadIdentity reads one identity into a named local.
func cborReadIdentity(tab, name string, t CborType) string {
	goType := t.ElemIdentityGo
	call := "ReadUint64"
	switch {
	case goType == "string":
		call = "ReadText"
	case strings.HasPrefix(goType, "int"):
		call = "ReadInt64"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%sraw%s, err := r.%s()\n", tab, name, call)
	fmt.Fprintf(&b, "%sif err != nil {\n%s\treturn err\n%s}\n", tab, tab, tab)
	fmt.Fprintf(&b, "%s%s := %s(raw%s)\n", tab, name, goType, name)
	return b.String()
}

// cborBaselineError is the refusal a delta naming an entity the baseline does
// not hold produces. It is the receiver's evidence that it is holding a
// different baseline from the one the sender diffed against.
func cborBaselineError(elem, verb string) string {
	return fmt.Sprintf("&cbor.Error{Path: %q, Err: cbor.ErrUnexpectedToken}",
		fmt.Sprintf("%s: the delta %s an entity this baseline does not hold", elem, verb))
}

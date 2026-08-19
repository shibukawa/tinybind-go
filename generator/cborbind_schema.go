package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// cborSchemaAndVersion writes the canonical description of everything in a plan
// that reaches the wire, and the digest of it.
//
// The digest covers wire-observable shape and nothing else. This is the part
// most easily got wrong, and getting it wrong is expensive in both directions:
// if it moves when the bytes did not, every generator upgrade becomes a
// coordinated redeploy of both ends; if it fails to move when the bytes did,
// two versions of a protocol answer to one identifier.
//
// So the description below names the profile, the field order, the wire key,
// the kind and the width of every field, and the identity of every type -- and
// names nothing about this generator, its version, the file it wrote, or which
// directions were asked for. Emitting a decoder for a message the client only
// ever sends changes no byte on the wire, and must change no version.
//
// It is deliberately a separate digest from the one deciding whether to
// regenerate, which legitimately covers the generator binary and go.mod.
func cborSchemaAndVersion(plan *CborPackagePlan) (string, string) {
	var b strings.Builder
	for _, item := range plan.Types {
		fmt.Fprintf(&b, "type %s %s\n", item.Name, item.Profile)
		if item.IntKeys {
			b.WriteString("keys int\n")
		}
		for _, field := range item.Fields {
			b.WriteString("  ")
			if item.Profile == CborWorld {
				if item.IntKeys {
					fmt.Fprintf(&b, "%d ", field.IntKey)
				} else {
					fmt.Fprintf(&b, "%q ", field.Key)
				}
			}
			b.WriteString(cborSchemaType(field.Type))
			b.WriteString("\n")
		}
	}
	schema := b.String()
	sum := sha256.Sum256([]byte(schema))
	return schema, hex.EncodeToString(sum[:8])
}

// cborSchemaType writes one field type as the schema describes it.
//
// A self-encoding type is named rather than described, because its bytes are
// its own method's and this generator cannot see them. That name is the whole
// record of it: renaming the type moves the version, which is right, since a
// type carrying a different scale is a different type by decision.
func cborSchemaType(t CborType) string {
	switch t.Kind {
	case CborUint, CborInt:
		bits := t.Bits
		if bits == 0 {
			// A platform-sized int is read at 64 everywhere, so that is what
			// the schema records; a target's word size is not part of the wire.
			bits = 64
		}
		return fmt.Sprintf("%s%d", t.Kind, bits)
	case CborSlice:
		return "[]" + cborSchemaType(*t.Elem)
	case CborStruct:
		return "struct " + t.Struct
	case CborSelf:
		return "self " + t.Go
	default:
		return string(t.Kind)
	}
}

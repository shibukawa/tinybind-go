package generator

import (
	"fmt"
	"strconv"
)

func (e *firestoreEmitter) decodeEntity(entity FirestoreEntityPlan, receiver string) error {
	fmt.Fprintf(&e.body, "// DecodeEntity fills %s from a Datastore entity. A property the entity does\n", entity.Name)
	e.body.WriteString("// not carry leaves its field untouched, since Datastore is schemaless and an\n")
	e.body.WriteString("// older writer is normal rather than exceptional.\n")
	fmt.Fprintf(&e.body, "func (%s *%s) DecodeEntity(e datastore.Entity) error {\n", receiver, entity.Name)

	for _, f := range entity.Properties() {
		value := e.temp("value")
		fmt.Fprintf(&e.body, "\tif %s, ok := e.Properties[%s]; ok {\n", value, strconv.Quote(f.Property))
		if err := e.decodeInto(receiver+"."+f.Name, value, f.Property, f.Type, "\t\t"); err != nil {
			return fmt.Errorf("%s.%s: %w", entity.Name, f.Name, err)
		}
		e.body.WriteString("\t}\n")
	}

	// The key fields come from the key, not from the properties, so a decoded
	// value carries its own identity without a second read.
	if identity, ok := entity.Identity(); ok {
		e.body.WriteString("\tif e.Key != nil && len(e.Key.Path) > 0 {\n")
		e.body.WriteString("\t\tleaf := e.Key.Path[len(e.Key.Path)-1]\n")
		switch identity.Role {
		case "name":
			fmt.Fprintf(&e.body, "\t\t%s.%s = %s(leaf.Name)\n", receiver, identity.Name, identity.Type.Go)
		case "id":
			fmt.Fprintf(&e.body, "\t\t%s.%s = %s(leaf.ID)\n", receiver, identity.Name, identity.Type.Go)
		}
		if parent, ok := entity.Parent(); ok && parent.Type.Kind == FirestoreKeyRef {
			// The ancestor path is everything before the leaf. An entity with
			// no ancestor leaves the field zero rather than holding a key whose
			// path is empty but whose namespace is not.
			e.body.WriteString("\t\tif len(e.Key.Path) > 1 {\n")
			fmt.Fprintf(&e.body, "\t\t\t%s.%s = datastore.Key{Namespace: e.Key.Namespace, Path: e.Key.Path[:len(e.Key.Path)-1]}\n", receiver, parent.Name)
			e.body.WriteString("\t\t}\n")
		}
		e.body.WriteString("\t}\n")
	}
	if version, ok := entity.Version(); ok {
		fmt.Fprintf(&e.body, "\t%s.%s = %s(e.Version)\n", receiver, version.Name, version.Type.Go)
	}

	e.body.WriteString("\treturn nil\n}\n\n")
	return nil
}

// decodeInto writes the statements that fill target from the value expression
// source. It is used for fields and array elements alike, so target is any
// assignable expression and source any expression of type datastore.Value.
//
// Every failure names the property and both kinds, because the caller's next
// question is always which property and what was actually stored there.
func (e *firestoreEmitter) decodeInto(target, source, property string, t FirestoreType, indent string) error {
	name := strconv.Quote(property)
	typeError := func(expected string) string {
		return fmt.Sprintf("return firestorebind.TypeError(%s, %q, %s)", name, expected, source)
	}

	switch t.Kind {
	case FirestoreString:
		text := e.temp("text")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsString()\n", indent, text, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("string"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, text)

	case FirestoreInt, FirestoreUint:
		// The stored form is text, so it is parsed at the field's own width: a
		// value the field cannot hold is an error rather than a silent wrap.
		text := e.temp("text")
		number := e.temp("number")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsNumber()\n", indent, text, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("integer"), indent)
		parse, expected := firestoreParseNumber(text, t)
		fmt.Fprintf(&e.body, "%s%s, err := %s\n", indent, number, parse)
		fmt.Fprintf(&e.body, "%sif err != nil {\n%s\treturn firestorebind.ValueError(%s, %q, err)\n%s}\n",
			indent, indent, name, "integer is not a "+expected, indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, number)

	case FirestoreDouble:
		// A double is a real JSON number and never arrives as an integer: the
		// two are different types to Datastore, and accepting either here would
		// make a value stop matching the filter it was written for.
		number := e.temp("number")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsFloat()\n", indent, number, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("double"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, number)

	case FirestoreBool:
		value := e.temp("flag")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsBool()\n", indent, value, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("boolean"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, value)

	case FirestoreBlob:
		value := e.temp("blob")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsBytes()\n", indent, value, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("blob"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, value)

	case FirestoreTime:
		when := e.temp("when")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsTime()\n", indent, when, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("timestamp"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, when)

	case FirestoreKeyRef:
		key := e.temp("key")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsKey()\n", indent, key, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("key"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, key)

	case FirestoreGeo:
		point := e.temp("point")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsGeoPoint()\n", indent, point, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("geoPoint"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, point)

	case FirestoreRaw:
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, source)

	case FirestoreStruct:
		nested := e.temp("nested")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsEntity()\n", indent, nested, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("entity"), indent)
		fmt.Fprintf(&e.body, "%sif err := %s.DecodeEntity(%s); err != nil {\n%s\treturn err\n%s}\n",
			indent, target, nested, indent, indent)

	case FirestorePointer:
		value := e.temp("value")
		fmt.Fprintf(&e.body, "%sif %s.IsNull() {\n%s\t%s = nil\n%s} else {\n", indent, source, indent, target, indent)
		fmt.Fprintf(&e.body, "%s\tvar %s %s\n", indent, value, t.Elem.Go)
		if err := e.decodeInto(value, source, property, *t.Elem, indent+"\t"); err != nil {
			return err
		}
		fmt.Fprintf(&e.body, "%s\t%s = &%s\n%s}\n", indent, target, value, indent)

	case FirestoreArray:
		members := e.temp("members")
		decoded := e.temp("decoded")
		value := e.temp("value")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsArray()\n", indent, members, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("array"), indent)
		fmt.Fprintf(&e.body, "%s%s := make(%s, 0, len(%s))\n", indent, decoded, t.Go, members)
		fmt.Fprintf(&e.body, "%sfor _, member := range %s {\n", indent, members)
		fmt.Fprintf(&e.body, "%s\tvar %s %s\n", indent, value, t.Elem.Go)
		if err := e.decodeInto(value, "member", property, *t.Elem, indent+"\t"); err != nil {
			return err
		}
		fmt.Fprintf(&e.body, "%s\t%s = append(%s, %s)\n", indent, decoded, decoded, value)
		fmt.Fprintf(&e.body, "%s}\n", indent)
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, decoded)

	default:
		return fmt.Errorf("no decoder for property kind %s", t.Kind)
	}
	return nil
}

// firestoreParseNumber returns the strconv call that turns stored text into a Go
// integer, and the name of what it parses for the error message.
func firestoreParseNumber(source string, t FirestoreType) (call, expected string) {
	switch t.Kind {
	case FirestoreInt:
		return fmt.Sprintf("strconv.ParseInt(%s, 10, %d)", source, t.Bits), "signed integer"
	case FirestoreUint:
		return fmt.Sprintf("strconv.ParseUint(%s, 10, %d)", source, t.Bits), "unsigned integer"
	default:
		return source, "integer"
	}
}

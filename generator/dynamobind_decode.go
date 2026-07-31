package generator

import (
	"fmt"
	"strconv"
)

func (e *dynamoEmitter) decodeItem(item DynamoItemPlan, receiver string) error {
	fmt.Fprintf(&e.body, "// DecodeItem fills %s from a DynamoDB item. An attribute the item does\n", item.Name)
	e.body.WriteString("// not carry leaves its field untouched.\n")
	fmt.Fprintf(&e.body, "func (%s *%s) DecodeItem(item dynamodb.Item) error {\n", receiver, item.Name)
	for _, f := range item.Fields {
		attr := e.temp("attr")
		fmt.Fprintf(&e.body, "\tif %s, ok := item[%s]; ok {\n", attr, strconv.Quote(f.Attribute))
		if err := e.decodeInto(receiver+"."+f.Name, attr, f.Attribute, f.Type, "\t\t"); err != nil {
			return fmt.Errorf("%s.%s: %w", item.Name, f.Name, err)
		}
		e.body.WriteString("\t}\n")
	}
	e.body.WriteString("\treturn nil\n}\n\n")
	return nil
}

// decodeInto writes the statements that fill target from the attribute value
// expression source. It is used for fields, list elements and map values alike,
// so target is any assignable expression and source any expression of type
// dynamodb.AttributeValue.
//
// Every failure names the attribute and both kinds, because the caller's next
// question is always which attribute and what was actually stored there.
func (e *dynamoEmitter) decodeInto(target, source, attribute string, t DynamoType, indent string) error {
	name := strconv.Quote(attribute)
	typeError := func(expected string) string {
		return fmt.Sprintf("return dynamobind.TypeError(%s, %q, %s)", name, expected, source)
	}

	switch t.Kind {
	case DynamoString:
		text := e.temp("text")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsString()\n", indent, text, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("S"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, text)

	case DynamoInt, DynamoUint, DynamoFloat:
		text := e.temp("text")
		number := e.temp("number")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsNumber()\n", indent, text, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("N"), indent)
		parse, expected := dynamoParseNumber(text, t)
		fmt.Fprintf(&e.body, "%s%s, err := %s\n", indent, number, parse)
		fmt.Fprintf(&e.body, "%sif err != nil {\n%s\treturn dynamobind.ValueError(%s, %q, err)\n%s}\n",
			indent, indent, name, "number is not a "+expected, indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, number)

	case DynamoBool:
		value := e.temp("flag")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsBool()\n", indent, value, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("BOOL"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, value)

	case DynamoBytes:
		value := e.temp("binary")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsBytes()\n", indent, value, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("B"), indent)
		fmt.Fprintf(&e.body, "%s%s = %s(%s)\n", indent, target, t.Go, value)

	case DynamoTime:
		text := e.temp("text")
		when := e.temp("when")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsString()\n", indent, text, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("S"), indent)
		fmt.Fprintf(&e.body, "%s%s, err := time.Parse(time.RFC3339Nano, %s)\n", indent, when, text)
		fmt.Fprintf(&e.body, "%sif err != nil {\n%s\treturn dynamobind.ValueError(%s, %q, err)\n%s}\n",
			indent, indent, name, "time is not RFC 3339", indent)
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, when)

	case DynamoUnixTime:
		text := e.temp("text")
		seconds := e.temp("seconds")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsNumber()\n", indent, text, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("N"), indent)
		fmt.Fprintf(&e.body, "%s%s, err := strconv.ParseInt(%s, 10, 64)\n", indent, seconds, text)
		fmt.Fprintf(&e.body, "%sif err != nil {\n%s\treturn dynamobind.ValueError(%s, %q, err)\n%s}\n",
			indent, indent, name, "unix time is not an int64", indent)
		fmt.Fprintf(&e.body, "%s%s = time.Unix(%s, 0).UTC()\n", indent, target, seconds)

	case DynamoRaw:
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, source)

	case DynamoStruct:
		entries := e.temp("entries")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsMap()\n", indent, entries, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("M"), indent)
		fmt.Fprintf(&e.body, "%sif err := %s.DecodeItem(%s); err != nil {\n%s\treturn err\n%s}\n",
			indent, target, entries, indent, indent)

	case DynamoPointer:
		value := e.temp("value")
		fmt.Fprintf(&e.body, "%sif %s.IsNull() {\n%s\t%s = nil\n%s} else {\n", indent, source, indent, target, indent)
		fmt.Fprintf(&e.body, "%s\tvar %s %s\n", indent, value, t.Elem.Go)
		if err := e.decodeInto(value, source, attribute, *t.Elem, indent+"\t"); err != nil {
			return err
		}
		fmt.Fprintf(&e.body, "%s\t%s = &%s\n%s}\n", indent, target, value, indent)

	case DynamoStringSet:
		e.decodeSet(target, source, attribute, t, indent, "SS", "KindStringSet", func(member string) string {
			return fmt.Sprintf("%s(%s)", t.Elem.Go, member)
		})

	case DynamoBinarySet:
		e.decodeSet(target, source, attribute, t, indent, "BS", "KindBinarySet", func(member string) string {
			return fmt.Sprintf("%s(%s)", t.Elem.Go, member)
		})

	case DynamoNumberSet:
		members := e.temp("members")
		number := e.temp("number")
		fmt.Fprintf(&e.body, "%sif %s.Kind() != dynamodb.KindNumberSet {\n%s\t%s\n%s}\n",
			indent, source, indent, typeError("NS"), indent)
		fmt.Fprintf(&e.body, "%s%s := make(%s, 0, len(%s.NS))\n", indent, members, t.Go, source)
		fmt.Fprintf(&e.body, "%sfor _, member := range %s.NS {\n", indent, source)
		parse, expected := dynamoParseNumber("member", *t.Elem)
		fmt.Fprintf(&e.body, "%s\t%s, err := %s\n", indent, number, parse)
		fmt.Fprintf(&e.body, "%s\tif err != nil {\n%s\t\treturn dynamobind.ValueError(%s, %q, err)\n%s\t}\n",
			indent, indent, name, "number is not a "+expected, indent)
		fmt.Fprintf(&e.body, "%s\t%s = append(%s, %s(%s))\n", indent, members, members, t.Elem.Go, number)
		fmt.Fprintf(&e.body, "%s}\n", indent)
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, members)

	case DynamoList:
		list := e.temp("list")
		members := e.temp("members")
		value := e.temp("value")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsList()\n", indent, list, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("L"), indent)
		fmt.Fprintf(&e.body, "%s%s := make(%s, 0, len(%s))\n", indent, members, t.Go, list)
		fmt.Fprintf(&e.body, "%sfor _, member := range %s {\n", indent, list)
		fmt.Fprintf(&e.body, "%s\tvar %s %s\n", indent, value, t.Elem.Go)
		if err := e.decodeInto(value, "member", attribute, *t.Elem, indent+"\t"); err != nil {
			return err
		}
		fmt.Fprintf(&e.body, "%s\t%s = append(%s, %s)\n", indent, members, members, value)
		fmt.Fprintf(&e.body, "%s}\n", indent)
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, members)

	case DynamoMap:
		entries := e.temp("entries")
		members := e.temp("members")
		value := e.temp("value")
		fmt.Fprintf(&e.body, "%s%s, ok := %s.AsMap()\n", indent, entries, source)
		fmt.Fprintf(&e.body, "%sif !ok {\n%s\t%s\n%s}\n", indent, indent, typeError("M"), indent)
		fmt.Fprintf(&e.body, "%s%s := make(%s, len(%s))\n", indent, members, t.Go, entries)
		fmt.Fprintf(&e.body, "%sfor key, member := range %s {\n", indent, entries)
		fmt.Fprintf(&e.body, "%s\tvar %s %s\n", indent, value, t.Elem.Go)
		if err := e.decodeInto(value, "member", attribute, *t.Elem, indent+"\t"); err != nil {
			return err
		}
		fmt.Fprintf(&e.body, "%s\t%s[%s(key)] = %s\n", indent, members, t.MapKey, value)
		fmt.Fprintf(&e.body, "%s}\n", indent)
		fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, members)

	default:
		return fmt.Errorf("no decoder for attribute kind %s", t.Kind)
	}
	return nil
}

// decodeSet emits the shape shared by the string and binary sets, whose members
// need conversion but no parsing.
func (e *dynamoEmitter) decodeSet(target, source, attribute string, t DynamoType, indent, field, kind string, convert func(string) string) {
	members := e.temp("members")
	fmt.Fprintf(&e.body, "%sif %s.Kind() != dynamodb.%s {\n", indent, source, kind)
	fmt.Fprintf(&e.body, "%s\treturn dynamobind.TypeError(%s, %q, %s)\n%s}\n",
		indent, strconv.Quote(attribute), field, source, indent)
	fmt.Fprintf(&e.body, "%s%s := make(%s, 0, len(%s.%s))\n", indent, members, t.Go, source, field)
	fmt.Fprintf(&e.body, "%sfor _, member := range %s.%s {\n", indent, source, field)
	fmt.Fprintf(&e.body, "%s\t%s = append(%s, %s)\n", indent, members, members, convert("member"))
	fmt.Fprintf(&e.body, "%s}\n", indent)
	fmt.Fprintf(&e.body, "%s%s = %s\n", indent, target, members)
}

// dynamoParseNumber returns the strconv call that turns stored text into a Go
// number, and the name of what it parses for the error message. The text is
// parsed at the field's own width, so a value the field cannot hold is an error
// rather than a silent wrap.
func dynamoParseNumber(source string, t DynamoType) (call, expected string) {
	switch t.Kind {
	case DynamoInt:
		return fmt.Sprintf("strconv.ParseInt(%s, 10, %d)", source, t.Bits), "signed integer"
	case DynamoUint:
		return fmt.Sprintf("strconv.ParseUint(%s, 10, %d)", source, t.Bits), "unsigned integer"
	case DynamoFloat:
		bits := t.Bits
		if bits == 0 {
			bits = 64
		}
		return fmt.Sprintf("strconv.ParseFloat(%s, %d)", source, bits), "float"
	default:
		return source, "number"
	}
}

// Package codegen emits reflection-free apply functions from intermediate minitoml.Document keys.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"strconv"
	"time"
)

// FieldKind is the Go field kind supported by the generator.
type FieldKind int

const (
	// FieldString is a string field.
	FieldString FieldKind = iota
	// FieldBool is a bool field.
	FieldBool
	// FieldInt is an int field.
	FieldInt
	// FieldDuration is a time.Duration field parsed from a Go duration string.
	FieldDuration
	// FieldStringSlice is a []string field.
	FieldStringSlice
	// FieldStruct is a nested struct field.
	FieldStruct
	// FieldStructSlice is a slice of structs read from an array of tables.
	FieldStructSlice
)

// IntFieldSpec describes how one integer field is parsed and assigned.
type IntFieldSpec struct {
	// Name is the Go type the generated code converts to.
	Name string
	// Signed selects strconv.ParseInt over strconv.ParseUint.
	Signed bool
	// BitSize is the strconv bit size. Zero means the platform-sized int or
	// uint, which is the value strconv itself takes for that case, so the
	// accepted range follows the build target instead of the generator host.
	BitSize int
}

var intFieldSpecs = map[string]IntFieldSpec{
	"int":    {Name: "int", Signed: true},
	"int8":   {Name: "int8", Signed: true, BitSize: 8},
	"int16":  {Name: "int16", Signed: true, BitSize: 16},
	"int32":  {Name: "int32", Signed: true, BitSize: 32},
	"int64":  {Name: "int64", Signed: true, BitSize: 64},
	"uint":   {Name: "uint"},
	"uint8":  {Name: "uint8", BitSize: 8},
	"uint16": {Name: "uint16", BitSize: 16},
	"uint32": {Name: "uint32", BitSize: 32},
	"uint64": {Name: "uint64", BitSize: 64},
}

// IntFieldSpecOf resolves a Field.GoType. An empty name means int, so a field
// built before widths were carried keeps its exact generated form.
func IntFieldSpecOf(goType string) (IntFieldSpec, error) {
	if goType == "" {
		goType = "int"
	}
	spec, ok := intFieldSpecs[goType]
	if !ok {
		return IntFieldSpec{}, fmt.Errorf("unsupported integer type %q", goType)
	}
	return spec, nil
}

// ParseLiteral reads one default value at the field's own width. The returned
// text is the value re-rendered as an untyped Go integer literal.
func (s IntFieldSpec) ParseLiteral(text string) (string, error) {
	if s.Signed {
		n, err := strconv.ParseInt(text, 10, s.BitSize)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(n, 10), nil
	}
	n, err := strconv.ParseUint(text, 10, s.BitSize)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(n, 10), nil
}

// Field describes one struct field for intermediate-form apply generation.
type Field struct {
	// GoName is the exported Go field name.
	GoName string
	// Key is the relative TOML/intermediate key segment under the parent prefix.
	Key string
	// Kind is the field type.
	Kind FieldKind
	// Nested holds child fields when Kind is FieldStruct or FieldStructSlice.
	// For FieldStructSlice they are the element struct's fields.
	Nested []Field
	// ElemType is the element struct's Go type name when Kind is FieldStructSlice.
	ElemType string
	// GoType is the Go type an integer field is assigned as (int64, uint32, or
	// a defined type over one of them). Empty means int, which keeps the
	// generated form of a plain int field unchanged.
	GoType string
	// Default is an optional default used when the key is absent (string form).
	Default string
	// Opt is an optional CLI name override ("long" or "long,short").
	Opt string
	// Env is an optional exact environment variable override, or "-" to disable it.
	Env string
	// Help is optional CLI/scaffold help text.
	Help string
	// Arg is a subcommand-only positional role: required, optional, or *.
	Arg string
	// DependsOn is the absolute parent config key from a dependon tag. When the
	// parent reads as empty, this field is omitted from provenance output.
	DependsOn string
	// Falsy is the enum choice from a falsy tag that means "off" for this field.
	// An empty value resolves to it, and it hides fields that depend on this one.
	Falsy string
	// Secret is the disclosure mode from a secret tag: hide, mask, or show. On
	// a nested struct it covers every field of the subtree.
	Secret string
}

// Spec describes one Bind-style config struct and its prefix table name.
type Spec struct {
	// TypeName is the Go struct type name (e.g. WebServiceConfig).
	TypeName string
	// Prefix is the Bind prefix / top-level TOML table (e.g. webservice).
	Prefix string
	// Fields are the struct fields.
	Fields []Field
}

// Generate emits Go source with Apply* functions that map minitoml.Document keys onto structs.
// Key paths are resolved at generation time; generated code does not use reflection or tag parsing.
func Generate(packageName string, specs []Spec) ([]byte, error) {
	if packageName == "" {
		return nil, fmt.Errorf("codegen: package name is required")
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("codegen: at least one Spec is required")
	}

	var b bytes.Buffer
	b.WriteString("// Code generated by minitoml/codegen; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", packageName)
	b.WriteString("import (\n")
	b.WriteString("\t\"fmt\"\n")
	if specsNeedTime(specs) {
		b.WriteString("\t\"time\"\n")
	}
	b.WriteString("\n")
	b.WriteString("\t\"github.com/shibukawa/tinybind-go/minitoml\"\n")
	b.WriteString(")\n\n")

	for _, spec := range specs {
		if err := emitApply(&b, spec); err != nil {
			return nil, err
		}
	}

	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return b.Bytes(), fmt.Errorf("codegen: format: %w\n%s", err, b.String())
	}
	return formatted, nil
}

func emitApply(b *bytes.Buffer, spec Spec) error {
	if spec.TypeName == "" || spec.Prefix == "" {
		return fmt.Errorf("codegen: TypeName and Prefix are required")
	}
	fn := "Apply" + spec.TypeName
	fmt.Fprintf(b, "// %s maps intermediate document keys under prefix %q onto dst.\n", fn, spec.Prefix)
	fmt.Fprintf(b, "func %s(dst *%s, doc minitoml.Document) error {\n", fn, spec.TypeName)
	b.WriteString("\tif dst == nil {\n")
	b.WriteString("\t\treturn fmt.Errorf(\"minitoml: nil destination\")\n")
	b.WriteString("\t}\n")
	// Top-level lookup keys already carry the prefix, so diagnostics need no extra one.
	scope := emitScope{doc: "doc", indent: "\t"}
	if err := emitFields(b, scope, "dst", spec.Prefix, spec.Fields); err != nil {
		return err
	}
	b.WriteString("\treturn nil\n")
	b.WriteString("}\n\n")
	return nil
}

// emitScope is the destination one group of fields reads from: the Go
// expression holding its minitoml.Document, the indent of the emitted block,
// and the key path used in diagnostics (which differs from the lookup key
// inside an array-of-tables element, whose keys are element-relative).
type emitScope struct {
	doc        string
	indent     string
	diagPrefix string
	depth      int
}

func (s emitScope) diagKey(lookupKey string) string {
	return joinKey(s.diagPrefix, lookupKey)
}

func emitFields(b *bytes.Buffer, scope emitScope, recv, prefix string, fields []Field) error {
	for _, f := range fields {
		if f.GoName == "" || f.Key == "" {
			return fmt.Errorf("codegen: field GoName and Key are required")
		}
		fullKey := joinKey(prefix, f.Key)
		access := recv + "." + f.GoName
		switch f.Kind {
		case FieldString:
			emitStringField(b, scope, access, fullKey, f.Default)
		case FieldBool:
			emitBoolField(b, scope, access, fullKey, f.Default)
		case FieldInt:
			if err := emitIntField(b, scope, access, fullKey, f); err != nil {
				return fmt.Errorf("codegen: %s: %w", f.GoName, err)
			}
		case FieldDuration:
			if err := emitDurationField(b, scope, access, fullKey, f.Default); err != nil {
				return fmt.Errorf("codegen: %s: %w", f.GoName, err)
			}
		case FieldStringSlice:
			emitStringSliceField(b, scope, access, fullKey)
		case FieldStruct:
			if err := emitFields(b, scope, access, fullKey, f.Nested); err != nil {
				return err
			}
		case FieldStructSlice:
			if err := emitStructSliceField(b, scope, access, fullKey, f); err != nil {
				return err
			}
		default:
			return fmt.Errorf("codegen: unsupported field kind %d for %s", f.Kind, f.GoName)
		}
	}
	return nil
}

func emitStringField(b *bytes.Buffer, scope emitScope, access, fullKey, def string) {
	in := scope.indent
	fmt.Fprintf(b, "%sif v, ok := %s.Get(%q); ok {\n", in, scope.doc, fullKey)
	fmt.Fprintf(b, "%s\ts, err := v.AsString()\n", in)
	fmt.Fprintf(b, "%s\tif err != nil {\n", in)
	fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%w\", err)\n", in, scope.diagKey(fullKey))
	fmt.Fprintf(b, "%s\t}\n", in)
	fmt.Fprintf(b, "%s\t%s = s\n", in, access)
	if def != "" {
		fmt.Fprintf(b, "%s} else {\n", in)
		fmt.Fprintf(b, "%s\t%s = %s\n", in, access, strconv.Quote(def))
	}
	fmt.Fprintf(b, "%s}\n", in)
}

func emitBoolField(b *bytes.Buffer, scope emitScope, access, fullKey, def string) {
	in := scope.indent
	fmt.Fprintf(b, "%sif v, ok := %s.Get(%q); ok {\n", in, scope.doc, fullKey)
	fmt.Fprintf(b, "%s\tbb, err := v.AsBool()\n", in)
	fmt.Fprintf(b, "%s\tif err != nil {\n", in)
	fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%w\", err)\n", in, scope.diagKey(fullKey))
	fmt.Fprintf(b, "%s\t}\n", in)
	fmt.Fprintf(b, "%s\t%s = bb\n", in, access)
	if def != "" {
		fmt.Fprintf(b, "%s} else {\n", in)
		fmt.Fprintf(b, "%s\t%s = %v\n", in, access, def == "true")
	}
	fmt.Fprintf(b, "%s}\n", in)
}

// emitIntField reads the key as an int64 and converts it to the field's own
// integer type. The round-trip guard rejects a value the target type cannot
// hold instead of storing a wrapped one; it holds for every width, including
// the platform-sized int and uint, without importing math.
func emitIntField(b *bytes.Buffer, scope emitScope, access, fullKey string, f Field) error {
	spec, err := IntFieldSpecOf(f.GoType)
	if err != nil {
		return err
	}
	in := scope.indent
	fmt.Fprintf(b, "%sif v, ok := %s.Get(%q); ok {\n", in, scope.doc, fullKey)
	fmt.Fprintf(b, "%s\tn, err := v.AsInt()\n", in)
	fmt.Fprintf(b, "%s\tif err != nil {\n", in)
	fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%w\", err)\n", in, scope.diagKey(fullKey))
	fmt.Fprintf(b, "%s\t}\n", in)
	if !spec.Signed {
		fmt.Fprintf(b, "%s\tif n < 0 {\n", in)
		fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%d out of range for %s\", n)\n", in, scope.diagKey(fullKey), spec.Name)
		fmt.Fprintf(b, "%s\t}\n", in)
	}
	if spec.Name != "int64" {
		fmt.Fprintf(b, "%s\tif int64(%s(n)) != n {\n", in, spec.Name)
		fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%d out of range for %s\", n)\n", in, scope.diagKey(fullKey), spec.Name)
		fmt.Fprintf(b, "%s\t}\n", in)
	}
	fmt.Fprintf(b, "%s\t%s = %s(n)\n", in, access, spec.Name)
	if f.Default != "" {
		literal, err := spec.ParseLiteral(f.Default)
		if err != nil {
			return fmt.Errorf("invalid %s default %q: %w", spec.Name, f.Default, err)
		}
		fmt.Fprintf(b, "%s} else {\n", in)
		fmt.Fprintf(b, "%s\t%s = %s\n", in, access, literal)
	}
	fmt.Fprintf(b, "%s}\n", in)
	return nil
}

// emitDurationField reads the key as a string and parses it with
// time.ParseDuration. A bare number is rejected: the unit is never implied.
func emitDurationField(b *bytes.Buffer, scope emitScope, access, fullKey, def string) error {
	in := scope.indent
	fmt.Fprintf(b, "%sif v, ok := %s.Get(%q); ok {\n", in, scope.doc, fullKey)
	fmt.Fprintf(b, "%s\ts, err := v.AsString()\n", in)
	fmt.Fprintf(b, "%s\tif err != nil {\n", in)
	fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%w\", err)\n", in, scope.diagKey(fullKey))
	fmt.Fprintf(b, "%s\t}\n", in)
	fmt.Fprintf(b, "%s\td, err := time.ParseDuration(s)\n", in)
	fmt.Fprintf(b, "%s\tif err != nil {\n", in)
	fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%w\", err)\n", in, scope.diagKey(fullKey))
	fmt.Fprintf(b, "%s\t}\n", in)
	fmt.Fprintf(b, "%s\t%s = d\n", in, access)
	if def != "" {
		value, err := time.ParseDuration(def)
		if err != nil {
			return fmt.Errorf("invalid duration default %q: %w", def, err)
		}
		fmt.Fprintf(b, "%s} else {\n", in)
		fmt.Fprintf(b, "%s\t%s = %d // %s\n", in, access, int64(value), value)
	}
	fmt.Fprintf(b, "%s}\n", in)
	return nil
}

func specsNeedTime(specs []Spec) bool {
	var need func([]Field) bool
	need = func(fields []Field) bool {
		for _, f := range fields {
			if f.Kind == FieldDuration ||
				((f.Kind == FieldStruct || f.Kind == FieldStructSlice) && need(f.Nested)) {
				return true
			}
		}
		return false
	}
	for _, spec := range specs {
		if need(spec.Fields) {
			return true
		}
	}
	return false
}

func emitStringSliceField(b *bytes.Buffer, scope emitScope, access, fullKey string) {
	in := scope.indent
	fmt.Fprintf(b, "%sif v, ok := %s.Get(%q); ok {\n", in, scope.doc, fullKey)
	fmt.Fprintf(b, "%s\tsl, err := v.AsStringSlice()\n", in)
	fmt.Fprintf(b, "%s\tif err != nil {\n", in)
	fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%w\", err)\n", in, scope.diagKey(fullKey))
	fmt.Fprintf(b, "%s\t}\n", in)
	fmt.Fprintf(b, "%s\t%s = sl\n", in, access)
	fmt.Fprintf(b, "%s}\n", in)
}

// emitStructSliceField expands one array of tables into a loop that fills a
// slice of structs. Element keys are relative to the array key, so the nested
// fields are emitted with an empty prefix against the element document.
func emitStructSliceField(b *bytes.Buffer, scope emitScope, access, fullKey string, f Field) error {
	if f.ElemType == "" {
		return fmt.Errorf("codegen: field %s needs ElemType for a struct slice", f.GoName)
	}
	in := scope.indent
	diagKey := scope.diagKey(fullKey)
	elems := fmt.Sprintf("elems%d", scope.depth+1)
	index := fmt.Sprintf("i%d", scope.depth+1)
	fmt.Fprintf(b, "%sif v, ok := %s.Get(%q); ok {\n", in, scope.doc, fullKey)
	fmt.Fprintf(b, "%s\t%s, err := v.AsTables()\n", in, elems)
	fmt.Fprintf(b, "%s\tif err != nil {\n", in)
	fmt.Fprintf(b, "%s\t\treturn fmt.Errorf(\"minitoml: %s: %%w\", err)\n", in, diagKey)
	fmt.Fprintf(b, "%s\t}\n", in)
	fmt.Fprintf(b, "%s\t%s = make([]%s, len(%s))\n", in, access, f.ElemType, elems)
	fmt.Fprintf(b, "%s\tfor %s := range %s {\n", in, index, elems)
	inner := emitScope{
		doc:        fmt.Sprintf("%s[%s]", elems, index),
		indent:     in + "\t\t",
		diagPrefix: diagKey,
		depth:      scope.depth + 1,
	}
	if err := emitFields(b, inner, fmt.Sprintf("%s[%s]", access, index), "", f.Nested); err != nil {
		return err
	}
	fmt.Fprintf(b, "%s\t}\n", in)
	fmt.Fprintf(b, "%s}\n", in)
	return nil
}

func joinKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + "." + key
}

// WebServiceFixtureSpec returns the representative Bind-style fixture used by tests and committed gen code.
func WebServiceFixtureSpec() Spec {
	return Spec{
		TypeName: "WebServiceConfig",
		Prefix:   "webservice",
		Fields: []Field{
			{GoName: "ListenAddr", Key: "listen_addr", Kind: FieldString, Default: ":8080"},
			{GoName: "MaxConns", Key: "max_conns", Kind: FieldInt, Default: "100"},
			{GoName: "CorsOrigins", Key: "cors_origins", Kind: FieldStringSlice},
			{
				GoName: "TLS",
				Key:    "tls",
				Kind:   FieldStruct,
				Nested: []Field{
					{GoName: "Enabled", Key: "enabled", Kind: FieldBool, Default: "false"},
					{GoName: "CertPath", Key: "cert_path", Kind: FieldString},
				},
			},
			{
				GoName:   "Listeners",
				Key:      "listeners",
				Kind:     FieldStructSlice,
				ElemType: "ListenerConfig",
				Nested: []Field{
					{GoName: "Addr", Key: "addr", Kind: FieldString},
					{GoName: "Port", Key: "port", Kind: FieldInt, Default: "80"},
					{
						GoName: "TLS",
						Key:    "tls",
						Kind:   FieldStruct,
						Nested: []Field{
							{GoName: "Enabled", Key: "enabled", Kind: FieldBool, Default: "false"},
							{GoName: "CertPath", Key: "cert_path", Kind: FieldString},
						},
					},
				},
			},
		},
	}
}

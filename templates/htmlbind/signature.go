package htmlbind

// Signature is one declaration's contract stated in Go terms.
//
// It exists so a caller that generates code around a template, such as a
// filesystem router, can read what a component takes without reimplementing
// the template type system. The Go types here are exactly the ones the
// generated parameter struct declares.
type Signature struct {
	// Name is the declaration name as written in the template.
	Name string
	// Exported reports the export modifier.
	Exported bool
	// Parameters are in declaration order.
	Parameters []SignatureParam
}

// SignatureParam is one declared parameter of a [Signature].
type SignatureParam struct {
	// Name is the parameter name as written in the template.
	Name string
	// GoType is the Go type of the generated parameter struct field. An async
	// parameter is already wrapped, so it reads htmlbind.Pending[T].
	GoType string
	// TemplateType is the type as written in the template, kept for diagnostics
	// that should quote the source rather than its lowering.
	TemplateType string
	// Async marks a parameter the caller settles through htmlbind.Pending.
	Async bool
	// Slot marks an html parameter, which a wrapper fills rather than a caller
	// passing data.
	Slot bool
}

// Signatures parses and analyzes a template module and returns the Go-typed
// signature of every component it declares, in declaration order.
//
// It runs the same analysis [Generate] does, so a module that fails to compile
// fails here with the same diagnostic rather than yielding a partial answer.
func Signatures(filename string, source []byte, options ...AnalysisOption) ([]Signature, error) {
	module, err := Parse(filename, source)
	if err != nil {
		return nil, err
	}
	bindings, err := applyAnalysisOptions(options)
	if err != nil {
		return nil, err
	}
	compiler := newCompiler(filename, string(source), module, true)
	compiler.bindings = bindings
	if err := compiler.analyze(); err != nil {
		return nil, err
	}

	var out []Signature
	// Declaration order comes from the module rather than the components map,
	// because a map iteration would make the result unstable.
	for _, declaration := range module.Declarations {
		decl, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		info, ok := compiler.components[decl.Name]
		if !ok {
			continue
		}
		signature := Signature{Name: decl.Name, Exported: decl.Exported}
		for _, parameter := range info.order {
			resolved := info.params[parameter.Name]
			signature.Parameters = append(signature.Parameters, SignatureParam{
				Name:         parameter.Name,
				GoType:       goType(resolved),
				TemplateType: resolved.String(),
				Async:        resolved.async,
				Slot:         resolved.kind == kindHTML,
			})
		}
		out = append(out, signature)
	}
	return out, nil
}

// Lookup returns the signature with the given name.
func Lookup(signatures []Signature, name string) (Signature, bool) {
	for _, signature := range signatures {
		if signature.Name == name {
			return signature, true
		}
	}
	return Signature{}, false
}

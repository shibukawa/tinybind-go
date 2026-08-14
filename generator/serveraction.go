package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// ServerActionParam is one declared input of a typed server action: the Go
// parameter name, and the source text of its type.
type ServerActionParam struct {
	Name string
	Type string
	// JSON is the member the caller writes it as. Empty derives it from Name
	// the way an untagged struct field's wire name is derived.
	JSON string
}

// ServerAction is one typed server action to generate an entry point for.
//
// It is an input rather than something discovered here, because the annotation
// that admits it is read by routetree, which parses the route package before it
// can compile. This phase does type-check, which is why the argument struct and
// the codecs are built here rather than there.
type ServerAction struct {
	// Func is the declared Go function name, which may be unexported: the
	// wrapper sits beside it in the same package.
	Func string
	// Wrapper is the exported entry point to emit. Empty derives one from Func.
	Wrapper string
	// Params are the declared inputs after any leading context was trimmed.
	Params []ServerActionParam
	// TakesContext passes the request's context as the first argument.
	TakesContext bool
	// Result is the type of the single non-error result. Empty means the
	// function returns only an error, and the entry point answers no content.
	Result string
}

// WrapperName is the exported entry point emitted for this action.
func (a ServerAction) WrapperName() string {
	if a.Wrapper != "" {
		return a.Wrapper
	}
	return "Action" + exportedName(a.Func)
}

// inputTypeName is the generated argument struct. It is unexported: nothing
// outside this package constructs one, and the wrapper that does is emitted
// beside it.
func (a ServerAction) inputTypeName() string {
	return "action" + exportedName(a.Func) + "Input"
}

func exportedName(name string) string {
	if name == "" {
		return name
	}
	runes := []rune(name)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] = runes[0] - 'a' + 'A'
	}
	return string(runes)
}

// planServerActions synthesizes one argument struct per action and marks the
// types the emitted wrapper will name.
//
// The struct is built as an ast.StructType from the declared parameters and run
// through the ordinary field analysis, so a parameter is classified by exactly
// the rules a hand-written field is: a scalar, a nested struct, a collection of
// those, or a type from another package carrying its own codec.
//
// Marking the result type is what rule:usage-directed-generation otherwise
// leaves undone. A codec is emitted for a type some discovered call names, and
// the only call naming this one is the one this phase is about to write, which
// discovery is required to skip. The declaration is the usage source instead.
func planServerActions(plan *PackagePlan, actions []ServerAction, binderNames map[string]bool, foreignCodecs map[string]ForeignCodec) error {
	if plan == nil || len(actions) == 0 {
		return nil
	}
	plan.ServerActions = actions
	byName := map[string]int{}
	for i, t := range plan.Types {
		byName[t.Name] = i
	}
	for _, action := range actions {
		st, err := actionInputStruct(action)
		if err != nil {
			return err
		}
		tp, ok, err := analyzeStruct(action.inputTypeName(), "", st, binderNames, foreignCodecs)
		if err != nil {
			return fmt.Errorf("server action %s: %w", action.Func, err)
		}
		if !ok {
			continue
		}
		// The wrapper decodes it and nothing else touches it, so it carries the
		// decode usage alone.
		tp.Usage = UsageDecodeJSON
		tp.DirectUsage = UsageDecodeJSON
		plan.Types = append(plan.Types, tp)
		if action.Result == "" {
			continue
		}
		i, ok := byName[action.Result]
		if !ok {
			// A result type this package does not declare is either foreign and
			// carrying its own encoder, which needs no plan, or unreachable —
			// and the emitted call names the method either way, so the compiler
			// is what reports the second case.
			continue
		}
		plan.Types[i].Usage |= UsageEncodeJSON
		plan.Types[i].DirectUsage |= UsageEncodeJSON
	}
	propagateNestedUsage(plan.Types)
	return nil
}

// actionInputStruct builds the argument struct from a declared signature.
//
// Each parameter's type is parsed back from its source text, so the field
// analysis sees the same ast.Expr it would have seen had the author written the
// struct out. Nothing is resolved here; the analysis resolves what it needs.
func actionInputStruct(action ServerAction) (*ast.StructType, error) {
	fields := &ast.FieldList{}
	for _, p := range action.Params {
		expr, err := parser.ParseExpr(p.Type)
		if err != nil {
			return nil, fmt.Errorf("server action %s: parameter %s has type %q, which is not a type expression: %w", action.Func, p.Name, p.Type, err)
		}
		wire := p.JSON
		if wire == "" {
			wire = lowerFirst(p.Name)
		}
		fields.List = append(fields.List, &ast.Field{
			Names: []*ast.Ident{{Name: exportedName(p.Name)}},
			Type:  expr,
			Tag: &ast.BasicLit{
				Kind:  token.STRING,
				Value: "`json:" + strconv.Quote(wire) + "`",
			},
		})
	}
	return &ast.StructType{Fields: fields}, nil
}

// emitServerActions writes the argument struct and the entry point for each
// action.
//
// The entry point names the generated codec directly rather than reaching a
// runtime registry. The registry exists to serve an author-written generic
// call, which cannot name a function the generator has not written yet; a call
// site the generator writes itself has no such problem. That removes the
// missing-codec failure mode, which would otherwise be a 500 on the first call.
func emitServerActions(b *bytes.Buffer, plan *PackagePlan, actions []ServerAction, target transportTarget) {
	planned := map[string]TypePlan{}
	for _, t := range plan.Types {
		planned[t.Name] = t
	}
	for _, action := range actions {
		input := action.inputTypeName()
		if t, ok := planned[input]; ok && len(t.Fields) > 0 {
			fmt.Fprintf(b, "// %s is the decoded argument list of the %s server action.\n", input, action.Func)
			fmt.Fprintf(b, "type %s struct {\n", input)
			for _, f := range t.Fields {
				fmt.Fprintf(b, "\t%s %s `json:%s`\n", f.Name, f.GoType(), strconv.Quote(f.JSON))
			}
			b.WriteString("}\n\n")
		}
		emitServerActionWrapper(b, action, planned, target)
	}
}

func emitServerActionWrapper(b *bytes.Buffer, action ServerAction, planned map[string]TypePlan, target transportTarget) {
	input := action.inputTypeName()
	inputPlan, hasInput := planned[input]
	hasFields := hasInput && len(inputPlan.Fields) > 0

	fmt.Fprintf(b, "// %s is the generated entry point of the %s server action.\n", action.WrapperName(), action.Func)
	fmt.Fprintf(b, "func %s(%s) {\n", action.WrapperName(), target.writerParams)
	fail := fmt.Sprintf("\t\thttpbind.WriteError(%s, r, err)\n\t\treturn\n", target.writerVar)

	if hasFields {
		b.WriteString("\tbody, err := httpbind.ReadActionBody(r)\n")
		b.WriteString("\tif err != nil {\n" + fail + "\t}\n")
		fmt.Fprintf(b, "\tinput, err := decode%sBytes(body)\n", input)
		b.WriteString("\tif err != nil {\n" + fail + "\t}\n")
	}

	args := make([]string, 0, len(action.Params)+1)
	if action.TakesContext {
		args = append(args, target.contextExpr)
	}
	for _, p := range action.Params {
		args = append(args, "input."+exportedName(p.Name))
	}

	call := action.Func + "(" + join(args, ", ") + ")"
	if action.Result == "" {
		fmt.Fprintf(b, "\tif err := %s; err != nil {\n", call)
		b.WriteString(fail + "\t}\n")
		// Nothing was produced, so nothing is the honest body.
		fmt.Fprintf(b, "\t%s.WriteHeader(http.StatusNoContent)\n", target.writerVar)
		b.WriteString("}\n\n")
		return
	}

	fmt.Fprintf(b, "\tout, err := %s\n", call)
	b.WriteString("\tif err != nil {\n" + fail + "\t}\n")
	b.WriteString("\tbuf := jsonbind.GetBuffer()\n")
	fmt.Fprintf(b, "\t*buf = %s\n", appendExpr(action.Result, planned))
	fmt.Fprintf(b, "\terr = httpbind.WriteJSONBytes(%s, http.StatusOK, *buf)\n", target.writerVar)
	b.WriteString("\tjsonbind.PutBuffer(buf)\n")
	b.WriteString("\tif err != nil {\n" + fail + "\t}\n")
	b.WriteString("}\n\n")
}

// appendExpr names the encoder for the result type: the generated function when
// this package planned it, and the type's own method when it carries one.
func appendExpr(result string, planned map[string]TypePlan) string {
	if _, ok := planned[result]; ok {
		return fmt.Sprintf("append%sJSON((*buf)[:0], out)", result)
	}
	return "out.AppendJSONTo((*buf)[:0])"
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

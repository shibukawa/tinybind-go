package generator

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// TransformOutput is the generated fasthttp source and what the run wants the
// caller to know about it.
type TransformOutput struct {
	Source []byte
	// LayoutWarnings name authored files the tag cannot cleanly exclude. See
	// checkLayout for why that is the application's problem to fix.
	LayoutWarnings []string
}

// RewriteTransform emits the fasthttp source for every admitted function.
//
// The rewrite is textual over the original bytes rather than a mutated syntax
// tree: comments and formatting survive without being reconstructed, and the
// loaded package's AST stays usable by the other generator phases, which share
// one type check.
func RewriteTransform(pkg *packages.Package, plan *TransformPlan, options TransformOptions) (*TransformOutput, error) {
	options, err := options.normalized()
	if err != nil {
		return nil, err
	}
	if plan == nil || len(plan.Admitted) == 0 {
		return &TransformOutput{}, nil
	}
	r := &transformRewriter{pkg: pkg, options: options, sources: map[string][]byte{}}

	bodies := make([]string, 0, len(plan.Admitted))
	imports := map[string]string{} // path -> local name
	for _, candidate := range plan.Admitted {
		text, used, err := r.rewriteFunc(candidate)
		if err != nil {
			return nil, err
		}
		bodies = append(bodies, text)
		for path, name := range used {
			imports[path] = name
		}
	}
	imports[options.ContextImport] = qualifierOf(options.ContextType)

	source, err := r.assemble(pkg.Name, imports, bodies)
	if err != nil {
		return nil, err
	}
	return &TransformOutput{Source: source, LayoutWarnings: r.checkLayout(plan)}, nil
}

type transformRewriter struct {
	pkg     *packages.Package
	options TransformOptions
	sources map[string][]byte
}

func (r *transformRewriter) source(candidate *TransformCandidate) ([]byte, error) {
	name := r.pkg.Fset.Position(candidate.Decl.Pos()).Filename
	if src, ok := r.sources[name]; ok {
		return src, nil
	}
	src, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("generator: transform reading %s: %w", name, err)
	}
	r.sources[name] = src
	return src, nil
}

// rewriteFunc applies the recorded edits plus the signature collapse, and
// reports which packages the result still names.
func (r *transformRewriter) rewriteFunc(candidate *TransformCandidate) (string, map[string]string, error) {
	src, err := r.source(candidate)
	if err != nil {
		return "", nil, err
	}
	contextName := r.contextName(candidate)

	edits := append([]sourceEdit(nil), candidate.edits...)
	paramEdits, err := r.signatureEdits(candidate, contextName)
	if err != nil {
		return "", nil, err
	}
	edits = append(edits, paramEdits...)

	start := r.offset(candidate.Decl.Pos())
	if candidate.Decl.Doc != nil {
		start = r.offset(candidate.Decl.Doc.Pos())
	}
	end := r.offset(candidate.Decl.End())

	used := r.usedImports(candidate, edits)

	text, err := applyEdits(src, start, end, edits)
	if err != nil {
		return "", nil, fmt.Errorf("generator: transform %s: %w", candidate.Name, err)
	}
	return strings.ReplaceAll(text, contextPlaceholder, contextName), used, nil
}

// contextName picks an identifier the function does not already use, so a
// handler that happens to have a local named ctx is not silently shadowed.
func (r *transformRewriter) contextName(candidate *TransformCandidate) string {
	taken := map[string]bool{}
	ast.Inspect(candidate.Decl, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			taken[id.Name] = true
		}
		return true
	})
	for _, name := range candidate.TransportParams {
		delete(taken, name)
	}
	base := r.options.ContextName
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		candidateName := fmt.Sprintf("%s%d", base, i)
		if !taken[candidateName] {
			return candidateName
		}
	}
}

// signatureEdits collapses the transport parameters into one. The first becomes
// the context and keeps its place; the rest are deleted with their comma.
func (r *transformRewriter) signatureEdits(candidate *TransformCandidate, contextName string) ([]sourceEdit, error) {
	list := candidate.Decl.Type.Params.List
	positions := map[*ast.Field]int{}
	for i, field := range list {
		positions[field] = i
	}
	var edits []sourceEdit
	for n, field := range candidate.transportFields {
		if len(field.Names) > 1 {
			return nil, fmt.Errorf("generator: %s groups transport parameters in one field, which has no single-value form", candidate.Name)
		}
		if n == 0 {
			edits = append(edits, sourceEdit{
				start: r.offset(field.Pos()),
				end:   r.offset(field.End()),
				text:  contextName + " " + r.options.ContextType,
			})
			continue
		}
		index := positions[field]
		if index == 0 {
			return nil, fmt.Errorf("generator: %s has a transport parameter before the one that becomes the context", candidate.Name)
		}
		edits = append(edits, sourceEdit{
			start: r.offset(list[index-1].End()),
			end:   r.offset(field.End()),
		})
	}
	return edits, nil
}

// usedImports lists the packages the rewritten function still names, with the
// local name the source used. A reference inside a replaced range does not
// count: the transport parameter types are the whole reason net/http drops out.
func (r *transformRewriter) usedImports(candidate *TransformCandidate, edits []sourceEdit) map[string]string {
	used := map[string]string{}
	ast.Inspect(candidate.Decl, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		pkgName, ok := r.pkg.TypesInfo.Uses[id].(*types.PkgName)
		if !ok {
			return true
		}
		offset := r.offset(id.Pos())
		for _, edit := range edits {
			if offset >= edit.start && offset < edit.end {
				return true
			}
		}
		path := pkgName.Imported().Path()
		if to, rewritten := r.options.RewriteImport(path); rewritten {
			path = to
		}
		used[path] = id.Name
		return true
	})
	return used
}

func (r *transformRewriter) offset(pos token.Pos) int {
	return r.pkg.Fset.Position(pos).Offset
}

func (r *transformRewriter) checkLayout(plan *TransformPlan) []string {
	// A build tag excludes a whole file. An authored file holding both a
	// transport handler and a type declaration cannot be tagged out without
	// taking the type with it, and the type is needed under both tags.
	byFile := map[*ast.File][]string{}
	for _, candidate := range plan.Admitted {
		byFile[candidate.file] = append(byFile[candidate.file], candidate.Name)
	}
	var warnings []string
	for file, funcs := range byFile {
		var shared []string
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok.String() != "import" {
					shared = append(shared, "a "+d.Tok.String()+" declaration")
				}
			case *ast.FuncDecl:
				if !contains(funcs, d.Name.Name) && !r.refused(plan, d.Name.Name) {
					shared = append(shared, "func "+d.Name.Name)
				}
			}
		}
		if len(shared) == 0 {
			continue
		}
		name := r.pkg.Fset.Position(file.Pos()).Filename
		warnings = append(warnings, fmt.Sprintf(
			"%s holds transport handlers beside %s; tagging the file !fasthttp removes those too, so move them to a file of their own",
			name, strings.Join(dedupe(shared), ", ")))
	}
	sort.Strings(warnings)
	return warnings
}

func (r *transformRewriter) refused(plan *TransformPlan, name string) bool {
	for _, refusal := range plan.Refusals {
		if refusal.Function == name {
			return true
		}
	}
	return false
}

func (r *transformRewriter) assemble(pkgName string, imports map[string]string, bodies []string) ([]byte, error) {
	var b strings.Builder
	b.WriteString("// Code generated by tinybind; DO NOT EDIT.\n\n")
	b.WriteString("//go:build fasthttp\n\n")
	fmt.Fprintf(&b, "package %s\n\n", pkgName)

	paths := make([]string, 0, len(imports))
	for path := range imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) > 0 {
		b.WriteString("import (\n")
		for _, path := range paths {
			local := imports[path]
			if local != "" && local != defaultLocalName(path) {
				fmt.Fprintf(&b, "\t%s %q\n", local, path)
			} else {
				fmt.Fprintf(&b, "\t%q\n", path)
			}
		}
		b.WriteString(")\n\n")
	}
	for i, body := range bodies {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(strings.TrimRight(body, "\n"))
	}
	b.WriteString("\n")

	out, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("generator: transform output does not parse: %w\n%s", err, b.String())
	}
	return out, nil
}

// applyEdits replaces byte ranges in src and returns the text between start and
// end. Edits are applied from the back so earlier offsets stay valid.
func applyEdits(src []byte, start, end int, edits []sourceEdit) (string, error) {
	within := make([]sourceEdit, 0, len(edits))
	for _, edit := range edits {
		if edit.start < start || edit.end > end {
			continue
		}
		if edit.start > edit.end {
			return "", fmt.Errorf("inverted edit at %d", edit.start)
		}
		within = append(within, edit)
	}
	sort.Slice(within, func(i, j int) bool { return within[i].start > within[j].start })
	out := append([]byte(nil), src[start:end]...)
	last := -1
	for _, edit := range within {
		if last >= 0 && edit.end > last {
			return "", fmt.Errorf("overlapping edits at %d", edit.start)
		}
		last = edit.start
		lo, hi := edit.start-start, edit.end-start
		out = append(out[:lo], append([]byte(edit.text), out[hi:]...)...)
	}
	return string(out), nil
}

func qualifierOf(typeExpr string) string {
	trimmed := strings.TrimLeft(typeExpr, "*")
	if dot := strings.Index(trimmed, "."); dot > 0 {
		return trimmed[:dot]
	}
	return ""
}

func defaultLocalName(path string) string {
	if slash := strings.LastIndex(path, "/"); slash >= 0 {
		return path[slash+1:]
	}
	return path
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := values[:0]
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

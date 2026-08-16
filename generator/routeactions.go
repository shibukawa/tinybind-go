package generator

import "github.com/shibukawa/tinybind-go/routetree"

// ServerActionsFor selects the typed server actions declared in one route
// package and converts them to what this phase takes.
//
// The two halves of a typed action are built by different phases: routetree
// reads the declaration, because it parses a route package before that package
// can compile, and this phase builds the argument struct and the codecs,
// because it type-checks. The conversion is here rather than in a caller so the
// wrapper name, the parameter list and the context flag cannot drift between
// what the registry registers and what the wrapper is emitted as.
//
// relDir selects the package: it is [routetree.Action.RelDir], empty for the
// route root. Raw handlers are skipped, since nothing is generated around one.
func ServerActionsFor(actions []routetree.Action, relDir string) []ServerAction {
	var out []ServerAction
	for _, action := range actions {
		if !action.Typed || action.RelDir != relDir {
			continue
		}
		converted := ServerAction{
			Func: action.Name,
			// routetree owns this name because it is what the registry writes.
			// Taking it rather than deriving one here is what keeps the
			// registration and the emitted entry point naming one symbol.
			Wrapper:      action.Wrapper,
			TakesContext: action.Signature.TakesContext,
			Result:       action.Signature.Result,
			// The declaring file, so the argument struct, its decoder and the
			// entry point all land in one artifact.
			SourcePath: action.File,
		}
		for _, p := range action.Signature.Params {
			converted.Params = append(converted.Params, ServerActionParam{Name: p.Name, Type: p.Type})
		}
		out = append(out, converted)
	}
	return out
}

// WithServerActionsFor is [Options] carrying the typed actions of one route
// package, which is what a caller generating over a whole tree needs per
// package.
func (o Options) WithServerActionsFor(actions []routetree.Action, relDir string) Options {
	o.ServerActions = ServerActionsFor(actions, relDir)
	return o
}

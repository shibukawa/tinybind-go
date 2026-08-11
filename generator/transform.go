package generator

import (
	"fmt"
	"go/token"
	"sort"

	"github.com/shibukawa/tinybind-go/parser"
)

// The net/http source is the authored form and the fasthttp source is derived
// from it. The derivation is two substitutions and an import swap: the writer
// and the request collapse into one context value, recognized calls drop the
// arguments that carried them, and every import naming a transport-shaped
// package is replaced by the package declaring the same names over the other
// transport.
//
// Which import becomes which is configuration, not knowledge this package
// holds. A framework shipping its own helpers in two packages registers the
// pair and its calls are rewritten like the built-in ones.

const fasthttpbindImportPath = "github.com/shibukawa/tinybind-go/fasthttpbind"
const fasthttpImportPath = "github.com/shibukawa/tinygodriver/fasthttp"

// TransformOptions configures the source transform.
type TransformOptions struct {
	// ImportRewrites maps an import path in the authored source to the path the
	// generated source imports instead. The local name is preserved, so a call
	// selector in a rewritten body is untouched: only the import line moves.
	//
	// A framework providing the same helper names over the other transport
	// registers its own pair here. Nothing about the mapping is built in beyond
	// the defaults below.
	ImportRewrites map[string]string

	// ContextType is the type the collapsed parameter takes, written verbatim,
	// and ContextImport the package supplying it.
	ContextType   string
	ContextImport string

	// ContextName is the preferred identifier for the collapsed parameter. A
	// function already using the name gets a fresh one instead.
	ContextName string

	// RequestSelectorRewrites maps a method or field named on the request to
	// the expression replacing the whole selector. "$ctx" stands for the
	// context identifier.
	//
	// Enumerated one entry at a time on purpose: the size of this map is the
	// honest measure of how much net/http semantics the transform claims to
	// reproduce, and a general rule would hide that.
	RequestSelectorRewrites map[string]string

	// Router names the router the generated route registration installs on.
	// The zero value uses DefaultRouterTarget.
	Router RouterTarget

	// Calls are the recognized calls whose transport arguments are dropped.
	// Empty uses the canonical set for the default runtime packages.
	Calls []CallPattern

	// ReportOnly lists what a transformed build would refuse and writes
	// nothing. Adoption is all-or-nothing per decision:backend-build-tag-mode,
	// so an application needs to see the whole cost before committing to the
	// migration rather than one refusal at a time after it.
	ReportOnly bool

	// GeneratedHeaders names header prefixes, beside this module's own, whose
	// files the transform skips. It carries Options.GeneratedHeaders to a
	// direct AnalyzeTransform caller, and a Generator fills it from there.
	//
	// The transform reads the whole loaded package, so a framework branding its
	// generated output writes a header nothing here recognizes on its own, and
	// its generated code is classified as if a user had authored it. Each entry
	// still requires the conventional "DO NOT EDIT." ending.
	GeneratedHeaders []string
}

// transformOptions is the configured transform carrying the generator's
// generated-header list. A framework sets Options.GeneratedHeaders once for
// discovery, and the transform has to skip the same files discovery does.
//
// Options.Transform must be set; every caller is already inside the check that
// a backend was selected.
func (g *Generator) transformOptions() TransformOptions {
	options := *g.Options.Transform
	if len(g.Options.GeneratedHeaders) > 0 {
		merged := make([]string, 0, len(options.GeneratedHeaders)+len(g.Options.GeneratedHeaders))
		merged = append(merged, options.GeneratedHeaders...)
		merged = append(merged, g.Options.GeneratedHeaders...)
		options.GeneratedHeaders = merged
	}
	return options
}

// DefaultTransformOptions targets the fasthttp runtime this module ships.
func DefaultTransformOptions() TransformOptions {
	return TransformOptions{
		ImportRewrites: map[string]string{
			httpbindImportPath:   fasthttpbindImportPath,
			htmlupdateImportPath: fasthttpupdateImportPath,
		},
		ContextType:   "*fasthttp.RequestCtx",
		ContextImport: fasthttpImportPath,
		ContextName:   "ctx",
		Router:        DefaultRouterTarget(),
		RequestSelectorRewrites: map[string]string{
			// RequestCtx satisfies context.Context, so the request's context is
			// the context. This is the whole enumerated set today.
			"Context": "$ctx",
		},
	}
}

func (o TransformOptions) normalized() (TransformOptions, error) {
	if o.ContextName == "" {
		o.ContextName = "ctx"
	}
	if o.ContextType == "" {
		return o, fmt.Errorf("generator: transform requires a ContextType")
	}
	if o.ImportRewrites == nil {
		o.ImportRewrites = map[string]string{}
	}
	for from, to := range o.ImportRewrites {
		if from == "" || to == "" {
			return o, fmt.Errorf("generator: transform import rewrite %q -> %q needs both paths", from, to)
		}
		if from == to {
			return o, fmt.Errorf("generator: transform import rewrite for %q maps to itself", from)
		}
	}
	if o.Router.Import == "" {
		o.Router = DefaultRouterTarget()
	}
	router, err := o.Router.normalized()
	if err != nil {
		return o, err
	}
	o.Router = router
	if o.Calls == nil {
		patterns, err := DefaultOptions().callPatterns()
		if err != nil {
			return o, err
		}
		o.Calls = patterns
	}
	return o, nil
}

// RewriteImport returns the path that replaces from, and whether it changes.
func (o TransformOptions) RewriteImport(from string) (string, bool) {
	to, ok := o.ImportRewrites[from]
	return to, ok && to != from
}

// TransformRefusalKind names why a function cannot be rewritten. It is the
// classification requirement:transform-diagnostics prints, so each value maps
// to one remedy a reader can act on.
type TransformRefusalKind string

const (
	// RefusalUnknownCall passes a transport value to a call the generator does
	// not recognize. The common shape of tracing, metrics and session libraries.
	RefusalUnknownCall TransformRefusalKind = "unknown_call"
	// RefusalUnknownSelector names a method or field absent from the rewrite table.
	RefusalUnknownSelector TransformRefusalKind = "unknown_selector"
	// RefusalEscapes assigns, stores, captures, returns or takes the address of
	// a transport value.
	RefusalEscapes TransformRefusalKind = "escapes"
	// RefusalTypeAssertion reaches for Flusher, Hijacker or another capability
	// the other transport does not present the same way.
	RefusalTypeAssertion TransformRefusalKind = "type_assertion"
	// RefusalInheritedFromCallee is refused only because something it calls is.
	RefusalInheritedFromCallee TransformRefusalKind = "inherited"
)

var refusalRemedies = map[TransformRefusalKind]string{
	RefusalUnknownCall:         "move the call behind a function taking neither the writer nor the request, or register it as a call pattern declaring its transport slots",
	RefusalUnknownSelector:     "read the value before the transform sees it, or add the selector to RequestSelectorRewrites with a rewrite that is correct on both transports",
	RefusalEscapes:             "keep the transport value inside the function; copy out what outlives it",
	RefusalTypeAssertion:       "use the runtime's own streaming entry instead of asserting a capability",
	RefusalInheritedFromCallee: "fix the refusal reported below this one",
}

// Remedy is the action that clears this kind of refusal.
func (k TransformRefusalKind) Remedy() string { return refusalRemedies[k] }

// TransformRefusalHop is one step from the handler to the occurrence.
type TransformRefusalHop struct {
	Function string
	Position token.Position
	Detail   string
}

// TransformRefusal reports one function the transform will not rewrite.
type TransformRefusal struct {
	Function string
	Kind     TransformRefusalKind
	Position token.Position
	Detail   string
	// Chain runs from this function to the occurrence that caused the refusal.
	// It is empty when the occurrence is in this function's own body.
	Chain []TransformRefusalHop
}

// Error renders the refusal the way requirement:transform-diagnostics asks: the
// position of the occurrence rather than of the declaration, the
// classification, every hop that inherited it, and the remedy.
func (r TransformRefusal) Error() string {
	out := fmt.Sprintf("%s is not transformable\n", r.Function)
	out += fmt.Sprintf("  %s  %s\n", r.Position, r.Detail)
	for _, hop := range r.Chain {
		out += fmt.Sprintf("  %s  %s\n", hop.Position, hop.Detail)
	}
	kind := r.Kind
	if kind == RefusalInheritedFromCallee && len(r.Chain) > 0 {
		kind = RefusalUnknownCall
	}
	if remedy := kind.Remedy(); remedy != "" {
		out += "  remedy: " + remedy + "\n"
	}
	return out
}

// Diagnostics renders the refusals the way the rest of the generator reports
// what it could not analyze, so a report-only run rides the same rail as
// --check instead of inventing a second one.
func (rs TransformRefusals) Diagnostics() []parser.Diagnostic {
	out := make([]parser.Diagnostic, 0, len(rs))
	for _, r := range rs {
		message := r.Function + " " + r.Detail
		for _, hop := range r.Chain {
			message += "; " + hop.Position.String() + " " + hop.Detail
		}
		if remedy := r.Kind.Remedy(); remedy != "" {
			message += "; remedy: " + remedy
		}
		out = append(out, parser.Diagnostic{
			File:   r.Position.Filename,
			Line:   r.Position.Line,
			Column: r.Position.Column,
			// The classification goes in Reason, where a consumer looks for it,
			// rather than being repeated in the prose.
			Reason:  string(r.Kind),
			Message: message,
		})
	}
	return out
}

// TransformRefusals is every refusal in one package, reported together so a
// shared helper's refusal and the handlers that inherited it arrive in one run.
type TransformRefusals []TransformRefusal

func (rs TransformRefusals) Error() string {
	out := ""
	for _, r := range rs {
		out += r.Error()
	}
	return out
}

func sortRefusals(refusals []TransformRefusal) {
	sort.SliceStable(refusals, func(i, j int) bool {
		if refusals[i].Position.Filename != refusals[j].Position.Filename {
			return refusals[i].Position.Filename < refusals[j].Position.Filename
		}
		if refusals[i].Position.Line != refusals[j].Position.Line {
			return refusals[i].Position.Line < refusals[j].Position.Line
		}
		return refusals[i].Function < refusals[j].Function
	})
}

package routetree

import (
	"fmt"
	"os"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// Reserved component names. decision:html-route-file-conventions ties each
// reserved file to one declaration name, so a handler can predict every symbol
// its route package exposes without opening the template.
const (
	PageComponentName     = "Page"
	LayoutComponentName   = "Layout"
	DocumentComponentName = "Document"
)

// ComponentSignature is one template declaration, split into the values a
// caller supplies and the slots composition fills.
type ComponentSignature struct {
	Name string
	// Inputs are the ordinary parameters, in declaration order.
	Inputs []Value
	// Slots are the html parameters, which a wrapper fills rather than a
	// caller passing data.
	Slots []Value
}

// Analysis is one route checked end to end: its template, its optional Go
// entry point, and the input list a generated decoder must bind.
type Analysis struct {
	Route     Route
	Component ComponentSignature
	// Page describes the Go entry point. It is never nil; a route with no
	// page.go still reports RungTemplateOnly.
	Page *PageFunc
	// Inputs is what [Emitter.Decoder] binds for this route. Which declaration
	// supplies it depends on the rung; see [Analyze].
	Inputs []Value
}

// Analyze reads one route's template and logic file and checks them against
// each other.
//
// The input list a decoder binds comes from a different declaration at each
// rung, because a different declaration is the thing the request reaches first:
//
//   - RungTemplateOnly: the component parameters, since the request renders the
//     component directly.
//   - RungTypedPage: the func Page parameters, since the request reaches Go
//     first and the component parameters are that function's results.
//   - RungHandlerPage: the route's dynamic segments as strings. The handler owns
//     decoding, so the generated decoder is a convenience covering only what the
//     filesystem already knows; anything else the handler reads itself.
//
// Every problem found is reported, so one run surfaces more than the first.
func Analyze(route Route) (Analysis, error) {
	component, err := PageComponent(route.PageFile)
	if err != nil {
		return Analysis{}, err
	}
	fn, err := InspectLogic(route.LogicFile)
	if err != nil {
		return Analysis{}, err
	}

	analysis := Analysis{Route: route, Component: component, Page: fn}
	var errs []error

	if len(component.Slots) > 0 {
		errs = append(errs, &Error{
			Path: route.PageFile,
			Message: fmt.Sprintf("component %s declares slot parameter %q; a page fills no wrapper, so slots belong on a layout",
				component.Name, component.Slots[0].Name),
		})
	}

	switch fn.Rung {
	case RungTypedPage:
		analysis.Inputs = fn.Params
		errs = append(errs, Validate(route, fn, component.Inputs)...)
	case RungHandlerPage:
		analysis.Inputs = pathInputs(route)
	default:
		analysis.Inputs = component.Inputs
		errs = append(errs, checkComponentInputs(route, component)...)
	}

	if len(errs) > 0 {
		return analysis, joinErrors(errs)
	}
	return analysis, nil
}

// PageComponent reads a page template and returns its reserved declaration.
func PageComponent(path string) (ComponentSignature, error) {
	return readComponent(path, PageComponentName)
}

// LayoutComponent reads a layout template and returns its reserved declaration.
func LayoutComponent(path string) (ComponentSignature, error) {
	return readComponent(path, LayoutComponentName)
}

func readComponent(path, name string) (ComponentSignature, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return ComponentSignature{}, err
	}
	signatures, err := htmlbind.Signatures(path, source)
	if err != nil {
		return ComponentSignature{}, err
	}
	signature, ok := htmlbind.Lookup(signatures, name)
	if !ok {
		return ComponentSignature{}, &Error{
			Path:    path,
			Message: fmt.Sprintf("no component named %s; this file must declare `export component %s(...)`", name, name),
		}
	}
	if !signature.Exported {
		return ComponentSignature{}, &Error{
			Path:    path,
			Message: fmt.Sprintf("component %s is not exported; generated code in another package cannot reach it", name),
		}
	}

	out := ComponentSignature{Name: signature.Name}
	for _, parameter := range signature.Parameters {
		value := Value{Name: parameter.Name, Type: parameter.GoType}
		if parameter.Slot {
			out.Slots = append(out.Slots, value)
			continue
		}
		out.Inputs = append(out.Inputs, value)
	}
	return out, nil
}

// checkComponentInputs applies the shared input rule to a template-only route,
// where the component parameter list is what the URL supplies.
func checkComponentInputs(route Route, component ComponentSignature) []error {
	where := route.PageFile
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, &Error{Path: where, Message: fmt.Sprintf(format, args...)})
	}

	if len(component.Inputs) < len(route.Params) {
		fail("component %s must begin with the %d path parameter(s) of %s (%s); it declares %d parameter(s)",
			component.Name, len(route.Params), route.Path, paramNames(route.Params), len(component.Inputs))
		return errs
	}
	for i, want := range route.Params {
		if got := component.Inputs[i]; got.Name != want.Name {
			fail("parameter %d of component %s is %q, but %s binds %q at that position",
				i+1, component.Name, got.Name, route.Path, want.Name)
		}
	}
	for i, input := range component.Inputs {
		_, optional, ok := bindableType(input.Type)
		if !ok {
			kind := "query parameter"
			if i < len(route.Params) {
				kind = "path parameter"
			}
			fail("%s %q has type %s; without a func %s every component parameter comes from the URL, so it must be a scalar",
				kind, input.Name, input.Type, PageFuncName)
			continue
		}
		if optional && i < len(route.Params) {
			fail("%s", optionalPathError(input.Name, input.Type, route.Params[i].Kind == CatchAllSegment))
		}
	}
	return errs
}

// pathInputs is the decoder input list for a route whose handler owns decoding:
// the dynamic segments, as the strings the stdlib hands over.
func pathInputs(route Route) []Value {
	if len(route.Params) == 0 {
		return nil
	}
	out := make([]Value, len(route.Params))
	for i, segment := range route.Params {
		out[i] = Value{Name: segment.Name, Type: "string"}
	}
	return out
}

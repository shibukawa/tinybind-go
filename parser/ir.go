package parser

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Result is the structured parse output for a single Go package.
type Result struct {
	Routes      []Route      `json:"routes"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// Diagnostic is a host-side analysis finding for an incomplete route-like site.
type Diagnostic struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	Column       int    `json:"column"`
	Reason       string `json:"reason"` // dynamic_pattern|opaque_middleware|cross_package_model|complex_type_arg|other
	Message      string `json:"message"`
	OmitsOpenAPI bool   `json:"omits_openapi"`
}

// Reason codes for diagnostics.
const (
	ReasonDynamicPattern      = "dynamic_pattern"
	ReasonOpaqueMiddleware    = "opaque_middleware"
	ReasonCrossPackageModel   = "cross_package_model"
	ReasonComplexTypeArg      = "complex_type_arg"
	ReasonCrossPackageHandler = "cross_package_handler"
	ReasonOther               = "other"
)

// Route is one statically discovered net/http route registration.
type Route struct {
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	Handler         Handler     `json:"handler"`
	Request         string      `json:"request,omitempty"`
	Response        string      `json:"response,omitempty"`
	Stream          string      `json:"stream,omitempty"` // element type when response is Stream[T]
	Errors          []string    `json:"errors,omitempty"`
	SuccessStatuses []int       `json:"success_statuses,omitempty"` // 200 from Write, others from WriteStatus
	Wrappers        WrapperMeta `json:"wrappers"`
}

// Handler describes how the leaf handler was expressed in source.
type Handler struct {
	Form string `json:"form"`           // named | inline | struct
	Name string `json:"name,omitempty"` // function or type name when known
	Doc  string `json:"doc,omitempty"`  // godoc of the handler func or type
}

// WrapperMeta holds statically known stdlib wrapper metadata.
type WrapperMeta struct {
	AllowQuerySemicolons bool   `json:"allow_query_semicolons"`
	MaxRequestBodyBytes  *int64 `json:"max_request_body_bytes,omitempty"`
	StrippedPrefix       string `json:"stripped_prefix,omitempty"`
	Timeout              string `json:"timeout,omitempty"`
	TimeoutMessage       string `json:"timeout_message,omitempty"`
}

// Normalize sorts routes and nested slices for stable golden comparisons.
func (r *Result) Normalize() {
	if r == nil {
		return
	}
	if r.Routes == nil {
		r.Routes = []Route{}
	}
	if r.Diagnostics == nil {
		r.Diagnostics = []Diagnostic{}
	}
	for i := range r.Routes {
		sortStrings(r.Routes[i].Errors)
		sort.Ints(r.Routes[i].SuccessStatuses)
	}
	sortRoutes(r.Routes)
	sort.SliceStable(r.Diagnostics, func(i, j int) bool {
		a, b := r.Diagnostics[i], r.Diagnostics[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return a.Reason < b.Reason
	})
}

// JSON returns canonical indented JSON for golden files.
func (r Result) JSON() ([]byte, error) {
	r.Normalize()
	return json.MarshalIndent(r, "", "  ")
}

// String formats a diagnostic for CLI output.
func (d Diagnostic) String() string {
	loc := d.File
	if d.Line > 0 {
		loc = fmt.Sprintf("%s:%d:%d", d.File, d.Line, d.Column)
	}
	return fmt.Sprintf("%s: %s (%s)", loc, d.Message, d.Reason)
}

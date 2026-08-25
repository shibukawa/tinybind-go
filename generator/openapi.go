package generator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/shibukawa/tinybind-go/internal/godoc"
	"github.com/shibukawa/tinybind-go/parser"
)

// Document is an OpenAPI 3.1 document represented as ordered JSON-friendly maps.
// Keys under paths/operations/components are sorted for deterministic output.
type Document map[string]any

// BuildOpenAPI analyzes dir with the route parser and field planner and returns
// an OpenAPI 3.1 document derived only from Go source (not from handwritten YAML).
func BuildOpenAPI(dir string) (Document, error) {
	return New(DefaultOptions()).BuildOpenAPI(dir)
}

// BuildOpenAPI builds a document using this generator's discovery identities.
func (g *Generator) BuildOpenAPI(dir string) (Document, error) {
	return g.buildOpenAPI(newPackageLoad(dir))
}

// buildOpenAPI is BuildOpenAPI over a package the run already loaded. Routes and
// type plans are two readings of the same type-checked package.
func (g *Generator) buildOpenAPI(load *packageLoad) (Document, error) {
	normalized, err := g.Options.normalized()
	if err != nil {
		return nil, err
	}
	if !normalized.openAPI {
		return nil, fmt.Errorf("%w: %s", ErrFeatureDisabled, FeatureOpenAPI)
	}
	routes, err := load.routes(normalized.parserConfig)
	if err != nil {
		return nil, fmt.Errorf("parse routes: %w", err)
	}
	plan, err := analyzeLoadedPackage(load, g.Options)
	if err != nil {
		return nil, fmt.Errorf("analyze types: %w", err)
	}
	types := indexTypes(plan)
	doc := Document{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   plan.Package + " API",
			"version": "0.0.0",
		},
		"paths":      map[string]any{},
		"components": map[string]any{"schemas": map[string]any{}},
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	// Always include Problem Details schema for error responses.
	schemas["ProblemDetails"] = problemDetailsSchema()

	paths := doc["paths"].(map[string]any)
	for _, route := range routes.Routes {
		if route.Method == "" || route.Path == "" {
			continue
		}
		pathItem, _ := paths[route.Path].(map[string]any)
		if pathItem == nil {
			pathItem = map[string]any{}
			paths[route.Path] = pathItem
		}
		op := buildOperation(route, types, schemas, g.Options.EnableCBORHTTP)
		pathItem[strings.ToLower(route.Method)] = op
	}
	return doc, nil
}

// MarshalJSON returns deterministic indented OpenAPI JSON.
func (d Document) MarshalJSON() ([]byte, error) {
	return json.MarshalIndent(map[string]any(d), "", "  ")
}

// JSON is an alias for MarshalJSON bytes for callers.
func (d Document) JSON() ([]byte, error) {
	return d.MarshalJSON()
}

func indexTypes(plan *PackagePlan) map[string]TypePlan {
	out := make(map[string]TypePlan, len(plan.Types))
	for _, t := range plan.Types {
		out[t.Name] = t
	}
	return out
}

func buildOperation(route parser.Route, types map[string]TypePlan, schemas map[string]any, cborHTTP bool) map[string]any {
	scope := schemaScope{schemas: schemas, types: types}
	op := map[string]any{
		"responses": map[string]any{},
	}
	if route.Handler.Name != "" {
		op["operationId"] = route.Handler.Name
	}
	// Handler godoc documents the operation: first sentence as summary, rest as description.
	if summary, description := godoc.Split(route.Handler.Doc); summary != "" {
		op["summary"] = summary
		if description != "" {
			op["description"] = description
		}
		if godoc.Deprecated(route.Handler.Doc) {
			op["deprecated"] = true
		}
	}

	var params []any
	var bodyProps map[string]any
	var bodyRequired []string
	needBody := false
	bodyAdditionalProps := false
	hasCheckValidation := false

	if route.Request != "" {
		reqName := stripPackage(route.Request)
		ensureSchema(scope, reqName, types[reqName])
		if tp, ok := types[reqName]; ok {
			for _, f := range tp.Fields {
				if f.HasValidation() {
					hasCheckValidation = true
				}
				switch f.Source {
				case SourcePath:
					params = append(params, parameter("path", f, true || f.Check.Required))
				case SourceHeader:
					params = append(params, parameter("header", f, f.Check.Required))
				case SourceCookie:
					params = append(params, parameter("cookie", f, f.Check.Required))
				case SourceQuery:
					params = append(params, parameter("query", f, f.Check.Required))
				case SourceInput:
					if f.IsRest() {
						// rest is payload-only; input:"*" is rejected at plan time
						continue
					}
					params = append(params, parameter("query", f, f.Check.Required))
					needBody = true
					if bodyProps == nil {
						bodyProps = map[string]any{}
					}
					bodyProps[f.Wire] = schemaForField(f, scope)
					if f.Check.Required {
						bodyRequired = append(bodyRequired, f.Wire)
					}
				case SourcePayload:
					needBody = true
					if f.IsRest() {
						bodyAdditionalProps = true
						if bodyProps == nil {
							bodyProps = map[string]any{}
						}
						continue
					}
					if bodyProps == nil {
						bodyProps = map[string]any{}
					}
					bodyProps[f.Wire] = schemaForField(f, scope)
					if f.Check.Required {
						bodyRequired = append(bodyRequired, f.Wire)
					}
				case SourceMethod:
					// method tag is not an OpenAPI parameter
				}
			}
		}
	}
	if len(params) > 0 {
		sort.SliceStable(params, func(i, j int) bool {
			a := params[i].(map[string]any)
			b := params[j].(map[string]any)
			if a["in"] != b["in"] {
				return fmt.Sprint(a["in"]) < fmt.Sprint(b["in"])
			}
			return fmt.Sprint(a["name"]) < fmt.Sprint(b["name"])
		})
		op["parameters"] = params
	}
	if needBody && bodyProps != nil {
		mediaSchema := map[string]any{
			"type":       "object",
			"properties": bodyProps,
		}
		if bodyAdditionalProps {
			mediaSchema["additionalProperties"] = true
		}
		if len(bodyRequired) > 0 {
			mediaSchema["required"] = stringSliceAny(bodyRequired)
		}
		content := map[string]any{
			"application/json": map[string]any{
				"schema": mediaSchema,
			},
			"application/x-www-form-urlencoded": map[string]any{
				"schema": mediaSchema,
			},
			"multipart/form-data": map[string]any{
				"schema": mediaSchema,
			},
		}
		// A rest map is refused by the CBOR emitter, so a body that carries one
		// must not be advertised as acceptable CBOR either.
		if cborHTTP && !bodyAdditionalProps {
			content["application/cbor"] = map[string]any{
				"schema": mediaSchema,
			}
		}
		op["requestBody"] = map[string]any{
			"required": false,
			"content":  content,
		}
	}

	responses := op["responses"].(map[string]any)
	// Success response(s): Write → 200; WriteStatus → static status list
	successStatuses := route.SuccessStatuses
	if len(successStatuses) == 0 {
		successStatuses = []int{200}
	}
	if route.Stream != "" || strings.Contains(route.Response, "Stream[") {
		elem := route.Stream
		if elem == "" {
			elem = extractStreamElem(route.Response)
		}
		elem = stripPackage(elem)
		ensureSchema(scope, elem, types[elem])
		ref := schemaRef(elem)
		content := map[string]any{
			"text/event-stream":    map[string]any{"schema": ref},
			"application/x-ndjson": map[string]any{"schema": ref},
			"application/json":     map[string]any{"schema": ref},
		}
		for _, st := range successStatuses {
			responses[strconv.Itoa(st)] = map[string]any{
				"description": http.StatusText(st),
				"content":     content,
			}
		}
	} else if route.Response != "" {
		respName := stripPackage(route.Response)
		// skip Stream-only names already handled
		if !strings.Contains(respName, "Stream[") {
			ensureSchema(scope, respName, types[respName])
			for _, st := range successStatuses {
				resp := map[string]any{
					"description": http.StatusText(st),
				}
				if st != http.StatusNoContent {
					respContent := map[string]any{
						"application/json": map[string]any{
							"schema": schemaRef(respName),
						},
					}
					if cborHTTP {
						respContent["application/cbor"] = map[string]any{
							"schema": schemaRef(respName),
						}
					}
					resp["content"] = respContent
				}
				responses[strconv.Itoa(st)] = resp
			}
		}
	} else {
		for _, st := range successStatuses {
			responses[strconv.Itoa(st)] = map[string]any{"description": http.StatusText(st)}
		}
	}

	// Error responses from discovered helpers
	for _, e := range route.Errors {
		status := errorStatus(e)
		if status == "" {
			continue
		}
		responses[status] = map[string]any{
			"description": e,
			"content": map[string]any{
				"application/problem+json": map[string]any{
					"schema": schemaRef("ProblemDetails"),
				},
			},
		}
	}
	if hasCheckValidation {
		if _, ok := responses["400"]; !ok {
			responses["400"] = map[string]any{
				"description": "Validation",
				"content": map[string]any{
					"application/problem+json": map[string]any{
						"schema": schemaRef("ProblemDetails"),
					},
				},
			}
		}
	}

	// Wrapper-derived responses (optional metadata)
	if route.Wrappers.MaxRequestBodyBytes != nil {
		if _, ok := responses["413"]; !ok {
			responses["413"] = map[string]any{
				"description": "Payload Too Large",
				"content": map[string]any{
					"application/problem+json": map[string]any{
						"schema": schemaRef("ProblemDetails"),
					},
				},
			}
		}
	}
	if route.Wrappers.Timeout != "" {
		if _, ok := responses["503"]; !ok {
			responses["503"] = map[string]any{
				"description": "Service Unavailable",
				"content": map[string]any{
					"application/problem+json": map[string]any{
						"schema": schemaRef("ProblemDetails"),
					},
				},
			}
		}
	}

	return op
}

func parameter(in string, f FieldPlan, required bool) map[string]any {
	schema := schemaForField(f, schemaScope{})
	// A repeated query key is an array on the wire, so the document has to say
	// array rather than the string the default arm would name. Nothing else is
	// written: the OpenAPI default for a query parameter is style form with
	// explode true, which already serializes an array as the repeated key the
	// binder reads.
	//
	// Only required applies to a slice, and the parameter object carries that
	// itself, so no check-tag constraint is lost by replacing the schema here.
	if in == "query" && f.BindsRepeatedFromQuery() {
		schema = map[string]any{"type": "array", "items": schemaForKind(f.ElemKind)}
	}
	// Field docs belong on the parameter object, so drop the schema copies.
	delete(schema, "description")
	delete(schema, "deprecated")
	return describe(map[string]any{
		"name":     f.Wire,
		"in":       in,
		"required": required,
		"schema":   schema,
	}, f.Doc)
}

// describe attaches godoc text to an OpenAPI schema or parameter object.
func describe(target map[string]any, doc string) map[string]any {
	if doc == "" {
		return target
	}
	target["description"] = doc
	if godoc.Deprecated(doc) {
		target["deprecated"] = true
	}
	return target
}

func schemaForKind(kind string) map[string]any {
	switch kind {
	case "int", "int64":
		return map[string]any{"type": "integer"}
	case "int8", "int16", "int32", "uint", "uint8", "uint16", "uint32", "uint64":
		// The binder enforces the declared width, so the document states it;
		// without this the default arm below would call a uint32 a string.
		bits, unsigned, _ := intKindBits(kind)
		schema := map[string]any{"type": "integer"}
		if bits > 0 && bits <= 32 {
			schema["format"] = "int32"
		} else {
			schema["format"] = "int64"
		}
		if unsigned {
			schema["minimum"] = 0
			if bits > 0 && bits < 64 {
				schema["maximum"] = uint64(1)<<bits - 1
			}
		} else if bits > 0 && bits < 64 {
			schema["minimum"] = -(int64(1) << (bits - 1))
			schema["maximum"] = int64(1)<<(bits-1) - 1
		}
		return schema
	case "bool":
		return map[string]any{"type": "boolean"}
	case "float64":
		return map[string]any{"type": "number"}
	case "file":
		return map[string]any{"type": "string", "format": "binary"}
	case KindBytes:
		// The OpenAPI spelling of a base64 string, which is what the codec
		// writes for a byte slice.
		return map[string]any{"type": "string", "format": "byte"}
	case KindRestAny, KindRestRaw:
		// Rest maps are also expressed as additionalProperties on the parent object
		// when present; this schema is used if the field appears as a property.
		return map[string]any{"type": "object", "additionalProperties": true}
	default:
		return map[string]any{"type": "string"}
	}
}

// schemaScope is what a schema needs to describe a field whose value is not a
// scalar: the component map a named type registers itself into, and the type
// index to build that type from.
//
// A parameter passes the zero value. A path, header, cookie, or query field is
// a scalar, a byte sequence, or a slice of scalars, and none of those reaches a
// named type.
type schemaScope struct {
	schemas map[string]any
	types   map[string]TypePlan
}

// schemaForField builds an OpenAPI schema object including check-tag constraints
// and the enum and default tags.
func schemaForField(f FieldPlan, scope schemaScope) map[string]any {
	if composite, ok := compositeSchema(f, scope); ok {
		// validateCheckAgainstKind allows only required on a composite, and enum
		// and default are scalars-only too, so nothing below this point could
		// have reached one. The godoc still can.
		return describe(composite, f.Doc)
	}
	s := schemaForKind(f.Kind)
	c := f.Check
	if c.Min != nil {
		s["minimum"] = *c.Min
	}
	if c.Max != nil {
		s["maximum"] = *c.Max
	}
	if c.MinLen != nil {
		s["minLength"] = *c.MinLen
	}
	if c.MaxLen != nil {
		s["maxLength"] = *c.MaxLen
	}
	if c.Len != nil {
		s["minLength"] = *c.Len
		s["maxLength"] = *c.Len
	}
	if f.Enum.Set {
		enums := make([]any, 0, len(f.Enum.Values))
		for _, v := range f.Enum.Values {
			enums = append(enums, enumJSONValue(f.Kind, v))
		}
		s["enum"] = enums
	}
	if c.Pattern != "" {
		s["pattern"] = c.Pattern
	}
	if c.Email {
		s["format"] = "email"
	}
	if c.UUID {
		s["format"] = "uuid"
	}
	if c.Date {
		s["format"] = "date"
	}
	if c.Time {
		s["format"] = "time"
	}
	if c.DateTime {
		s["format"] = "date-time"
	}
	if f.Default.Set {
		s["default"] = enumJSONValue(f.Kind, f.Default.Value)
	}
	return describe(s, f.Doc)
}

// compositeSchema builds the schema of a field whose value is not a scalar. It
// reports false for every kind schemaForKind already spells, which is every
// scalar plus a file, a byte sequence, and a rest map.
//
// Without this, a slice, a map, and a nested struct all fell to the default arm
// of schemaForKind and were documented as strings.
func compositeSchema(f FieldPlan, scope schemaScope) (map[string]any, bool) {
	switch f.Kind {
	case KindSlice, KindArray:
		out := map[string]any{"type": "array", "items": elementSchema(f, scope)}
		// A fixed-length array has exactly that many elements, and the codec
		// enforces it. The length is kept as it was written, so a constant name
		// stays unstated rather than being resolved behind the author's back.
		if n, err := strconv.Atoi(f.ArrayLen); err == nil && n >= 0 {
			out["minItems"], out["maxItems"] = n, n
		}
		return out, true
	case KindMap:
		// A JSON object keys by string and a urlencoded body has no other
		// spelling either, so the key type says nothing a document carries. The
		// value type is the whole content.
		return map[string]any{"type": "object", "additionalProperties": elementSchema(f, scope)}, true
	case KindStruct:
		return namedSchema(f.TypeName, scope), true
	}
	return nil, false
}

// elementSchema describes one element of a collection. A collection of
// collections never reaches here: the analysis records no field plan for one,
// so nothing downstream — this document included — ever sees it.
func elementSchema(f FieldPlan, scope schemaScope) map[string]any {
	if f.ElemKind == KindStruct {
		return namedSchema(f.TypeName, scope)
	}
	// A named element such as the Mark of a []Mark documents as the kind it is
	// written over, which is what the codec reads and writes.
	return schemaForKind(f.ElemKind)
}

// namedSchema registers a struct type as a component and refers to it, so one
// type reached from several places is described once.
//
// A type the analysis did not plan, and a parameter with no scope to register
// into, both fall back to an unconstrained object rather than to the string the
// default arm would have produced.
func namedSchema(name string, scope schemaScope) map[string]any {
	name = stripPackage(name)
	if name == "" || scope.schemas == nil {
		return map[string]any{"type": "object"}
	}
	ensureSchema(scope, name, scope.types[name])
	return schemaRef(name)
}

func enumJSONValue(kind, val string) any {
	switch kind {
	case "int", "int64":
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return val
		}
		return n
	case "float64":
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return val
		}
		return n
	case "bool":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return val
		}
		return b
	default:
		return val
	}
}

func ensureSchema(scope schemaScope, name string, tp TypePlan) {
	if name == "" || scope.schemas == nil {
		return
	}
	if _, ok := scope.schemas[name]; ok {
		return
	}
	if tp.Name == "" {
		// unknown type: generic object
		scope.schemas[name] = map[string]any{"type": "object"}
		return
	}
	// Claim the name before walking the fields. A type holding a slice of
	// itself refers to itself through namedSchema, and the entry it finds here
	// is what stops that walk; the finished schema replaces this one below,
	// and a reference addresses the name rather than the value.
	scope.schemas[name] = map[string]any{"type": "object"}
	props := map[string]any{}
	var required []string
	additionalProps := false
	for _, f := range tp.Fields {
		if f.IsRest() {
			additionalProps = true
			continue
		}
		if f.JSONSkip {
			continue // json:"-": not part of the document the schema describes
		}
		key := f.JSON
		if key == "" {
			key = f.Wire
		}
		props[key] = schemaForField(f, scope)
		if f.Check.Required {
			required = append(required, key)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if additionalProps {
		schema["additionalProperties"] = true
	}
	if len(required) > 0 {
		schema["required"] = stringSliceAny(required)
	}
	scope.schemas[name] = describe(schema, tp.Doc)
}

// stringSliceAny converts []string to []any for OpenAPI document maps / YAML.
func stringSliceAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func schemaRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func problemDetailsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":   map[string]any{"type": "string"},
			"title":  map[string]any{"type": "string"},
			"status": map[string]any{"type": "integer"},
			"detail": map[string]any{"type": "string"},
			"code":   map[string]any{"type": "string"},
			"errors": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"field":    map[string]any{"type": "string"},
						"location": map[string]any{"type": "string"},
						"message":  map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func errorStatus(name string) string {
	switch name {
	case "BadRequest", "Validation":
		return "400"
	case "Unauthorized":
		return "401"
	case "Forbidden":
		return "403"
	case "NotFound":
		return "404"
	case "Conflict":
		return "409"
	case "PayloadTooLarge":
		return "413"
	case "Internal":
		return "500"
	default:
		return ""
	}
}

func stripPackage(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	// httpbind.Stream[ChatEvent] already handled elsewhere
	return name
}

func extractStreamElem(resp string) string {
	// ...Stream[ChatEvent]
	i := strings.Index(resp, "Stream[")
	if i < 0 {
		return ""
	}
	s := resp[i+len("Stream["):]
	if j := strings.Index(s, "]"); j >= 0 {
		return s[:j]
	}
	return s
}

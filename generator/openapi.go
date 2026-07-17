package generator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/shibukawa/httpbind-go/parser"
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
	normalized := g.Options.normalized()
	if !normalized.openAPI {
		return nil, fmt.Errorf("%w: %s", ErrFeatureDisabled, FeatureOpenAPI)
	}
	routes, err := parser.ParsePackageWithConfig(dir, normalized.parserConfig)
	if err != nil {
		return nil, fmt.Errorf("parse routes: %w", err)
	}
	plan, err := g.Analyze(dir)
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
		op := buildOperation(route, types, schemas)
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

// YAML returns a minimal YAML encoding of the document (deterministic key order).
func (d Document) YAML() ([]byte, error) {
	var b strings.Builder
	if err := writeYAML(&b, map[string]any(d), 0); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func indexTypes(plan *PackagePlan) map[string]TypePlan {
	out := make(map[string]TypePlan, len(plan.Types))
	for _, t := range plan.Types {
		out[t.Name] = t
	}
	return out
}

func buildOperation(route parser.Route, types map[string]TypePlan, schemas map[string]any) map[string]any {
	op := map[string]any{
		"responses": map[string]any{},
	}
	if route.Handler.Name != "" {
		op["operationId"] = route.Handler.Name
	}

	var params []any
	var bodyProps map[string]any
	var bodyRequired []string
	needBody := false
	bodyAdditionalProps := false

	if route.Request != "" {
		reqName := stripPackage(route.Request)
		ensureSchema(schemas, reqName, types[reqName])
		if tp, ok := types[reqName]; ok {
			for _, f := range tp.Fields {
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
					bodyProps[f.Wire] = schemaForField(f)
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
					bodyProps[f.Wire] = schemaForField(f)
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
		ensureSchema(schemas, elem, types[elem])
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
			ensureSchema(schemas, respName, types[respName])
			for _, st := range successStatuses {
				resp := map[string]any{
					"description": http.StatusText(st),
				}
				if st != http.StatusNoContent {
					resp["content"] = map[string]any{
						"application/json": map[string]any{
							"schema": schemaRef(respName),
						},
					}
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
	return map[string]any{
		"name":     f.Wire,
		"in":       in,
		"required": required,
		"schema":   schemaForField(f),
	}
}

func schemaForKind(kind string) map[string]any {
	switch kind {
	case "int", "int64":
		return map[string]any{"type": "integer"}
	case "bool":
		return map[string]any{"type": "boolean"}
	case "float64":
		return map[string]any{"type": "number"}
	case "file":
		return map[string]any{"type": "string", "format": "binary"}
	case KindRestAny, KindRestRaw:
		// Rest maps are also expressed as additionalProperties on the parent object
		// when present; this schema is used if the field appears as a property.
		return map[string]any{"type": "object", "additionalProperties": true}
	default:
		return map[string]any{"type": "string"}
	}
}

// schemaForField builds an OpenAPI schema object including check-tag constraints.
func schemaForField(f FieldPlan) map[string]any {
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
	if len(c.Enum) > 0 {
		enums := make([]any, 0, len(c.Enum))
		for _, v := range c.Enum {
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
	if c.HasDefault {
		s["default"] = enumJSONValue(f.Kind, c.Default)
	}
	return s
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

func ensureSchema(schemas map[string]any, name string, tp TypePlan) {
	if name == "" {
		return
	}
	if _, ok := schemas[name]; ok {
		return
	}
	if tp.Name == "" {
		// unknown type: generic object
		schemas[name] = map[string]any{"type": "object"}
		return
	}
	props := map[string]any{}
	var required []string
	additionalProps := false
	for _, f := range tp.Fields {
		if f.IsRest() {
			additionalProps = true
			continue
		}
		key := f.JSON
		if key == "" {
			key = f.Wire
		}
		props[key] = schemaForField(f)
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
	schemas[name] = schema
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
	// httpbinder.Stream[ChatEvent] already handled elsewhere
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

func writeYAML(b *strings.Builder, v any, indent int) error {
	// Normalize typed string slices so required: etc. emit as YAML lists.
	if ss, ok := v.([]string); ok {
		return writeYAML(b, stringSliceAny(ss), indent)
	}
	ind := strings.Repeat("  ", indent)
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			val := x[k]
			if ss, ok := val.([]string); ok {
				val = stringSliceAny(ss)
			}
			switch val.(type) {
			case map[string]any, []any:
				fmt.Fprintf(b, "%s%s:\n", ind, k)
				if err := writeYAML(b, val, indent+1); err != nil {
					return err
				}
			default:
				fmt.Fprintf(b, "%s%s: %s\n", ind, k, yamlScalar(val))
			}
		}
	case []any:
		for _, item := range x {
			if ss, ok := item.([]string); ok {
				item = stringSliceAny(ss)
			}
			switch item.(type) {
			case map[string]any, []any:
				fmt.Fprintf(b, "%s-\n", ind)
				if err := writeYAML(b, item, indent+1); err != nil {
					return err
				}
			default:
				fmt.Fprintf(b, "%s- %s\n", ind, yamlScalar(item))
			}
		}
	default:
		fmt.Fprintf(b, "%s%s\n", ind, yamlScalar(x))
	}
	return nil
}

func yamlScalar(v any) string {
	switch x := v.(type) {
	case string:
		// quote if needed
		if x == "" || strings.ContainsAny(x, ":#\n'\"") || strings.Contains(x, " ") {
			return strconvQuote(x)
		}
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%v", x)
	case nil:
		return "null"
	default:
		return strconvQuote(fmt.Sprint(x))
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

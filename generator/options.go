package generator

import (
	"errors"
	"fmt"

	"github.com/shibukawa/tinybind-go/parser"
)

// ErrFeatureDisabled is returned when a disabled generator artifact is invoked directly.
var ErrFeatureDisabled = errors.New("generator: feature disabled")

// PatternSet is an authoritative set of discovery identities. Set replaces,
// rather than extends, any defaults. Disabled suppresses the feature entirely.
type PatternSet[T any] struct {
	Set      []T
	Disabled bool
}

// SymbolPattern identifies a package-level declaration by go/types identity.
type SymbolPattern struct{ PackagePath, Name string }

// TypePattern identifies a named type by go/types identity.
type TypePattern struct{ PackagePath, Name string }

// MethodPattern identifies a method and its receiver type.
type MethodPattern struct {
	PackagePath         string
	Name                string
	ReceiverPackagePath string
	ReceiverType        string
}

// Feature identifies a generator capability that can be permanently disabled.
type Feature string

const (
	FeatureRouteDiscovery Feature = "route-discovery"
	FeatureOpenAPI        Feature = "openapi"
	FeatureBind           Feature = "bind"
	FeatureWrite          Feature = "write"
	FeatureWriteStatus    Feature = "write-status"
	FeatureDecodeJSON     Feature = "decode-json"
	FeatureEncodeJSON     Feature = "encode-json"
	FeatureStreaming      Feature = "streaming"
	FeatureScanRows       Feature = "scan-rows"
	FeatureMultipartFile  Feature = "multipart-file"
)

// Options configures discovery identities and generated template APIs. A zero
// Options value intentionally discovers nothing and disables optional wrappers;
// use DefaultOptions for standard behavior.
type Options struct {
	ServeMuxes      PatternSet[TypePattern]
	RouteMethods    PatternSet[MethodPattern]
	RouteFunctions  PatternSet[SymbolPattern]
	RuntimePackages PatternSet[string]
	Calls           PatternSet[CallPattern]
	FileTypes       PatternSet[TypePattern]
	// HTMLTemplatePattern and SQLTemplatePattern are filepath.Match patterns
	// applied to file base names. Empty values use the standard patterns.
	HTMLTemplatePattern string
	SQLTemplatePattern  string
	// SQLContextAPI adds Context-resolved wrappers for exported SQL templates.
	SQLContextAPI bool
	// SQLContextOnlyAPI publishes only the Context-resolved SQL surface under
	// the name declared in the template. The executor-taking function becomes
	// unexported and no <Component>Context wrapper is generated. It implies
	// SQLContextAPI.
	SQLContextOnlyAPI bool
	// SQLExecutorResolver selects a framework-specific Context resolver and
	// implies SQLContextAPI. Nil uses sqlbind.SQLExecutorFromContext.
	SQLExecutorResolver *SymbolPattern

	DisableFeatures []Feature
	GenerateAll     bool
}

// DefaultOptions returns the standard tinybind runtime setup.
func DefaultOptions() Options {
	return Options{
		ServeMuxes: PatternSet[TypePattern]{Set: []TypePattern{
			{PackagePath: "net/http", Name: "ServeMux"},
			{PackagePath: "github.com/shibukawa/tinygodriver/httpmux", Name: "ServeMux"},
		}},
		RouteFunctions: PatternSet[SymbolPattern]{Set: []SymbolPattern{
			{PackagePath: "net/http", Name: "Handle"},
			{PackagePath: "net/http", Name: "HandleFunc"},
		}},
		RuntimePackages:     PatternSet[string]{Set: []string{httpbindImportPath, jsonbindImportPath, sqlbindImportPath}},
		FileTypes:           PatternSet[TypePattern]{Set: []TypePattern{{PackagePath: httpbindImportPath, Name: "File"}}},
		HTMLTemplatePattern: DefaultHTMLTemplatePattern,
		SQLTemplatePattern:  DefaultSQLTemplatePattern,
	}
}

type normalizedOptions struct {
	symbols      []DiscoverySymbol
	fileTypes    []TypePattern
	parserConfig parser.Config
	enabledUsage Usage
	openAPI      bool
}

func (o Options) normalized() (normalizedOptions, error) {
	disabled := make(map[Feature]bool, len(o.DisableFeatures))
	for _, feature := range o.DisableFeatures {
		disabled[feature] = true
	}
	n := normalizedOptions{openAPI: !disabled[FeatureOpenAPI]}

	callPatterns, err := o.callPatterns()
	if err != nil {
		return normalizedOptions{}, err
	}
	if !disabled[FeatureRouteDiscovery] {
		if !o.RouteFunctions.Disabled {
			for _, symbol := range o.RouteFunctions.Set {
				callPatterns = append(callPatterns, RouteRegisterCall(
					Function(symbol.PackagePath, symbol.Name), Argument("pattern", 0), Argument("handler", 1),
				))
			}
		}
		if !o.RouteMethods.Disabled {
			for _, symbol := range o.RouteMethods.Set {
				callPatterns = append(callPatterns, RouteRegisterCall(
					Method(symbol.PackagePath, symbol.Name, symbol.ReceiverPackagePath, symbol.ReceiverType), Argument("pattern", 0), Argument("handler", 1),
				))
			}
		}
		if !o.ServeMuxes.Disabled {
			for _, mux := range o.ServeMuxes.Set {
				for _, name := range []string{"Handle", "HandleFunc"} {
					callPatterns = append(callPatterns, RouteRegisterCall(
						Method(mux.PackagePath, name, mux.PackagePath, mux.Name), Argument("pattern", 0), Argument("handler", 1),
					))
				}
			}
		}
	}
	normalizedCalls := NewCallRegistry()
	if err := normalizedCalls.Register(callPatterns...); err != nil {
		return normalizedOptions{}, err
	}
	callPatterns = normalizedCalls.patterns
	sortCallPatterns(callPatterns)
	for _, pattern := range callPatterns {
		if featureDisabledForCall(pattern.Operation, disabled) {
			continue
		}
		usage := usageForCallOperation(pattern.Operation)
		if usage != 0 {
			n.enabledUsage |= usage
			typeSource := primaryTypeSource(pattern)
			symbol := DiscoverySymbol{Usage: usage}
			if typeSource.GenericArgument != nil {
				symbol.TypeArgument = *typeSource.GenericArgument
			}
			symbol.ArgumentType = typeSource.ArgumentType
			if target := pattern.Target.Function; target != nil {
				symbol.PackagePath, symbol.Name = target.PackagePath, target.Name
			} else if target := pattern.Target.Method; target != nil {
				symbol.PackagePath, symbol.Name = target.PackagePath, target.Name
				symbol.ReceiverPackagePath, symbol.ReceiverType = target.ReceiverPackagePath, target.ReceiverType
			}
			n.symbols = append(n.symbols, symbol)
		}
		if parserPattern, ok := toParserCallPattern(pattern); ok {
			n.parserConfig.Calls = append(n.parserConfig.Calls, parserPattern)
		}
	}

	if !o.FileTypes.Disabled && !disabled[FeatureMultipartFile] {
		n.fileTypes = append(n.fileTypes, o.FileTypes.Set...)
	}
	return n, nil
}

func (o Options) callPatterns() ([]CallPattern, error) {
	if o.Calls.Disabled {
		return nil, nil
	}
	patterns := o.Calls.Set
	if patterns == nil && !o.RuntimePackages.Disabled {
		for _, path := range o.RuntimePackages.Set {
			patterns = append(patterns, canonicalRuntimeCalls(path)...)
		}
		if len(o.RuntimePackages.Set) > 0 {
			patterns = append(patterns, ConfigBindCall(
				Function(configbindImportPath, "Bind"),
				GenericType("config", 0), Argument("prefix", 0),
			))
			patterns = append(patterns, ConfigSubCommandCall(
				Function(configbindImportPath, "SubCommand"),
				GenericType("config", 0), Argument("name", 0), Argument("help", 1),
			))
		}
	}
	registry := NewCallRegistry()
	if err := registry.Register(patterns...); err != nil {
		return nil, err
	}
	result := append([]CallPattern(nil), registry.patterns...)
	sortCallPatterns(result)
	return result, nil
}

func canonicalRuntimeCalls(path string) []CallPattern {
	patterns := []CallPattern{
		RequestBindCall(Function(path, "Bind"), GenericType("request", 0)),
		ResponseWriteCall(Function(path, "Write"), GenericType("response", 0)),
		ResponseWriteStatusCall(Function(path, "WriteStatus"), GenericType("response", 0), Argument("status", 2)),
		StreamCreateCall(Function(path, "NewStream"), GenericType("stream", 0)),
		JSONDecodeCall(Function(path, "DecodeJSON"), GenericType("decode", 0)),
		JSONEncodeCall(Function(path, "EncodeJSON"), GenericType("encode", 0)),
		RowsScanCall(Function(path, "ScanRows"), GenericType("row", 0)),
	}
	statuses := map[string]int{
		"BadRequest": 400, "Validation": 400, "Unauthorized": 401, "Forbidden": 403,
		"NotFound": 404, "Conflict": 409, "PayloadTooLarge": 413, "Internal": 500,
	}
	for name, status := range statuses {
		patterns = append(patterns, ErrorResponseCall(
			Function(path, name), Constant("status", status), Constant("error_name", name),
		))
	}
	return patterns
}

func usageForCallOperation(operation CallOperation) Usage {
	switch operation {
	case OperationRequestBind:
		return UsageBind
	case OperationResponseWrite, OperationResponseWriteStatus, OperationStreamCreate:
		return UsageWrite
	case OperationJSONEncode:
		return UsageEncodeJSON
	case OperationJSONDecode:
		return UsageDecodeJSON
	case OperationRowsScan:
		return UsageScanRows
	default:
		return 0
	}
}

func featureDisabledForCall(operation CallOperation, disabled map[Feature]bool) bool {
	switch operation {
	case OperationRequestBind:
		return disabled[FeatureBind]
	case OperationResponseWrite:
		return disabled[FeatureWrite]
	case OperationResponseWriteStatus:
		return disabled[FeatureWriteStatus]
	case OperationStreamCreate:
		return disabled[FeatureStreaming]
	case OperationJSONDecode:
		return disabled[FeatureDecodeJSON]
	case OperationJSONEncode:
		return disabled[FeatureEncodeJSON]
	case OperationRowsScan:
		return disabled[FeatureScanRows]
	default:
		return false
	}
}

func primaryTypeSource(pattern CallPattern) TypeSource {
	roles := []string{"request", "response", "stream", "decode", "encode", "row", "config"}
	for _, role := range roles {
		if source, ok := pattern.TypeRoles[role]; ok {
			return source
		}
	}
	return TypeSource{}
}

func toParserCallPattern(pattern CallPattern) (parser.CallPattern, bool) {
	operation := parser.CallOperation("")
	role := ""
	switch pattern.Operation {
	case OperationRequestBind:
		operation, role = parser.CallRequestBind, "request"
	case OperationResponseWrite:
		operation, role = parser.CallResponseWrite, "response"
	case OperationResponseWriteStatus:
		operation, role = parser.CallResponseWriteStatus, "response"
	case OperationStreamCreate:
		operation, role = parser.CallStreamCreate, "stream"
	case OperationErrorResponse:
		operation = parser.CallErrorResponse
	case OperationRouteRegister:
		operation = parser.CallRouteRegister
	default:
		return parser.CallPattern{}, false
	}
	target := parser.RouteSymbol{}
	if pattern.Target.Function != nil {
		target.PackagePath, target.Name = pattern.Target.Function.PackagePath, pattern.Target.Function.Name
	} else if pattern.Target.Method != nil {
		method := pattern.Target.Method
		target = parser.RouteSymbol{PackagePath: method.PackagePath, Name: method.Name, ReceiverPackagePath: method.ReceiverPackagePath, ReceiverType: method.ReceiverType}
	}
	result := parser.CallPattern{Target: target, Operation: operation}
	if role != "" {
		source := pattern.TypeRoles[role]
		if source.GenericArgument != nil {
			result.TypeArgument = *source.GenericArgument
		} else if source.ArgumentType != nil {
			index := *source.ArgumentType
			result.TypeValueArgument = &index
		} else {
			return parser.CallPattern{}, false
		}
	}
	if operation == parser.CallResponseWriteStatus {
		source := pattern.ArgumentRoles["status"]
		if source.Argument != nil {
			index := *source.Argument
			result.StatusArgument = &index
		} else if source.IsConstant {
			status, ok := source.Constant.(int)
			if !ok {
				return parser.CallPattern{}, false
			}
			result.StatusConstant = &status
		}
	}
	if operation == parser.CallErrorResponse {
		status, ok := pattern.ArgumentRoles["status"].Constant.(int)
		if !ok {
			return parser.CallPattern{}, false
		}
		if name, ok := pattern.ArgumentRoles["error_name"].Constant.(string); ok {
			result.ErrorName = name
		} else {
			result.ErrorName = errorNameForStatus(status)
		}
	}
	if operation == parser.CallRouteRegister {
		patternSource := pattern.ArgumentRoles["pattern"]
		handlerSource := pattern.ArgumentRoles["handler"]
		if handlerSource.Argument == nil {
			return parser.CallPattern{}, false
		}
		if patternSource.Argument != nil {
			result.PatternArgument = *patternSource.Argument
		} else if patternSource.IsConstant {
			value, ok := patternSource.Constant.(string)
			if !ok {
				return parser.CallPattern{}, false
			}
			result.PatternConstant = &value
		} else {
			return parser.CallPattern{}, false
		}
		result.HandlerArgument = *handlerSource.Argument
	}
	return result, true
}

func errorNameForStatus(status int) string {
	switch status {
	case 400:
		return "BadRequest"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "NotFound"
	case 409:
		return "Conflict"
	case 413:
		return "PayloadTooLarge"
	case 500:
		return "Internal"
	default:
		return fmt.Sprintf("Status%d", status)
	}
}

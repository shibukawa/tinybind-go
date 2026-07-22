package generator

import (
	"errors"

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

	Bind        PatternSet[SymbolPattern]
	Write       PatternSet[SymbolPattern]
	WriteStatus PatternSet[SymbolPattern]
	DecodeJSON  PatternSet[SymbolPattern]
	EncodeJSON  PatternSet[SymbolPattern]
	NewStream   PatternSet[SymbolPattern]
	ScanRows    PatternSet[SymbolPattern]
	FileTypes   PatternSet[TypePattern]
	// SQLContextAPI adds Context-resolved wrappers for exported SQL templates.
	SQLContextAPI bool
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
		RuntimePackages: PatternSet[string]{Set: []string{httpbindImportPath, jsonbindImportPath, sqlbindImportPath}},
		FileTypes:       PatternSet[TypePattern]{Set: []TypePattern{{PackagePath: httpbindImportPath, Name: "File"}}},
	}
}

type normalizedOptions struct {
	symbols      []DiscoverySymbol
	fileTypes    []TypePattern
	parserConfig parser.Config
	enabledUsage Usage
	openAPI      bool
}

func (o Options) normalized() normalizedOptions {
	disabled := make(map[Feature]bool, len(o.DisableFeatures))
	for _, feature := range o.DisableFeatures {
		disabled[feature] = true
	}
	runtimePackages := o.RuntimePackages.Set
	if o.RuntimePackages.Disabled {
		runtimePackages = nil
	}
	resolve := func(set PatternSet[SymbolPattern], name string, feature Feature) []SymbolPattern {
		if set.Disabled || disabled[feature] {
			return nil
		}
		if set.Set != nil {
			return append([]SymbolPattern(nil), set.Set...)
		}
		out := make([]SymbolPattern, 0, len(runtimePackages))
		for _, path := range runtimePackages {
			out = append(out, SymbolPattern{PackagePath: path, Name: name})
		}
		return out
	}

	bind := resolve(o.Bind, "Bind", FeatureBind)
	write := resolve(o.Write, "Write", FeatureWrite)
	writeStatus := resolve(o.WriteStatus, "WriteStatus", FeatureWriteStatus)
	decode := resolve(o.DecodeJSON, "DecodeJSON", FeatureDecodeJSON)
	encode := resolve(o.EncodeJSON, "EncodeJSON", FeatureEncodeJSON)
	stream := resolve(o.NewStream, "NewStream", FeatureStreaming)
	scanRows := resolve(o.ScanRows, "ScanRows", FeatureScanRows)

	n := normalizedOptions{openAPI: !disabled[FeatureOpenAPI]}
	add := func(patterns []SymbolPattern, usage Usage) {
		if len(patterns) == 0 {
			return
		}
		n.enabledUsage |= usage
		for _, pattern := range patterns {
			n.symbols = append(n.symbols, DiscoverySymbol{PackagePath: pattern.PackagePath, Name: pattern.Name, Usage: usage})
			n.parserConfig.GenericFunctions = append(n.parserConfig.GenericFunctions, parser.FunctionSymbol{PackagePath: pattern.PackagePath, Name: pattern.Name})
		}
	}
	add(bind, UsageBind)
	add(write, UsageWrite)
	add(writeStatus, UsageEncodeJSON)
	add(decode, UsageDecodeJSON)
	add(encode, UsageEncodeJSON)
	add(stream, UsageEncodeJSON)
	add(scanRows, UsageScanRows)

	if !o.FileTypes.Disabled && !disabled[FeatureMultipartFile] {
		n.fileTypes = append(n.fileTypes, o.FileTypes.Set...)
	}
	if !disabled[FeatureRouteDiscovery] {
		if !o.RouteFunctions.Disabled {
			for _, pattern := range o.RouteFunctions.Set {
				n.parserConfig.RouteRegistrations = append(n.parserConfig.RouteRegistrations, parser.RouteSymbol{PackagePath: pattern.PackagePath, Name: pattern.Name})
			}
		}
		if !o.RouteMethods.Disabled {
			for _, pattern := range o.RouteMethods.Set {
				n.parserConfig.RouteRegistrations = append(n.parserConfig.RouteRegistrations, parser.RouteSymbol{
					PackagePath: pattern.PackagePath, Name: pattern.Name,
					ReceiverPackagePath: pattern.ReceiverPackagePath, ReceiverType: pattern.ReceiverType,
				})
			}
		}
		if !o.ServeMuxes.Disabled {
			for _, pattern := range o.ServeMuxes.Set {
				for _, name := range []string{"Handle", "HandleFunc"} {
					n.parserConfig.RouteRegistrations = append(n.parserConfig.RouteRegistrations, parser.RouteSymbol{
						PackagePath: pattern.PackagePath, Name: name,
						ReceiverPackagePath: pattern.PackagePath, ReceiverType: pattern.Name,
					})
				}
			}
		}
	}
	for _, path := range runtimePackages {
		for _, name := range []string{"BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "PayloadTooLarge", "Internal", "Validation"} {
			n.parserConfig.ErrorFunctions = append(n.parserConfig.ErrorFunctions, parser.FunctionSymbol{PackagePath: path, Name: name})
		}
	}
	return n
}

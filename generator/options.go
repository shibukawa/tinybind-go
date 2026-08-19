package generator

import (
	"errors"
	"fmt"

	"github.com/shibukawa/tinybind-go/parser"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
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
	FeatureWebSocket      Feature = "websocket"
	FeatureScanRows       Feature = "scan-rows"
	FeatureMultipartFile  Feature = "multipart-file"
	// FeatureItemCodec turns off DynamoDB item codec generation entirely.
	FeatureItemCodec Feature = "item-codec"
	// FeatureItemTable turns off only the generated table definition, leaving
	// the codec and the key builder in place. Emitting it is the default,
	// because it is what makes a key name single-source; a project that manages
	// tables with IaC and never creates one in Go can drop it.
	FeatureItemTable Feature = "item-table"
	// FeatureEntityCodec turns off Firestore entity codec generation entirely.
	FeatureEntityCodec Feature = "entity-codec"
	// FeatureCBORWireCodec and FeatureCBORWorldCodec turn off CBOR codec
	// generation for one profile. The determinism check inside a generated
	// codec is not a feature and cannot be turned off: it is a build gate, and
	// leaving it off is how a float reaches production as a desync.
	FeatureCBORWireCodec  Feature = "cbor-wire-codec"
	FeatureCBORWorldCodec Feature = "cbor-world-codec"
	// FeatureCBORDelta turns off delta generation while leaving the codecs
	// standing, for a project that sends whole messages and never diffs them.
	FeatureCBORDelta Feature = "cbor-delta"
	// FeatureCacheKey turns off cache key generation entirely.
	FeatureCacheKey Feature = "cache-key"
	// FeatureHelpBackfill writes help tags derived from godoc into config
	// structs. Disable it to keep hand-written sources untouched.
	FeatureHelpBackfill Feature = "help-backfill"
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
	// DynamoTemplatePattern is the base-name glob for DynamoDB query
	// declarations. An empty value uses DefaultDynamoTemplatePattern.
	DynamoTemplatePattern string
	// FirestoreTemplatePattern is the base-name glob for Firestore query
	// declarations. An empty value uses DefaultFirestoreTemplatePattern.
	FirestoreTemplatePattern string
	// SQLDialect names the target database for SQL templates: "postgresql",
	// "mysql", or "sqlite". A run that discovers a SQL template must set it.
	// There is no default, because an assumed dialect emits placeholders the
	// target engine rejects, and nothing about the templates reveals the
	// mistake.
	SQLDialect string
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
	// DynamoParameterAPI gives every generated DynamoDB query a leading
	// dynamobind.Handle parameter instead of resolving one from the Context.
	// The declared name is unchanged; only the signature moves. False keeps the
	// Context form, which is the default and what every existing run generates.
	DynamoParameterAPI bool
	// DynamoHandleResolver selects a framework function that answers a
	// dynamobind.Handle for one Context, so generated code reads the
	// framework's own Context value instead of the one dynamobind installs.
	// It is how a framework carrying every value it manages in one struct
	// serves generated queries with a single lookup.
	//
	// The signature is func(context.Context) (dynamobind.Handle, error). Nil
	// uses dynamobind's own Context key. DynamoParameterAPI takes precedence:
	// a signature that already carries the Handle resolves nothing.
	DynamoHandleResolver *SymbolPattern
	// FirestoreParameterAPI is DynamoParameterAPI for Firestore queries,
	// giving each a leading firestorebind.Handle parameter.
	FirestoreParameterAPI bool
	// FirestoreHandleResolver is DynamoHandleResolver for Firestore queries.
	// The signature is func(context.Context) (firestorebind.Handle, error).
	FirestoreHandleResolver *SymbolPattern
	// DataAttributePrefix names the data attributes generated HTML uses for
	// partial update boundaries. Empty uses the standard prefix. A project
	// overriding it must use a browser runtime built for the same prefix,
	// because the runtime hardcodes it rather than discovering it.
	DataAttributePrefix string
	// Transform selects the source transform and names its target backend. Nil
	// generates the authored net/http backend alone, which is the default and
	// what keeps a run predating this feature byte-identical.
	//
	// Set it to DefaultTransformOptions() for the fasthttp backend this module
	// ships, or to a value carrying your own ImportRewrites for a framework
	// providing the same helper names over the other transport.
	Transform *TransformOptions

	// GeneratedHeaders names header prefixes, beside this module's own, whose
	// files every discovery pass must skip. A framework generating with tinybind
	// and branding its output writes a header nothing here recognizes on its own,
	// and an unrecognized generated registry is analyzed as if a user had written
	// it: its page registrations become routes, and an HTML page enters an OpenAPI
	// document. Each entry still requires the conventional "DO NOT EDIT." ending.
	GeneratedHeaders []string
	// PreserveTemplateWhitespace keeps the authoring indentation and newlines of
	// HTML templates in generated static output instead of collapsing each run
	// to one space. The default collapses, which renders identically and drops
	// every indentation byte from the generated source and the binary.
	PreserveTemplateWhitespace bool

	// PublicDir is the filesystem directory receiving the static files
	// extracted from component style and script blocks. Empty uses
	// DefaultPublicDir.
	PublicDir string
	// PublicURLBase is the URL prefix under which those files are served. It is
	// either an absolute URL path or a full URL, and is used verbatim either
	// way, so a CDN base changes the reference and nothing else. Empty uses
	// DefaultPublicURLBase.
	//
	// Neither option is derived from the other, and setting one explicitly
	// requires setting the other.
	PublicURLBase string

	// ReferenceHooks rewrite the static values of the attributes they are
	// registered for, at generation time, and declare the conversions those
	// rewrites depend on. They are how a build converts a file a template points
	// at, such as an image to a modern format or a TypeScript entry point to
	// JavaScript.
	//
	// A hook converts and returns the bytes, so the rewrite may depend on how
	// the conversion turned out; an encode larger than its source is worth
	// declining, and only the converted bytes can say so.
	ReferenceHooks []htmlbind.ReferenceHook
	// ContentHooks compile the component script blocks whose lang attribute
	// they claim. A block marked lang="ts" reaches the browser as JavaScript
	// through the transform registered here, so the compiler is this command's
	// dependency and never the module's.
	ContentHooks []htmlbind.ContentHook
	// ImplicitBindings are the names an embedder puts in every HTML template's
	// scope, so an application does not thread a framework value through every
	// component and every layout in a chain.
	//
	// They reach every compile path this command drives. That is a
	// checklist item rather than a consequence: the same field on the
	// context-external seam once reached one path and not the other, which
	// shipped a feature that was simply absent on filesystem routes. See
	// .knowledge requirement:route-package-context-externals.
	ImplicitBindings []htmlbind.ImplicitBinding
	// Messages maps a resolved message id to the Go symbol it calls, and
	// MessageContextBinding names the ImplicitBindings entry supplying those
	// symbols' leading argument.
	//
	// The mapping is data because an id is not a Go identifier; whoever owns
	// the catalog decides how a slug becomes a symbol. [htmlbind.MessageRefs]
	// reports what a template needs before this can be filled.
	Messages              map[string]htmlbind.MessageSymbol
	MessageContextBinding string
	// ConversionCacheDir stores the outcome of each conversion, keyed by what
	// the hook's CacheKey declared it depends on. An unchanged asset then costs
	// a digest instead of an encode, and a source that once lost a size
	// comparison is never re-encoded to rediscover it.
	//
	// Empty converts every build, which is correct and slow. It is a plain
	// directory of generated data: deleting it costs time and nothing else.
	ConversionCacheDir string
	// DerivedAssetDir receives the files those conversions produce. It is
	// deliberately not derived from PublicDir: a hook chooses the URL it rewrites
	// to, and only the caller knows which directory is served there.
	//
	// A produced file with no directory configured is a configuration error
	// rather than a silent discard.
	DerivedAssetDir string
	// ConversionWorkers converts what the compile is about to ask for ahead of
	// it, on this many goroutines, instead of one encode at a time inside a
	// sequential compile. It changes wall clock and nothing else: the same
	// bytes, the same produced files, and the same diagnostics in the same
	// order.
	//
	// Zero or one keeps every transform on one goroutine, which is the default
	// because concurrency is a promise about the caller's transform that only
	// the caller can make. Being a pure function of what it reads is necessary
	// and not sufficient: a transform holding a shared scratch buffer is pure by
	// that definition and unsafe by this one. Set this only once Transform is
	// safe for concurrent use.
	//
	// A warm cache converts nothing and starts nothing whatever this says.
	//
	// It is excluded from the hashed options deliberately. Every other field
	// here can change what is generated, and this one cannot: it says how many
	// goroutines do the same work. Hashing it would stamp the machine that ran
	// the build into the output, so a four-core laptop and a sixteen-core runner
	// would disagree on bytes that are identical in every way that matters, and
	// `--check` would fail on the difference between two correct builds.
	ConversionWorkers int `json:"-"`

	DisableFeatures []Feature
	GenerateAll     bool

	// ServerActions are the typed server actions to emit an entry point for in
	// the package being generated.
	//
	// They are supplied rather than discovered because the annotation admitting
	// one is read by routetree, which parses a route package before that
	// package can compile. This phase type-checks, which is why the argument
	// struct and the codecs are built here and the declaration is read there.
	ServerActions []ServerAction `json:"-"`
}

const (
	// DefaultPublicDir receives extracted static assets when a project
	// configures no directory. Extraction always happens, so a
	// zero-configuration project still gets working asset URLs.
	DefaultPublicDir = "public/generated"
	// DefaultPublicURLBase serves those files when a project configures no URL
	// base.
	DefaultPublicURLBase = htmlbind.DefaultPublicURLBase
)

// ErrPublicAssetPairing reports a public asset configuration that sets only one
// of the two independent options.
var ErrPublicAssetPairing = errors.New(
	"generator: PublicDir and PublicURLBase must be set together; " +
		"neither is derived from the other, so configure both or leave both empty for " +
		DefaultPublicDir + " and " + DefaultPublicURLBase)

// resolvedPublicDir is the directory extracted assets are written to.
func (o Options) resolvedPublicDir() string {
	if o.PublicDir == "" {
		return DefaultPublicDir
	}
	return o.PublicDir
}

// resolvedPublicURLBase is the URL prefix extracted assets are referenced by.
func (o Options) resolvedPublicURLBase() string {
	if o.PublicURLBase == "" {
		return DefaultPublicURLBase
	}
	return o.PublicURLBase
}

// checkPublicAssetPairing rejects a half-configured pair rather than inferring
// the missing half from the other.
func checkPublicAssetPairing(dir, urlBase string) error {
	if (dir == "") != (urlBase == "") {
		return ErrPublicAssetPairing
	}
	return nil
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
		RuntimePackages:          PatternSet[string]{Set: []string{httpbindImportPath, jsonbindImportPath, sqlbindImportPath, dynamobindImportPath, firestorebindImportPath, cachekeybindImportPath, cborbindImportPath}},
		FileTypes:                PatternSet[TypePattern]{Set: []TypePattern{{PackagePath: httpbindImportPath, Name: "File"}}},
		HTMLTemplatePattern:      DefaultHTMLTemplatePattern,
		SQLTemplatePattern:       DefaultSQLTemplatePattern,
		DynamoTemplatePattern:    DefaultDynamoTemplatePattern,
		FirestoreTemplatePattern: DefaultFirestoreTemplatePattern,
		PublicDir:                DefaultPublicDir,
		PublicURLBase:            DefaultPublicURLBase,
	}
}

type normalizedOptions struct {
	symbols      []DiscoverySymbol
	fileTypes    []TypePattern
	parserConfig parser.Config
	enabledUsage Usage
	openAPI      bool
}

// featureDisabled reports whether one feature was turned off for this run.
func (o Options) featureDisabled(feature Feature) bool {
	for _, disabled := range o.DisableFeatures {
		if disabled == feature {
			return true
		}
	}
	return false
}

func (o Options) normalized() (normalizedOptions, error) {
	if err := checkPublicAssetPairing(o.PublicDir, o.PublicURLBase); err != nil {
		return normalizedOptions{}, err
	}
	// A malformed hook is a fault in the generate command, so it is reported
	// against the registration rather than against the first template position
	// that happens to reach it.
	if err := htmlbind.ValidateReferenceHooks(o.ReferenceHooks); err != nil {
		return normalizedOptions{}, err
	}
	if err := htmlbind.ValidateContentHooks(o.ContentHooks); err != nil {
		return normalizedOptions{}, err
	}
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

	n.parserConfig.GeneratedHeaders = o.GeneratedHeaders

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
			if path == cborbindImportPath {
				// cborbind shares no entry with the others. It declares six
				// annotations and no operation at all, so the canonical set
				// would register forty names against a package that has none
				// of them.
				patterns = append(patterns, canonicalCBORCalls(path)...)
				continue
			}
			if path == firestorebindImportPath {
				// firestorebind shares no signature with the others: its entries
				// name neither a table nor a client, so the value argument sits
				// one place earlier and the canonical set would mis-read it.
				patterns = append(patterns, canonicalFirestoreCalls(path)...)
				continue
			}
			patterns = append(patterns, withHTTPTransportSlots(path, canonicalRuntimeCalls(path))...)
		}
		if len(o.RuntimePackages.Set) > 0 {
			patterns = append(patterns, htmlupdateTransportOnlyCalls()...)
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

// withHTTPTransportSlots declares which arguments of the canonical calls carry
// the transport, for the one runtime that has a transport.
//
// The canonical set is spelled once for every runtime package, so most of these
// names resolve nowhere and never match. Slots are attached only for the HTTP
// runtime, so a same-named function in another runtime cannot inherit a claim
// about arguments it does not have.
func withHTTPTransportSlots(path string, patterns []CallPattern) []CallPattern {
	if path != httpbindImportPath {
		return patterns
	}
	for i, pattern := range patterns {
		name := ""
		if pattern.Target.Function != nil {
			name = pattern.Target.Function.Name
		}
		switch name {
		case "Bind":
			// func Bind[T](r *http.Request) (T, error)
			RequestArgument(0)(&patterns[i])
		case "Write", "WriteStatus", "WriteStream", "WebSocket", "WebSocketWith":
			// (w, r) leads, and whatever follows keeps its place.
			WriterArgument(0)(&patterns[i])
			RequestArgument(1)(&patterns[i])
		}
	}
	return append(patterns, httpTransportOnlyCalls(path)...)
}

// httpTransportOnlyCalls declares the runtime calls that take a transport value
// and name no model. Discovery reads nothing from them, but the transform must
// know them: WriteError is in every handler ever written against this runtime,
// and without a pattern it would look like an unrecognized call and refuse the
// handler for making it.
//
// Every name here exists under the same spelling on both runtimes.
func httpTransportOnlyCalls(path string) []CallPattern {
	writerThenRequest := []string{"WriteError"}
	writerOnly := []string{"WriteJSONBytes"}
	requestOnly := []string{
		"Queries", "QueryValue", "PathValue", "HeaderValue", "CookieValue",
		"ReadBody", "ReadJSONObject", "ParseFormMap", "ParseMultipartMap",
		"IsJSONRequest", "IsFormRequest", "IsMultipartRequest",
		"NegotiateStreamFormat",
	}
	patterns := make([]CallPattern, 0, len(writerThenRequest)+len(writerOnly)+len(requestOnly))
	for _, name := range writerThenRequest {
		patterns = append(patterns, TransportCall(Function(path, name), WriterArgument(0), RequestArgument(1)))
	}
	for _, name := range writerOnly {
		patterns = append(patterns, TransportCall(Function(path, name), WriterArgument(0)))
	}
	for _, name := range requestOnly {
		patterns = append(patterns, TransportCall(Function(path, name), RequestArgument(0)))
	}
	return patterns
}

func canonicalRuntimeCalls(path string) []CallPattern {
	patterns := []CallPattern{
		RequestBindCall(Function(path, "Bind"), GenericType("request", 0)),
		ResponseWriteCall(Function(path, "Write"), GenericType("response", 0)),
		ResponseWriteStatusCall(Function(path, "WriteStatus"), GenericType("response", 0), Argument("status", 2)),
		StreamCreateCall(Function(path, "WriteStream"), GenericType("stream", 0)),
		// Two patterns against one target: index 0 is decoded, index 1 is
		// encoded, and no single operation can carry both directions.
		SocketReceiveCall(Function(path, "WebSocket"), GenericType("socket-in", 0)),
		SocketSendCall(Function(path, "WebSocket"), GenericType("socket-out", 1)),
		SocketReceiveCall(Function(path, "WebSocketWith"), GenericType("socket-in", 0)),
		SocketSendCall(Function(path, "WebSocketWith"), GenericType("socket-out", 1)),
		JSONDecodeCall(Function(path, "DecodeJSON"), GenericType("decode", 0)),
		JSONEncodeCall(Function(path, "EncodeJSON"), GenericType("encode", 0)),
		// The annotations of requirement:declared-json-codec. They are ordinary
		// configured calls, so the package-level var initializer holding one is
		// walked by the same file inspection every other call site is found by.
		//
		// GenerateCodec is two patterns on one target rather than an operation
		// of its own, so disabling one codec direction leaves the other half of
		// the annotation standing instead of taking the whole thing.
		JSONEncoderDeclareCall(Function(path, "GenerateCodec"), GenericType("encode", 0)),
		JSONDecoderDeclareCall(Function(path, "GenerateCodec"), GenericType("decode", 0)),
		JSONEncoderDeclareCall(Function(path, "GenerateEncoder"), GenericType("encode", 0)),
		JSONDecoderDeclareCall(Function(path, "GenerateDecoder"), GenericType("decode", 0)),
		RowsScanCall(Function(path, "ScanRows"), GenericType("row", 0)),
		ItemDecodeCall(Function(path, "Load"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "LoadAll"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "Query"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "QueryPage"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "Scan"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "ScanPage"), GenericType("item", 0)),
		// The write side reads its type from the value argument, not from the
		// type parameter. Its constraints are the generated interfaces, so
		// before the first generation the call does not type-check and no
		// instantiation is recorded; the argument's own type resolves anyway.
		// Index 2 is the value: the signature is (ctx, table, v, opts...), the
		// client having moved into the Context.
		ItemEncodeCall(Function(path, "Store"), ArgumentType("item", 2)),
		ItemEncodeCall(Function(path, "StoreAll"), ArgumentType("item", 2)),
		ItemEncodeDecodeCall(Function(path, "StoreReturning"), ArgumentType("item", 2)),
		ItemKeyCall(Function(path, "Remove"), ArgumentType("item", 2)),
		ItemKeyCall(Function(path, "Update"), ArgumentType("item", 2)),
		ItemKeyDecodeCall(Function(path, "RemoveReturning"), ArgumentType("item", 2)),
		// The Handle-taking twins of requirement:dynamo-parameter-api. The read
		// side still names its type in the same type parameter, and the write
		// side reads its value one place later, the Handle sitting between the
		// Context and the table.
		ItemDecodeCall(Function(path, "LoadOn"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "LoadAllOn"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "QueryOn"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "QueryPageOn"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "ScanOn"), GenericType("item", 0)),
		ItemDecodeCall(Function(path, "ScanPageOn"), GenericType("item", 0)),
		ItemEncodeCall(Function(path, "StoreOn"), ArgumentType("item", 3)),
		ItemEncodeCall(Function(path, "StoreAllOn"), ArgumentType("item", 3)),
		ItemEncodeDecodeCall(Function(path, "StoreReturningOn"), ArgumentType("item", 3)),
		ItemKeyCall(Function(path, "RemoveOn"), ArgumentType("item", 3)),
		ItemKeyCall(Function(path, "UpdateOn"), ArgumentType("item", 3)),
		ItemKeyDecodeCall(Function(path, "RemoveReturningOn"), ArgumentType("item", 3)),
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

// canonicalFirestoreCalls declares the firestorebind entries discovery reads.
//
// The read side names its type explicitly, since T appears only in the result,
// so the AST carries it even before any codec exists. The write side takes it
// from the value argument at index 1: the signature is (ctx, v, opts...), with
// no table and no client, so it is one earlier than the DynamoDB equivalent.
//
// The Handle-taking twins of requirement:firestore-parameter-api follow the
// same rule one place later, and the *Tx entries have no twin because the
// receiver already carries the handle.
func canonicalFirestoreCalls(path string) []CallPattern {
	return []CallPattern{
		EntityDecodeCall(Function(path, "Load"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "LoadAll"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "LoadTx"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "LoadAllTx"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "Query"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "QueryPage"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "QueryPageTx"), GenericType("entity", 0)),
		EntityEncodeCall(Function(path, "Store"), ArgumentType("entity", 1)),
		EntityEncodeCall(Function(path, "Insert"), ArgumentType("entity", 1)),
		EntityEncodeCall(Function(path, "Update"), ArgumentType("entity", 1)),
		EntityEncodeCall(Function(path, "StoreAll"), ArgumentType("entity", 1)),
		EntityEncodeCall(Function(path, "InsertAll"), ArgumentType("entity", 1)),
		EntityKeyCall(Function(path, "Remove"), ArgumentType("entity", 1)),
		EntityKeyCall(Function(path, "RemoveAll"), ArgumentType("entity", 1)),
		EntityDecodeCall(Function(path, "LoadOn"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "LoadAllOn"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "QueryOn"), GenericType("entity", 0)),
		EntityDecodeCall(Function(path, "QueryPageOn"), GenericType("entity", 0)),
		EntityEncodeCall(Function(path, "StoreOn"), ArgumentType("entity", 2)),
		EntityEncodeCall(Function(path, "InsertOn"), ArgumentType("entity", 2)),
		EntityEncodeCall(Function(path, "UpdateOn"), ArgumentType("entity", 2)),
		EntityEncodeCall(Function(path, "StoreAllOn"), ArgumentType("entity", 2)),
		EntityEncodeCall(Function(path, "InsertAllOn"), ArgumentType("entity", 2)),
		EntityKeyCall(Function(path, "RemoveOn"), ArgumentType("entity", 2)),
		EntityKeyCall(Function(path, "RemoveAllOn"), ArgumentType("entity", 2)),
		// The transaction writes are methods, and their value is the first
		// argument because the receiver carries the handle.
		EntityEncodeCall(Method(path, "Store", path, "Tx"), ArgumentType("entity", 0)),
		EntityEncodeCall(Method(path, "Insert", path, "Tx"), ArgumentType("entity", 0)),
		EntityEncodeCall(Method(path, "Update", path, "Tx"), ArgumentType("entity", 0)),
		EntityKeyCall(Method(path, "Remove", path, "Tx"), ArgumentType("entity", 0)),
	}
}

// canonicalCBORCalls declares the cborbind annotations discovery reads.
//
// Each codec form is two patterns against one target rather than an operation
// of its own, so disabling one direction leaves the other half of the
// annotation standing instead of taking the whole thing -- which is the shape
// the JSON declaration arrived at after a single both-directions operation
// turned out not to be half-removable.
func canonicalCBORCalls(path string) []CallPattern {
	return []CallPattern{
		CBORWireEncoderDeclareCall(Function(path, "GenerateWireCodec"), GenericType("cbor-encode", 0)),
		CBORWireDecoderDeclareCall(Function(path, "GenerateWireCodec"), GenericType("cbor-decode", 0)),
		CBORWireEncoderDeclareCall(Function(path, "GenerateWireEncoder"), GenericType("cbor-encode", 0)),
		CBORWireDecoderDeclareCall(Function(path, "GenerateWireDecoder"), GenericType("cbor-decode", 0)),
		CBORWorldEncoderDeclareCall(Function(path, "GenerateWorldCodec"), GenericType("cbor-encode", 0)),
		CBORWorldDecoderDeclareCall(Function(path, "GenerateWorldCodec"), GenericType("cbor-decode", 0)),
		CBORWorldEncoderDeclareCall(Function(path, "GenerateWorldEncoder"), GenericType("cbor-encode", 0)),
		CBORWorldDecoderDeclareCall(Function(path, "GenerateWorldDecoder"), GenericType("cbor-decode", 0)),
		CBORWireDeltaDeclareCall(Function(path, "GenerateWireDelta"), GenericType("cbor-delta", 0)),
		CBORWorldDeltaDeclareCall(Function(path, "GenerateWorldDelta"), GenericType("cbor-delta", 0)),
	}
}

func usageForCallOperation(operation CallOperation) Usage {
	switch operation {
	case OperationRequestBind:
		return UsageBind
	case OperationResponseWrite:
		return UsageWrite
	case OperationResponseWriteStatus:
		// WriteStatus serializes through jsonbind.EncodeJSON, so a
		// write-status call site needs the encoder registered too.
		return UsageWrite | UsageEncodeJSON
	case OperationStreamCreate:
		// Stream.Write encodes events through the jsonbind codec registry,
		// so stream event types need their encoder registered too.
		return UsageWrite | UsageEncodeJSON
	case OperationSocketReceive:
		// Socket.Read decodes the inbound type and nothing encodes it.
		return UsageDecodeJSON
	case OperationSocketSend:
		// Socket.Write encodes the outbound type and nothing decodes it.
		return UsageEncodeJSON
	case OperationJSONEncode:
		return UsageEncodeJSON
	case OperationJSONDecode:
		return UsageDecodeJSON
	case OperationJSONEncoderDeclare:
		// An annotation asks for the codec and for it to be published, so one
		// operation carries both meanings rather than needing two call sites.
		return UsageEncodeJSON | UsageAppendMethod
	case OperationJSONDecoderDeclare:
		return UsageDecodeJSON | UsageDecodeMethod
	case OperationCBORWireEncoderDeclare:
		return UsageCBORWireEncode
	case OperationCBORWireDecoderDeclare:
		return UsageCBORWireDecode
	case OperationCBORWorldEncoderDeclare:
		return UsageCBORWorldEncode
	case OperationCBORWorldDecoderDeclare:
		return UsageCBORWorldDecode
	case OperationCBORWireDeltaDeclare:
		// A delta implies the codec it is diffed from, in both directions: the
		// sender encodes whole entities into the set group and the receiver
		// decodes them.
		return UsageCBORWireDelta | UsageCBORWireEncode | UsageCBORWireDecode
	case OperationCBORWorldDeltaDeclare:
		return UsageCBORWorldDelta | UsageCBORWorldEncode | UsageCBORWorldDecode
	case OperationRowsScan:
		return UsageScanRows
	case OperationItemEncode:
		return UsageEncodeItem
	case OperationItemDecode:
		return UsageDecodeItem
	case OperationItemKey:
		return UsageItemKey
	case OperationItemEncodeDecode:
		return UsageEncodeItem | UsageDecodeItem
	case OperationItemKeyDecode:
		return UsageItemKey | UsageDecodeItem
	case OperationEntityEncode:
		return UsageEncodeEntity
	case OperationEntityDecode:
		return UsageDecodeEntity
	case OperationEntityKey:
		return UsageEntityKey
	case OperationCacheKey:
		return UsageCacheKey
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
	case OperationSocketReceive:
		// A socket's inbound half is a JSON decode, so turning off decoder
		// emission turns it off here too. Gating on the socket feature alone
		// would leave "disable decode-json" false for any package the socket
		// patterns reach — which, since enabled usage is computed from the
		// configured patterns rather than from discovered calls, is every
		// package.
		return disabled[FeatureWebSocket] || disabled[FeatureDecodeJSON]
	case OperationSocketSend:
		return disabled[FeatureWebSocket] || disabled[FeatureEncodeJSON]
	case OperationJSONDecode, OperationJSONDecoderDeclare:
		return disabled[FeatureDecodeJSON]
	case OperationJSONEncode, OperationJSONEncoderDeclare:
		return disabled[FeatureEncodeJSON]
	case OperationRowsScan:
		return disabled[FeatureScanRows]
	case OperationItemEncode, OperationItemDecode, OperationItemKey,
		OperationItemEncodeDecode, OperationItemKeyDecode:
		return disabled[FeatureItemCodec]
	case OperationEntityEncode, OperationEntityDecode, OperationEntityKey:
		return disabled[FeatureEntityCodec]
	case OperationCBORWireEncoderDeclare, OperationCBORWireDecoderDeclare:
		return disabled[FeatureCBORWireCodec]
	case OperationCBORWorldEncoderDeclare, OperationCBORWorldDecoderDeclare:
		return disabled[FeatureCBORWorldCodec]
	case OperationCBORWireDeltaDeclare:
		return disabled[FeatureCBORDelta] || disabled[FeatureCBORWireCodec]
	case OperationCBORWorldDeltaDeclare:
		return disabled[FeatureCBORDelta] || disabled[FeatureCBORWorldCodec]
	case OperationCacheKey:
		return disabled[FeatureCacheKey]
	default:
		return false
	}
}

func primaryTypeSource(pattern CallPattern) TypeSource {
	roles := []string{"request", "response", "stream", "socket-in", "socket-out", "decode", "encode", "cbor-decode", "cbor-encode", "cbor-delta", "row", "item", "entity", "key", "config"}
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
	case OperationSocketReceive:
		operation, role = parser.CallSocketReceive, "socket-in"
	case OperationSocketSend:
		operation, role = parser.CallSocketSend, "socket-out"
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

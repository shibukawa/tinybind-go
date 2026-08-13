package generator

import (
	"fmt"
	"reflect"
	"sort"
)

// CallOperation identifies the generator meaning of a configured wrapper call.
type CallOperation string

const (
	OperationRequestBind         CallOperation = "request_bind"
	OperationResponseWrite       CallOperation = "response_write"
	OperationResponseWriteStatus CallOperation = "response_write_status"
	OperationStreamCreate        CallOperation = "stream_create"
	OperationSocketReceive       CallOperation = "socket_receive"
	OperationSocketSend          CallOperation = "socket_send"
	OperationJSONDecode          CallOperation = "json_decode"
	OperationJSONEncode          CallOperation = "json_encode"
	OperationRowsScan            CallOperation = "rows_scan"
	OperationItemEncode          CallOperation = "item_encode"
	OperationItemDecode          CallOperation = "item_decode"
	OperationItemKey             CallOperation = "item_key"
	// OperationItemEncodeDecode is a write that reads back what it replaced, and
	// OperationItemKeyDecode a delete that does. One call needs two generated
	// methods, and a call target carries exactly one operation, so the pair gets
	// its own operation rather than two patterns for one function.
	OperationItemEncodeDecode CallOperation = "item_encode_decode"
	OperationItemKeyDecode    CallOperation = "item_key_decode"
	// The Firestore entity operations. They are separate from the DynamoDB item
	// ones rather than shared, because the two runtimes emit different methods
	// onto the same struct and a call has to say which.
	OperationEntityEncode CallOperation = "entity_encode"
	OperationEntityDecode CallOperation = "entity_decode"
	OperationEntityKey    CallOperation = "entity_key"
	// OperationCacheKey marks the argument a cache reads a key from. It selects
	// an argument type rather than a type parameter, because the value passed is
	// the key itself and a framework's memo call is generic over the result.
	OperationCacheKey         CallOperation = "cache_key"
	OperationConfigBind       CallOperation = "config_bind"
	OperationConfigSubCommand CallOperation = "config_subcommand"
	OperationRouteRegister    CallOperation = "route_register"
	OperationErrorResponse    CallOperation = "error_response"
	// OperationTransportOnly carries transport slots and nothing else. It exists
	// for calls the transform has to recognize but discovery reads nothing from:
	// WriteError and the request accessors take a writer or a request, yet name
	// no model. Without a pattern they would look like any unrecognized call and
	// refuse every handler that makes one.
	OperationTransportOnly CallOperation = "transport_only"
)

// CallTarget identifies either a package function or a named-receiver method.
type CallTarget struct {
	Function *SymbolPattern
	Method   *MethodPattern
}

// Function identifies a package function used as a generator call target.
func Function(packagePath, name string) CallTarget {
	return CallTarget{Function: &SymbolPattern{PackagePath: packagePath, Name: name}}
}

// Method identifies a method used as a generator call target.
func Method(packagePath, name, receiverPackagePath, receiverType string) CallTarget {
	return CallTarget{Method: &MethodPattern{
		PackagePath: packagePath, Name: name,
		ReceiverPackagePath: receiverPackagePath, ReceiverType: receiverType,
	}}
}

// TypeSource selects a semantic type from a generic argument or value argument.
type TypeSource struct {
	GenericArgument *int
	ArgumentType    *int
}

// ValueSource selects a semantic value from a value argument or a fixed constant.
type ValueSource struct {
	Argument   *int
	Constant   any
	IsConstant bool
}

// TransportSlots names the argument positions holding the transport values a
// call receives: the response writer and the request.
//
// The other roles say where a semantic value is read from. These say the
// opposite — which arguments carry nothing semantic and exist only because the
// net/http shape passes both halves separately. A backend carrying both in one
// value drops exactly these positions, so a call whose slots are undeclared is
// one the transform cannot rewrite.
type TransportSlots struct {
	Writer  *int
	Request *int
}

// Declared reports whether either slot was named.
func (s TransportSlots) Declared() bool { return s.Writer != nil || s.Request != nil }

// Drops reports whether the zero-based argument index is a transport slot, and
// so is removed when the call is rewritten for a single-value transport.
func (s TransportSlots) Drops(index int) bool {
	return (s.Writer != nil && *s.Writer == index) || (s.Request != nil && *s.Request == index)
}

// CallPattern maps a framework call identity onto one generator operation.
type CallPattern struct {
	Target        CallTarget
	Operation     CallOperation
	TypeRoles     map[string]TypeSource
	ArgumentRoles map[string]ValueSource
	Transport     TransportSlots
}

// CallPatternOption adds one semantic role source to a CallPattern.
type CallPatternOption func(*CallPattern)

// GenericType reads a type role from a zero-based generic argument index.
func GenericType(role string, index int) CallPatternOption {
	return func(pattern *CallPattern) {
		if pattern.TypeRoles == nil {
			pattern.TypeRoles = map[string]TypeSource{}
		}
		value := index
		pattern.TypeRoles[role] = TypeSource{GenericArgument: &value}
	}
}

// ArgumentType reads a type role from a zero-based value argument index.
func ArgumentType(role string, index int) CallPatternOption {
	return func(pattern *CallPattern) {
		if pattern.TypeRoles == nil {
			pattern.TypeRoles = map[string]TypeSource{}
		}
		value := index
		pattern.TypeRoles[role] = TypeSource{ArgumentType: &value}
	}
}

// Argument reads a value role from a zero-based value argument index.
func Argument(role string, index int) CallPatternOption {
	return func(pattern *CallPattern) {
		if pattern.ArgumentRoles == nil {
			pattern.ArgumentRoles = map[string]ValueSource{}
		}
		value := index
		pattern.ArgumentRoles[role] = ValueSource{Argument: &value}
	}
}

// WriterArgument names the zero-based argument holding the response writer.
// Declare it on any wrapper that takes one, so a single-value transport knows
// which argument disappears.
func WriterArgument(index int) CallPatternOption {
	return func(pattern *CallPattern) {
		value := index
		pattern.Transport.Writer = &value
	}
}

// RequestArgument names the zero-based argument holding the request.
func RequestArgument(index int) CallPatternOption {
	return func(pattern *CallPattern) {
		value := index
		pattern.Transport.Request = &value
	}
}

// Constant provides a fixed semantic value hidden by a wrapper.
func Constant(role string, value any) CallPatternOption {
	return func(pattern *CallPattern) {
		if pattern.ArgumentRoles == nil {
			pattern.ArgumentRoles = map[string]ValueSource{}
		}
		pattern.ArgumentRoles[role] = ValueSource{Constant: value, IsConstant: true}
	}
}

// Call constructs a semantic wrapper call pattern.
func Call(operation CallOperation, target CallTarget, options ...CallPatternOption) CallPattern {
	pattern := CallPattern{Target: target, Operation: operation}
	for _, option := range options {
		option(&pattern)
	}
	return pattern
}

// RequestBindCall declares a request-model binding wrapper.
func RequestBindCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationRequestBind, target, options...)
}

// ResponseWriteCall declares a default-status response writer wrapper.
func ResponseWriteCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationResponseWrite, target, options...)
}

// ResponseWriteStatusCall declares a response writer wrapper with a status role.
func ResponseWriteStatusCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationResponseWriteStatus, target, options...)
}

// StreamCreateCall declares a streaming response constructor wrapper.
func StreamCreateCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationStreamCreate, target, options...)
}

// SocketReceiveCall declares the inbound half of a WebSocket entry. It is
// paired with SocketSendCall against the same target: the two type arguments
// run in opposite directions, so one pattern cannot stand for both.
func SocketReceiveCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationSocketReceive, target, options...)
}

// SocketSendCall declares the outbound half of a WebSocket entry.
func SocketSendCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationSocketSend, target, options...)
}

// JSONDecodeCall declares a standalone JSON decoder wrapper.
func JSONDecodeCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationJSONDecode, target, options...)
}

// JSONEncodeCall declares a standalone JSON encoder wrapper.
func JSONEncodeCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationJSONEncode, target, options...)
}

// RowsScanCall declares a SQL row scanner wrapper.
func RowsScanCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationRowsScan, target, options...)
}

// ItemEncodeCall declares a DynamoDB item writer wrapper.
func ItemEncodeCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationItemEncode, target, options...)
}

// ItemDecodeCall declares a DynamoDB item reader wrapper.
func ItemDecodeCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationItemDecode, target, options...)
}

// ItemKeyCall declares a wrapper that needs only a type's primary key.
func ItemKeyCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationItemKey, target, options...)
}

// ItemEncodeDecodeCall declares a wrapper that writes an item and decodes the
// item it replaced.
func ItemEncodeDecodeCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationItemEncodeDecode, target, options...)
}

// ItemKeyDecodeCall declares a wrapper that deletes by key and decodes the item
// it deleted.
func ItemKeyDecodeCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationItemKeyDecode, target, options...)
}

// EntityEncodeCall declares a Firestore entity writer wrapper.
func EntityEncodeCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationEntityEncode, target, options...)
}

// EntityDecodeCall declares a Firestore entity reader wrapper.
func EntityDecodeCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationEntityDecode, target, options...)
}

// EntityKeyCall declares a wrapper that needs only a type's key.
func EntityKeyCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationEntityKey, target, options...)
}

// CacheKeyCall declares a wrapper that reads a cache key from an argument.
//
// The key role takes ArgumentType rather than GenericType: a memo call is
// generic over the result it caches, and the key is the value beside it.
func CacheKeyCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationCacheKey, target, options...)
}

// ConfigBindCall declares a configbind registration wrapper.
func ConfigBindCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationConfigBind, target, options...)
}

// ConfigSubCommandCall declares a configbind subcommand registration wrapper.
func ConfigSubCommandCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationConfigSubCommand, target, options...)
}

// RouteRegisterCall declares an HTTP route registration wrapper.
func RouteRegisterCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationRouteRegister, target, options...)
}

// ErrorResponseCall declares an error constructor with a fixed HTTP status.
func ErrorResponseCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationErrorResponse, target, options...)
}

// TransportCall declares a call that takes a transport value and yields no
// model. Discovery ignores it; the transform needs it, so that a handler
// calling one is not refused for making an ordinary runtime call.
func TransportCall(target CallTarget, options ...CallPatternOption) CallPattern {
	return Call(OperationTransportOnly, target, options...)
}

// CallRegistry accumulates framework wrapper declarations without global state.
type CallRegistry struct {
	patterns []CallPattern
}

// NewCallRegistry creates an empty framework-local call registry.
func NewCallRegistry() *CallRegistry { return &CallRegistry{} }

// Register validates and adds call patterns.
func (registry *CallRegistry) Register(patterns ...CallPattern) error {
	if registry == nil {
		return fmt.Errorf("generator: nil call registry")
	}
	for _, pattern := range patterns {
		if err := validateCallPattern(pattern); err != nil {
			return err
		}
		key := callTargetKey(pattern.Target)
		for _, existing := range registry.patterns {
			if callTargetKey(existing.Target) != key {
				continue
			}
			if reflect.DeepEqual(existing, pattern) {
				key = ""
				break
			}
			if composeOnOneTarget(existing, pattern) {
				continue
			}
			return fmt.Errorf("generator: conflicting call patterns for %s", key)
		}
		if key != "" {
			registry.patterns = append(registry.patterns, cloneCallPattern(pattern))
		}
	}
	return nil
}

// composeOnOneTarget reports whether two patterns for one target are
// complementary rather than conflicting.
//
// Two patterns on one target are normally a framework claiming a meaning the
// runtime already claimed, and refusing that is what the guard above is for.
// The socket entry is the exception, and the only one: its type arguments run
// in opposite directions, a pattern carries a single type, and so one direction
// per pattern is the only way to say it. The pair is admitted by naming both
// operations rather than by relaxing the guard to any two operations, which
// would let the framework case back in.
func composeOnOneTarget(a, b CallPattern) bool {
	return a.Operation != b.Operation &&
		socketDirection(a.Operation) && socketDirection(b.Operation)
}

func socketDirection(operation CallOperation) bool {
	return operation == OperationSocketReceive || operation == OperationSocketSend
}

// Options returns an immutable options snapshot containing defaults and wrappers.
func (registry *CallRegistry) Options(base Options) (Options, error) {
	patterns, err := base.callPatterns()
	if err != nil {
		return Options{}, err
	}
	combined := NewCallRegistry()
	if err := combined.Register(patterns...); err != nil {
		return Options{}, err
	}
	if registry != nil {
		if err := combined.Register(registry.patterns...); err != nil {
			return Options{}, err
		}
	}
	base.Calls = PatternSet[CallPattern]{Set: make([]CallPattern, len(combined.patterns))}
	for i, pattern := range combined.patterns {
		base.Calls.Set[i] = cloneCallPattern(pattern)
	}
	base.RuntimePackages = PatternSet[string]{Disabled: true}
	if _, err := base.normalized(); err != nil {
		return Options{}, err
	}
	return base, nil
}

func validateCallPattern(pattern CallPattern) error {
	key := callTargetKey(pattern.Target)
	if key == "" {
		return fmt.Errorf("generator: call pattern requires exactly one target")
	}
	if !supportedCallOperation(pattern.Operation) {
		return fmt.Errorf("generator: call pattern %s has unsupported operation %q", key, pattern.Operation)
	}
	for role, source := range pattern.TypeRoles {
		if role == "" || (source.GenericArgument == nil) == (source.ArgumentType == nil) {
			return fmt.Errorf("generator: call pattern %s has invalid type role %q", key, role)
		}
		if source.GenericArgument != nil && *source.GenericArgument < 0 || source.ArgumentType != nil && *source.ArgumentType < 0 {
			return fmt.Errorf("generator: call pattern %s has negative type role index", key)
		}
	}
	for role, source := range pattern.ArgumentRoles {
		if role == "" || (source.Argument != nil) == source.IsConstant {
			return fmt.Errorf("generator: call pattern %s has invalid argument role %q", key, role)
		}
		if source.Argument != nil && *source.Argument < 0 {
			return fmt.Errorf("generator: call pattern %s has negative argument role index", key)
		}
		if source.IsConstant && !isScalarCallConstant(source.Constant) {
			return fmt.Errorf("generator: call pattern %s role %q requires a scalar constant", key, role)
		}
	}
	requiredType, requiredValues := requiredCallRoles(pattern.Operation)
	for _, role := range requiredType {
		if _, ok := pattern.TypeRoles[role]; !ok {
			return fmt.Errorf("generator: call pattern %s operation %s requires type role %q", key, pattern.Operation, role)
		}
	}
	for _, role := range requiredValues {
		source, ok := pattern.ArgumentRoles[role]
		if !ok {
			return fmt.Errorf("generator: call pattern %s operation %s requires argument role %q", key, pattern.Operation, role)
		}
		if source.IsConstant {
			switch role {
			case "prefix", "name", "help", "pattern":
				if _, ok := source.Constant.(string); !ok {
					return fmt.Errorf("generator: call pattern %s role %q requires a string constant", key, role)
				}
			case "status":
				if _, ok := source.Constant.(int); !ok {
					return fmt.Errorf("generator: call pattern %s role %q requires an int constant", key, role)
				}
			case "handler":
				return fmt.Errorf("generator: call pattern %s role %q must come from an argument", key, role)
			}
		}
	}
	if pattern.Operation == OperationErrorResponse && !pattern.ArgumentRoles["status"].IsConstant {
		return fmt.Errorf("generator: call pattern %s error_response status must be a fixed constant", key)
	}
	if err := validateTransportSlots(key, pattern); err != nil {
		return err
	}
	return nil
}

func validateTransportSlots(key string, pattern CallPattern) error {
	writer, request := pattern.Transport.Writer, pattern.Transport.Request
	if pattern.Operation == OperationTransportOnly && !pattern.Transport.Declared() {
		return fmt.Errorf("generator: call pattern %s is transport_only and must declare a transport slot", key)
	}
	if writer != nil && *writer < 0 || request != nil && *request < 0 {
		return fmt.Errorf("generator: call pattern %s has negative transport slot index", key)
	}
	if writer != nil && request != nil && *writer == *request {
		return fmt.Errorf("generator: call pattern %s names argument %d as both writer and request", key, *writer)
	}
	// A transport slot carries no semantic value, so an argument cannot be both
	// something to read and something to drop.
	for role, source := range pattern.ArgumentRoles {
		if source.Argument != nil && pattern.Transport.Drops(*source.Argument) {
			return fmt.Errorf("generator: call pattern %s argument %d is a transport slot and cannot also supply role %q", key, *source.Argument, role)
		}
	}
	for role, source := range pattern.TypeRoles {
		if source.ArgumentType != nil && pattern.Transport.Drops(*source.ArgumentType) {
			return fmt.Errorf("generator: call pattern %s argument %d is a transport slot and cannot also supply type role %q", key, *source.ArgumentType, role)
		}
	}
	return nil
}

func isScalarCallConstant(value any) bool {
	switch value.(type) {
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

func supportedCallOperation(operation CallOperation) bool {
	switch operation {
	case OperationRequestBind, OperationResponseWrite, OperationResponseWriteStatus,
		OperationStreamCreate, OperationSocketReceive, OperationSocketSend,
		OperationJSONDecode, OperationJSONEncode,
		OperationRowsScan, OperationConfigBind, OperationConfigSubCommand,
		OperationRouteRegister, OperationErrorResponse, OperationTransportOnly,
		OperationItemEncode, OperationItemDecode, OperationItemKey,
		OperationItemEncodeDecode, OperationItemKeyDecode,
		OperationEntityEncode, OperationEntityDecode, OperationEntityKey,
		OperationCacheKey:
		return true
	default:
		return false
	}
}

func requiredCallRoles(operation CallOperation) (types, values []string) {
	switch operation {
	case OperationRequestBind:
		return []string{"request"}, nil
	case OperationResponseWrite:
		return []string{"response"}, nil
	case OperationResponseWriteStatus:
		return []string{"response"}, []string{"status"}
	case OperationStreamCreate:
		return []string{"stream"}, nil
	case OperationSocketReceive:
		return []string{"socket-in"}, nil
	case OperationSocketSend:
		return []string{"socket-out"}, nil
	case OperationJSONDecode:
		return []string{"decode"}, nil
	case OperationJSONEncode:
		return []string{"encode"}, nil
	case OperationRowsScan:
		return []string{"row"}, nil
	case OperationItemEncode, OperationItemDecode, OperationItemKey,
		OperationItemEncodeDecode, OperationItemKeyDecode:
		return []string{"item"}, nil
	case OperationEntityEncode, OperationEntityDecode, OperationEntityKey:
		return []string{"entity"}, nil
	case OperationCacheKey:
		return []string{"key"}, nil
	case OperationConfigBind:
		return []string{"config"}, []string{"prefix"}
	case OperationConfigSubCommand:
		return []string{"config"}, []string{"name", "help"}
	case OperationRouteRegister:
		return nil, []string{"pattern", "handler"}
	case OperationErrorResponse:
		return nil, []string{"status"}
	default:
		return nil, nil
	}
}

func callTargetKey(target CallTarget) string {
	if (target.Function == nil) == (target.Method == nil) {
		return ""
	}
	if target.Function != nil {
		if target.Function.PackagePath == "" || target.Function.Name == "" {
			return ""
		}
		return target.Function.PackagePath + "." + target.Function.Name
	}
	if target.Method.PackagePath == "" || target.Method.Name == "" || target.Method.ReceiverPackagePath == "" || target.Method.ReceiverType == "" {
		return ""
	}
	return target.Method.PackagePath + ".(" + target.Method.ReceiverPackagePath + "." + target.Method.ReceiverType + ")." + target.Method.Name
}

func cloneCallPattern(pattern CallPattern) CallPattern {
	clone := pattern
	if pattern.Transport.Writer != nil {
		index := *pattern.Transport.Writer
		clone.Transport.Writer = &index
	}
	if pattern.Transport.Request != nil {
		index := *pattern.Transport.Request
		clone.Transport.Request = &index
	}
	if pattern.Target.Function != nil {
		target := *pattern.Target.Function
		clone.Target.Function = &target
	}
	if pattern.Target.Method != nil {
		target := *pattern.Target.Method
		clone.Target.Method = &target
	}
	clone.TypeRoles = make(map[string]TypeSource, len(pattern.TypeRoles))
	for role, source := range pattern.TypeRoles {
		copy := source
		if source.GenericArgument != nil {
			index := *source.GenericArgument
			copy.GenericArgument = &index
		}
		if source.ArgumentType != nil {
			index := *source.ArgumentType
			copy.ArgumentType = &index
		}
		clone.TypeRoles[role] = copy
	}
	clone.ArgumentRoles = make(map[string]ValueSource, len(pattern.ArgumentRoles))
	for role, source := range pattern.ArgumentRoles {
		copy := source
		if source.Argument != nil {
			index := *source.Argument
			copy.Argument = &index
		}
		clone.ArgumentRoles[role] = copy
	}
	return clone
}

func sortCallPatterns(patterns []CallPattern) {
	sort.Slice(patterns, func(i, j int) bool { return callTargetKey(patterns[i].Target) < callTargetKey(patterns[j].Target) })
}

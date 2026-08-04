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
	OperationEntityEncode     CallOperation = "entity_encode"
	OperationEntityDecode     CallOperation = "entity_decode"
	OperationEntityKey        CallOperation = "entity_key"
	OperationConfigBind       CallOperation = "config_bind"
	OperationConfigSubCommand CallOperation = "config_subcommand"
	OperationRouteRegister    CallOperation = "route_register"
	OperationErrorResponse    CallOperation = "error_response"
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

// CallPattern maps a framework call identity onto one generator operation.
type CallPattern struct {
	Target        CallTarget
	Operation     CallOperation
	TypeRoles     map[string]TypeSource
	ArgumentRoles map[string]ValueSource
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
			return fmt.Errorf("generator: conflicting call patterns for %s", key)
		}
		if key != "" {
			registry.patterns = append(registry.patterns, cloneCallPattern(pattern))
		}
	}
	return nil
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
		OperationStreamCreate, OperationJSONDecode, OperationJSONEncode,
		OperationRowsScan, OperationConfigBind, OperationConfigSubCommand,
		OperationRouteRegister, OperationErrorResponse,
		OperationItemEncode, OperationItemDecode, OperationItemKey,
		OperationItemEncodeDecode, OperationItemKeyDecode,
		OperationEntityEncode, OperationEntityDecode, OperationEntityKey:
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

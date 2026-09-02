package htmlbind

import (
	"context"
	"testing"
)

// The three entries below carry no type parameter beyond the receiver's own, so
// the method form was always available in today's Go. Each test pins that the
// deprecated function and the method are the same operation, which is what lets
// a generated plan move when it is convenient.

type methodFormParams struct {
	Name string
}

func TestRequireFunctionAndMethodAgree(t *testing.T) {
	check := func(p methodFormParams) error { return nil }
	fromFunc := Require(check)
	fromMethod := Builder[methodFormParams]{}.Require(check)
	if _, ok := fromFunc.(requireOp[methodFormParams]); !ok {
		t.Fatalf("Require function returned %T", fromFunc)
	}
	if _, ok := fromMethod.(requireOp[methodFormParams]); !ok {
		t.Fatalf("Require method returned %T", fromMethod)
	}
}

func TestBindFunctionAndMethodAgree(t *testing.T) {
	plan := &Plan[methodFormParams]{Head: []string{"<meta>"}}
	params := methodFormParams{Name: "x"}
	fromFunc := Bind(plan, params)
	fromMethod := plan.Bind(params)
	if len(fromFunc.head) != len(fromMethod.head) {
		t.Fatalf("head differs: %d vs %d", len(fromFunc.head), len(fromMethod.head))
	}
	if !fromFunc.Present() || !fromMethod.Present() {
		t.Fatal("both forms must produce a present fragment")
	}
}

func TestBindWrapperFunctionAndMethodAgree(t *testing.T) {
	plan := &Plan[methodFormParams]{Head: []string{"<meta>"}}
	params := methodFormParams{Name: "x"}
	set := func(p *methodFormParams, children Fragment) {}
	fromFunc := BindWrapper(plan, params, set)
	fromMethod := plan.BindWrapper(params, set)
	if len(fromFunc.head) != len(fromMethod.head) {
		t.Fatalf("head differs: %d vs %d", len(fromFunc.head), len(fromMethod.head))
	}
	if fromFunc.render == nil || fromMethod.render == nil {
		t.Fatal("both forms must install a render function")
	}
}

// The five entries below carry a type parameter of their own beyond the
// receiver's, which no Go before 1.27 let a method declare. Each test pins that
// the deprecated function and the method build the same op.

type methodFormItem struct{ Name string }

type methodFormScope struct {
	Item  methodFormItem
	Index int
}

func TestForFunctionAndMethodAgree(t *testing.T) {
	items := func(p methodFormParams) []methodFormItem { return nil }
	scope := func(p methodFormParams, e methodFormItem, i int) methodFormScope {
		return methodFormScope{Item: e, Index: i}
	}
	fromFunc := For(items, scope, nil)
	fromMethod := Builder[methodFormParams]{}.For(items, scope, nil)
	if _, ok := fromFunc.(forOp[methodFormParams, methodFormItem, methodFormScope]); !ok {
		t.Fatalf("For function returned %T", fromFunc)
	}
	if _, ok := fromMethod.(forOp[methodFormParams, methodFormItem, methodFormScope]); !ok {
		t.Fatalf("For method returned %T", fromMethod)
	}
}

func TestForCtxFunctionAndMethodAgree(t *testing.T) {
	items := func(context.Context, methodFormParams) []methodFormItem { return nil }
	scope := func(p methodFormParams, e methodFormItem, i int) methodFormScope {
		return methodFormScope{Item: e, Index: i}
	}
	fromFunc := ForCtx(items, scope, nil)
	fromMethod := Builder[methodFormParams]{}.ForCtx(items, scope, nil)
	if _, ok := fromFunc.(forCtxOp[methodFormParams, methodFormItem, methodFormScope]); !ok {
		t.Fatalf("ForCtx function returned %T", fromFunc)
	}
	if _, ok := fromMethod.(forCtxOp[methodFormParams, methodFormItem, methodFormScope]); !ok {
		t.Fatalf("ForCtx method returned %T", fromMethod)
	}
}

func TestAwaitFunctionAndMethodAgree(t *testing.T) {
	resolve := func(context.Context, methodFormParams) (methodFormScope, error) {
		return methodFormScope{}, nil
	}
	recovery := func(methodFormParams, AsyncError) methodFormItem { return methodFormItem{} }
	fromFunc := Await(resolve, recovery, nil, nil, nil)
	fromMethod := Builder[methodFormParams]{}.Await(resolve, recovery, nil, nil, nil)
	if _, ok := fromFunc.(awaitOp[methodFormParams, methodFormScope, methodFormItem]); !ok {
		t.Fatalf("Await function returned %T", fromFunc)
	}
	if _, ok := fromMethod.(awaitOp[methodFormParams, methodFormScope, methodFormItem]); !ok {
		t.Fatalf("Await method returned %T", fromMethod)
	}
}

func TestLiveFunctionAndMethodAgree(t *testing.T) {
	bindings := func(context.Context, methodFormParams) []LiveBinding[methodFormScope] { return nil }
	scope := func(methodFormParams) methodFormScope { return methodFormScope{} }
	recovery := func(methodFormParams, AsyncError) methodFormItem { return methodFormItem{} }
	fromFunc := Live(bindings, scope, recovery, nil, nil, nil)
	fromMethod := Builder[methodFormParams]{}.Live(bindings, scope, recovery, nil, nil, nil)
	if _, ok := fromFunc.(liveOp[methodFormParams, methodFormScope, methodFormItem]); !ok {
		t.Fatalf("Live function returned %T", fromFunc)
	}
	if _, ok := fromMethod.(liveOp[methodFormParams, methodFormScope, methodFormItem]); !ok {
		t.Fatalf("Live method returned %T", fromMethod)
	}
}

func TestProvideFunctionAndMethodAgree(t *testing.T) {
	fn := func(context.Context) (methodFormItem, error) { return methodFormItem{}, nil }
	fromFunc := Provide("meta", "token", fn, []Segment[methodFormParams, methodFormItem]{})
	fromMethod := Builder[methodFormParams]{}.Provide("meta", "token", fn, nil)
	if _, ok := fromFunc.(provideOp[methodFormParams, methodFormItem]); !ok {
		t.Fatalf("Provide function returned %T", fromFunc)
	}
	if _, ok := fromMethod.(provideOp[methodFormParams, methodFormItem]); !ok {
		t.Fatalf("Provide method returned %T", fromMethod)
	}
}

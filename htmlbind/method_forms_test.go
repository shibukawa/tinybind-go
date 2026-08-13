package htmlbind

import "testing"

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

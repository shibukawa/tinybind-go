package sqlbind

import (
	"errors"
	"testing"
)

// AppendValues carries the element type as a type parameter of its own, which
// no Go before 1.27 let a method declare. The test pins that the deprecated
// function and the method write the same placeholders, bind the same
// arguments, and refuse an empty list the same way.
func TestAppendValuesFunctionAndMethodAgree(t *testing.T) {
	viaFunc := NewBuilder(Dollar)
	viaMethod := NewBuilder(Dollar)
	viaFunc.WriteString("IN (")
	viaMethod.WriteString("IN (")
	if err := AppendValues(&viaFunc, []int{1, 2}); err != nil {
		t.Fatalf("AppendValues function: %v", err)
	}
	if err := viaMethod.AppendValues([]int{1, 2}); err != nil {
		t.Fatalf("AppendValues method: %v", err)
	}
	fromFunc, fromMethod := viaFunc.Statement(), viaMethod.Statement()
	if fromFunc.SQL != "IN ($1, $2" || fromMethod.SQL != fromFunc.SQL {
		t.Fatalf("SQL differs: %q and %q", fromFunc.SQL, fromMethod.SQL)
	}
	if len(fromFunc.Args) != 2 || len(fromMethod.Args) != 2 {
		t.Fatalf("argument counts differ: %d and %d", len(fromFunc.Args), len(fromMethod.Args))
	}
	for i := range fromFunc.Args {
		if fromFunc.Args[i] != fromMethod.Args[i] {
			t.Fatalf("argument %d differs: %v and %v", i, fromFunc.Args[i], fromMethod.Args[i])
		}
	}

	var empty Builder
	if err := AppendValues(&empty, []int{}); !errors.Is(err, ErrEmptyValueList) {
		t.Fatalf("function on an empty list: %v", err)
	}
	if err := empty.AppendValues([]int{}); !errors.Is(err, ErrEmptyValueList) {
		t.Fatalf("method on an empty list: %v", err)
	}
}

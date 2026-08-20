package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestAnalyze_NestedKinds(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

type NestedCustomer struct {
	ID   string ` + "`payload:\"id\"`" + `
	Name string ` + "`payload:\"name\"`" + `
}

type NestedLineItem struct {
	SKU string ` + "`payload:\"sku\"`" + `
	Qty int    ` + "`payload:\"qty\"`" + `
}

type NestedOrderRequest struct {
	Customer NestedCustomer    ` + "`payload:\"customer\"`" + `
	Items    []NestedLineItem  ` + "`payload:\"items\"`" + `
	Labels   map[string]string ` + "`payload:\"labels\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	var order *generator.TypePlan
	for i := range plan.Types {
		if plan.Types[i].Name == "NestedOrderRequest" {
			order = &plan.Types[i]
			break
		}
	}
	if order == nil {
		t.Fatalf("NestedOrderRequest not planned: %+v", plan.Types)
	}
	want := map[string]struct {
		kind, typeName, elem string
	}{
		"Customer": {generator.KindStruct, "NestedCustomer", ""},
		"Items":    {generator.KindSlice, "NestedLineItem", generator.KindStruct},
		"Labels":   {generator.KindMap, "", "string"},
	}
	for _, f := range order.Fields {
		w, ok := want[f.Name]
		if !ok {
			continue
		}
		if f.Kind != w.kind || f.TypeName != w.typeName || f.ElemKind != w.elem {
			t.Fatalf("field %s: kind=%q type=%q elem=%q want %+v", f.Name, f.Kind, f.TypeName, f.ElemKind, w)
		}
		delete(want, f.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing fields: %+v", want)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatalf("Emit: %v\n", err)
	}
	s := string(code)
	for _, n := range []string{
		"decodeNestedOrderRequestJSON",
		"decodeNestedCustomerJSON",
		"decodeNestedLineItemJSON",
		"RegisterDecode[NestedOrderRequest]",
		"RegisterEncode[NestedOrderRequest]",
		"jsonbind.ParseSlice(p, ",
		"jsonbind.ParseMap(p, ",
	} {
		if !strings.Contains(s, n) {
			t.Fatalf("missing %q in emit", n)
		}
	}
}

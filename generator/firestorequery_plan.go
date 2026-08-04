package generator

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// FirestoreQueryPlan is one checked declaration, ready to emit. Every name in it
// has been matched against the bound type's tags, so the emitter looks nothing
// up.
type FirestoreQueryPlan struct {
	Decl FirestoreQueryDecl
	// Entity is the type the query decodes into, and whose Kind the query runs
	// against.
	Entity FirestoreEntityPlan
	// Filters pairs each predicate with the field it names and the parameter
	// that fills it, in the order the source wrote them.
	Filters []FirestoreQueryFilter
	// Where is the checked filter tree, or nil when there is no where clause.
	// Filters is its leaves; the tree is what the emitter walks when the
	// declaration uses or.
	Where *FirestoreCondition
	// HasOr reports whether the tree needs datastore.Where at all. Without it
	// the emitter keeps to the per-predicate Filter calls it always wrote.
	HasOr bool
	// Orders are the checked sort keys.
	Orders []FirestoreQueryOrder
	// Ancestor is the parameter holding the ancestor key, or "".
	Ancestor string
	// Select and Distinct are the checked property names, resolved to what the
	// tags call them.
	Select   []string
	Distinct []string
	// ProjectsAnArray reports whether any projected property is a slice, which
	// makes the service return one result per element rather than one per
	// entity. The godoc says so; nothing here can prevent it.
	ProjectsAnArray bool
	// Start and End are the parameters holding the cursors.
	Start string
	End   string
}

// FirestoreQueryFilter is one checked predicate.
type FirestoreQueryFilter struct {
	Predicate FirestorePredicate
	Field     FirestoreFieldPlan
	Param     FirestoreQueryParam
}

// FirestoreQueryOrder is one checked sort key.
type FirestoreQueryOrder struct {
	Order FirestoreOrder
	Field FirestoreFieldPlan
}

// planFirestoreQueries checks every declaration against the entity plans of the
// package and builds what the emitter needs.
func planFirestoreQueries(decls []FirestoreQueryDecl, entities []FirestoreEntityPlan) ([]FirestoreQueryPlan, error) {
	index := make(map[string]FirestoreEntityPlan, len(entities))
	for _, entity := range entities {
		index[entity.Name] = entity
	}
	seen := map[string]string{}
	out := make([]FirestoreQueryPlan, 0, len(decls))
	for _, decl := range decls {
		if first, duplicate := seen[decl.Name]; duplicate {
			return nil, fmt.Errorf("%s:%d: statement %s is already declared in %s", decl.SourcePath, decl.Line, decl.Name, first)
		}
		seen[decl.Name] = decl.SourcePath
		plan, err := planFirestoreQuery(decl, index)
		if err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func planFirestoreQuery(decl FirestoreQueryDecl, entities map[string]FirestoreEntityPlan) (FirestoreQueryPlan, error) {
	fail := func(line int, format string, args ...any) (FirestoreQueryPlan, error) {
		return FirestoreQueryPlan{}, fmt.Errorf("%s:%d: statement %s: %s",
			decl.SourcePath, line, decl.Name, fmt.Sprintf(format, args...))
	}

	entity, ok := entities[decl.EntityType]
	if !ok {
		return fail(decl.Line, "no type %s in this package carries %s tags", decl.EntityType, firestoreTag)
	}

	params := map[string]FirestoreQueryParam{}
	for _, param := range decl.Params {
		if _, duplicate := params[param.Name]; duplicate {
			return fail(param.Line, "parameter %s is declared twice", param.Name)
		}
		params[param.Name] = param
	}

	plan := FirestoreQueryPlan{Decl: decl, Entity: entity, Ancestor: decl.Ancestor}
	used := map[string]bool{}

	if decl.Where != nil {
		plan.Where, plan.HasOr = decl.Where, decl.Where.HasOr()
		// The checks are the same whatever the tree shape; only the walk is new.
		// A leaf is checked where it sits, so an error still names the line the
		// author wrote the comparison on.
		var failure error
		_ = decl.Where.Walk(func(predicate *FirestorePredicate) error {
			field, err := firestoreQueryField(entity, predicate.Property)
			if err != nil {
				failure = fmt.Errorf("%s:%d: statement %s: %s", decl.SourcePath, predicate.Line, decl.Name, err)
				return err
			}
			param, ok := params[predicate.Param]
			if !ok {
				failure = fmt.Errorf("%s:%d: statement %s: no parameter named %s is declared",
					decl.SourcePath, predicate.Line, decl.Name, predicate.Param)
				return errStopWalk
			}
			if err := checkFirestoreParamType(param, field, predicate.Op); err != nil {
				failure = fmt.Errorf("%s:%d: statement %s: %s", decl.SourcePath, param.Line, decl.Name, err)
				return err
			}
			used[param.Name] = true
			plan.Filters = append(plan.Filters, FirestoreQueryFilter{Predicate: *predicate, Field: field, Param: param})
			return nil
		})
		if failure != nil {
			return FirestoreQueryPlan{}, failure
		}
	}

	if decl.Ancestor != "" {
		param, ok := params[decl.Ancestor]
		if !ok {
			return fail(decl.AncestorLine, "no parameter named %s is declared", decl.Ancestor)
		}
		if param.Type != "datastore.Key" {
			return fail(param.Line, "an ancestor is a key path, so parameter %s must be datastore.Key, not %s", param.Name, param.Type)
		}
		used[param.Name] = true
	}

	for _, order := range decl.Order {
		field, err := firestoreQueryField(entity, order.Property)
		if err != nil {
			return fail(order.Line, "%s", err)
		}
		plan.Orders = append(plan.Orders, FirestoreQueryOrder{Order: order, Field: field})
	}

	for _, bound := range []struct {
		name  string
		value FirestoreBound
	}{{"limit", decl.Limit}, {"offset", decl.Offset}} {
		if !bound.value.Present || bound.value.Param == "" {
			continue
		}
		param, ok := params[bound.value.Param]
		if !ok {
			return fail(bound.value.Line, "no parameter named %s is declared", bound.value.Param)
		}
		if !isFirestoreIntegerTypeName(param.Type) {
			return fail(param.Line, "a %s is a count, so parameter %s must be an integer, not %s", bound.name, param.Name, param.Type)
		}
		used[param.Name] = true
	}

	for _, projection := range decl.Select {
		field, err := firestoreQueryField(entity, projection.Name)
		if err != nil {
			return fail(projection.Line, "%s", err)
		}
		if field.Type.Kind == FirestoreArray {
			plan.ProjectsAnArray = true
		}
		plan.Select = append(plan.Select, field.Property)
	}
	for _, projection := range decl.Distinct {
		field, err := firestoreQueryField(entity, projection.Name)
		if err != nil {
			return fail(projection.Line, "%s", err)
		}
		plan.Distinct = append(plan.Distinct, field.Property)
	}
	// Datastore requires the distinct-on properties to lead the ordering. Both
	// clauses are right here, so this is a structural check rather than a guess
	// about the service.
	if len(plan.Distinct) > 0 && len(plan.Orders) > 0 {
		if len(plan.Orders) < len(plan.Distinct) {
			return fail(decl.DistinctLine,
				"a distinct clause names %d properties but the order clause has only %d; the distinct properties have to lead the ordering",
				len(plan.Distinct), len(plan.Orders))
		}
		for i, property := range plan.Distinct {
			if plan.Orders[i].Field.Property != property {
				return fail(decl.DistinctLine,
					"distinct property %s is %s in the ordering, and the distinct properties have to lead it in the same order",
					property, ordinalPosition(i, plan.Orders))
			}
		}
	}

	for _, cursor := range []struct {
		name  string
		param string
		line  int
	}{{"start", decl.Start, decl.StartLine}, {"end", decl.End, decl.EndLine}} {
		if cursor.param == "" {
			continue
		}
		param, ok := params[cursor.param]
		if !ok {
			return fail(cursor.line, "no parameter named %s is declared", cursor.param)
		}
		if param.Type != "datastore.Cursor" {
			return fail(param.Line, "a %s is an opaque position, so parameter %s must be datastore.Cursor, not %s",
				cursor.name, param.Name, param.Type)
		}
		used[param.Name] = true
	}
	plan.Start, plan.End = decl.Start, decl.End

	// An index clause names properties, so the same rename check applies to it.
	for _, property := range decl.Index {
		if _, err := firestoreQueryField(entity, property.Name); err != nil {
			return fail(decl.IndexLine, "%s", err)
		}
	}

	var unused []string
	for _, param := range decl.Params {
		if !used[param.Name] {
			unused = append(unused, param.Name)
		}
	}
	if len(unused) > 0 {
		sort.Strings(unused)
		return fail(decl.Line, "parameter %s is declared but never used", strings.Join(unused, ", "))
	}
	return plan, nil
}

// errStopWalk ends a Walk once the failure it found has been recorded. The
// value never reaches a caller; Walk stops on any error and the recorded one is
// what gets returned.
var errStopWalk = errors.New("firestorebind: stop")

// firestoreQueryField resolves the property a clause names.
//
// Unlike a DynamoDB key condition, a Datastore filter reaches any property,
// because single-property indexes are automatic. What it cannot reach is a
// property that is not stored: an identity field lives on the key, and an
// unindexed one is in no index at all, so a filter naming either can never
// match.
func firestoreQueryField(entity FirestoreEntityPlan, property string) (FirestoreFieldPlan, error) {
	for _, f := range entity.Properties() {
		if f.Property == property {
			if f.NoIndex {
				return FirestoreFieldPlan{}, fmt.Errorf(
					"%s is tagged noindex on %s, so it is in no index and a query naming it can never match",
					property, entity.Name)
			}
			return f, nil
		}
	}
	for _, f := range entity.Fields {
		if f.Property != property {
			continue
		}
		switch f.Role {
		case "name", "id":
			return FirestoreFieldPlan{}, fmt.Errorf(
				"%s carries the key of %s rather than a property, so it is not stored; filter on the key or on an ancestor instead",
				property, entity.Name)
		case "parent":
			return FirestoreFieldPlan{}, fmt.Errorf(
				"%s is the ancestor of %s; write an ancestor clause rather than a filter", property, entity.Name)
		case "version":
			return FirestoreFieldPlan{}, fmt.Errorf(
				"%s is the entity version of %s, which the server assigns and stores outside the properties", property, entity.Name)
		}
	}
	return FirestoreFieldPlan{}, fmt.Errorf("%s has no property %q", entity.Name, property)
}

// checkFirestoreParamType matches the declared Go type against the property's
// own, so a rename on either side is a generation error rather than a filter
// that quietly matches nothing.
func checkFirestoreParamType(param FirestoreQueryParam, field FirestoreFieldPlan, op FirestoreOp) error {
	want := field.Type.Go
	if op.Multi() {
		// in and not in take the candidates, so the parameter is a slice of what
		// the property holds.
		want = "[]" + want
	}
	if param.Type != want {
		if op.Multi() {
			return fmt.Errorf("parameter %s is %s, but %s takes a slice of what property %s stores, which is %s",
				param.Name, param.Type, op, field.Property, want)
		}
		return fmt.Errorf("parameter %s is %s, but property %s is stored from %s",
			param.Name, param.Type, field.Property, want)
	}
	return nil
}

// ordinalPosition names where a property actually sits in the ordering, so the
// message points at the fix rather than only at the rule.
func ordinalPosition(want int, orders []FirestoreQueryOrder) string {
	for i, order := range orders {
		if i == want {
			return fmt.Sprintf("behind %s", order.Field.Property)
		}
	}
	return "absent from"
}

func isFirestoreIntegerTypeName(name string) bool {
	switch name {
	case "int", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32":
		return true
	}
	return false
}

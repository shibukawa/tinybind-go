package generator

import (
	"fmt"
	"sort"
	"strings"
)

// DynamoQueryPlan is one checked declaration, ready to emit. Every name in it
// has been matched against the bound type's tags, so the emitter looks nothing
// up.
type DynamoQueryPlan struct {
	Decl DynamoQueryDecl
	// Item is the type the query decodes into.
	Item DynamoItemPlan
	// Expression is the KeyConditionExpression, written with aliases.
	Expression string
	// Names maps each alias to the attribute it stands for.
	Names map[string]string
	// Values pairs each ":v" placeholder with the parameter and attribute that
	// fill it, in emission order.
	Values []DynamoQueryValue
}

// DynamoQueryValue is one bound placeholder.
type DynamoQueryValue struct {
	Placeholder string
	Param       DynamoQueryParam
	Attribute   DynamoFieldPlan
}

// planDynamoQueries checks every declaration against the item plans of the
// package and builds what the emitter needs.
func planDynamoQueries(decls []DynamoQueryDecl, items []DynamoItemPlan) ([]DynamoQueryPlan, error) {
	index := make(map[string]DynamoItemPlan, len(items))
	for _, item := range items {
		index[item.Name] = item
	}
	seen := map[string]string{}
	out := make([]DynamoQueryPlan, 0, len(decls))
	for _, decl := range decls {
		if first, duplicate := seen[decl.Name]; duplicate {
			return nil, fmt.Errorf("%s:%d: statement %s is already declared in %s", decl.SourcePath, decl.Line, decl.Name, first)
		}
		seen[decl.Name] = decl.SourcePath
		plan, err := planDynamoQuery(decl, index)
		if err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func planDynamoQuery(decl DynamoQueryDecl, items map[string]DynamoItemPlan) (DynamoQueryPlan, error) {
	fail := func(line int, format string, args ...any) (DynamoQueryPlan, error) {
		return DynamoQueryPlan{}, fmt.Errorf("%s:%d: statement %s: %s",
			decl.SourcePath, line, decl.Name, fmt.Sprintf(format, args...))
	}

	item, ok := items[decl.ItemType]
	if !ok {
		return fail(decl.Line, "no type %s in this package carries %s tags", decl.ItemType, dynamoTag)
	}
	partition, hasPartition := item.PartitionKey()
	if !hasPartition {
		return fail(decl.Line, "%s declares no partitionkey, so it cannot be queried", item.Name)
	}
	sortKey, hasSort := item.SortKey()

	params := map[string]DynamoQueryParam{}
	for _, param := range decl.Params {
		if _, duplicate := params[param.Name]; duplicate {
			return fail(param.Line, "parameter %s is declared twice", param.Name)
		}
		params[param.Name] = param
	}

	plan := DynamoQueryPlan{Decl: decl, Item: item, Names: map[string]string{}}
	aliases := map[string]string{}
	var expression []string
	used := map[string]bool{}

	for i, predicate := range decl.Key {
		field, err := dynamoKeyField(item, predicate, partition, sortKey, hasSort, i)
		if err != nil {
			return fail(predicate.Line, "%s", err)
		}
		if err := dynamoCheckOp(predicate, field, i); err != nil {
			return fail(predicate.Line, "%s", err)
		}

		alias, seen := aliases[field.Attribute]
		if !seen {
			alias = fmt.Sprintf("#k%d", len(aliases))
			aliases[field.Attribute] = alias
			plan.Names[alias] = field.Attribute
		}

		bound := make([]string, 0, len(predicate.Params))
		for _, name := range predicate.Params {
			param, ok := params[name]
			if !ok {
				return fail(predicate.Line, "no parameter named %s is declared", name)
			}
			if param.Type != field.Type.Go {
				return fail(param.Line, "parameter %s is %s, but attribute %s is stored from %s",
					param.Name, param.Type, field.Attribute, field.Type.Go)
			}
			used[name] = true
			placeholder := fmt.Sprintf(":v%d", len(plan.Values))
			plan.Values = append(plan.Values, DynamoQueryValue{Placeholder: placeholder, Param: param, Attribute: field})
			bound = append(bound, placeholder)
		}
		expression = append(expression, dynamoPredicateText(alias, predicate.Op, bound))
	}

	var unused []string
	for _, param := range decl.Params {
		if !used[param.Name] {
			unused = append(unused, param.Name)
		}
	}
	if len(unused) > 0 {
		sort.Strings(unused)
		return fail(decl.Line, "parameter %s is declared but never used in the key condition", strings.Join(unused, ", "))
	}

	plan.Expression = strings.Join(expression, " AND ")
	return plan, nil
}

// dynamoKeyField resolves the attribute a predicate names, and rejects anything
// that is not a key of the target. A key condition reaches the partition key
// and the sort key and nothing else.
func dynamoKeyField(item DynamoItemPlan, predicate DynamoPredicate, partition, sortKey DynamoFieldPlan, hasSort bool, position int) (DynamoFieldPlan, error) {
	switch predicate.Attribute {
	case partition.Attribute:
		if position != 0 {
			return DynamoFieldPlan{}, fmt.Errorf("the partition key %s must be the first predicate", predicate.Attribute)
		}
		return partition, nil
	case sortKey.Attribute:
		if !hasSort {
			break
		}
		if position == 0 {
			return DynamoFieldPlan{}, fmt.Errorf("a key condition starts with the partition key %s, not the sort key %s", partition.Attribute, predicate.Attribute)
		}
		if position > 1 {
			return DynamoFieldPlan{}, fmt.Errorf("a key condition takes at most one sort key predicate")
		}
		return sortKey, nil
	}
	if field, ok := dynamoAttributeField(item, predicate.Attribute); ok {
		return DynamoFieldPlan{}, fmt.Errorf("%s is not a key of %s; a key condition reaches %s, and a non-key attribute belongs in a filter",
			field.Attribute, item.Name, dynamoKeyNames(partition, sortKey, hasSort))
	}
	return DynamoFieldPlan{}, fmt.Errorf("%s has no attribute %q", item.Name, predicate.Attribute)
}

func dynamoAttributeField(item DynamoItemPlan, attribute string) (DynamoFieldPlan, bool) {
	for _, f := range item.Fields {
		if f.Attribute == attribute {
			return f, true
		}
	}
	return DynamoFieldPlan{}, false
}

func dynamoKeyNames(partition, sortKey DynamoFieldPlan, hasSort bool) string {
	if hasSort {
		return fmt.Sprintf("%s and %s", partition.Attribute, sortKey.Attribute)
	}
	return partition.Attribute
}

// dynamoCheckOp enforces what DynamoDB allows of each operand: the partition
// key takes equality only, and begins_with reads a string.
func dynamoCheckOp(predicate DynamoPredicate, field DynamoFieldPlan, position int) error {
	if position == 0 && predicate.Op != DynamoEqual {
		return fmt.Errorf("the partition key %s takes %q only, not %q", field.Attribute, "=", predicate.Op)
	}
	if predicate.Op == DynamoBeginsWith {
		switch field.Type.Kind {
		case DynamoString, DynamoTime:
			return nil
		default:
			return fmt.Errorf("begins_with reads a string attribute, and %s is stored as %s", field.Attribute, dynamoAttributeLetter(field.Type))
		}
	}
	return nil
}

func dynamoAttributeLetter(t DynamoType) string {
	if letter, ok := dynamoKeyAttributeType(t); ok {
		switch letter {
		case "TypeString":
			return "S"
		case "TypeNumber":
			return "N"
		case "TypeBinary":
			return "B"
		}
	}
	return string(t.Kind)
}

// dynamoPredicateText writes one predicate with its alias and placeholders.
func dynamoPredicateText(alias string, op DynamoOp, params []string) string {
	switch op {
	case DynamoBetween:
		return fmt.Sprintf("%s BETWEEN %s AND %s", alias, params[0], params[1])
	case DynamoBeginsWith:
		return fmt.Sprintf("begins_with(%s, %s)", alias, params[0])
	default:
		return fmt.Sprintf("%s %s %s", alias, op, params[0])
	}
}

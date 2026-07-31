// Package dynamobind provides typed, reflection-free DynamoDB item binding on
// top of github.com/shibukawa/tinygodriver/nosql/dynamodb.
//
// A struct declares its attributes once with dynamo tags, tinybind-gen emits the
// codec, and the call site never handles a map[string]dynamodb.AttributeValue:
//
//	type Reading struct {
//		Sensor  string  `dynamo:"sensor,partitionkey"`
//		At      int64   `dynamo:"at,sortkey"`
//		Celsius float64 `dynamo:"celsius"`
//	}
//
//	got, err := dynamobind.Load[Reading](ctx, client, "readings", want.ItemKey())
//
// Dispatch is by type constraint rather than by a registry, so a type without
// generated code fails to compile instead of failing at run time on a missing
// registration. Nothing here reflects on application fields.
//
// # What this package does not do
//
// It adds no retry loop: the driver already retries with backoff, and a second
// loop would multiply the delivery count silently. It hides no page boundary:
// Query and Scan iterate, but QueryPage stays public and returns
// LastEvaluatedKey. It swallows no error: every driver sentinel survives
// errors.Is and *dynamodb.Error survives errors.As through every helper here.
package dynamobind

import (
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// ItemEncoder converts a value into a DynamoDB item. Generated code implements
// it on the value receiver.
type ItemEncoder interface {
	EncodeItem() dynamodb.Item
}

// ItemDecoder fills a value from a DynamoDB item. Generated code implements it
// on the pointer receiver.
type ItemDecoder interface {
	DecodeItem(item dynamodb.Item) error
}

// Keyer reports the primary key of a value. Generated code implements it when
// the type carries a partitionkey tag.
type Keyer interface {
	ItemKey() dynamodb.Key
}

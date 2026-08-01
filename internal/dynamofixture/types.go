// Package dynamofixture exercises the generated DynamoDB item codec against
// the dynamobind runtime and the driver's wire protocol.
//
// The calls below are what generation is directed by: each one names the type
// it binds, and the generator emits exactly the methods those calls need.
package dynamofixture

import (
	"context"
	"iter"
	"time"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// Sensor is a named string, so the codec has to convert rather than assign.
type Sensor string

// Site is a nested item, stored as an M attribute.
type Site struct {
	City string `dynamo:"city"`
	Zip  string `dynamo:"zip,omitempty"`
}

// Reading covers every attribute form the generator supports.
type Reading struct {
	Sensor Sensor    `dynamo:"sensor,partitionkey"`
	At     int64     `dynamo:"at,sortkey"`
	Note   string    `dynamo:"note"`
	Scale  float32   `dynamo:"scale"`
	Count  uint16    `dynamo:"count"`
	Active bool      `dynamo:"active"`
	Blob   []byte    `dynamo:"blob"`
	Taken  time.Time `dynamo:"taken"`
	Seen   time.Time `dynamo:"seen,unixtime"`

	Tags   []string          `dynamo:"tags,stringset,omitempty"`
	Counts []int             `dynamo:"counts,numberset,omitempty"`
	Chunks [][]byte          `dynamo:"chunks,binaryset,omitempty"`
	Words  []Sensor          `dynamo:"words"`
	Scores map[string]int32  `dynamo:"scores"`
	Sites  map[string]string `dynamo:"sites,omitempty"`

	Site    Site  `dynamo:"site"`
	Backup  *Site `dynamo:"backup"`
	Skipped int   `dynamo:"-"`

	// Exact is the escape hatch: a number with more significant digits than any
	// Go type carries stays text from end to end through it.
	Exact dynamodb.AttributeValue `dynamo:"exact"`
}

// Fetch reads one reading by key.
func Fetch(ctx context.Context, table string, key dynamodb.Key) (Reading, error) {
	return dynamobind.Load[Reading](ctx, table, key)
}

// Save stores one reading, replacing whatever shared its key.
func Save(ctx context.Context, table string, r Reading) error {
	return dynamobind.Store(ctx, table, r)
}

// Replace stores one reading and returns the one it replaced, if any.
func Replace(ctx context.Context, table string, r Reading) (Reading, bool, error) {
	return dynamobind.StoreReturning(ctx, table, r)
}

// Delete removes the reading sharing r's key.
func Delete(ctx context.Context, table string, r Reading) error {
	return dynamobind.Remove(ctx, table, r)
}

// Retire deletes the reading sharing r's key and returns what was deleted.
func Retire(ctx context.Context, table string, r Reading) (Reading, bool, error) {
	return dynamobind.RemoveReturning(ctx, table, r)
}

// Correct applies an update expression to the reading sharing r's key.
func Correct(ctx context.Context, table string, r Reading, update string, opts ...dynamodb.WriteOption) error {
	return dynamobind.Update(ctx, table, r, update, opts...)
}

// Page reads one page of a sensor's readings.
func Page(ctx context.Context, table, keyCond string, opts ...dynamodb.QueryOption) (dynamobind.Page[Reading], error) {
	return dynamobind.QueryPage[Reading](ctx, table, keyCond, opts...)
}

// Each iterates every reading a query matches, one request per page.
func Each(ctx context.Context, table, keyCond string, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error] {
	return dynamobind.Query[Reading](ctx, table, keyCond, opts...)
}

// Sweep iterates a whole table.
func Sweep(ctx context.Context, table string, opts ...dynamodb.ScanOption) iter.Seq2[Reading, error] {
	return dynamobind.Scan[Reading](ctx, table, opts...)
}

// SaveAll stores every reading, in requests DynamoDB accepts.
func SaveAll(ctx context.Context, table string, rs []Reading) ([]Reading, error) {
	return dynamobind.StoreAll(ctx, table, rs)
}

// FetchAll reads every key, in requests DynamoDB accepts.
func FetchAll(ctx context.Context, table string, keys []dynamodb.Key) ([]Reading, []dynamodb.Key, error) {
	return dynamobind.LoadAll[Reading](ctx, table, keys)
}

// Table describes the table these readings live in.
func Table(name string) dynamodb.TableDefinition { return ReadingTable(name) }

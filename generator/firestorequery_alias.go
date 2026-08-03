package generator

import "github.com/shibukawa/tinybind-go/templates/firestorebind"

// The Firestore access-pattern grammar lives in templates/firestorebind beside
// the HTML, SQL and DynamoDB template packages, per
// decision:template-package-boundaries. The generator keeps the names its
// planning and emission stages already use.

// DefaultFirestoreTemplatePattern is the base-name glob for query declarations.
const DefaultFirestoreTemplatePattern = firestorebind.DefaultTemplatePattern

// FirestoreResultShape is what a declaration asks the generated function to
// return.
type FirestoreResultShape = firestorebind.ResultShape

const (
	// FirestoreBatch issues one request and returns a page.
	FirestoreBatch = firestorebind.Batch
	// FirestoreMany iterates every batch.
	FirestoreMany = firestorebind.Many
	// FirestoreCount runs an aggregation query.
	FirestoreCount = firestorebind.Count
	// FirestoreKeys runs a keys-only query.
	FirestoreKeys = firestorebind.Keys
)

// FirestoreOp is a property filter comparison.
type FirestoreOp = firestorebind.Op

const (
	FirestoreEqual          = firestorebind.OpEqual
	FirestoreNotEqual       = firestorebind.OpNotEqual
	FirestoreLess           = firestorebind.OpLess
	FirestoreLessOrEqual    = firestorebind.OpLessOrEqual
	FirestoreGreater        = firestorebind.OpGreater
	FirestoreGreaterOrEqual = firestorebind.OpGreaterOrEqual
	FirestoreIn             = firestorebind.OpIn
	FirestoreNotIn          = firestorebind.OpNotIn
)

// FirestoreDirection is a sort direction.
type FirestoreDirection = firestorebind.Direction

const (
	FirestoreAscending  = firestorebind.Ascending
	FirestoreDescending = firestorebind.Descending
)

// FirestoreQueryParam is one declared parameter of a query function.
type FirestoreQueryParam = firestorebind.QueryParam

// FirestorePredicate is one comparison in a where clause.
type FirestorePredicate = firestorebind.Predicate

// FirestoreOrder is one sort key of an order clause.
type FirestoreOrder = firestorebind.Order

// FirestoreProjection is one property a select or distinct clause names.
type FirestoreProjection = firestorebind.Projection

// FirestoreIndexProperty is one property of a declared composite index.
type FirestoreIndexProperty = firestorebind.IndexProperty

// FirestoreBound is a limit or an offset.
type FirestoreBound = firestorebind.Bound

// FirestoreQueryDecl is one declared access pattern.
type FirestoreQueryDecl = firestorebind.QueryDecl

// parseFirestoreQueries reads every declaration in one .tb.firestore source.
func parseFirestoreQueries(path string, source []byte) ([]FirestoreQueryDecl, error) {
	return firestorebind.ParseQueries(path, source)
}

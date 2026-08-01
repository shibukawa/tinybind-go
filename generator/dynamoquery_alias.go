package generator

import "github.com/shibukawa/tinybind-go/templates/dynamobind"

// The DynamoDB access-pattern grammar lives in templates/dynamobind beside the
// HTML and SQL template packages, per decision:template-package-boundaries. The
// generator keeps the names its planning and emission stages already use.

// DefaultDynamoTemplatePattern is the base-name glob for query declarations,
// beside the HTML and SQL template patterns.
const DefaultDynamoTemplatePattern = dynamobind.DefaultTemplatePattern

// DynamoResultShape is what a declaration asks the generated function to
// return.
type DynamoResultShape = dynamobind.ResultShape

const (
	// DynamoPage issues one request and returns a page.
	DynamoPage = dynamobind.Page
	// DynamoMany iterates every page.
	DynamoMany = dynamobind.Many
)

// DynamoOp is a key condition operator.
type DynamoOp = dynamobind.Op

const (
	DynamoEqual          = dynamobind.OpEqual
	DynamoLess           = dynamobind.OpLess
	DynamoLessOrEqual    = dynamobind.OpLessOrEqual
	DynamoGreater        = dynamobind.OpGreater
	DynamoGreaterOrEqual = dynamobind.OpGreaterOrEqual
	DynamoBetween        = dynamobind.OpBetween
	DynamoBeginsWith     = dynamobind.OpBeginsWith
)

// DynamoQueryParam is one declared parameter of a query function.
type DynamoQueryParam = dynamobind.QueryParam

// DynamoPredicate is one comparison in a key clause.
type DynamoPredicate = dynamobind.Predicate

// DynamoQueryDecl is one declared access pattern.
type DynamoQueryDecl = dynamobind.QueryDecl

// parseDynamoQueries reads every declaration in one .tb.dynamo source.
func parseDynamoQueries(path string, source []byte) ([]DynamoQueryDecl, error) {
	return dynamobind.ParseQueries(path, source)
}

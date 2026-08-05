// Package benchfixture backs the benchmark table in the project README.
//
// Every generated path is paired with the standard-library way of doing the
// same job — encoding/json for the codecs, a hand-written net/http handler for
// bind and write, html/template for rendering — and the tests here assert the
// two halves of each pair produce the same result. A number is only worth
// quoting if the thing being compared is the same thing.
//
//	go test ./internal/benchfixture -run xxx -bench . -benchmem
package benchfixture

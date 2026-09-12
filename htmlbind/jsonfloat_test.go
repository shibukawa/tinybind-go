package htmlbind_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// JSONFloat writes what encoding/json writes, so a number a template embeds
// reads the same as one a response body carries.
func TestJSONFloatMatchesEncodingJSON(t *testing.T) {
	for _, v := range []float64{0, 1, -1, 0.5, 40.8, 1000000, 1e15, 1e20, 1e21, 1e22, 1e-6, 1e-7, 123456789.125, 1.0000000000000002, math.MaxFloat64, math.SmallestNonzeroFloat64} {
		want, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if got := htmlbind.JSONFloat(v); got != string(want) {
			t.Fatalf("JSONFloat(%v) = %s, encoding/json writes %s", v, got, want)
		}
	}
}

package jsonbind

import "testing"

var (
	benchString  string
	benchInt     int
	benchFloat64 float64
	benchBool    bool
	benchAny     any
	benchStrings []string
	benchInts    map[string]int
)

func BenchmarkDecodeJSONString(b *testing.B) {
	raw := []byte(`"hello world"`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, err := DecodeJSONString(raw)
		if err != nil {
			b.Fatal(err)
		}
		benchString = v
	}
}

func BenchmarkDecodeJSONInt(b *testing.B) {
	raw := []byte(`123456`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, err := DecodeJSONInt(raw)
		if err != nil {
			b.Fatal(err)
		}
		benchInt = v
	}
}

func BenchmarkDecodeJSONFloat64(b *testing.B) {
	raw := []byte(`312.23`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, err := DecodeJSONFloat64(raw)
		if err != nil {
			b.Fatal(err)
		}
		benchFloat64 = v
	}
}

func BenchmarkDecodeJSONBool(b *testing.B) {
	raw := []byte(`true`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, err := DecodeJSONBool(raw)
		if err != nil {
			b.Fatal(err)
		}
		benchBool = v
	}
}

func BenchmarkDecodeJSONAny(b *testing.B) {
	raw := []byte(`"note"`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, err := DecodeJSONAny(raw)
		if err != nil {
			b.Fatal(err)
		}
		benchAny = v
	}
}

func BenchmarkParseSliceString(b *testing.B) {
	raw := []byte(`["priority","gift","repeat"]`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var p Parser
		p.Reset(raw)
		v, err := ParseSlice(&p, "tags", "invalid string", (*Parser).String)
		if err != nil {
			b.Fatal(err)
		}
		benchStrings = v
	}
}

func BenchmarkParseMapInt(b *testing.B) {
	raw := []byte(`{"a":1,"b":2,"c":3}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var p Parser
		p.Reset(raw)
		v, err := ParseMap(&p, "counts", "invalid int", (*Parser).Int)
		if err != nil {
			b.Fatal(err)
		}
		benchInts = v
	}
}

// BenchmarkRestAny walks a pre-parsed object into a map[string]any the way a
// binder's rest default arm does, with one member skipped.
func BenchmarkRestAny(b *testing.B) {
	raw := []byte(`{"id":"ord-1","note":"n","total":3,"paid":true,"extra":{"a":1}}`)
	obj, err := ParseObject(raw)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	var p Parser
	for i := 0; i < b.N; i++ {
		m := make(map[string]any)
		for j := 0; j < obj.Len(); j++ {
			name, value := obj.Member(j)
			if name == "id" {
				continue
			}
			p.Reset(value)
			v, err := p.Any()
			if err != nil {
				b.Fatal(err)
			}
			m[name] = v
		}
		benchAny = m
	}
}

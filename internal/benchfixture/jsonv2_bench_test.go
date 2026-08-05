//go:build goexperiment.jsonv2

// encoding/json/v2 is still behind GOEXPERIMENT on Go 1.26, so these only build
// when the experiment is on:
//
//	GOEXPERIMENT=jsonv2 go test ./internal/benchfixture -run xxx -bench JSON -benchmem
//
// Two questions are being answered. Whether turning the experiment on is worth
// it on its own — the v1 API is reimplemented over v2, so the plain Stdlib
// benchmarks in this package answer that by changing under the flag. And
// whether generated code should target jsontext instead of jsonbind's own
// parser, which the token benchmarks below answer.

package benchfixture

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"io"
	"reflect"
	"testing"

	"encoding/json/jsontext"
)

func BenchmarkJSONDecodeV2Reflect(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v Order
		if err := jsonv2.Unmarshal(orderJSON, &v); err != nil {
			b.Fatal(err)
		}
		sinkOrder = v
	}
}

func BenchmarkJSONDecodeV2ReflectRead(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v Order
		if err := jsonv2.UnmarshalRead(bytes.NewReader(orderJSON), &v); err != nil {
			b.Fatal(err)
		}
		sinkOrder = v
	}
}

func BenchmarkJSONEncodeV2Reflect(b *testing.B) {
	var w io.Writer = devNull{}
	b.ReportAllocs()
	for b.Loop() {
		if err := jsonv2.MarshalWrite(w, order); err != nil {
			b.Fatal(err)
		}
	}
}

// The generated-code shape, driven by the v2 tokenizer instead of
// jsonbind.Parser: same key switch, same in-place values, different reader.
func v2DecodeCustomer(d *jsontext.Decoder) (Customer, error) {
	var out Customer
	if _, err := d.ReadToken(); err != nil { // '{'
		return out, err
	}
	for {
		tok, err := d.ReadToken()
		if err != nil {
			return out, err
		}
		if tok.Kind() == '}' {
			return out, nil
		}
		switch tok.String() {
		case "name":
			out.Name, err = v2String(d)
		case "email":
			out.Email, err = v2String(d)
		case "tier":
			out.Tier, err = v2String(d)
		default:
			err = d.SkipValue()
		}
		if err != nil {
			return out, err
		}
	}
}

func v2String(d *jsontext.Decoder) (string, error) {
	tok, err := d.ReadToken()
	if err != nil {
		return "", err
	}
	if tok.Kind() == 'n' {
		return "", nil
	}
	return tok.String(), nil
}

func v2DecodeLineItem(d *jsontext.Decoder) (LineItem, error) {
	var out LineItem
	if _, err := d.ReadToken(); err != nil {
		return out, err
	}
	for {
		tok, err := d.ReadToken()
		if err != nil {
			return out, err
		}
		if tok.Kind() == '}' {
			return out, nil
		}
		switch tok.String() {
		case "sku":
			out.SKU, err = v2String(d)
		case "qty":
			var t jsontext.Token
			if t, err = d.ReadToken(); err == nil {
				out.Qty = int(t.Int())
			}
		case "price":
			var t jsontext.Token
			if t, err = d.ReadToken(); err == nil {
				out.Price = t.Float()
			}
		default:
			err = d.SkipValue()
		}
		if err != nil {
			return out, err
		}
	}
}

func v2DecodeOrder(d *jsontext.Decoder) (Order, error) {
	var out Order
	if _, err := d.ReadToken(); err != nil {
		return out, err
	}
	for {
		tok, err := d.ReadToken()
		if err != nil {
			return out, err
		}
		if tok.Kind() == '}' {
			return out, nil
		}
		switch tok.String() {
		case "id":
			out.ID, err = v2String(d)
		case "customer":
			out.Customer, err = v2DecodeCustomer(d)
		case "items":
			if _, err = d.ReadToken(); err == nil { // '['
				items := []LineItem{}
				for i := 0; d.PeekKind() != ']'; i++ {
					if i == 0 {
						items = make([]LineItem, 0, 8)
					}
					var item LineItem
					if item, err = v2DecodeLineItem(d); err != nil {
						break
					}
					items = append(items, item)
				}
				if err == nil {
					_, err = d.ReadToken() // ']'
				}
				out.Items = items
			}
		case "total":
			var t jsontext.Token
			if t, err = d.ReadToken(); err == nil {
				out.Total = t.Float()
			}
		case "paid":
			var t jsontext.Token
			if t, err = d.ReadToken(); err == nil {
				out.Paid = t.Bool()
			}
		case "tags":
			if _, err = d.ReadToken(); err == nil { // '['
				tags := []string{}
				for i := 0; d.PeekKind() != ']'; i++ {
					if i == 0 {
						tags = make([]string, 0, 8)
					}
					var s string
					if s, err = v2String(d); err != nil {
						break
					}
					tags = append(tags, s)
				}
				if err == nil {
					_, err = d.ReadToken() // ']'
				}
				out.Tags = tags
			}
		case "note":
			out.Note, err = v2String(d)
		default:
			err = d.SkipValue()
		}
		if err != nil {
			return out, err
		}
	}
}

func TestV2TokenDecodeMatchesGenerated(t *testing.T) {
	want, err := DecodeOrder(bytes.NewReader(orderJSON))
	if err != nil {
		t.Fatal(err)
	}
	got, err := v2DecodeOrder(jsontext.NewDecoder(bytes.NewReader(orderJSON)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func BenchmarkJSONDecodeV2Tokens(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		v, err := v2DecodeOrder(jsontext.NewDecoder(bytes.NewReader(orderJSON)))
		if err != nil {
			b.Fatal(err)
		}
		sinkOrder = v
	}
}

// Same, reusing the decoder and reader, which is the best a generated codec
// could do with jsontext.
func BenchmarkJSONDecodeV2TokensReuse(b *testing.B) {
	d := jsontext.NewDecoder(bytes.NewReader(nil))
	rd := bytes.NewReader(nil)
	b.ReportAllocs()
	for b.Loop() {
		rd.Reset(orderJSON)
		d.Reset(rd)
		v, err := v2DecodeOrder(d)
		if err != nil {
			b.Fatal(err)
		}
		sinkOrder = v
	}
}

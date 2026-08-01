package dynamofixture_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinybind-go/internal/dynamofixture"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

const table = "readings"

// exact carries more significant digits than float64 or int64 can hold. It is
// the reason numbers stay text from end to end.
const exact = "12345678901234567890123456789012345678"

func sample() dynamofixture.Reading {
	return dynamofixture.Reading{
		Sensor: "室温センサー",
		At:     1700000000,
		Note:   "",
		Scale:  1.5,
		Count:  65535,
		Active: true,
		Blob:   []byte{0x00, 0x80, 0xff, 0xfe},
		Taken:  time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC),
		Seen:   time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
		Tags:   []string{"a", "b"},
		Counts: []int{1, 2, 3},
		Chunks: [][]byte{{0x01}, {0xff}},
		Words:  []dynamofixture.Sensor{"x", "y"},
		Scores: map[string]int32{"good": 1, "bad": -2},
		Site:   dynamofixture.Site{City: "東京", Zip: "100-0001"},
		Backup: &dynamofixture.Site{City: "大阪"},
		Exact:  dynamodb.NString(exact),
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	want := sample()
	var got dynamofixture.Reading
	if err := got.DecodeItem(want.EncodeItem()); err != nil {
		t.Fatalf("DecodeItem: %v", err)
	}

	if got.Sensor != want.Sensor || got.At != want.At || got.Scale != want.Scale || got.Count != want.Count {
		t.Errorf("scalars: got %+v want %+v", got, want)
	}
	if got.Note != "" {
		t.Errorf("empty string: got %q", got.Note)
	}
	if !got.Active {
		t.Error("bool lost")
	}
	if !bytes.Equal(got.Blob, want.Blob) {
		t.Errorf("binary: got %v want %v", got.Blob, want.Blob)
	}
	if !got.Taken.Equal(want.Taken) {
		t.Errorf("time: got %s want %s", got.Taken, want.Taken)
	}
	if !got.Seen.Equal(want.Seen) {
		t.Errorf("unixtime: got %s want %s", got.Seen, want.Seen)
	}
	if strings.Join(got.Tags, ",") != "a,b" {
		t.Errorf("string set: %v", got.Tags)
	}
	if len(got.Counts) != 3 || got.Counts[2] != 3 {
		t.Errorf("number set: %v", got.Counts)
	}
	if len(got.Chunks) != 2 || !bytes.Equal(got.Chunks[1], []byte{0xff}) {
		t.Errorf("binary set: %v", got.Chunks)
	}
	if len(got.Words) != 2 || got.Words[1] != "y" {
		t.Errorf("list: %v", got.Words)
	}
	if got.Scores["good"] != 1 || got.Scores["bad"] != -2 {
		t.Errorf("map: %v", got.Scores)
	}
	if got.Site != want.Site {
		t.Errorf("nested struct: %+v", got.Site)
	}
	if got.Backup == nil || got.Backup.City != "大阪" || got.Backup.Zip != "" {
		t.Errorf("nested pointer: %+v", got.Backup)
	}
	if number, ok := got.Exact.AsNumber(); !ok || number != exact {
		t.Errorf("38-digit number: got %q ok=%v", number, ok)
	}
}

// TestEncodeOmitsEmpty proves omitempty writes no attribute at all, which is
// the difference between an absent attribute and an empty one.
func TestEncodeOmitsEmpty(t *testing.T) {
	item := dynamofixture.Reading{Sensor: "s", At: 1}.EncodeItem()
	for _, attribute := range []string{"tags", "counts", "chunks", "sites"} {
		if _, present := item[attribute]; present {
			t.Errorf("omitempty attribute %q was written", attribute)
		}
	}
	// Without omitempty an empty value is still stored, since an empty string
	// and a missing attribute are different things to DynamoDB.
	if _, present := item["note"]; !present {
		t.Error("note is not omitempty and must be written even when empty")
	}
	if _, present := item["skipped"]; present {
		t.Error(`a "-" tag must not be stored`)
	}
	if _, present := item["exact"]; present {
		t.Error("a zero AttributeValue has no type and must not be written")
	}
}

func TestDecodeNilPointerFromNull(t *testing.T) {
	item := dynamofixture.Reading{Sensor: "s", At: 1}.EncodeItem()
	if kind := item["backup"].Kind(); kind != dynamodb.KindNull {
		t.Fatalf("nil pointer encoded as kind %v", kind)
	}
	var got dynamofixture.Reading
	if err := got.DecodeItem(item); err != nil {
		t.Fatal(err)
	}
	if got.Backup != nil {
		t.Errorf("NULL decoded as %+v", got.Backup)
	}
}

func TestDecodeReportsTheAttributeAndBothKinds(t *testing.T) {
	item := sample().EncodeItem()
	item["at"] = dynamodb.S("not a number")

	var got dynamofixture.Reading
	err := got.DecodeItem(item)
	if err == nil {
		t.Fatal("expected a type error")
	}
	mapping, ok := dynamobind.AsError(err)
	if !ok {
		t.Fatalf("error is not a dynamobind.Error: %v", err)
	}
	if mapping.Attribute != "at" || mapping.Expected != "N" || mapping.Got != "S" {
		t.Fatalf("got %+v", mapping)
	}
}

func TestDecodeRejectsANumberTheFieldCannotHold(t *testing.T) {
	item := sample().EncodeItem()
	item["count"] = dynamodb.NString("70000") // uint16 stops at 65535

	var got dynamofixture.Reading
	err := got.DecodeItem(item)
	if err == nil {
		t.Fatal("expected a range error rather than a wrapped value")
	}
	if got.Count == 4464 {
		t.Fatal("value wrapped instead of failing")
	}
}

func TestMissingAttributeLeavesTheFieldAlone(t *testing.T) {
	got := dynamofixture.Reading{Note: "kept"}
	if err := got.DecodeItem(dynamodb.Item{"sensor": dynamodb.S("s")}); err != nil {
		t.Fatal(err)
	}
	if got.Sensor != "s" || got.Note != "kept" {
		t.Fatalf("got %+v", got)
	}
}

func TestItemKeyAndTableAgree(t *testing.T) {
	key := sample().ItemKey()
	definition := dynamofixture.Table(table)

	if _, ok := key[definition.PartitionKey.Name]; !ok {
		t.Fatalf("key %v has no attribute named by the table partition key %q", key, definition.PartitionKey.Name)
	}
	if definition.SortKey == nil {
		t.Fatal("table lost the sort key")
	}
	if _, ok := key[definition.SortKey.Name]; !ok {
		t.Fatalf("key %v has no attribute named by the table sort key %q", key, definition.SortKey.Name)
	}
	if definition.PartitionKey.Type != dynamodb.TypeString || definition.SortKey.Type != dynamodb.TypeNumber {
		t.Fatalf("key types: %+v %+v", definition.PartitionKey, definition.SortKey)
	}
	if len(key) != 2 {
		t.Fatalf("the key carries more than the key attributes: %v", key)
	}
}

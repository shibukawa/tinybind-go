package jsonbind

import (
	"io"
	"strings"
	"testing"
)

// point stands for a type from a package this module never analyzed: it carries
// its own codec and is registered nowhere.
type point struct {
	X int
	Y int
}

func (p point) AppendJSONTo(dst []byte) []byte {
	dst = append(dst, `{"x":`...)
	dst = AppendInt(dst, int64(p.X))
	dst = append(dst, `,"y":`...)
	dst = AppendInt(dst, int64(p.Y))
	return append(dst, '}')
}

func (p *point) DecodeJSONFrom(data []byte) error {
	m, err := DecodeJSONMapStringInt(data)
	if err != nil {
		return err
	}
	p.X, p.Y = m["x"], m["y"]
	return nil
}

// namedString has an underlying builtin type, so it reaches AppendAny's
// Appender arm rather than the string case, which matches by identity.
type namedString string

func (n namedString) AppendJSONTo(dst []byte) []byte {
	return AppendString(dst, "named:"+string(n))
}

type plainStruct struct{ Value string }

func TestAppenderSatisfiedByValue(t *testing.T) {
	var a Appender = point{X: 1, Y: 2}
	if got := string(a.AppendJSONTo(nil)); got != `{"x":1,"y":2}` {
		t.Fatalf("got %s", got)
	}
}

func TestDecoderSatisfiedByPointer(t *testing.T) {
	var p point
	var d Decoder = &p
	if err := d.DecodeJSONFrom([]byte(`{"x":3,"y":4}`)); err != nil {
		t.Fatal(err)
	}
	if p.X != 3 || p.Y != 4 {
		t.Fatalf("got %+v", p)
	}
}

func TestDecoderReportsAFailure(t *testing.T) {
	var p point
	err := (&p).DecodeJSONFrom([]byte(`not json`))
	if err == nil {
		t.Fatal("want an error")
	}
	if _, ok := AsError(err); !ok {
		t.Fatalf("want a jsonbind error, got %#v", err)
	}
}

func TestAppendAnyEncodesAnAppenderRatherThanNull(t *testing.T) {
	if got := string(AppendAny(nil, point{X: 5, Y: 6})); got != `{"x":5,"y":6}` {
		t.Fatalf("got %s", got)
	}
}

func TestAppendAnyReachesAnAppenderInsideARestMap(t *testing.T) {
	rest := map[string]any{"origin": point{}, "label": "home"}
	if got := string(AppendAny(nil, rest)); got != `{"label":"home","origin":{"x":0,"y":0}}` {
		t.Fatalf("got %s", got)
	}
}

func TestAppendAnyPrefersTheConcreteCaseOverTheMethodSet(t *testing.T) {
	// A builtin shape keeps its own arm; only a type the switch does not name
	// falls through to Appender.
	if got := string(AppendAny(nil, namedString("x"))); got != `"named:x"` {
		t.Fatalf("named type should reach the Appender arm, got %s", got)
	}
	if got := string(AppendAny(nil, "x")); got != `"x"` {
		t.Fatalf("plain string should reach the string arm, got %s", got)
	}
}

func TestAppendAnyStillWritesNullForATypeCarryingNoCodec(t *testing.T) {
	if got := string(AppendAny(nil, plainStruct{Value: "v"})); got != "null" {
		t.Fatalf("got %s", got)
	}
}

// The consuming half: a type this build never planned reaches the generic entry
// points anyway, because it carries its own codec. Nothing registers point.

func TestEncodeJSONReachesATypeItNeverPlanned(t *testing.T) {
	var buf strings.Builder
	if err := EncodeJSON(&buf, point{X: 7, Y: 8}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `{"x":7,"y":8}` {
		t.Fatalf("got %s", got)
	}
}

func TestDecodeJSONBytesReachesATypeItNeverPlanned(t *testing.T) {
	got, err := DecodeJSONBytes[point]([]byte(`{"x":9,"y":10}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.X != 9 || got.Y != 10 {
		t.Fatalf("got %+v", got)
	}
}

func TestDecodeJSONReaderReachesATypeItNeverPlanned(t *testing.T) {
	got, err := DecodeJSON[point](strings.NewReader(`{"x":11,"y":12}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.X != 11 || got.Y != 12 {
		t.Fatalf("got %+v", got)
	}
}

// A type carrying neither a codec nor a method is still the error it was.
func TestEncodeJSONStillFailsForATypeCarryingNothing(t *testing.T) {
	var buf strings.Builder
	if err := EncodeJSON(&buf, plainStruct{Value: "v"}); err == nil {
		t.Fatal("want a missing-encoder error")
	}
}

func TestDecodeJSONBytesStillFailsForATypeCarryingNothing(t *testing.T) {
	if _, err := DecodeJSONBytes[plainStruct]([]byte(`{}`)); err == nil {
		t.Fatal("want a missing-decoder error")
	}
}

// The reader entry point must not consume the body for a type it can decode by
// neither route, which is what the check before the read is for.
func TestDecodeJSONFailsBeforeReadingForATypeCarryingNothing(t *testing.T) {
	r := strings.NewReader(`{"value":"v"}`)
	if _, err := DecodeJSON[plainStruct](r); err == nil {
		t.Fatal("want a missing-decoder error")
	}
	if r.Len() != len(`{"value":"v"}`) {
		t.Fatalf("reader was consumed: %d bytes left", r.Len())
	}
}

// A registered codec still wins for a type carrying no method, which is every
// type generation reached through an ordinary call site.
func TestRegisteredCodecStillServesATypeCarryingNoMethod(t *testing.T) {
	RegisterEncode[registeredOnly](func(w io.Writer, v registeredOnly) error {
		_, err := w.Write([]byte(`"registered"`))
		return err
	})
	var buf strings.Builder
	if err := EncodeJSON(&buf, registeredOnly{}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `"registered"` {
		t.Fatalf("got %s", got)
	}
}

type registeredOnly struct{}

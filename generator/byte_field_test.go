package generator_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// analyzeOneField plans a struct holding exactly the field given, so a refusal
// names that field and nothing else can mask it.
func analyzeOneField(t *testing.T, decls, field string) (string, error) {
	t.Helper()
	src := "package main\n\nimport \"github.com/shibukawa/tinybind-go/jsonbind\"\n\n" + decls +
		"\ntype Doc struct {\n\t" + field + "\n}\n\nvar _ = jsonbind.GenerateCodec[Doc]()\n\nfunc main() {}\n"
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		return "", err
	}
	code, err := generator.Emit(plan)
	if err != nil {
		return "", err
	}
	return string(code), nil
}

// byte and rune are alternative spellings of uint8 and int32, not the
// *types.Alias node the analysis unwraps, so every one of these fell past the
// scalar names and was refused -- while a named type over byte was accepted,
// because that path reads Underlying().
func TestByteAndRuneAreTheWidthsTheyName(t *testing.T) {
	for _, tc := range []struct{ name, field, want string }{
		{"bare byte", "B byte `json:\"b\"`", "jsonbind.AppendUint(dst, uint64(v.B))"},
		{"bare rune", "R rune `json:\"r\"`", "jsonbind.AppendInt(dst, int64(v.R))"},
		{"rune slice", "N []rune `json:\"n\"`", "jsonbind.AppendInt(dst, int64(v.N[i]))"},
		{"byte map", "M map[string]byte `json:\"m\"`", "jsonbind.AppendUint(dst, uint64(v.M[k]))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, err := analyzeOneField(t, "", tc.field)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !strings.Contains(code, tc.want) {
				t.Fatalf("generated source is missing %q:\n%s", tc.want, code)
			}
		})
	}
}

// A byte sequence is a string on the wire whatever it was spelled as, since
// []byte and []uint8 are one type.
func TestByteSequencesEncodeAsBase64(t *testing.T) {
	for _, tc := range []struct{ name, field, want string }{
		{"byte slice", "L []byte `json:\"l\"`", "jsonbind.AppendBase64(dst, v.L)"},
		{"uint8 slice", "L []uint8 `json:\"l\"`", "jsonbind.AppendBase64(dst, v.L)"},
		{"byte array", "A [4]byte `json:\"a\"`", "jsonbind.AppendBase64(dst, v.A[:])"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, err := analyzeOneField(t, "", tc.field)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !strings.Contains(code, tc.want) {
				t.Fatalf("generated source is missing %q:\n%s", tc.want, code)
			}
		})
	}
}

// The refusal used to read "type byte is  underneath", with a hole where the
// underlying type should be, because a predeclared type is not a *types.Named
// and nothing was recorded for it.
func TestUnmappableTypeDiagnosticHasNoHole(t *testing.T) {
	_, err := analyzeOneField(t, "", "C complex128 `json:\"c\"`")
	if err == nil {
		t.Fatal("complex128 was accepted")
	}
	if strings.Contains(err.Error(), "is  underneath") {
		t.Fatalf("the diagnostic still has a hole in it: %v", err)
	}
	if !strings.Contains(err.Error(), "cannot map complex128") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
	// A named type still says what it is underneath, which is the half that
	// carries information.
	_, err = analyzeOneField(t, "type Odd complex128\n", "O Odd `json:\"o\"`")
	if err == nil || !strings.Contains(err.Error(), "type Odd is complex128 underneath") {
		t.Fatalf("a named type lost its underlying type: %v", err)
	}
}

// bindOneField is analyzeOneField through the binder, since only a bound type
// emits the value-source arms.
func bindOneField(t *testing.T, field string) (string, error) {
	t.Helper()
	src := "package main\n\nimport (\n\t\"net/http\"\n\n\t\"github.com/shibukawa/tinybind-go\"\n)\n\n" +
		"type Doc struct {\n\t" + field + "\n}\n\n" +
		"func Handler(w http.ResponseWriter, r *http.Request) {\n" +
		"\tin, err := httpbind.Bind[Doc](r)\n\tif err != nil {\n\t\thttpbind.WriteError(w, r, err)\n\t\treturn\n\t}\n\t_ = in\n}\n\n" +
		"func main() {\n\thttp.HandleFunc(\"GET /d/{l}\", Handler)\n}\n"
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		return "", err
	}
	code, err := generator.Emit(plan)
	if err != nil {
		return "", err
	}
	return string(code), nil
}

// A byte sequence is the one composite with a spelling outside a document, so
// unlike a slice or a map it binds from a value source.
func TestAByteSequenceBindsFromAValueSource(t *testing.T) {
	for _, tag := range []string{"query:\"l\"", "path:\"l\"", "header:\"L\"", "cookie:\"l\""} {
		t.Run(tag, func(t *testing.T) {
			code, err := bindOneField(t, "L []byte `"+tag+"`")
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !strings.Contains(code, "httpbind.ParseBytes(") {
				t.Fatalf("no base64 parse emitted:\n%s", code)
			}
		})
	}
	// method has no value to carry, and every other composite still has no
	// spelling outside the document.
	for _, field := range []string{"L []byte `method:\"l\"`", "S []string `query:\"s\"`", "M map[string]string `query:\"m\"`"} {
		if _, err := bindOneField(t, field); err == nil || !strings.Contains(err.Error(), "only supports payload/input sources") {
			t.Fatalf("%s: %v", field, err)
		}
	}
}

// The binder accepts base64 there, so the document has to say so.
func TestOpenAPIDocumentsAByteQueryParameter(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(byteFieldSource), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("openapi: %v", err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"format":"byte"`) {
		t.Fatalf("a byte field is not documented as base64: %s", encoded)
	}
}

// byteFieldSource reaches both byte spellings, both rune shapes and the
// fixed-length blob from one handler.
const byteFieldSource = `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Message struct {
	Level byte    ` + "`query:\"level\"`" + `
	Sig   []byte  ` + "`query:\"sig\"`" + `
	Nonce [4]byte ` + "`header:\"X-Nonce\"`" + `
	Flag  byte    ` + "`payload:\"flag\"`" + `
	Sym   rune    ` + "`payload:\"sym\"`" + `
	Body  []byte  ` + "`payload:\"body\"`" + `
	Tag   [4]byte ` + "`payload:\"tag\"`" + `
	Runes []rune  ` + "`payload:\"runes\"`" + `
}

type Echo struct {
	Level byte    ` + "`json:\"level\"`" + `
	Sig   []byte  ` + "`json:\"sig\"`" + `
	Nonce [4]byte ` + "`json:\"nonce\"`" + `
	Flag  byte    ` + "`json:\"flag\"`" + `
	Sym   rune    ` + "`json:\"sym\"`" + `
	Body  []byte  ` + "`json:\"body\"`" + `
	Tag   [4]byte ` + "`json:\"tag\"`" + `
	Runes []rune  ` + "`json:\"runes\"`" + `
}

func Handler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[Message](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Echo](w, r, Echo(in))
}

func main() {
	http.HandleFunc("POST /messages", Handler)
}
`

// The end of the line: the generated code compiles, agrees with encoding/json
// about the base64 a byte slice becomes, and carries a CBOR byte string rather
// than an array of one-byte integers.
func TestByteFieldsRoundTripOverHTTP(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(byteFieldSource), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), code, 0o644); err != nil {
		t.Fatal(err)
	}
	probe := `package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/tinygodriver/encoding/cbor"
)

func post(t *testing.T, payload string) *httptest.ResponseRecorder {
	t.Helper()
	return postWith(t, payload, url.Values{}, http.Header{})
}

func postWith(t *testing.T, payload string, query url.Values, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	query.Set("level", "200")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/messages?"+query.Encode(), strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")
	for k, vs := range header {
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}
	Handler(w, r)
	return w
}

// echoWire is Echo as encoding/json can read it back. It differs in one member:
// encoding/json base64s a byte slice but not a byte array, so it will not read
// the string this codec writes for tag into a [4]byte.
type echoWire struct {
	Level byte   ` + "`" + `json:"level"` + "`" + `
	Sig   []byte ` + "`" + `json:"sig"` + "`" + `
	Nonce string ` + "`" + `json:"nonce"` + "`" + `
	Flag  byte   ` + "`" + `json:"flag"` + "`" + `
	Sym   rune   ` + "`" + `json:"sym"` + "`" + `
	Body  []byte ` + "`" + `json:"body"` + "`" + `
	Tag   string ` + "`" + `json:"tag"` + "`" + `
	Runes []rune ` + "`" + `json:"runes"` + "`" + `
}

func decode(t *testing.T, w *httptest.ResponseRecorder) echoWire {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got echoWire
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func tagOf(t *testing.T, w echoWire) [4]byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(w.Tag)
	if err != nil {
		t.Fatalf("tag %q is not base64: %v", w.Tag, err)
	}
	var out [4]byte
	if len(raw) != len(out) {
		t.Fatalf("tag decoded to %d bytes", len(raw))
	}
	copy(out[:], raw)
	return out
}

const full = ` + "`" + `{"flag":7,"sym":128169,"body":"3q2+7w==","tag":"AQIDBA==","runes":[97,128169]}` + "`" + `

func TestByteFieldsRoundTrip(t *testing.T) {
	got := decode(t, post(t, full))
	if got.Flag != 7 || got.Sym != 128169 {
		t.Fatalf("scalars %v %v", got.Flag, got.Sym)
	}
	// A lone byte is a scalar, so it binds from the query like any other width.
	if got.Level != 200 {
		t.Fatalf("level %v", got.Level)
	}
	if !bytes.Equal(got.Body, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Fatalf("body %v", got.Body)
	}
	if tagOf(t, got) != [4]byte{1, 2, 3, 4} {
		t.Fatalf("tag %v", got.Tag)
	}
	if len(got.Runes) != 2 || got.Runes[0] != 'a' || got.Runes[1] != 128169 {
		t.Fatalf("runes %v", got.Runes)
	}
}

// The point of picking base64: another decoder reads what this one writes.
func TestTheWireFormMatchesEncodingJSON(t *testing.T) {
	w := post(t, full)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var mine map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &mine); err != nil {
		t.Fatal(err)
	}
	theirs, err := json.Marshal(Echo{
		Level: 200, Flag: 7, Sym: 128169,
		Body:  []byte{0xde, 0xad, 0xbe, 0xef},
		Tag:   [4]byte{1, 2, 3, 4},
		Runes: []rune{'a', 128169},
	})
	if err != nil {
		t.Fatal(err)
	}
	var std map[string]json.RawMessage
	if err := json.Unmarshal(theirs, &std); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"level", "flag", "sym", "body", "runes"} {
		if string(mine[key]) != string(std[key]) {
			t.Fatalf("member %q: %s, encoding/json says %s", key, mine[key], std[key])
		}
	}
	// The one deliberate divergence, pinned so it cannot drift into being an
	// accident: encoding/json applies its base64 rule to a byte slice only, and
	// writes a byte array as a list of numbers. A fixed-length blob is exactly
	// the case base64 is wanted for, so this codec treats both spellings alike.
	if string(std["tag"]) != ` + "`" + `[1,2,3,4]` + "`" + ` {
		t.Fatalf("encoding/json changed how it writes a byte array: %s", std["tag"])
	}
	if string(mine["tag"]) != ` + "`" + `"AQIDBA=="` + "`" + ` {
		t.Fatalf("tag %s", mine["tag"])
	}
}

func TestAShortBlobZeroesTheTail(t *testing.T) {
	got := decode(t, post(t, ` + "`" + `{"tag":"AQ=="}` + "`" + `))
	if tagOf(t, got) != [4]byte{1, 0, 0, 0} {
		t.Fatalf("tag %v", got.Tag)
	}
}

func TestABlobTooLongForTheFieldIs400(t *testing.T) {
	if w := post(t, ` + "`" + `{"tag":"AQIDBAU="}` + "`" + `); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestInvalidBase64Is400(t *testing.T) {
	if w := post(t, ` + "`" + `{"body":"not base64!!"}` + "`" + `); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

// encoding/json writes null for a nil byte slice; this codec writes "", the
// same way it writes [] for a nil slice and {} for a nil map.
func TestANilBlobIsAnEmptyString(t *testing.T) {
	for _, payload := range []string{` + "`" + `{}` + "`" + `, ` + "`" + `{"body":""}` + "`" + `, ` + "`" + `{"body":null}` + "`" + `} {
		w := post(t, payload)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", payload, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), ` + "`" + `"body":""` + "`" + `) {
			t.Fatalf("%s: body is not an empty string: %s", payload, w.Body.String())
		}
	}
}

func postCBOR(t *testing.T, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/cbor")
	r.Header.Set("Accept", "application/cbor")
	Handler(w, r)
	return w
}

// CBOR has a byte string; using an array of one-byte integers would cost a byte
// an element and lose the type on the wire.
func TestCBORCarriesAByteString(t *testing.T) {
	in := cbor.AppendMapHeader(nil, 2)
	in = cbor.AppendText(in, "body")
	in = cbor.AppendBytes(in, []byte{0xde, 0xad})
	in = cbor.AppendText(in, "tag")
	in = cbor.AppendBytes(in, []byte{1, 2})
	w := postCBOR(t, in)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	rd := cbor.ReaderOver(w.Body.Bytes(), cbor.DecoderOptions{})
	pairs, indef, err := rd.ReadMapHeader()
	if err != nil || indef || pairs != 8 {
		t.Fatalf("map header %d indef=%v err=%v", pairs, indef, err)
	}
	for i := 0; i < pairs; i++ {
		key, err := rd.ReadText()
		if err != nil {
			t.Fatal(err)
		}
		if key != "body" && key != "tag" {
			if err := rd.Skip(); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := rd.ReadBytes()
		if err != nil {
			t.Fatalf("%s is not a CBOR byte string: %v", key, err)
		}
		switch key {
		case "body":
			if !bytes.Equal(got, []byte{0xde, 0xad}) {
				t.Fatalf("body %v", got)
			}
		case "tag":
			if !bytes.Equal(got, []byte{1, 2, 0, 0}) {
				t.Fatalf("tag %v", got)
			}
		}
	}
}

// A blob has a spelling outside a document, so a value source can carry it.
// The query is why both alphabets are accepted: a + in a standard-alphabet
// value arrives as a space unless the client percent-encoded it.
func TestAQueryCarriesABlobAsBase64(t *testing.T) {
	blob := []byte{0xfb, 0xef, 0xbe, 0x3f, 0xf0}
	std := base64.StdEncoding.EncodeToString(blob)
	if !strings.ContainsAny(std, "+/") {
		t.Fatalf("pick a blob whose standard base64 needs escaping: %s", std)
	}
	for _, encoded := range []string{std, base64.RawURLEncoding.EncodeToString(blob)} {
		// url.Values.Encode percent-escapes, which is what a correct client does.
		got := decode(t, postWith(t, ` + "`" + `{}` + "`" + `, url.Values{"sig": {encoded}}, http.Header{}))
		if !bytes.Equal(got.Sig, blob) {
			t.Fatalf("%s bound as %v", encoded, got.Sig)
		}
	}
}

// A raw + in a query is a space by the time the binder sees it, so it is a 400
// rather than a blob missing a byte.
func TestARawPlusInAQueryIs400(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/messages?level=1&sig=++++", strings.NewReader(` + "`" + `{}` + "`" + `))
	r.Header.Set("Content-Type", "application/json")
	Handler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestAHeaderCarriesAFixedLengthBlob(t *testing.T) {
	got := decode(t, postWith(t, ` + "`" + `{}` + "`" + `, url.Values{}, http.Header{"X-Nonce": {"AQID"}}))
	raw, err := base64.StdEncoding.DecodeString(got.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, []byte{1, 2, 3, 0}) {
		t.Fatalf("nonce %v", raw)
	}
}

func TestAHeaderBlobTooLongForTheFieldIs400(t *testing.T) {
	w := postWith(t, ` + "`" + `{}` + "`" + `, url.Values{}, http.Header{"X-Nonce": {"AQIDBAU="}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestAnInvalidBase64QueryValueIs400(t *testing.T) {
	w := postWith(t, ` + "`" + `{}` + "`" + `, url.Values{"sig": {"not base64!!"}}, http.Header{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestCBORRefusesABlobTooLongForTheField(t *testing.T) {
	in := cbor.AppendMapHeader(nil, 1)
	in = cbor.AppendText(in, "tag")
	in = cbor.AppendBytes(in, []byte{1, 2, 3, 4, 5})
	if w := postCBOR(t, in); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(probe), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("byte fields do not round-trip: %v\n%s\n%s", err, output, code)
	}
	if strings.Contains(string(output), "no test files") {
		t.Fatalf("the probe did not run: %s", output)
	}
}

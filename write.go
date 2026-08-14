package httpbind

import (
	"encoding/json"
	"net/http"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

// Write serializes a typed response value to the HTTP response via a registered
// writer, or, for a type carrying its own encoder, through jsonbind.Appender.
// Status is always 200 OK; use WriteStatus for other success codes.
//
// The interface is tried first, for the reason jsonbind.EncodeJSON states: a
// type that carries a method has an author-written encoder, and going through a
// generated one instead would produce bytes they did not intend. It is also
// what lets a value from a package this build never analyzed be answered with
// at all, which no registration could reach.
func Write[T any](w http.ResponseWriter, r *http.Request, value T) error {
	_ = r
	if source, ok := any(value).(jsonbind.Appender); ok {
		buf := jsonbind.GetBuffer()
		*buf = source.AppendJSONTo((*buf)[:0])
		err := WriteJSONBytes(w, http.StatusOK, *buf)
		jsonbind.PutBuffer(buf)
		return err
	}
	fn, ok := lookupWriter[T]()
	if !ok {
		return missingWriterError()
	}
	return fn(w, r, value)
}

// WriteStatus serializes value with an explicit HTTP status code using the
// registered encoder for T (no field-walking reflection on T).
// For status 204 No Content, the body is not written.
func WriteStatus[T any](w http.ResponseWriter, r *http.Request, status int, value T) error {
	_ = r
	if status == http.StatusNoContent {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return jsonbind.EncodeJSON(w, value)
}

// WriteError writes err as an RFC 9457 Problem Details response.
// Internal causes are not exposed in the client body.
//
// JSON is written without encoding/json for the problem document so TinyGo
// does not hit unimplemented reflect.AssignableTo when binders also use
// json.RawMessage (a known interaction in TinyGo's encoding/json).
// The document itself is derived in bindcore, so the other transport runtime
// writes the same bytes for the same error rather than reimplementing the rule.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	_ = r
	status, body, ok := bindcore.ProblemResponse(err)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", bindcore.ProblemContentType)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteJSON is a helper for generated writers: encode a pre-built map/slice without
// reflecting over application structs. Content-Type is application/json.
//
// It hands the value to encoding/json and so does not follow the rules a
// generated encoder does: a nil slice comes out as null here rather than [],
// and json tag options are read the v1 way. Anything wanting the codec's
// behaviour should go through a generated encoder and WriteJSONBytes.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// WriteJSONBytes writes an already-encoded document. Generated writers build
// the body into a pooled buffer and hand it over here, so the response path
// never reflects over the value and never allocates an intermediate map.
func WriteJSONBytes(w http.ResponseWriter, status int, data []byte) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		if _, err := w.Write(data); err != nil {
			return err
		}
		_, err := w.Write(newline)
		return err
	}
	_, err := w.Write(data)
	return err
}

var newline = []byte("\n")

// AppendFileJSON appends an uploaded file the way encoding/json rendered it
// before generated encoders stopped going through reflection: exported fields
// in declaration order, with the content base64-encoded.
func AppendFileJSON(dst []byte, f File) []byte {
	return bindcore.AppendFileJSON(dst, f)
}

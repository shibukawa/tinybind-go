package httpbind

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// Write serializes a typed response value to the HTTP response via a registered writer.
// Status is always 200 OK; use WriteStatus for other success codes.
func Write[T any](w http.ResponseWriter, r *http.Request, value T) error {
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
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	_ = r
	status := http.StatusInternalServerError
	title := "Internal Server Error"
	detail := "internal error"
	code := "internal"
	var fields []FieldError

	if he, ok := AsHTTPError(err); ok {
		status = he.Status
		if he.Title != "" {
			title = he.Title
		} else {
			title = http.StatusText(status)
		}
		if he.Problem.Message != "" {
			detail = he.Problem.Message
		} else {
			detail = title
		}
		if he.Problem.Code != "" {
			code = he.Problem.Code
		}
		// Hide internal implementation details from clients for 5xx.
		if status >= 500 {
			detail = title
			code = "internal"
		}
		fields = he.Fields
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write(encodeProblemJSON(title, detail, code, status, fields))
}

func encodeProblemJSON(title, detail, code string, status int, fields []FieldError) []byte {
	b := append([]byte(nil), `{"type":"about:blank","title":`...)
	b = jsonbind.AppendString(b, title)
	b = append(b, `,"status":`...)
	b = jsonbind.AppendInt(b, int64(status))
	b = append(b, `,"detail":`...)
	b = jsonbind.AppendString(b, detail)
	b = append(b, `,"code":`...)
	b = jsonbind.AppendString(b, code)
	if len(fields) > 0 {
		b = append(b, `,"errors":[`...)
		for i, f := range fields {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, `{"field":`...)
			b = jsonbind.AppendString(b, f.Field)
			b = append(b, `,"location":`...)
			b = jsonbind.AppendString(b, f.Location)
			b = append(b, `,"message":`...)
			b = jsonbind.AppendString(b, f.Message)
			b = append(b, '}')
		}
		b = append(b, ']')
	}
	return append(b, '}')
}

// WriteJSON is a helper for generated writers: encode a pre-built map/slice without
// reflecting over application structs. Content-Type is application/json.
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
	dst = append(dst, `{"Filename":`...)
	dst = jsonbind.AppendString(dst, f.Filename)
	dst = append(dst, `,"ContentType":`...)
	dst = jsonbind.AppendString(dst, f.ContentType)
	dst = append(dst, `,"Size":`...)
	dst = jsonbind.AppendInt(dst, f.Size)
	dst = append(dst, `,"Content":`...)
	if f.Content == nil {
		dst = append(dst, "null"...)
	} else {
		dst = append(dst, '"')
		dst = base64.StdEncoding.AppendEncode(dst, f.Content)
		dst = append(dst, '"')
	}
	return append(dst, '}')
}

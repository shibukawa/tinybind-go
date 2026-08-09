package bindcore

import "github.com/shibukawa/tinybind-go/jsonbind"

// ProblemContentType is the media type of an RFC 9457 error document.
const ProblemContentType = "application/problem+json"

// ProblemResponse derives the status and body of the RFC 9457 document for err.
// ok is false when err is nil and nothing should be written.
//
// Both transport runtimes call this rather than deriving the document
// themselves, which is what makes their error bytes identical by construction
// instead of by two implementations agreeing.
func ProblemResponse(err error) (status int, body []byte, ok bool) {
	if err == nil {
		return 0, nil, false
	}
	status = 500
	title := "Internal Server Error"
	detail := "internal error"
	code := "internal"
	var fields []FieldError

	if he, found := AsHTTPError(err); found {
		status = he.Status
		if he.Title != "" {
			title = he.Title
		} else {
			title = StatusText(status)
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
	return status, encodeProblemJSON(title, detail, code, status, fields), true
}

// encodeProblemJSON writes the problem document without encoding/json so TinyGo
// does not hit unimplemented reflect.AssignableTo when binders also use
// json.RawMessage (a known interaction in TinyGo's encoding/json).
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

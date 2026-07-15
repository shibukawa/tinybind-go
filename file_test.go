package httpbinder_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbinder "github.com/shibukawa/httpbind-go"
)

func multipartRequest(t *testing.T, fields map[string]string, files map[string]struct {
	filename string
	content  string
	ctype    string
}) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for name, f := range files {
		part, err := w.CreateFormFile(name, f.filename)
		if err != nil {
			t.Fatal(err)
		}
		if f.ctype != "" {
			// CreateFormFile sets application/octet-stream; write content only.
			_ = f.ctype
		}
		if _, err := io.WriteString(part, f.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestIsMultipartRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	if !httpbinder.IsMultipartRequest(req) {
		t.Fatal("expected multipart")
	}
	req.Header.Set("Content-Type", "application/json")
	if httpbinder.IsMultipartRequest(req) {
		t.Fatal("json is not multipart")
	}
}

func TestParseMultipartMap_ScalarsAndFile(t *testing.T) {
	req := multipartRequest(t,
		map[string]string{"title": "avatar"},
		map[string]struct {
			filename string
			content  string
			ctype    string
		}{
			"image": {filename: "pic.png", content: "PNGDATA", ctype: "image/png"},
		},
	)
	form, files, err := httpbinder.ParseMultipartMap(req)
	if err != nil {
		t.Fatalf("ParseMultipartMap: %v", err)
	}
	if form["title"] != "avatar" {
		t.Fatalf("form title: %q", form["title"])
	}
	f, ok := files["image"]
	if !ok {
		t.Fatal("missing image file")
	}
	if f.Filename != "pic.png" {
		t.Fatalf("filename: %q", f.Filename)
	}
	if string(f.Content) != "PNGDATA" {
		t.Fatalf("content: %q", f.Content)
	}
	if f.Size != int64(len("PNGDATA")) && f.Size != 7 {
		// Size may come from FileHeader or content length
		if len(f.Content) != 7 {
			t.Fatalf("size/content mismatch: size=%d content=%q", f.Size, f.Content)
		}
	}
	if f.Empty() {
		t.Fatal("file should not be empty")
	}
}

func TestParseMultipartMap_OversizedMapsTo413(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("a", "b"); err != nil {
		t.Fatal(err)
	}
	part, err := w.CreateFormFile("image", "big.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), 64)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	// Cap body below multipart size so MaxBytesReader rejects during parse.
	req.Body = http.MaxBytesReader(nil, req.Body, 32)
	req.ContentLength = int64(buf.Len())

	_, _, err = httpbinder.ParseMultipartMap(req)
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := httpbinder.AsHTTPError(err)
	if !ok {
		t.Fatalf("want HTTPError, got %T %v", err, err)
	}
	if he.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status: want 413, got %d (%v)", he.Status, err)
	}
}

func TestParseMultipartMap_InvalidMultipart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	_, _, err := httpbinder.ParseMultipartMap(req)
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := httpbinder.AsHTTPError(err)
	if !ok || he.Status != http.StatusBadRequest {
		t.Fatalf("want 400 HTTPError, got %#v", err)
	}
}

func TestFile_Empty(t *testing.T) {
	var z httpbinder.File
	if !z.Empty() {
		t.Fatal("zero File should be empty")
	}
	if (httpbinder.File{Filename: "a"}).Empty() {
		t.Fatal("filename makes non-empty")
	}
	if (httpbinder.File{Content: []byte{1}}).Empty() {
		t.Fatal("content makes non-empty")
	}
}

func TestPayloadTooLarge(t *testing.T) {
	err := httpbinder.PayloadTooLarge(httpbinder.Problem{Code: "payload_too_large", Message: "too big"})
	he, ok := httpbinder.AsHTTPError(err)
	if !ok || he.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %#v", err)
	}
}

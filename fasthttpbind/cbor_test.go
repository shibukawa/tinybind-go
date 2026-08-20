package fasthttpbind

import (
	"testing"

	"github.com/shibukawa/tinygodriver/fasthttp"
)

func TestIsCBORRequest(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType("application/cbor")
	if !IsCBORRequest(ctx) {
		t.Fatal("application/cbor not recognized")
	}
	ctx.Request.Header.SetContentType("application/json")
	if IsCBORRequest(ctx) {
		t.Fatal("application/json recognized as CBOR")
	}
}

func TestAcceptsCBOR(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	if AcceptsCBOR(ctx) {
		t.Fatal("no Accept header accepted CBOR")
	}
	ctx.Request.Header.Set("Accept", "application/json, application/cbor")
	if !AcceptsCBOR(ctx) {
		t.Fatal("explicit application/cbor not accepted")
	}
	ctx.Request.Header.Set("Accept", "*/*")
	if AcceptsCBOR(ctx) {
		t.Fatal("a wildcard flipped the response format")
	}
}

func TestReadCBORBodyHonoursTheLimitAndCopies(t *testing.T) {
	SetMaxCBORBodyBytes(8)
	defer SetMaxCBORBodyBytes(0)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody([]byte("123456789"))
	if _, err := ReadCBORBody(ctx); err == nil {
		t.Fatal("an oversize body was read")
	}

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.SetBody([]byte("12345678"))
	data, err := ReadCBORBody(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "12345678" {
		t.Fatalf("read %q", data)
	}
	// The bytes are owned, not borrowed from the pooled request.
	ctx.Request.SetBody([]byte("overwrite"))
	if string(data) != "12345678" {
		t.Fatalf("body was borrowed: %q", data)
	}
}

func TestWriteCBORBytes(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	if err := WriteCBORBytes(ctx, 201, []byte{0xa0}); err != nil {
		t.Fatal(err)
	}
	if got := ctx.Response.StatusCode(); got != 201 {
		t.Fatalf("status %d", got)
	}
	if ct := string(ctx.Response.Header.ContentType()); ct != "application/cbor" {
		t.Fatalf("content type %q", ct)
	}
	if body := ctx.Response.Body(); len(body) != 1 || body[0] != 0xa0 {
		t.Fatalf("body %x", body)
	}
}

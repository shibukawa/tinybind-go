package mappingfixture

import (
	"encoding/json"

	"github.com/shibukawa/tinybind-go"
)

// CreateUserRequest exercises default input, path, and header sources.
type CreateUserRequest struct {
	Name  string
	Email string
	OrgID string `path:"org_id"`
	Token string `header:"Authorization"`
}

// CreateUserResponse is a normal JSON response.
type CreateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SearchRequest restricts sources with query/payload tags.
type SearchRequest struct {
	Keyword string `query:"keyword"`
	Page    int    `query:"page"`
	Filter  string `payload:"filter"`
}

// SearchResponse is returned from search.
type SearchResponse struct {
	Keyword string `json:"keyword"`
	Page    int    `json:"page"`
	Filter  string `json:"filter"`
}

// UploadAvatarRequest exercises multipart File + scalar form fields + path.
type UploadAvatarRequest struct {
	UserID string        `path:"user_id"`
	Title  string        `payload:"title"`
	Image  httpbind.File `payload:"image"`
}

// PatchWithExtrasRequest exercises payload:"*" rest map for leftover body keys.
type PatchWithExtrasRequest struct {
	Name  string         `payload:"name"`
	Email string         `payload:"email"`
	Extra map[string]any `payload:"*"`
}

// PatchWithExtrasRawRequest exercises rest as map[string]json.RawMessage.
type PatchWithExtrasRawRequest struct {
	Name  string                     `payload:"name"`
	Extra map[string]json.RawMessage `payload:"*"`
}

// NestedCustomer is a nested object in NestedOrderRequest.
type NestedCustomer struct {
	ID   string `payload:"id"`
	Name string `payload:"name"`
}

// NestedLineItem is an element of NestedOrderRequest.Items.
type NestedLineItem struct {
	SKU string `payload:"sku"`
	Qty int    `payload:"qty"`
}

// NestedOrderRequest exercises nested struct, slice of structs, and map[string]string.
type NestedOrderRequest struct {
	Customer NestedCustomer    `payload:"customer"`
	Items    []NestedLineItem  `payload:"items"`
	Labels   map[string]string `payload:"labels"`
}

// CodecOnlyNote is referenced only via DecodeJSON/EncodeJSON in tests.
type CodecOnlyNote struct {
	Text string `payload:"text"`
	N    int    `payload:"n"`
}

// OmitInner is the nested object OmitTagged asks about with omitzero.
type OmitInner struct {
	A string `json:"a"`
	B []int  `json:"b"`
}

// OmitTagged exercises every json tag option the encoder acts on, and the order
// that makes the separators interesting: omittable members first, so the comma
// before Type is only settled at run time, then one after Type whose comma is
// certain again.
type OmitTagged struct {
	Lead   string            `json:"lead,omitempty"`
	Live   int               `json:"live,omitzero"`
	Tags   []string          `json:"tags,omitempty"`
	Meta   map[string]string `json:"meta,omitzero"`
	Sub    OmitInner         `json:"sub,omitzero"`
	Type   string            `json:"type"`
	After  string            `json:"after,omitempty"`
	Secret string            `json:"-"`
}

// OmitRest pairs an omittable member with a rest map, which is what leaves the
// comma in front of the rest members undecided until run time.
type OmitRest struct {
	Note  string         `payload:"note" json:"note,omitempty"`
	Extra map[string]any `payload:"*"`
}

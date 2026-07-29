package checkfixture

// CheckRequest exercises v1 check-tag validation on query/input fields.
type CheckRequest struct {
	Name     string `query:"name" check:"required,minlen=1,maxlen=64"`
	Email    string `query:"email" check:"required,email,maxlen=254"`
	Age      int    `query:"age" check:"min=0,max=150"`
	Sort     string `query:"sort" enum:"asc,desc"`
	Code     string `query:"code" check:"pattern=^[A-Z]{3}$"`
	ID       string `query:"id" check:"uuid"`
	Born     string `query:"born" check:"date"`
	At       string `query:"at" check:"time"`
	When     string `query:"when" check:"datetime"`
	Page     int    `query:"page" check:"min=1" default:"-1"`
	Optional string `query:"optional" check:"email"`
}

// CheckResponse is a simple write target so codegen emits a writer.
type CheckResponse struct {
	OK bool `json:"ok"`
}

// OpenAPICheckRequest is registered via a handler in handlers.go for OpenAPI tests.
type OpenAPICheckRequest struct {
	Name  string `payload:"name" check:"required,minlen=1"`
	Age   int    `payload:"age" check:"min=1,max=120"`
	Sort  string `query:"sort" enum:"asc,desc" default:"asc"`
	Email string `payload:"email" check:"email"`
}

// OpenAPICheckResponse is the success body.
type OpenAPICheckResponse struct {
	OK bool `json:"ok"`
}

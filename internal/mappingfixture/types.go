package mappingfixture

import "github.com/shibukawa/httpbind-go"

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
	UserID string          `path:"user_id"`
	Title  string          `payload:"title"`
	Image  httpbinder.File `payload:"image"`
}

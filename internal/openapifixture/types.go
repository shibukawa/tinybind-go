package openapifixture

// CreateUserRequest exercises default input, path, and header for OpenAPI mapping.
type CreateUserRequest struct {
	// Name is the display name of the new user.
	Name string
	// Email is the contact address of the new user.
	Email string
	OrgID string `path:"org_id"` // OrgID owns the created user.
	Token string `header:"Authorization"`
}

// CreateUserResponse is the success body for create user.
type CreateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SearchRequest uses restricted query/payload sources.
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

package benchfixture

import (
	"io"
	"net/http"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

// --- jsonbind: a nested document ---

type Customer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

type LineItem struct {
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type Order struct {
	ID       string     `json:"id"`
	Customer Customer   `json:"customer"`
	Items    []LineItem `json:"items"`
	Total    float64    `json:"total"`
	Paid     bool       `json:"paid"`
	Tags     []string   `json:"tags"`
	Note     string     `json:"note"`
}

func DecodeOrder(r io.Reader) (Order, error) { return jsonbind.DecodeJSON[Order](r) }

func EncodeOrder(w io.Writer, v Order) error { return jsonbind.EncodeJSON(w, v) }

// --- httpbind: a request bound from body + path + header ---

type CreateUserRequest struct {
	Name  string `input:"name"`
	Email string `input:"email"`
	OrgID string `path:"org_id"`
	Token string `header:"Authorization"`
}

type CreateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	OrgID string `json:"org_id"`
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write(w, r, CreateUserResponse{
		ID:    "u_1",
		Name:  in.Name,
		Email: in.Email,
		OrgID: in.OrgID,
	})
}

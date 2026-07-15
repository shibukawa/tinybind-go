package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/shibukawa/httpbind-go"
	"github.com/shibukawa/httpbind-go/internal/mappingfixture"
)

func main() {
	body := `{"name":"Alice","email":"a@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	req.SetPathValue("org_id", "acme")
	got, err := httpbinder.Bind[mappingfixture.CreateUserRequest](req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("BIND name=%q email=%q org=%q token=%q\n", got.Name, got.Email, got.OrgID, got.Token)

	rec := httptest.NewRecorder()
	wreq := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := httpbinder.Write[mappingfixture.CreateUserResponse](rec, wreq, mappingfixture.CreateUserResponse{
		ID: "user_123", Name: "Alice", Email: "a@example.com",
	}); err != nil {
		panic(err)
	}
	fmt.Printf("WRITE status=%d ctype=%q body=%s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	fmt.Printf("WRITE decoded=%v\n", m)
}

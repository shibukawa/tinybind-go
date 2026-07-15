package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/shibukawa/httpbind-go"
	"github.com/shibukawa/httpbind-go/internal/mappingfixture"
)

func main() {
	fmt.Println("start")
	body := `{"name":"Alice","email":"a@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	req.SetPathValue("org_id", "acme")
	fmt.Println("request ready")
	got, err := httpbinder.Bind[mappingfixture.CreateUserRequest](req)
	fmt.Printf("result err=%v got=%+v\n", err, got)
	if err != nil {
		panic(err)
	}
	if got.Name != "Alice" || got.Email != "a@example.com" || got.OrgID != "acme" || got.Token != "Bearer secret" {
		panic(fmt.Sprintf("unexpected: %+v", got))
	}
	fmt.Println("bind ok")

	rec := httptest.NewRecorder()
	wreq := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := httpbinder.Write[mappingfixture.CreateUserResponse](rec, wreq, mappingfixture.CreateUserResponse{
		ID: "user_123", Name: "Alice", Email: "a@example.com",
	}); err != nil {
		panic(err)
	}
	fmt.Printf("write status=%d body=%s\n", rec.Code, rec.Body.String())
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "user_123") {
		panic("write failed")
	}
	fmt.Println("write ok")
}

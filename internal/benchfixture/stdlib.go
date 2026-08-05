package benchfixture

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
)

// stdlibCreateUser is the hand-written equivalent of the generated handler:
// same sources, same response shape, no binding library.
func stdlibCreateUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}
	if q := r.URL.Query(); in.Name == "" || in.Email == "" {
		if v := q.Get("name"); v != "" {
			in.Name = v
		}
		if v := q.Get("email"); v != "" {
			in.Email = v
		}
	}
	orgID := r.PathValue("org_id")
	_ = r.Header.Get("Authorization")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(CreateUserResponse{
		ID:    "u_1",
		Name:  in.Name,
		Email: in.Email,
		OrgID: orgID,
	})
}

// stdlibPage renders the same document as the UserPage component.
var stdlibPage = template.Must(template.New("page").Parse(
	`<!DOCTYPE html>` +
		`<html lang="en"><head><title>{{.Title}}</title></head><body><h1>{{.Title}}</h1><ul>` +
		`{{range $i, $row := .Rows}}<li data-index="{{$i}}"><span class="name">{{$row.Name}}</span>` +
		`<span class="email">{{$row.Email}}</span>{{if $row.Active}}<em>active</em>{{end}}</li>{{end}}` +
		`</ul></body></html>`))

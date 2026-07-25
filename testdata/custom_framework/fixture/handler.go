package fixture

import (
	"net/http"

	"tempmod/pw"
)

// CreateUserRequest is the parsed request body.
type CreateUserRequest struct {
	Name string `json:"name"`
}

// CreateUserResponse is the API response body.
type CreateUserResponse struct {
	ID int `json:"id"`
}

func createUser(w http.ResponseWriter, r *http.Request) {
	request, err := pw.Parse[CreateUserRequest](r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = pw.WriteAPI(w, r, CreateUserResponse{ID: len(request.Name)})
}

func showPage(w http.ResponseWriter, r *http.Request) {
	_ = pw.WriteHTML(w, r, UserPage(UserPageParams{User: User{Name: "Ada"}}))
}

// Register wires the fixture routes.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /users", createUser)
	mux.HandleFunc("GET /page", showPage)
}

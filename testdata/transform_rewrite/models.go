// Package app is laid out the way decision:backend-build-tag-mode needs: the
// shared declarations sit in an untagged file, and the transport handlers in a
// file the tag can exclude whole.
package app

type CreateUserRequest struct {
	Name string
}

type CreateUserResponse struct {
	ID   string
	Name string
}

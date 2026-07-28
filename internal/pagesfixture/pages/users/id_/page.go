package id_

import "strings"

// Load loads the display name for one user. It is the typed rung: the request
// reaches Go first and the component parameters are this function's results.
func Load(id string) (string, error) {
	if id == "" {
		return "", nil
	}
	return strings.ToUpper(id), nil
}

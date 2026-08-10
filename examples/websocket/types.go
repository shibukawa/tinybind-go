package main

// ClientMsg is everything the browser can send. One direction, one type, with
// the variants told apart by Type — the same shape a stream's events use.
//
// A variant that does not use a field leaves it zero, and the generated
// encoder writes it anyway: the JSON codec reads a json tag for the field's
// name and ignores omitempty, so there is no point spelling it. That the union
// carries fields a given variant does not need is the admitted cost of typing
// a direction with one struct; the alternative is a library-owned dispatch,
// which would put the discriminator's spelling in the library rather than
// here.
type ClientMsg struct {
	Type string `json:"type"` // "join" | "say" | "leave"
	Name string `json:"name"` // join
	Text string `json:"text"` // say
}

// ServerMsg is everything the server can send back.
type ServerMsg struct {
	Type string `json:"type"` // "welcome" | "message" | "presence" | "error"
	From string `json:"from"` // message
	Text string `json:"text"` // welcome, message, presence
	Code string `json:"code"` // error
	Live int    `json:"live"` // welcome, presence
}

// HealthResponse is an ordinary REST response, here to show the socket living
// on the same port and the same mux as everything else.
type HealthResponse struct {
	Status string `json:"status"`
	Live   int    `json:"live"`
}

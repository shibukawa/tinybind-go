package main

// ClientMsg is everything the browser can send. One direction, one type, with
// the variants told apart by Type — the same shape a stream's events use.
//
// A variant that does not use a field leaves it zero, and omitzero keeps it off
// the wire: a "say" carries Type and Text, not an empty Name beside them. That
// the union still declares fields a given variant does not need is the admitted
// cost of typing a direction with one struct; the alternative is a
// library-owned dispatch, which would put the discriminator's spelling in the
// library rather than here.
type ClientMsg struct {
	Type string `json:"type"`          // "join" | "say" | "leave"
	Name string `json:"name,omitzero"` // join
	Text string `json:"text,omitzero"` // say
}

// ServerMsg is everything the server can send back. Same union, same use of
// omitzero — except on Live, where zero is a real count and not the absence of
// one: a presence event saying the room emptied has to say `"live":0` rather
// than leave the reader to infer it.
type ServerMsg struct {
	Type string `json:"type"`          // "welcome" | "message" | "presence" | "error"
	From string `json:"from,omitzero"` // message
	Text string `json:"text,omitzero"` // welcome, message, presence
	Code string `json:"code,omitzero"` // error
	Live int    `json:"live"`          // welcome, presence
}

// HealthResponse is an ordinary REST response, here to show the socket living
// on the same port and the same mux as everything else.
type HealthResponse struct {
	Status string `json:"status"`
	Live   int    `json:"live"`
}

package main

import "sync"

// The hub is the application's, not the library's. httpbind owns one
// connection each — its limits, its deadlines, its close — and deliberately
// ships no registry, because who may hear what is an application's design
// rather than a transport concern.
//
// What the library does give this file is the reason it can stay this short:
// Socket.Write is safe from any goroutine, so broadcasting means holding a
// slice of sockets and writing to them. Against a raw gorilla connection this
// would need a per-connection outbound channel and a writer goroutine, because
// two goroutines writing to one connection interleave frames with no error.
type hub struct {
	mu      sync.Mutex
	members map[*chatSocket]string // socket -> display name
}

type chatSocket = socketOf

func newHub() *hub {
	return &hub{members: map[*chatSocket]string{}}
}

func (h *hub) join(s *chatSocket, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.members[s] = name
}

func (h *hub) leave(s *chatSocket) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.members, s)
}

func (h *hub) size() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.members)
}

// name reports the display name of a member, and whether it is one at all.
func (h *hub) name(s *chatSocket) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	name, ok := h.members[s]
	return name, ok
}

// broadcast sends to every member from the caller's goroutine.
//
// The sockets are copied out under the lock and written to outside it: a slow
// peer must not hold the registry while its write deadline runs down.
func (h *hub) broadcast(msg ServerMsg) {
	h.mu.Lock()
	targets := make([]*chatSocket, 0, len(h.members))
	for s := range h.members {
		targets = append(targets, s)
	}
	h.mu.Unlock()

	for _, s := range targets {
		// A failed write is that peer's problem: its own callback will see the
		// error on its next turn and end. Broadcasting is not the place to
		// unwind someone else's connection.
		_ = s.Write(msg)
	}
}

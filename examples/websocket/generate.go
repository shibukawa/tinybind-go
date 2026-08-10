package main

// No -generate-all here on purpose: discovery alone is what should produce a
// decoder for ClientMsg, an encoder for ServerMsg, and a writer for
// HealthResponse — one codec each, in the direction that type is actually
// used. Regenerating and finding anything more means discovery got wider than
// it should be, and finding anything less means the socket stopped being
// discovered at all.
//
//go:generate go run ../../cmd/tinybind-gen generate -dir . -name tinybind_gen.go

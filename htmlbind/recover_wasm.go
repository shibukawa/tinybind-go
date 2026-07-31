//go:build tinygo && wasm

package htmlbind

// panicRecovery is false on TinyGo's wasm targets, where recover does not run
// at all: a panic traps and the program stops.
//
// The consequence is that a panicking async external or live source cannot be
// normalized into an AsyncError there, so it ends the program rather than
// rendering a recover subtree. Nothing in the runtime can paper over that, so
// the code that would rely on it says so instead.
const panicRecovery = false

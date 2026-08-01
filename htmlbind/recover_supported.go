//go:build !(tinygo && wasm)

package htmlbind

// panicRecovery reports whether recover works on this build. Everywhere except
// TinyGo's wasm targets it does, which is what lets a panic in an external
// become an AsyncError instead of ending the process.
const panicRecovery = true

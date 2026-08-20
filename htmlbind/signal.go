package htmlbind

import "errors"

// Signal is the module half of an application signal: a named instruction the
// server sends to client code, travelling beside the deliveries a live boundary
// renders rather than replacing any of them.
//
// An application declares its own type and embeds this one:
//
//	type Toast struct{ htmlbind.Signal }
//
//	func NewToast(text string) Toast {
//		return Toast{htmlbind.NewSignal("app.toast", toastPayload{Text: text})}
//	}
//
// Embedding promotes Error, so the application type satisfies error with no
// boilerplate, which is what lets a live source yield one in the error position
// of its sequence without any signature growing a third value. It also promotes
// an unexported accessor, and that accessor is how the runtime recognizes a
// signal: an unexported method name is qualified by the package that declared
// it, so nothing outside this package can claim to be a signal without
// embedding this struct.
//
// A bare Signal is usable on its own; the embed exists for an application that
// wants a named Go type per signal, so its emit sites are type-checked.
//
// The runtime never reads an application's own fields. A signal type may hold
// whatever it likes beside the embed.
type Signal struct {
	name string
	data []byte
	// set separates a constructed signal from an embedded zero one. An
	// application writing Toast{} satisfies the accessor and carries no name,
	// and that has to be a reported fault rather than an unnamed signal on the
	// wire. A private field is what makes the difference visible; it is not
	// what identifies a signal, because no package can read it from outside.
	set bool
	// bad holds a construction fault. It travels with the value rather than
	// being returned, because a constructor is called at a yield site where an
	// extra error result has nowhere to go.
	bad error
}

// SignalPayload is a value that can append itself as one JSON value.
//
// It is the interface a generated encoder already satisfies, and taking it
// rather than a type parameter is what keeps this package free of the codec
// registry: the payload encodes itself, so nothing here has to find a codec for
// a type it only holds as an interface.
type SignalPayload interface {
	AppendJSON(dst []byte) []byte
}

// ErrSignal matches any signal under errors.Is, for a caller that wants the
// classification without the value. Reading the name or the payload needs
// AsSignal.
var ErrSignal = errors.New("htmlbind: signal")

// signalCarrier is satisfied only by a type embedding Signal. It is the sealed
// interface the runtime asserts against, and the reason recognition is a plain
// type assertion rather than errors.As — which would link reflect, the thing
// findPublicError is written out by hand to avoid.
type signalCarrier interface {
	signalValue() Signal
}

func (s Signal) signalValue() Signal { return s }

// Error reports the signal by name. It deliberately omits the payload: an
// unclassified signal reaches a log or an error page, and error values are
// printed by code that has no idea what this one is.
func (s Signal) Error() string {
	if s.bad != nil {
		return "htmlbind: invalid signal: " + s.bad.Error()
	}
	if !s.set {
		return "htmlbind: invalid signal: uninitialized"
	}
	return "htmlbind: signal " + s.name
}

// Is reports ErrSignal, so errors.Is classifies a signal without this package
// exporting the accessor that identifies one.
func (s Signal) Is(target error) bool { return target == ErrSignal }

// Name is the dispatch key the client looks up.
func (s Signal) Name() string { return s.name }

// Payload is the encoded JSON value, or nil when the signal carries none. The
// bytes belong to the signal; a caller that keeps them past the write must copy
// them.
func (s Signal) Payload() []byte { return s.data }

// fault reports why this value is not a usable signal, or nil when it is one.
func (s Signal) fault() error {
	if s.bad != nil {
		return s.bad
	}
	if !s.set {
		return errUninitializedSignal
	}
	return nil
}

// AppendJSON appends the signal as a JSON object with a name and an optional
// data field, and returns the extended slice.
//
// The name is escaped for a script context as well as a JSON one, using the
// same rules as every other record this package writes, so the result stays
// safe to embed in an inline data block as well as to send as a body. The
// payload is appended as the encoder produced it. Framing around the record is
// the caller's, as it is for a completion.
func (s Signal) AppendJSON(dst []byte) []byte {
	dst = append(dst, `{"name":`...)
	dst = AppendJSONString(dst, s.name)
	if len(s.data) > 0 {
		dst = append(dst, `,"data":`...)
		dst = append(dst, s.data...)
	}
	return append(dst, '}')
}

// NewSignal builds a signal carrying an encoded payload.
//
// The payload is encoded here, at the call site, so the value is immutable once
// yielded and the runtime seam holds bytes rather than something it would have
// to reflect on to write. A nil payload is legal and means an instruction with
// no arguments.
func NewSignal(name string, payload SignalPayload) Signal {
	signal := NamedSignal(name)
	if signal.bad != nil || payload == nil {
		return signal
	}
	signal.data = payload.AppendJSON(nil)
	return signal
}

// NewRawSignal builds a signal from a payload that is already encoded JSON.
//
// Nothing here validates those bytes. A caller passing something that is not
// one JSON value produces a record a client cannot parse, which is why the
// encoding path above is the ordinary one.
func NewRawSignal(name string, payload []byte) Signal {
	signal := NamedSignal(name)
	if signal.bad != nil {
		return signal
	}
	signal.data = payload
	return signal
}

// NamedSignal builds a signal with no payload.
func NamedSignal(name string) Signal {
	if err := checkSignalName(name); err != nil {
		return Signal{bad: err}
	}
	return Signal{name: name, set: true}
}

// AsSignal reports whether err is a signal, and returns it.
//
// It walks the wrap chain the way findPublicError does, for the same reason:
// errors.As reflects on its target, and nothing here may link reflect. A joined
// error reports the first signal in its list, so errors.Join of two signals is
// read by calling this once per forwarded value rather than once per join.
func AsSignal(err error) (Signal, bool) {
	for err != nil {
		if carrier, ok := err.(signalCarrier); ok {
			return carrier.signalValue(), true
		}
		switch wrapper := err.(type) {
		case interface{ Unwrap() error }:
			err = wrapper.Unwrap()
		case interface{ Unwrap() []error }:
			for _, wrapped := range wrapper.Unwrap() {
				if signal, ok := AsSignal(wrapped); ok {
					return signal, true
				}
			}
			return Signal{}, false
		default:
			return Signal{}, false
		}
	}
	return Signal{}, false
}

// signalFaultError reports a value that reached the runtime as a signal but was
// not usable as one: an embed that was never constructed, or a name a client
// could not have dispatched.
//
// It deliberately does not wrap the signal itself. A caller classifies with
// AsSignal, which walks the wrap chain, so a fault that stayed recognizable as
// a signal would be skipped as one instead of ending the boundary it ended.
type signalFaultError struct{ err error }

func (e *signalFaultError) Error() string { return "htmlbind: invalid signal: " + e.err.Error() }

func (e *signalFaultError) Unwrap() error { return e.err }

// reservedSignalPrefix belongs to the module. An application may register a
// client handler for a reserved name and may never emit one: a client trusts a
// lifecycle name precisely because only its own runtime produces it, and a name
// that reached one from application data would be an instruction forged by
// whoever filled that data.
const reservedSignalPrefix = "tb."

// maxSignalNameLen bounds the dispatch key. It is looked up in a table, and an
// unbounded key is a way to spend memory on a miss.
const maxSignalNameLen = 64

var (
	errUninitializedSignal = errors.New("signal was embedded but never constructed")
	errEmptySignalName     = errors.New("signal name is empty")
	errLongSignalName      = errors.New("signal name is too long")
	errReservedSignalName  = errors.New("signal name uses the reserved " + reservedSignalPrefix + " prefix")
	errSignalNameChar      = errors.New("signal name holds a character outside [A-Za-z0-9._-] or does not start with a letter")
)

// checkSignalName keeps a name a lookup key. It is never a selector, a path, or
// an expression, and the client resolves it against its own table alone, so the
// charset is bounded here rather than trusted there.
func checkSignalName(name string) error {
	switch {
	case name == "":
		return errEmptySignalName
	case len(name) > maxSignalNameLen:
		return errLongSignalName
	case len(name) >= len(reservedSignalPrefix) && name[:len(reservedSignalPrefix)] == reservedSignalPrefix:
		return errReservedSignalName
	}
	if !isSignalNameLetter(name[0]) {
		return errSignalNameChar
	}
	for index := 1; index < len(name); index++ {
		char := name[index]
		if isSignalNameLetter(char) || (char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return errSignalNameChar
	}
	return nil
}

func isSignalNameLetter(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

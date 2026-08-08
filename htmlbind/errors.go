package htmlbind

// wrappedError carries a prebuilt message alongside the error it extends, so a
// sentinel stays matchable through errors.Is and errors.Unwrap without pulling
// fmt's formatter into the runtime.
type wrappedError struct {
	msg string
	err error
}

func (e *wrappedError) Error() string { return e.msg }

func (e *wrappedError) Unwrap() error { return e.err }

// wrapError extends a sentinel with detail text, producing the same message
// fmt.Errorf("%w"+detail, err) would.
func wrapError(err error, detail string) error {
	return &wrappedError{msg: err.Error() + detail, err: err}
}

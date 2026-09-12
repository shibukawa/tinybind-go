package jsonbind

import "unsafe"

func unsafeStringData(s string) *byte { return unsafe.StringData(s) }

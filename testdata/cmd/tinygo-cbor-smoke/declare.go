package main

import "github.com/shibukawa/tinybind-go/cborbind"

var _ = cborbind.GenerateWireCodec[PlayerInput]()

var _ = cborbind.GenerateWorldDelta[World]()

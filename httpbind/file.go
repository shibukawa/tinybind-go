package httpbind

import "github.com/shibukawa/tinybind-go/internal/bindcore"

// File is an uploaded file bound from a multipart/form-data part.
// After a successful bind, Filename, ContentType (when the client sent one),
// Size, and Content are populated from the named file part.
//
// It is an alias so a model struct declaring a File field compiles against
// either transport runtime without naming one of them.
type File = bindcore.File

package bindcore

// File is an uploaded file bound from a multipart/form-data part.
// After a successful bind, Filename, ContentType (when the client sent one),
// Size, and Content are populated from the named file part.
type File struct {
	Filename    string
	ContentType string
	Size        int64
	Content     []byte
}

// Empty reports whether f has no filename and no content.
func (f File) Empty() bool {
	return f.Filename == "" && len(f.Content) == 0
}

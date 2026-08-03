package syntax

import "strings"

// Comment placement for the declaration languages, which parse by tokenizing and
// therefore drop comments on the way through.
//
// The HTML and SQL languages carry comments in their own syntax tree and need
// none of this. The .tb.dynamo and .tb.firestore grammars do not, so their
// formatters read the comments from the source in a second pass and place them
// by line. Both do it the same way, which is why the machinery lives here rather
// than once per grammar.

// DeclComment is one comment kept for requirement:template-comment-retention.
type DeclComment struct {
	Line        int
	Text        string
	Trailing    bool
	BlankBefore bool
	used        bool
}

// ScanDeclComments collects every comment with the line it sits on.
//
// It scans the raw source rather than the token stream, so a comment survives a
// grammar that discards it. Only "//" comments exist in these languages.
func ScanDeclComments(source string) []DeclComment {
	var out []DeclComment
	line := 1
	blankRun := 0
	lineHasContent := false
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\n':
			if !lineHasContent {
				blankRun++
			} else {
				blankRun = 0
			}
			lineHasContent = false
			line++
			i++
		case source[i] == ' ' || source[i] == '\t' || source[i] == '\r':
			i++
		case source[i] == '/' && i+1 < len(source) && source[i+1] == '/':
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				end = len(source) - i
			}
			out = append(out, DeclComment{
				Line:        line,
				Text:        strings.TrimRight(source[i:i+end], " \t\r"),
				Trailing:    lineHasContent,
				BlankBefore: blankRun > 0 && !lineHasContent,
			})
			lineHasContent = true
			i += end
		default:
			lineHasContent = true
			i++
		}
	}
	return out
}

// DeclFormatter places scanned comments around whatever a grammar's own
// formatter writes. It owns the blank lines between constructs, so a comment
// stays attached to what it documents.
type DeclFormatter struct {
	P        *Printer
	comments []DeclComment
	wrote    bool
	// afterComment and lastCommentLine keep a comment attached to what it
	// documents, exactly as the shared module printer does.
	afterComment    bool
	lastCommentLine int
}

// NewDeclFormatter starts a formatter over one source's comments.
func NewDeclFormatter(p *Printer, comments []DeclComment) *DeclFormatter {
	return &DeclFormatter{P: p, comments: comments}
}

// Wrote reports whether anything has been written yet, which is what decides
// whether a separator is wanted.
func (f *DeclFormatter) Wrote() bool { return f.wrote }

// MarkWrote records that the grammar's own formatter wrote something.
func (f *DeclFormatter) MarkWrote() {
	f.wrote = true
	f.afterComment = false
}

// SeparateFor opens the blank line above a construct, unless a comment sits
// directly on the line above it.
func (f *DeclFormatter) SeparateFor(line int) {
	attached := f.afterComment && line <= f.lastCommentLine+1
	f.afterComment = false
	if !f.wrote || attached {
		return
	}
	f.P.Blank()
}

// FlushBefore writes the standalone comments that stood above the given line.
func (f *DeclFormatter) FlushBefore(line int) {
	for i := range f.comments {
		c := &f.comments[i]
		if c.used || c.Trailing || c.Line >= line {
			continue
		}
		if f.wrote && (c.BlankBefore || !f.afterComment) {
			f.P.Blank()
		}
		f.P.Write(c.Text)
		f.P.Line()
		c.used = true
		f.wrote = true
		f.afterComment = true
		f.lastCommentLine = c.Line
	}
}

// FlushTrailing writes the comment that ended the given line, if any.
func (f *DeclFormatter) FlushTrailing(line int) {
	for i := range f.comments {
		c := &f.comments[i]
		if c.used || !c.Trailing || c.Line != line {
			continue
		}
		f.P.Write(" " + c.Text)
		c.used = true
	}
}

// FlushRemaining writes the comments that stood after every construct, so a
// trailing note at the end of a file is not dropped.
func (f *DeclFormatter) FlushRemaining() {
	for i := range f.comments {
		c := &f.comments[i]
		if c.used {
			continue
		}
		if f.wrote {
			f.P.Blank()
		}
		f.P.Write(c.Text)
		f.P.Line()
		c.used = true
	}
}

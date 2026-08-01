package syntax

import "sort"

// Comment is one comment read from the declaration part of a source file. The
// parser keeps it because a formatter that drops comments is a formatter nobody
// may run; every compiler stage ignores it.
type Comment struct {
	Pos Position `json:"pos"`
	// Text is the comment including its own delimiters, so a printer never has
	// to reconstruct whether it was a line or a block comment.
	Text string `json:"text"`
	// Block reports a /* */ comment.
	Block bool `json:"block,omitempty"`
	// Trailing reports a comment that began on a line which already had code on
	// it, which is what decides whether a printer keeps it on that line.
	Trailing bool `json:"trailing,omitempty"`
	// BlankBefore reports a blank line between the previous content and this
	// comment, so a deliberately detached comment stays detached.
	BlankBefore bool `json:"blankBefore,omitempty"`
	// offset is the file-global byte offset. It stays unexported for the same
	// reason Position carries no offset: later stages compare line and column.
	offset int
}

// commentSet collects comments during one parse. Backtracking re-scans the same
// bytes, so a comment is recorded by its start offset and never twice.
type commentSet struct {
	comments []Comment
	seen     map[int]bool
}

func (s *commentSet) add(c Comment) {
	if s.seen == nil {
		s.seen = map[int]bool{}
	}
	if s.seen[c.offset] {
		return
	}
	s.seen[c.offset] = true
	s.comments = append(s.comments, c)
}

// sorted returns the comments in source order. Backtracking means they are not
// recorded in it.
func (s *commentSet) sorted() []Comment {
	if len(s.comments) == 0 {
		return nil
	}
	out := make([]Comment, len(s.comments))
	copy(out, s.comments)
	sort.Slice(out, func(i, j int) bool { return out[i].offset < out[j].offset })
	return out
}

// CommentsBefore returns the module comments positioned before pos, and the
// remainder. A printer walks its declarations in order and flushes what belongs
// above each one, which is why attachment needs no separate pass.
func CommentsBefore(comments []Comment, pos Position) (before, rest []Comment) {
	for i, c := range comments {
		if c.Pos.Line > pos.Line || (c.Pos.Line == pos.Line && c.Pos.Col >= pos.Col) {
			return comments[:i], comments[i:]
		}
	}
	return comments, nil
}

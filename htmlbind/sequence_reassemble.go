package htmlbind

import (
	"errors"
	"strconv"
	"strings"
)

// ErrSequenceMismatch reports values that do not fit the sequence they were
// walked against, which means the two came from different renders or different
// builds.
var ErrSequenceMismatch = errors.New("htmlbind: values do not match the sequence")

// Reassemble rebuilds a fragment's markup from its sequence and the values one
// render produced.
//
// It is the reference for what a client does: walk the tree, take the static
// text as it stands, and consume one value at each hole, each conditional, and
// each loop. Nothing is escaped here, because the values were escaped by the
// render that produced them — which is the whole reason a client needs no
// escaping rules of its own.
//
// It exists on the server so the round trip is testable: a sequence plus its
// values must reproduce the bytes the render wrote, or the split is not a split.
func (s *Sequence) Reassemble(values []string) (string, error) {
	var out strings.Builder
	rest, err := walkSequence(&out, s.Nodes, values)
	if err != nil {
		return "", err
	}
	if len(rest) != 0 {
		return "", ErrSequenceMismatch
	}
	return out.String(), nil
}

func walkSequence(out *strings.Builder, nodes []SeqNode, values []string) ([]string, error) {
	var err error
	for _, node := range nodes {
		switch node.Kind {
		case SeqStatic:
			out.WriteString(node.Text)
		case SeqSlot:
			if len(values) == 0 {
				return nil, ErrSequenceMismatch
			}
			out.WriteString(values[0])
			values = values[1:]
		case SeqIf:
			if len(values) == 0 {
				return nil, ErrSequenceMismatch
			}
			branch, rest := values[0], values[1:]
			taken := node.Then
			if branch == seqBranchElse {
				taken = node.Else
			} else if branch != seqBranchThen {
				return nil, ErrSequenceMismatch
			}
			if values, err = walkSequence(out, taken, rest); err != nil {
				return nil, err
			}
		case SeqRepeat:
			if len(values) == 0 {
				return nil, ErrSequenceMismatch
			}
			count, convErr := strconv.Atoi(values[0])
			if convErr != nil || count < 0 {
				return nil, ErrSequenceMismatch
			}
			values = values[1:]
			for i := 0; i < count; i++ {
				if values, err = walkSequence(out, node.Then, values); err != nil {
					return nil, err
				}
			}
		case SeqComponent:
			if len(values) == 0 {
				return nil, ErrSequenceMismatch
			}
			shape, rest := values[0], values[1:]
			taken := node.Then
			if shape == seqInline {
				taken = node.Else
			} else if shape != seqBoundary {
				return nil, ErrSequenceMismatch
			}
			if values, err = walkSequence(out, taken, rest); err != nil {
				return nil, err
			}
		}
	}
	return values, nil
}

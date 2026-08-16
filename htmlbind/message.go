package htmlbind

import "context"

// MessageSegment is one piece of a resolved rich-text message.
//
// A catalog's generated function returns these rather than writing markup,
// which is what keeps escaping here: a text run arrives complete, with the
// message's own arguments already interpolated, and this module escapes it for
// the position it lands in. Nothing in a translation can introduce a tag.
//
// See .knowledge decision:message-hole-lowering.
type MessageSegment struct {
	// Hole names the hole this segment fills, empty for a literal run.
	Hole string
	// Text is the literal run, or the text inside the hole when Hole is set.
	Text string
}

// MessageHoleOps is the markup a template bound to one hole. Inner is written
// where the hole's element has its content.
type MessageHoleOps[P any] struct {
	Name string
	Ops  []Op[P]
}

// Message renders a rich-text message: the catalog decides the order of the
// text and the holes, and the template decides what each hole is made of.
//
// The interleaving is here rather than in the generated function because the
// text runs have to pass this module's escaper. A translation reordering its
// holes — which is the whole reason the form exists — changes only the segment
// order and nothing about the template.
func (Builder[P]) Message(segments func(context.Context, P) []MessageSegment, holes []MessageHoleOps[P]) Op[P] {
	return messageOp[P]{segments: segments, holes: holes}
}

type messageOp[P any] struct {
	segments func(context.Context, P) []MessageSegment
	holes    []MessageHoleOps[P]
}

func (o messageOp[P]) Exec(r *Renderer, params P) error {
	for _, segment := range o.segments(r.boundaryContext(), params) {
		if segment.Hole == "" {
			if err := r.WriteEscaped(segment.Text); err != nil {
				return err
			}
			continue
		}
		ops, ok := o.hole(segment.Hole)
		if !ok {
			// A hole the template did not bind cannot render, and dropping it
			// would lose the translated text inside it. Writing the text alone
			// keeps the sentence readable and loses only the markup, which is
			// the failure a reader can still act on.
			if err := r.WriteEscaped(segment.Text); err != nil {
				return err
			}
			continue
		}
		previous := r.messageInner
		r.messageInner = segment.Text
		err := execOps(r, ops, params)
		r.messageInner = previous
		if err != nil {
			return err
		}
	}
	return nil
}

func (o messageOp[P]) hole(name string) ([]Op[P], bool) {
	for _, hole := range o.holes {
		if hole.Name == name {
			return hole.Ops, true
		}
	}
	return nil, false
}

// MessageInner writes the translated text inside the hole being rendered. It is
// emitted where a bound element has its content, so the template supplies the
// element and the catalog supplies what is between its tags.
func (Builder[P]) MessageInner() Op[P] { return messageInnerOp[P]{} }

type messageInnerOp[P any] struct{}

func (messageInnerOp[P]) Exec(r *Renderer, params P) error {
	return r.WriteEscaped(r.messageInner)
}

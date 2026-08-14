package htmlbind

import (
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"sync"
)

// A sequence is the static half of a component's output, separated from the
// values that vary between renders.
//
// It is derived from the plan rather than from a render, which is what makes it
// addressable: a server can serve one back from its address because it can
// rebuild it from the plan, where a sequence assembled from whatever a render
// happened to produce would have to have been stored. That in turn is why it is
// a tree rather than a list — a conditional and a loop change which instructions
// run, and enumerating a sequence per path would be exponential in a component's
// conditionals. The branch taken and the iteration count travel with the values
// instead, so a five-row list and a six-row list share one address.

// SeqKind names what a sequence node contributes.
type SeqKind int

const (
	// SeqStatic is literal output, identical in every render.
	SeqStatic SeqKind = iota
	// SeqSlot is one instruction's output, which travels as a value.
	SeqSlot
	// SeqIf is a conditional. The value stream says which branch ran.
	SeqIf
	// SeqRepeat is a loop body. The value stream says how many times it ran.
	SeqRepeat
	// SeqComponent is a called component, which either opened an update
	// boundary — leaving a placeholder whose only varying part is the id — or
	// rendered inline, which is opaque because the callee's plan is chosen by a
	// closure this walk cannot evaluate. The value stream says which.
	SeqComponent
)

// SeqNode is one node of a sequence tree.
type SeqNode struct {
	Kind SeqKind
	// Text is the literal output of a static node.
	Text string
	// Then and Else are the branches of a conditional; Then alone is a loop
	// body, and the placeholder frame of a component node.
	Then []SeqNode
	Else []SeqNode
}

// Sequence is one component's static half, with the address a client caches it
// under.
type Sequence struct {
	// Address identifies this sequence. It is a digest of the tree, computed
	// once per plan rather than per request.
	Address string
	Nodes   []SeqNode
}

// The value stream markers. A conditional and a loop contribute one entry each,
// so a client walking the tree knows what to consume at every node.
const (
	seqBranchThen = "t"
	seqBranchElse = "f"
	seqBoundary   = "b"
	seqInline     = "i"
)

// sequenceOf derives the sequence of one instruction list.
func sequenceOf[P any](ops []Op[P]) []SeqNode {
	nodes := make([]SeqNode, 0, len(ops))
	for _, op := range ops {
		switch typed := op.(type) {
		case staticOp[P]:
			nodes = append(nodes, SeqNode{Kind: SeqStatic, Text: string(typed)})
		case ifOp[P]:
			nodes = append(nodes, SeqNode{
				Kind: SeqIf,
				Then: sequenceOf(typed.then),
				Else: sequenceOf(typed.otherwise),
			})
		case ifCtxOp[P]:
			nodes = append(nodes, SeqNode{
				Kind: SeqIf,
				Then: sequenceOf(typed.then),
				Else: sequenceOf(typed.otherwise),
			})
		case componentOp[P], componentCtxOp[P]:
			nodes = append(nodes, componentNode())
		case slotOp[P]:
			nodes = append(nodes, SeqNode{
				Kind: SeqIf,
				Then: []SeqNode{componentNode()},
				Else: sequenceOf(typed.fallback),
			})
		case slotCtxOp[P]:
			nodes = append(nodes, SeqNode{
				Kind: SeqIf,
				Then: []SeqNode{componentNode()},
				Else: sequenceOf(typed.fallback),
			})
		default:
			// An op that runs one body exactly once contributes no marker of its
			// own, so its nodes are spliced where it stands rather than becoming
			// a node the client has to walk into.
			if inline, ok := op.(interface{ sequenceInline() []SeqNode }); ok {
				nodes = append(nodes, inline.sequenceInline()...)
				continue
			}
			if body, ok := op.(interface{ sequenceBody() []SeqNode }); ok {
				nodes = append(nodes, SeqNode{Kind: SeqRepeat, Then: body.sequenceBody()})
				continue
			}
			nodes = append(nodes, SeqNode{Kind: SeqSlot})
		}
	}
	return nodes
}

// componentNode splits a called component into the two shapes it can take. A
// boundary leaves a placeholder whose only varying part is the id, so the frame
// around it stays static and does not travel per row — which is the whole reason
// a list of a hundred holes is not a hundred copies of the same markup.
func componentNode() SeqNode {
	return SeqNode{
		Kind: SeqComponent,
		Then: []SeqNode{
			{Kind: SeqStatic, Text: "<template "},
			{Kind: SeqSlot},
			{Kind: SeqStatic, Text: `="`},
			{Kind: SeqSlot},
			{Kind: SeqStatic, Text: `"></template>`},
		},
		Else: []SeqNode{{Kind: SeqSlot}},
	}
}

// appendEncoded writes a sequence in a form two different trees cannot share, so
// the digest below identifies the tree rather than an accident of it.
func appendEncoded(dst []byte, nodes []SeqNode) []byte {
	for _, node := range nodes {
		dst = append(dst, byte('0'+node.Kind))
		switch node.Kind {
		case SeqStatic:
			dst = strconv.AppendInt(dst, int64(len(node.Text)), 10)
			dst = append(dst, ':')
			dst = append(dst, node.Text...)
		case SeqIf, SeqRepeat, SeqComponent:
			dst = append(dst, '(')
			dst = appendEncoded(dst, node.Then)
			dst = append(dst, '|')
			dst = appendEncoded(dst, node.Else)
			dst = append(dst, ')')
		}
	}
	return dst
}

func sequenceAddress(nodes []SeqNode) string {
	sum := sha256.Sum256(appendEncoded(nil, nodes))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// registry holds every sequence this process has derived, so a client that
// received an address can ask for the tree behind it.
//
// It fills as plans render rather than from a generated table: an address only
// reaches a client because the plan behind it just rendered, so by the time the
// client asks, the entry is there. An address this process has never rendered is
// answered as unknown, and a client falls back to the assembled form it can
// always be sent instead.
var registry sync.Map // address -> *Sequence

// LookupSequence returns the sequence an address names, for a caller answering a
// client that holds the address and not the tree.
func LookupSequence(address string) (*Sequence, bool) {
	value, ok := registry.Load(address)
	if !ok {
		return nil, false
	}
	return value.(*Sequence), true
}

// Sequence derives this plan's sequence, once, and registers it.
//
// The derivation walks the instruction list and evaluates nothing, so it needs
// no parameters and yields the same tree for every render of this component.
func (p *Plan[P]) Sequence() *Sequence {
	p.sequenceOnce.Do(func() {
		nodes := sequenceOf(p.Ops)
		p.sequenceMemo = &Sequence{Address: sequenceAddress(nodes), Nodes: nodes}
		registry.Store(p.sequenceMemo.Address, p.sequenceMemo)
	})
	return p.sequenceMemo
}

// AppendJSON writes a sequence as the tree a client walks.
func (s *Sequence) AppendJSON(dst []byte) []byte {
	dst = append(dst, `{"addr":`...)
	dst = appendJSONString(dst, s.Address)
	dst = append(dst, `,"nodes":`...)
	dst = appendNodesJSON(dst, s.Nodes)
	return append(dst, '}')
}

func appendNodesJSON(dst []byte, nodes []SeqNode) []byte {
	dst = append(dst, '[')
	for i, node := range nodes {
		if i > 0 {
			dst = append(dst, ',')
		}
		switch node.Kind {
		case SeqStatic:
			dst = appendJSONString(dst, node.Text)
		case SeqSlot:
			dst = append(dst, '0')
		default:
			dst = append(dst, `{"k":`...)
			dst = strconv.AppendInt(dst, int64(node.Kind), 10)
			dst = append(dst, `,"t":`...)
			dst = appendNodesJSON(dst, node.Then)
			if len(node.Else) > 0 {
				dst = append(dst, `,"e":`...)
				dst = appendNodesJSON(dst, node.Else)
			}
			dst = append(dst, '}')
		}
	}
	return append(dst, ']')
}

// String renders a sequence for a diagnostic, joining its static text and
// marking every hole.
func (s *Sequence) String() string {
	var out strings.Builder
	var walk func([]SeqNode)
	walk = func(nodes []SeqNode) {
		for _, node := range nodes {
			switch node.Kind {
			case SeqStatic:
				out.WriteString(node.Text)
			case SeqSlot:
				out.WriteString("{}")
			case SeqIf:
				out.WriteString("{if ")
				walk(node.Then)
				out.WriteString("|")
				walk(node.Else)
				out.WriteString("}")
			case SeqRepeat:
				out.WriteString("{for ")
				walk(node.Then)
				out.WriteString("}")
			case SeqComponent:
				out.WriteString("{component ")
				walk(node.Then)
				out.WriteString("}")
			}
		}
	}
	walk(s.Nodes)
	return out.String()
}

package htmlbind

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Boundary marks a component as an automatic partial update boundary. Chain
// members carry one; an ordinary component call does not become an instance,
// so a manifest stays the size of the layout chain rather than the size of the
// document.
//
// A boundary is declared by generated code and never by hand, because its
// identity must change with the generated component version.
type Boundary[P any] struct {
	// ComponentID is the stable declaration identity of this component,
	// including its generated version. A template edit changes it, which
	// invalidates every validator derived from it.
	ComponentID string
	// Attr is the data attribute carrying the instance ID on the boundary's
	// root element. The generator writes the configured prefix into it.
	Attr string
	// Input canonically encodes the declared parameters, excluding slot
	// arguments, which belong to the child boundary rather than this frame.
	Input func(P) string
}

// boundary is the type-erased form stored on a bound fragment, so a chain can
// open a boundary without naming the component's parameter struct.
type boundary struct {
	componentID string
	attr        string
	input       func() string
}

func bindBoundary[P any](decl *Boundary[P], params P) *boundary {
	if decl == nil {
		return nil
	}
	return &boundary{
		componentID: decl.ComponentID,
		attr:        decl.Attr,
		input:       func() string { return decl.Input(params) },
	}
}

// Instance is one update boundary as it appeared in a render.
type Instance struct {
	// ID identifies the same logical instance across two renders. It is derived
	// from the chain position, so changing search parameters does not rename it.
	ID string
	// ParentID is the enclosing boundary, empty for the outermost one.
	ParentID string
	// ComponentID is the generated declaration identity and version.
	ComponentID string
	// InputValidator digests the canonical declared inputs. It predicts equal
	// output only for a component whose output depends on nothing else, so it
	// is a cache and diagnostic key rather than authority to skip a render.
	InputValidator string
	// FrameValidator digests this boundary's own rendered bytes, excluding the
	// output of nested boundaries. A layout whose frame is unchanged can keep
	// its DOM while its child is replaced.
	FrameValidator string
}

// Manifest is the update state of one render, in document order.
type Manifest struct {
	Instances []Instance
}

// Find returns the instance with the given ID.
func (m Manifest) Find(id string) (Instance, bool) {
	for _, instance := range m.Instances {
		if instance.ID == id {
			return instance, true
		}
	}
	return Instance{}, false
}

// Changed reports the instances whose frame differs from the previous render,
// plus those the previous render did not contain. It is the server-side half of
// a delta: everything it returns must be sent, everything else may be omitted.
func (m Manifest) Changed(previous Manifest) []Instance {
	var changed []Instance
	for _, instance := range m.Instances {
		before, ok := previous.Find(instance.ID)
		if !ok || before.FrameValidator != instance.FrameValidator {
			changed = append(changed, instance)
		}
	}
	return changed
}

// ProtocolVersion identifies the wire contract between the server and the
// browser runtime: the attribute names, the manifest fields, the operation
// kinds, and the way validators are built. It is a framework constant rather
// than a project option, because it names a contract instead of a namespace.
//
// Bump it when any of those change. A version the peer does not accept is not
// an error: it falls back to a complete document, which is also what makes a
// rolling deploy safe when a page loaded from the old version sends its next
// request to a new server.
const ProtocolVersion = 1

// collector accumulates boundary state during one render.
type collector struct {
	key      []byte
	manifest Manifest
	stack    []*openBoundary
	// pending holds the boundary whose root element has not yet written its
	// instance attribute. Only the boundary's own root consumes it, so an
	// ordinary component nested inside cannot claim its parent's ID.
	pending *openBoundary
	// capture records each boundary's complete subtree HTML, which a delta
	// needs as the payload of a replace operation.
	capture  bool
	contents map[string]string
}

type openBoundary struct {
	// index is the instance's slot in the manifest, reserved when the boundary
	// opens so the manifest stays in document order. A parent therefore
	// precedes its children, which is the order a structural operation needs.
	index   int
	id      string
	frame   hash.Hash
	attr    string
	content strings.Builder
}

func (c *collector) open(id, componentID, attr, input string) {
	parent := ""
	if depth := len(c.stack); depth > 0 {
		parent = c.stack[depth-1].id
	}
	c.manifest.Instances = append(c.manifest.Instances, Instance{
		ID:             id,
		ParentID:       parent,
		ComponentID:    componentID,
		InputValidator: c.digest("input", componentID+"\x00"+input),
	})
	state := &openBoundary{
		index: len(c.manifest.Instances) - 1,
		id:    id,
		frame: hmac.New(sha256.New, c.key),
		attr:  attr,
	}
	// Seeding with the protocol version and the component identity keeps two
	// frames from ever comparing equal across a version bump or across two
	// components that happen to render the same bytes.
	state.frame.Write([]byte("frame\x00" + strconv.Itoa(ProtocolVersion) + "\x00" + componentID + "\x00"))
	c.stack = append(c.stack, state)
	c.pending = state
}

func (c *collector) close() {
	depth := len(c.stack)
	if depth == 0 {
		return
	}
	state := c.stack[depth-1]
	c.stack = c.stack[:depth-1]
	c.pending = nil
	c.manifest.Instances[state.index].FrameValidator = truncate(state.frame.Sum(nil))
	if c.capture {
		if c.contents == nil {
			c.contents = map[string]string{}
		}
		c.contents[state.id] = state.content.String()
	}
}

// write hashes into the innermost open boundary only, so a frame validator
// covers the component's own markup and not its child's. Captured content works
// the other way: a replace operation must carry the whole subtree, so every
// enclosing boundary records the bytes too.
func (c *collector) write(value string) {
	depth := len(c.stack)
	if depth == 0 {
		return
	}
	c.stack[depth-1].frame.Write([]byte(value))
	if !c.capture {
		return
	}
	for _, state := range c.stack {
		state.content.WriteString(value)
	}
}

func (c *collector) digest(tag, data string) string {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(tag))
	mac.Write([]byte{0})
	mac.Write([]byte(strconv.Itoa(ProtocolVersion)))
	mac.Write([]byte{0})
	mac.Write([]byte(data))
	return truncate(mac.Sum(nil))
}

// truncate shortens a digest to 128 bits, which keeps a manifest small while
// leaving collision probability negligible. A collision would silently keep
// stale DOM, so this length is a floor rather than a tuning knob.
func truncate(sum []byte) string {
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// Canonical input encoding. Every value is type tagged and length prefixed, so
// two different inputs cannot produce one encoding. JSON is deliberately not
// used: its float formatting, map ordering, and optional handling are all
// ambiguous, and an ambiguous encoding would let a changed input reuse a stale
// validator.

func canon(tag, payload string) string {
	return tag + ":" + strconv.Itoa(len(payload)) + ":" + payload
}

// CanonJoin concatenates the encoded parameters of one component in
// declaration order.
func CanonJoin(parts ...string) string { return strings.Join(parts, "") }

// CanonString encodes any string-kinded value, covering plain strings,
// decimals, generated enums, and the trusted string types.
func CanonString[T ~string](value T) string { return canon("s", string(value)) }

// CanonBool encodes a bool.
func CanonBool(value bool) string { return canon("b", strconv.FormatBool(value)) }

// CanonInt encodes an int.
func CanonInt(value int) string { return canon("i", strconv.Itoa(value)) }

// CanonFloat encodes a float64.
func CanonFloat(value float64) string {
	return canon("f", strconv.FormatFloat(value, 'g', -1, 64))
}

// CanonBytes encodes a byte slice.
func CanonBytes(value []byte) string { return canon("y", string(value)) }

// CanonTime encodes an instant in UTC, so two equal instants in different zones
// cannot produce two validators.
func CanonTime(value time.Time) string {
	return canon("d", value.UTC().Format(time.RFC3339Nano))
}

// CanonURL encodes a URL through its string form.
func CanonURL(value url.URL) string { return canon("u", value.String()) }

// CanonOptional encodes an absent value distinctly from any present one.
func CanonOptional[T any](value *T, encode func(T) string) string {
	if value == nil {
		return canon("n", "")
	}
	return encode(*value)
}

// CanonArray encodes a slice, delegating each element.
func CanonArray[T any](values []T, encode func(T) string) string {
	var out strings.Builder
	for _, item := range values {
		out.WriteString(encode(item))
	}
	return canon("a", out.String())
}

// CanonRecord wraps the already encoded fields of a declared record.
func CanonRecord(fields string) string { return canon("r", fields) }

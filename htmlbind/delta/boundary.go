package delta

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

// collector accumulates boundary state during one render. It implements
// htmlbind.Collector, which is how the render half stays free of the hashing
// done here.
type collector struct {
	key []byte
	// tag seeds every digest this render produces, so two renders that must
	// never compare equal cannot. WithValidatorTag supplies it; the transport
	// half passes its build identity, which is the axis that actually moves.
	//
	// It replaced a protocol version constant. A version the module owned was a
	// second, weaker copy of the same idea: it could only change when this
	// module's wire shape changed, while a build identity also covers a changed
	// template, a changed external function, and a changed client.
	tag string
	// element names the placeholder a decomposing capture writes where a nested
	// boundary sits. It is the same element a progressive render already writes
	// for an await boundary, so a client has one hole shape to recognise.
	element  string
	manifest Manifest
	stack    []*openBoundary
	// pending holds the boundary whose root element has not yet written its
	// instance attribute. Only the boundary's own root consumes it, so an
	// ordinary component nested inside cannot claim its parent's ID.
	pending *openBoundary
	// capture records each boundary's own HTML, with a placeholder where each
	// nested boundary sits rather than that boundary's bytes. A delta sends one
	// fragment per changed boundary and leaves a hole for every child, so an
	// unchanged child keeps the DOM it already has — and the state inside it —
	// instead of being recreated inside its parent's replacement.
	capture  bool
	contents map[string]string
	// children names the boundaries appearing as holes in each captured
	// fragment, so a client can tell a hole it must fill from one it retains.
	// Nothing in the markup distinguishes them.
	children map[string][]string
	// scratch is reused for the string-to-bytes conversion hashing needs, so a
	// collecting render does not allocate once per instruction. A collector is
	// only ever driven by the one goroutine walking the plan.
	scratch []byte
}

type openBoundary struct {
	// index is the instance's slot in the manifest, reserved when the boundary
	// opens so the manifest stays in document order. A parent therefore
	// precedes its children, which is the order a structural operation needs.
	index    int
	id       string
	frame    hash.Hash
	attr     string
	content  strings.Builder
	children []string
}

func (c *collector) Begin(validatorTag, boundaryElement string) {
	c.tag, c.element = validatorTag, boundaryElement
}

func (c *collector) Open(id, componentID, attr, input string) {
	parent := ""
	if depth := len(c.stack); depth > 0 {
		enclosing := c.stack[depth-1]
		parent = enclosing.id
		// The child's bytes never reach the parent's fragment. What lands there
		// instead is an inert element carrying the child's id, which a client
		// either fills from this response or moves the node it already holds
		// into.
		//
		// The placeholder is hashed into the parent's frame even though the
		// child's bytes are not. That is the difference between a frame that
		// excludes a child's content and one that cannot see the child at all:
		// without it, a parent that gained or lost a region would compare equal,
		// and the client would be sent a fragment with no hole to put it in.
		hole := c.placeholder(attr, id)
		c.scratch = append(c.scratch[:0], hole...)
		enclosing.frame.Write(c.scratch)
		if c.capture {
			enclosing.children = append(enclosing.children, id)
			enclosing.content.WriteString(hole)
		}
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
	// Seeding with the validator tag and the component identity keeps two frames
	// from ever comparing equal across two builds or across two components that
	// happen to render the same bytes.
	state.frame.Write([]byte("frame\x00" + c.tag + "\x00" + componentID + "\x00"))
	c.stack = append(c.stack, state)
	c.pending = state
}

func (c *collector) Close() {
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
			c.contents, c.children = map[string]string{}, map[string][]string{}
		}
		c.contents[state.id] = state.content.String()
		c.children[state.id] = state.children
	}
}

// placeholder is the hole one boundary leaves in its parent's fragment. It is
// the element a progressive render already writes for an await boundary, so a
// client recognises one shape rather than two, and display:contents keeps it out
// of layout for the moment before it is filled.
func (c *collector) placeholder(attr, id string) string {
	return `<` + c.element + ` ` + attr + `="` + id + `" style="display:contents"></` + c.element + `>`
}

// Write records into the innermost open boundary only. Both the frame validator
// and the captured fragment stop at a nested boundary: the validator so an
// ancestor whose own markup is unchanged compares equal while its child moves,
// and the fragment so that child is transferred once, as itself, rather than
// again inside every ancestor that encloses it.
func (c *collector) Write(value string) {
	depth := len(c.stack)
	if depth == 0 {
		return
	}
	state := c.stack[depth-1]
	c.scratch = append(c.scratch[:0], value...)
	state.frame.Write(c.scratch)
	if c.capture {
		state.content.WriteString(value)
	}
}

func (c *collector) TakePending() (attr, id string, ok bool) {
	if c.pending == nil {
		return "", "", false
	}
	state := c.pending
	c.pending = nil
	return state.attr, state.id, true
}

func (c *collector) digest(tag, data string) string {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(tag))
	mac.Write([]byte{0})
	mac.Write([]byte(c.tag))
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

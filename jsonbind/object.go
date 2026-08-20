package jsonbind

// Object is a JSON object split into its top-level members in one pass.
//
// The binder needs random access to body fields because a value may also come
// from the query string or a form, and the winner depends on the field's tag
// rather than on document order. Object gives that access without the cost of a
// map: names and values are subslices of the source buffer, so splitting a
// document allocates one slice rather than a map plus a copy per member.
//
// Values alias the buffer the Object was built from. Copy anything that has to
// outlive it.
type Object struct {
	members []member
}

type member struct {
	name  []byte
	value []byte
}

// EmptyObject returns an object with no members, for an absent or blank body.
func EmptyObject() *Object { return &Object{} }

// ParseObject splits a JSON object document into its members.
func ParseObject(data []byte) (*Object, error) {
	obj := &Object{}
	if len(data) == 0 {
		return obj, nil
	}
	// members starts at a capacity covering a typical request body so the split
	// costs one allocation instead of one per doubling; the parser itself stays
	// on the stack.
	obj.members = make([]member, 0, 8)
	var p Parser
	p.Reset(data)
	null, err := p.ObjectStart()
	if err != nil {
		return nil, err
	}
	if null {
		return nil, newError("json_parse", "JSON value must be an object", nil)
	}
	for n := 0; ; n++ {
		name, escaped, ok, err := p.objectKeySpan(n)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if escaped {
			// Only an escaped name needs its own storage; a plain one stays a
			// subslice of the source buffer.
			name = unescape(nil, name)
		}
		value, err := p.RawValue()
		if err != nil {
			return nil, err
		}
		obj.members = append(obj.members, member{name: name, value: value})
	}
	if err := p.End(); err != nil {
		return nil, err
	}
	return obj, nil
}

// Len returns the number of members.
func (o *Object) Len() int {
	if o == nil {
		return 0
	}
	return len(o.members)
}

// Member returns the i'th member's name and raw value in document order. The
// raw bytes alias the buffer the Object was built from, like Get's.
func (o *Object) Member(i int) (name string, raw []byte) {
	m := &o.members[i]
	return string(m.name), m.value
}

// Get returns the raw bytes of the named member.
//
// Lookup is a linear scan. Request bodies have few top-level fields, and
// comparing against a subslice beats hashing a freshly allocated key string.
func (o *Object) Get(name string) ([]byte, bool) {
	if o == nil {
		return nil, false
	}
	for i := range o.members {
		if string(o.members[i].name) == name {
			return o.members[i].value, true
		}
	}
	return nil, false
}

// Has reports whether the named member is present.
func (o *Object) Has(name string) bool {
	_, ok := o.Get(name)
	return ok
}

// Names appends every member name to dst as strings.
func (o *Object) Names(dst []string) []string {
	if o == nil {
		return dst
	}
	for i := range o.members {
		dst = append(dst, string(o.members[i].name))
	}
	return dst
}

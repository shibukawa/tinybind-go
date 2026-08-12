package htmlbind

import (
	"strings"
)

// DefaultCSRFFieldName is the hidden field an unsafe form carries. It is a
// generation-time name rather than a render-time one, so the whole tag but its
// value folds into static bytes.
const DefaultCSRFFieldName = "_csrf"

// CSRFMode says whether generated forms carry a token.
type CSRFMode string

const (
	// CSRFAuto is the zero value: every unsafe form gets the hidden field, and
	// a component reaching one becomes per-request.
	CSRFAuto CSRFMode = ""
	// CSRFOff emits no field and marks nothing per-request.
	//
	// It is for a deployment that has settled on Origin and Fetch Metadata
	// checks alone. Those are genuinely close to sufficient for a single origin,
	// and turning the token off is what gives such a deployment its cacheable
	// form-bearing components back.
	//
	// What it does not turn off is the origin checking itself, which this module
	// never performs: that is a check on an inbound request before any render, so
	// it belongs to middleware, and a deployment declines it by not wrapping its
	// handlers.
	CSRFOff CSRFMode = "off"
)

// unsafeFormMethods need a token. A GET form is a navigation whose fields become
// the query string, and a token in a URL reaches history, logs, and referrers.
var unsafeFormMethods = map[string]bool{"post": true, "put": true, "patch": true, "delete": true}

func (c *compiler) csrfFieldName() string {
	if c.csrfField == "" {
		return DefaultCSRFFieldName
	}
	return c.csrfField
}

// unsafeForm reports whether an element is a form this module must put a token
// in, and refuses the shapes it cannot decide.
//
// The two refusals are both cases where guessing would be worse than stopping.
// A dynamic method cannot be read at generation time, and choosing wrong either
// leaks the token into a query string or leaves a form unprotected. A static
// absolute action may be another origin, and inserting there would hand the
// session's secret to a third party.
func (c *compiler) unsafeForm(node *ElementNode) (bool, error) {
	if c.csrfMode == CSRFOff || node.Name != "form" {
		return false, nil
	}
	if c.hasOptOut(node) {
		return false, nil
	}
	method, dynamic := staticAttribute(node, "method")
	if dynamic {
		return false, c.error(node.Pos, "a form's method must be static, because it decides whether the form carries a CSRF token")
	}
	// A form carrying server-action is a POST form whether or not the author
	// wrote the method, because the lowering writes one. Reading the authored
	// attribute alone is what left the shipped markup with no token at all.
	//
	// Only when a selector resolved, though: with none the form keeps no native
	// channel, stays a GET form, and a token in it would reach history, logs, and
	// referrers.
	if name, action := serverActionName(node); action && c.actionSelectors[name] != "" {
		method = "post"
	}
	if !unsafeFormMethods[strings.ToLower(strings.TrimSpace(method))] {
		return false, nil
	}
	action, dynamic := staticAttribute(node, "action")
	// A relative or absolute-path action is this origin by construction. An
	// absolute URL may not be, and this package cannot know the deployment's
	// origin, so it refuses rather than deciding.
	if !dynamic && absoluteURL(action) {
		return false, c.error(node.Pos, "this form posts to the absolute URL "+action+
			", and a CSRF token must not be sent to another origin; mark it with data-"+c.attrPrefix+"-no-csrf if that is deliberate")
	}
	// A form already carrying the field is a hand-written token, which still
	// works and must not be doubled.
	if c.hasCSRFField(node) {
		return false, nil
	}
	return true, nil
}

// hasOptOut reports the author attribute taking one form out of the automatic
// insertion. The choice is per form because posting off-origin is per form; the
// choice of whether tokens exist at all stays a deployment's.
func (c *compiler) hasOptOut(node *ElementNode) bool {
	marker := "data-" + c.attrPrefix + "-no-csrf"
	for _, attribute := range node.Attributes {
		if attribute.Name == marker {
			return true
		}
	}
	return false
}

// hasCSRFField reports whether a form already writes the field itself, looking
// at its direct children only. A token hidden inside a conditional is not found,
// which is stated rather than solved: the automatic field is the supported way.
func (c *compiler) hasCSRFField(node *ElementNode) bool {
	for _, child := range node.Children {
		element, ok := child.(*ElementNode)
		if !ok || element.Name != "input" {
			continue
		}
		if name, dynamic := staticAttribute(element, "name"); !dynamic && name == c.csrfFieldName() {
			return true
		}
	}
	return false
}

// hasAttribute reports whether an element carries an attribute at all. It is
// distinct from [staticAttribute], whose second result says the value is an
// expression rather than that the attribute is present.
func hasAttribute(node *ElementNode, name string) bool {
	for _, attribute := range node.Attributes {
		if attribute.Name == name {
			return true
		}
	}
	return false
}

// staticAttribute returns an attribute's literal value, and reports whether it
// is an expression instead.
func staticAttribute(node *ElementNode, name string) (value string, dynamic bool) {
	for _, attribute := range node.Attributes {
		if attribute.Name != name {
			continue
		}
		var out strings.Builder
		for _, part := range attribute.Value {
			if part.Expression != nil {
				return "", true
			}
			out.WriteString(part.Text)
		}
		return out.String(), false
	}
	return "", false
}

// absoluteURL reports whether a value names a scheme and authority, which is the
// only shape that can reach another origin.
func absoluteURL(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "//") {
		return true
	}
	scheme, rest, found := strings.Cut(value, ":")
	if !found || scheme == "" || !strings.HasPrefix(rest, "//") {
		return false
	}
	for _, r := range scheme {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '+', r == '-', r == '.':
		default:
			return false
		}
	}
	return true
}

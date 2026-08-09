package htmlbind

// Cache scope is what lets a caller put a cache policy on the wire before the
// first body byte.
//
// The timing is the whole reason it is declared rather than observed. A private
// component four levels down renders long after a response's headers are
// committed, so a signal computed during a render would exist only on the
// buffered branch — and a response's cache policy would then depend on whether
// streaming happened to be on, which is a configuration switch silently moving a
// security-relevant header.
//
// Three states, not two. A component may say nothing, say private, or say
// public, and the three differ in what they do to a public assertion made
// around them:
//
//   - Undeclared reports private and does not contradict a public assertion, so
//     an ordinary component inherits whatever the chain asserts. A default that
//     also vetoed would make nothing publishable.
//   - Private reports private and vetoes a public assertion reaching it. That
//     veto is the only way to declare what static analysis cannot see: a
//     component calling an external Go function that reads request identity out
//     of a context looks shared to every check either side can write.
//   - Public reports shared, and generation has already refused it if its call
//     graph reaches a declared private component.

// IsPrivate reports whether this fragment's output belongs to one reader, so a
// caller knows before rendering whether the response may be stored by a shared
// cache. Reading it renders nothing.
//
// An undeclared fragment reports true. That is the safe direction and it is the
// framework default rather than a property of the annotation: a component
// treated as shared that is actually per-reader serves one reader's output to
// another, while a component treated as per-reader that is actually shared costs
// a cache miss. Those are not comparable, so forgetting is slow rather than
// wrong.
func (f Fragment) IsPrivate() bool { return f.declaresPrivate || !f.declaresPublic }

// PrivateSource names the component whose declaration made this fragment
// private, and is empty when nothing declared it — which includes the ordinary
// case of a fragment that is private only because nothing said otherwise.
//
// It exists for the same reason HeadSources does: an author who expected shared
// output and got per-reader output needs to know which component to change, and
// the answer alone does not say.
func (f Fragment) PrivateSource() string { return f.privateSource }

// IsPrivate is the Wrapper form of the accessor documented on Fragment. The
// child it wraps is counted separately, because a wrapper is bound before it is
// told what it wraps.
func (w Wrapper) IsPrivate() bool { return w.declaresPrivate || !w.declaresPublic }

// PrivateSource is the Wrapper form of the accessor documented on Fragment.
func (w Wrapper) PrivateSource() string { return w.privateSource }

// IsPrivate reports whether a whole chain's output belongs to one reader. It is
// the value a caller turns into a cache policy header, and it is readable before
// the chain renders, as ChainHead and HasAwaitBlock are.
//
// Two rules decide it, and both follow from what a chain member contains:
//
// Private wins. A member declaring private makes the response private whatever
// anything else says, because that member's bytes are in the response. This also
// covers a combination assembled at run time that generation never saw, where
// the refusal of public over private could not have fired.
//
// Public has to come from the outside in. A wrapper contains everything below
// it, so a public declaration on the outermost member covers the whole chain —
// which is what lets one annotation on a layout serve every page beneath it. A
// declaration further in covers only itself and what it wraps, and says nothing
// about the markup wrapped around it, so it cannot make the response shared on
// its own. A page asserting public under an undeclared layout therefore stays
// private: the layout's own output was never declared, and it is in the response
// too.
func IsPrivate(wrappers []Wrapper, leaf Fragment) bool {
	for _, wrapper := range wrappers {
		if wrapper.declaresPrivate {
			return true
		}
	}
	if leaf.declaresPrivate {
		return true
	}
	if len(wrappers) > 0 {
		return !wrappers[0].declaresPublic
	}
	return !leaf.declaresPublic
}

// PrivateSource names the component whose declaration made a chain private,
// searching outermost first. It is empty when the chain is private only because
// nothing declared otherwise, and when the chain is shared.
//
// It is the chain form of the accessor documented on Fragment, and it exists for
// the reason HeadSources gives: a caller explaining a private response has to be
// able to name the component to change.
func PrivateSource(wrappers []Wrapper, leaf Fragment) string {
	for _, wrapper := range wrappers {
		if wrapper.declaresPrivate {
			return wrapper.privateSource
		}
	}
	if leaf.declaresPrivate {
		return leaf.privateSource
	}
	return ""
}

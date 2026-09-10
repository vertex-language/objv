package analyzer

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Automatic reference counting: what it decides, and what it refuses.
//
// ARC is not a rewrite this package performs — the retains and releases are
// lower's, placed where the ownership of a value changes. What the analyzer
// owes it is the ownership itself: every object variable has one, most
// programs write none, and the defaults are the language's.

// ownership fills in the ARC ownership qualifier a declaration did not
// write.
//
// §5.6 makes __strong the default for an object variable, which is what
// makes ARC work at all: a local that holds an object keeps it alive for as
// long as the local is in scope. Under manual reference counting there is no
// ownership to infer and the type is left as written.
//
// external says the declaration has linkage, which changes nothing about the
// ownership and everything about where the release goes; it is passed so
// that the rule stays in one place if it ever does.
func (c *checker) ownership(t types.Type, external bool) types.Type {
	if !c.arc() || t == nil {
		return t
	}
	if !types.IsObjCObject(t) {
		return t
	}
	if types.LifetimeOf(t) != types.LifeNone {
		return t
	}
	return types.WithLifetime(t, types.LifeStrong)
}

// propertyOwnership is the ownership §4.8's attributes imply.
//
// A property's attribute says what the setter does with the value, and that
// is the same fact ARC records as a lifetime: `copy` and `strong` and
// `retain` all keep the object alive, `weak` does not, and
// `unsafe_unretained` and `assign` keep a pointer that may outlive what it
// points at.
//
// Under ARC an object property with no ownership attribute at all is
// strong — and one written `assign` is a mistake worth reporting, because
// the program almost certainly meant weak.
func (c *checker) propertyOwnership(t types.Type, attrs types.PropertyAttr, at ast.Node) types.Type {
	if !types.IsObjCObject(t) {
		if attrs&(types.PropCopy|types.PropRetain|types.PropStrong|types.PropWeak) != 0 {
			c.report(at, "an ownership attribute applies to an object property, not one of type "+
				t.String())
		}
		return t
	}
	if types.LifetimeOf(t) != types.LifeNone {
		return t
	}
	switch {
	case attrs&types.PropWeak != 0:
		return types.WithLifetime(t, types.LifeWeak)
	case attrs&types.PropUnsafeUnretained != 0:
		return types.WithLifetime(t, types.LifeUnsafeUnretained)
	case attrs&(types.PropCopy|types.PropRetain|types.PropStrong) != 0:
		return types.WithLifetime(t, types.LifeStrong)
	case attrs&types.PropAssign != 0:
		if c.arc() {
			c.warn(at, "'assign' on an object property does not keep it alive; "+
				"'weak' zeroes the reference when the object goes away")
		}
		return types.WithLifetime(t, types.LifeUnsafeUnretained)
	}
	if c.arc() {
		return types.WithLifetime(t, types.LifeStrong)
	}
	return t
}

// checkBridgeCast checks §6.5's three bridge keywords.
//
// Each says something different about ownership, and all three are about the
// same boundary: an object pointer on one side, something ARC does not
// manage on the other. Writing one where there is no boundary is not a
// dangerous mistake, but it is one — the keyword claims something the types
// do not support — and outside ARC there is no ownership for any of them to
// transfer.
func (c *checker) checkBridgeCast(e *ast.CastExpr, dst, src types.Type) {
	if src == nil || dst == nil {
		return
	}
	if !c.arc() {
		c.warn(e, "a bridge cast has no effect without ARC; it is a plain cast here")
		return
	}
	if types.Bridge(dst, src) != types.BridgeNeeded {
		c.warn(e, "'"+e.Op.String()+"' between "+src.String()+" and "+dst.String()+
			" bridges nothing: neither side crosses the boundary ARC manages")
		return
	}
	// The two transferring casts each name a direction, and the direction
	// has to be the one the conversion goes.
	switch e.Op.String() {
	case "__bridge_retained":
		if !types.IsObjCObject(src) {
			c.report(e, "__bridge_retained hands ownership out of ARC; "+
				"its operand is the object, and this one is "+src.String())
		}
	case "__bridge_transfer":
		if !types.IsObjCObject(dst) {
			c.report(e, "__bridge_transfer takes ownership into ARC; "+
				"its result is the object, and this one is "+dst.String())
		}
	}
}

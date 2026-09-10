package analyzer

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// The namespaces. C has three — ordinary identifiers, tags, labels — and
// Objective-C adds three more that are flat rather than nested: classes,
// protocols, and selectors.
//
// The flatness is the language's, not a simplification. A class declared
// inside an @implementation is a class for the rest of the unit, a protocol
// is never scoped to anything, and a selector is a name the whole program
// shares — two classes may implement `count` and it is one selector, which is
// exactly why a message send needs a receiver type to say which method it
// reaches.

type symKind uint8

const (
	symObject symKind = iota
	symFunc
	symTypedef
	symEnumConst
	// symIvar is an instance variable brought into scope inside a method
	// body. §4.5 makes them visible as bare names — `_count` in a method is
	// `self->_count` — and nothing else in the language declares a name
	// that way.
	symIvar
)

func (k symKind) String() string {
	switch k {
	case symFunc:
		return "function"
	case symTypedef:
		return "typedef"
	case symEnumConst:
		return "enumeration constant"
	case symIvar:
		return "instance variable"
	}
	return "object"
}

type symbol struct {
	kind   symKind
	typ    types.Type
	node   ast.Node
	extern bool  // file scope or declared extern: relinkable
	value  int64 // enum constants

	// ivar is the instance variable a symIvar names, so that a use can be
	// rewritten to a member access on self.
	ivar *types.Ivar

	// block marks a variable declared __block: shared with, and mutable
	// from, the blocks that capture it.
	block bool
}

type tagsym struct {
	typ  types.Type // *types.Record or *types.Enum
	node ast.Node
}

type scope struct {
	ordinary map[string]*symbol
	tags     map[string]*tagsym
}

func (c *checker) push() {
	c.scopes = append(c.scopes, &scope{
		ordinary: map[string]*symbol{},
		tags:     map[string]*tagsym{},
	})
}

func (c *checker) pop() { c.scopes = c.scopes[:len(c.scopes)-1] }

func (c *checker) fileScope() bool { return len(c.scopes) == 1 }

func (c *checker) lookup(name string) *symbol {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if s, ok := c.scopes[i].ordinary[name]; ok {
			return s
		}
	}
	return nil
}

// declare enters a name in the innermost scope. Same-scope redeclaration is
// permitted only where the language permits it: declarations with linkage
// (file scope, or extern), and typedefs redeclaring typedefs.
func (c *checker) declare(id *ast.Ident, s *symbol) {
	name := c.name(id)
	cur := c.scopes[len(c.scopes)-1]
	if prev, ok := cur.ordinary[name]; ok {
		switch {
		case prev.kind != s.kind:
			c.report(id, "'"+name+"' redeclared as a different kind of symbol (was "+prev.kind.String()+")")
			return
		case prev.kind == symEnumConst:
			c.report(id, "enumeration constant '"+name+"' redeclared")
			return
		case prev.kind == symTypedef, prev.extern && s.extern:
			return // permitted; keep the first
		default:
			c.report(id, "'"+name+"' redeclared in the same scope")
			return
		}
	}
	cur.ordinary[name] = s
}

// declareName enters a symbol under a name that no identifier node spells:
// self, _cmd, an instance variable, a method's parameters.
func (c *checker) declareName(name string, s *symbol) {
	c.scopes[len(c.scopes)-1].ordinary[name] = s
}

// lookupTag searches all scopes; declareTag enters in the innermost.
func (c *checker) lookupTag(name string) *tagsym {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if t, ok := c.scopes[i].tags[name]; ok {
			return t
		}
	}
	return nil
}

func (c *checker) currentTag(name string) *tagsym {
	t := c.scopes[len(c.scopes)-1].tags[name]
	return t
}

func (c *checker) declareTag(name string, t *tagsym) {
	c.scopes[len(c.scopes)-1].tags[name] = t
}

// ---- the Objective-C namespaces ----

// class returns the class of that name, creating an incomplete one if it has
// not been seen.
//
// A class exists as soon as it is named — that is what @class is for — and
// what a forward declaration leaves out is everything except the name. The
// entity is created once and completed in place, so that every pointer to it
// is the same class however it was reached.
func (c *checker) class(name string) *types.Class {
	if k, ok := c.classes[name]; ok {
		return k
	}
	k := &types.Class{Name: name}
	c.classes[name] = k
	c.classOrder = append(c.classOrder, k)
	return k
}

// protocol returns the protocol of that name, creating an incomplete one if
// it has not been seen.
func (c *checker) protocol(name string) *types.Protocol {
	if p, ok := c.protocols[name]; ok {
		return p
	}
	p := &types.Protocol{Name: name}
	c.protocols[name] = p
	c.protocolOrder = append(c.protocolOrder, p)
	return p
}

// selector records a selector the unit mentions. The set is what the runtime
// metadata's selector references are built from, and it is a set: a selector
// named twice is one selector.
func (c *checker) selector(sel string) {
	if sel == "" {
		return
	}
	if _, ok := c.selectors[sel]; !ok {
		c.selectors[sel] = struct{}{}
		c.selectorOrder = append(c.selectorOrder, sel)
	}
}

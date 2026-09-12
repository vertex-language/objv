package lower

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

// Computes which conditionally-emitted function definitions (static functions,
// C99 inline definitions, and gnu_inline functions) are used and should be emitted.
// Definitions not reachable from roots in the translation unit are omitted.

// useSet is the set of definition names this unit will emit.
type useSet map[string]bool

// planUsed computes the set of conditionally-emitted functions used in this unit.
func (u *unit) planUsed() useSet {
	// Keyed by emitted link name (to distinguish overloaded functions).
	// Collected over fileScope() to include static helper functions defined inside @implementation.
	bodies := make(map[string]*ast.FuncDecl)
	for _, d := range u.fileScope() {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Name == nil {
			continue
		}
		name := u.name(fd.Name)
		if u.isStatic(fd) || u.isInlineDefinition(name, fd) {
			bodies[u.linkName(name, fd)] = fd
		}
	}

	used := make(useSet, len(bodies))
	var work []string

	// The roots: everything the unit emits unconditionally — an external
	// function's body, a file-scope initializer, and every method body,
	// which a class's method list names whether or not anything calls it.
	for _, d := range u.file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body == nil || d.Name == nil || bodies[u.linkName(d.Name.Name(u.src), d)] != nil {
				continue
			}
			work = append(work, u.mentions(d.Body, bodies)...)
		case *ast.GenDecl:
			// Only the initializers. A declaration is not a use, and a
			// prototype mentions its own name by construction:
			// <CGGeometry.h> writes `static inline CGPoint CGPointMake(
			// CGFloat, CGFloat);` above the definition, and walking the
			// whole declaration would make every forward-declared static
			// function in every header count as used by itself. What is
			// left is `int (*f)(void) = g;`, which is a real one.
			for _, it := range d.List {
				if it != nil && it.Init != nil {
					work = append(work, u.mentions(it.Init, bodies)...)
				}
			}
		case *ast.ClassImplDecl:
			work = append(work, u.implRoots(d.Members, bodies)...)
		case *ast.CategoryImplDecl:
			work = append(work, u.implRoots(d.Members, bodies)...)
		}
	}

	// And the functions that run around main. A constructor is called by
	// dyld and named by nothing, so nothing mentions it: the use set would
	// drop every `static void __attribute__((constructor)) setup(void)` in
	// the language, which is most of them.
	for name := range u.planStaticInit() {
		if bodies[name] != nil {
			work = append(work, name)
		}
	}

	// The closure. A definition that is emitted brings in whatever its own
	// body mentions.
	for len(work) > 0 {
		name := work[len(work)-1]
		work = work[:len(work)-1]
		if used[name] {
			continue
		}
		used[name] = true
		work = append(work, u.mentions(bodies[name].Body, bodies)...)
	}
	return used
}

// implRoots collects names mentioned by @implementation members (methods are roots,
// while nested function definitions are conditionally emitted).
func (u *unit) implRoots(members []ast.Decl, bodies map[string]*ast.FuncDecl) []string {
	var out []string
	for _, m := range members {
		if fd, ok := m.(*ast.FuncDecl); ok && fd.Body != nil && fd.Name != nil &&
			bodies[u.linkName(u.name(fd.Name), fd)] != nil {
			continue
		}
		out = append(out, u.mentions(m, bodies)...)
	}
	return out
}

// mentions is every name in bodies that appears anywhere in n.
//
// An identifier is resolved through the overload the analyzer chose for it,
// so that naming one of a set does not bring in the rest.
func (u *unit) mentions(n ast.Node, bodies map[string]*ast.FuncDecl) []string {
	var out []string
	ast.Inspect(n, func(x ast.Node) bool {
		id, ok := x.(*ast.Ident)
		if !ok {
			return true
		}
		if name := u.callLinkName(id); bodies[name] != nil {
			out = append(out, name)
		}
		return true
	})
	return out
}

// isInlineDefinition reports whether every declaration of name in this unit
// says inline and none says extern — §6.7.4p7's condition.
//
// gcc's `extern inline` with __attribute__((gnu_inline)) is folded in here
// rather than tested separately: it also provides no external definition, so
// the answer this function gives about whether to emit is the same.
func (u *unit) isInlineDefinition(name string, def *ast.FuncDecl) bool {
	if !specHasKeyword(def.Specs, token.INLINE) {
		return false
	}
	if u.isStatic(def) {
		return false // a static inline is covered by the static rule
	}
	gnu := u.hasAttrNamed(def.Specs, "gnu_inline")
	for _, d := range u.file.Decls {
		var specs ast.DeclSpecs
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name == nil || u.name(d.Name) != name {
				continue
			}
			specs = d.Specs
		case *ast.GenDecl:
			if !u.declaresName(d, name) {
				continue
			}
			specs = d.Specs
		default:
			continue
		}
		if specHasKeyword(specs, token.EXTERN) && !gnu {
			return false
		}
		if !specHasKeyword(specs, token.INLINE) {
			return false
		}
	}
	return true
}

// declaresName reports whether a declaration declares name.
func (u *unit) declaresName(d *ast.GenDecl, name string) bool {
	for _, it := range d.List {
		if u.name(declName(it.Decl)) == name {
			return true
		}
	}
	return false
}

// hasAttrNamed reports whether an attribute of that name is present, with the
// two leading and trailing underscores ignored: every attribute may be
// written either way.
func (u *unit) hasAttrNamed(specs ast.DeclSpecs, name string) bool {
	for _, sp := range specs {
		a, ok := sp.(*ast.AttrSpec)
		if !ok {
			continue
		}
		for _, at := range a.Attrs {
			if at == nil || at.Name == nil {
				continue
			}
			if trimAttrName(u.name(at.Name)) == name {
				return true
			}
		}
	}
	return false
}

func trimAttrName(n string) string {
	for len(n) > 4 && n[:2] == "__" && n[len(n)-2:] == "__" {
		n = n[2 : len(n)-2]
	}
	return n
}

// specHasKeyword reports whether a specifier list carries a keyword.
func specHasKeyword(specs ast.DeclSpecs, k token.Kind) bool {
	for _, s := range specs {
		if ks, ok := s.(*ast.KeywordSpec); ok && ks.Kind == k {
			return true
		}
	}
	return false
}

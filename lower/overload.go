package lower

import (
	"strconv"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Telling the overloads of a name apart.
//
// `__attribute__((overloadable))` lets one name be several functions, and an
// object file has one symbol per name. clang resolves that by mangling: an
// overloaded function gets an Itanium-mangled symbol, so `simd_abs` taking a
// simd_float4 and `simd_abs` taking a simd_double2 are two different names by
// the time the linker sees them.
//
// objv does not mangle the way clang does, and says so rather than guessing:
// a *static* overload gets a name of objv's own, which is enough because
// nothing outside the unit can refer to it, and an *external* one is refused.
// A name this compiler and clang spell differently is a program that compiles
// and does not link, with an error naming a symbol the source never wrote —
// and, worse, one that could link against the wrong function if the spellings
// happened to collide. Every overload in <simd/simd.h> is static inline, so
// the refusal costs the platform's own headers nothing.
//
// Which overload a call means was settled by the analyzer; see
// analyzer/overload.go. This only spells the answer.

// overloadIndex is a declaration's position among the overloads of its name,
// and whether the name is overloaded at all.
func (u *unit) overloadIndex(n ast.Node) (int, bool) {
	if n == nil || u.info.OverloadIndex == nil {
		return 0, false
	}
	i, ok := u.info.OverloadIndex[n]
	return i, ok
}

// linkName is the name a function declaration is emitted under.
//
// The suffix is the overload's position in the order the declarations were
// read, which the analyzer recorded and which a prototype shares with the
// definition below it. It is not a signature encoding: position is already
// unique, and a name a human has to read in a stack trace is better for
// being short.
func (u *unit) linkName(name string, n ast.Node) string {
	i, ok := u.overloadIndex(n)
	if !ok || i == 0 {
		// The first overload keeps the plain name, so that a program with
		// one declaration and an attribute nobody needed is unchanged.
		return name
	}
	return name + ".overload." + strconv.Itoa(i)
}

// callLinkName is the name a call refers to: the overload the analyzer chose
// for this callee, spelled as its definition will be.
func (u *unit) callLinkName(id *ast.Ident) string {
	name := u.name(id)
	if u.info.Overloads == nil {
		return name
	}
	chosen, ok := u.info.Overloads[id]
	if !ok {
		return name
	}
	return u.linkName(name, chosen)
}

// checkOverloadLinkage refuses an overloaded function with external linkage,
// whose symbol objv would spell differently from every other compiler.
func (u *unit) checkOverloadLinkage(name string, n ast.Node, static bool) bool {
	if _, ok := u.overloadIndex(n); !ok || static {
		return true
	}
	u.unsupported(n, "an overloaded function with external linkage ('"+name+
		"'); objv does not mangle the way clang does, so the symbol would not "+
		"be the one anything else links against. Declare it static")
	return false
}

var _ = types.IsVoid

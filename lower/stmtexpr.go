package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// A block in expression position.
//
// §6.1's `({ … })` is GNU's, not C's, and it is in every system header that
// defines a macro which has to name something. <sys/param.h> writes MAX with
// it, <AssertMacros.h> writes most of itself with it, and the pattern is
// always the same:
//
//	#define MAX(a, b) ({ __typeof__(a) _a = (a); \
//	                     __typeof__(b) _b = (b); \
//	                     _a > _b ? _a : _b; })
//
// A macro that must evaluate each argument exactly once, name the results,
// and still be an expression has this and nothing else. Without it the macro
// is the two-evaluation version, and `MAX(i++, j)` increments twice.
//
// The lowering is the body, in order, with one difference: the last item is
// an expression whose value is kept rather than a statement whose value is
// dropped. Everything else about it is an ordinary block — it has its own
// scope, its own ARC scope, and its own stack mark — because it is one.

// stmtExpr lowers `({ … })`.
func (u *unit) stmtExpr(e *ast.StmtExpr, t types.Type) ir.Value {
	if e.Body == nil {
		return nil
	}
	u.push()
	u.pushARCScope()
	u.pushVLAScope()
	// An object value leaves the block at +1: the block's own cleanups run
	// before the enclosing expression looks at it, and `({ Obj *t = …; t; })`
	// would otherwise release t's object on the way out. It is a temporary
	// of the enclosing full expression from there, as a call's result is.
	owns := u.arcOn() && objectValued(t) && !u.isWeak(t)
	var val ir.Value
	defer func() {
		u.popVLAScope()
		u.popARCScope()
		u.pop()
		if owns && val != nil && u.at() {
			u.owns(val)
		}
	}()

	for i, item := range e.Body.Items {
		last := i == len(e.Body.Items)-1
		es, isExpr := item.(*ast.ExprStmt)
		if !last || !isExpr {
			u.stmt(item)
			continue
		}
		// The value, and the one expression statement in the language
		// whose temporaries are not released where it ends: the value is
		// still being computed. They belong to the full expression this
		// whole block sits inside, and that statement releases them.
		if owns {
			v, owned := u.rvalueOwned(es.X)
			if v != nil {
				v = u.convert(v, u.typeOf(es.X), t)
				if !owned {
					v = u.retain(v, t)
				}
			}
			val = v
			continue
		}
		val = u.rvalue(es.X)
		if val != nil && t != nil {
			val = u.convert(val, u.typeOf(es.X), t)
		}
	}

	// A block that ended in a jump has no value and no way to reach one,
	// which is what `({ goto out; })` in a macro's failure arm is. The
	// enclosing expression is unreachable too; nothing else needs saying.
	if !u.at() {
		return val
	}
	if val == nil && t != nil && !types.IsVoid(t) {
		u.internal(e, "the value of this statement expression")
	}
	return val
}

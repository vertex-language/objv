package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Arrays whose length the program computes.
//
// C99 §6.7.6.2: `int a[n];` is an array of n ints, where n is whatever the
// expression evaluated to when control reached the declaration. Nothing about
// it is known at compile time — not its size, not where in the frame it sits —
// so it cannot be a frame slot. It is stack the function takes while it runs
// and gives back when the block ends, which is what ir's alloca is and what
// the arm64 backend already lowers (§D3).
//
// Two properties have to hold, and both are about the length being read once:
//
//   - `int a[n++];` increments n once. The length is evaluated where the
//     declaration is, and the array keeps that size even if n changes
//     afterwards. So the byte count is computed into a value here, and
//     everything that needs the size later reads the value rather than the
//     expression.
//
//   - `for (…) { int a[n]; … }` must not grow the stack every iteration.
//     The enclosing block saves the stack pointer on the way in and restores
//     it on the way out, so the space the body took comes back. One save per
//     block that declares one, not one per declaration: the restore puts the
//     pointer where it was, which undoes all of them at once.
//
// What is here is the one-dimensional case, which is what programs write.
// `int a[n][m]` is refused rather than miscompiled: its element type is
// itself variably modified, so `a[i]` has a stride nobody can compute from
// the type alone, and every place that decays an array or indexes one would
// have to carry the extent along. See declareVLA.

// vlaInfo is what a variably modified local remembers: how many bytes it
// took, so that sizeof can answer.
type vlaInfo struct {
	bytes ir.Value
}

// isVLA reports whether t is an array whose length is an expression.
func isVLA(t types.Type) bool {
	a, ok := types.Unqualify(t).(*types.Array)
	return ok && a.Form == types.VLA
}

// declareVLA allocates a variably modified local and binds it.
//
// It reports whether it handled the declaration. A declaration it refuses has
// been diagnosed: the alternative is the frame slot sizeAlign would have
// given it, which is a pointer's worth of stack for an array of n elements
// and a program that writes past it on the first store.
func (u *unit) declareVLA(name string, t types.Type, it *ast.InitDeclarator) bool {
	if !isVLA(t) {
		return false
	}
	arr := types.Unqualify(t).(*types.Array)
	// A refusal still binds the name. The declaration was diagnosed and no
	// object file will be written, so what the slot holds does not matter;
	// what matters is that the uses below it do not each report a second
	// time that the variable they name was never declared.
	refuse := func(what string) bool {
		u.unsupported(it, what)
		u.bind(name, &storage{kind: stLocal, typ: t, addr: u.slot(types.Typ(types.Long), name)})
		return true
	}
	if hasVariableExtent(arr.Elem) {
		return refuse("an array of more than one variably modified dimension")
	}
	if it.Init != nil {
		// §6.7.6.2p5 forbids it, and the analyzer says so. Nothing to
		// lower either way.
		return true
	}

	lenExpr := vlaExtent(it.Decl)
	if lenExpr == nil {
		return refuse("this variably modified declarator")
	}
	n := u.rvalue(lenExpr)
	if n == nil {
		return true
	}
	// The extent is an integer of whatever type the source wrote, and the
	// allocation takes a pointer-wide count. Widening follows the written
	// type's signedness: `int a[n]` with n unsigned must not sign-extend a
	// length above 2^31.
	lt := u.typeOf(lenExpr)
	var count ir.I64
	switch x := n.(type) {
	case ir.I64:
		count = x
	case ir.I32:
		count = u.widen(x, lt)
	default:
		u.internal(it, "the length of this array")
		return true
	}

	elem, align := u.sizeAlign(arr.Elem)
	if elem == 0 {
		elem = 1
	}
	b := u.fn.cur
	bytes := b.I64.Mul(count, b.I64.Const(int64(elem)))

	// The save happens before the first allocation in this block, so that
	// the restore at its end gives back everything the block took.
	u.noteVLAScope()

	p := b.Ptr.Alloca(bytes, align)
	if name != "" {
		b.Name(p, name)
	}
	u.bind(name, &storage{kind: stLocal, typ: t, addr: p, vla: &vlaInfo{bytes: bytes}})
	return true
}

// hasVariableExtent reports whether t is variably modified below its own
// outermost bracket.
func hasVariableExtent(t types.Type) bool {
	for {
		a, ok := types.Unqualify(t).(*types.Array)
		if !ok {
			return false
		}
		if a.Form == types.VLA || a.Form == types.StarArray {
			return true
		}
		t = a.Elem
	}
}

// vlaExtent is the expression in the outermost pair of brackets.
//
// The type says the array is variably modified; only the declarator says by
// what, because types.Array keeps a length and not an expression. The walk
// stops at the first array declarator, which is the outermost dimension:
// `int (*a)[n]` is a pointer and never reaches here, and `int a[n][m]` is
// refused above.
func vlaExtent(d ast.Declarator) ast.Expr {
	for {
		switch x := d.(type) {
		case *ast.ArrayDeclarator:
			if x.Len != nil {
				return x.Len
			}
			return nil
		case *ast.ParenDeclarator:
			d = x.Inner
		case *ast.PtrDeclarator:
			d = x.Inner
		case *ast.BlockPtrDeclarator:
			d = x.Inner
		case *ast.FuncDeclarator:
			d = x.Inner
		default:
			return nil
		}
	}
}

// noteVLAScope saves the stack pointer once for the innermost block that
// allocates, so that leaving the block gives the space back.
func (u *unit) noteVLAScope() {
	if u.fn == nil || len(u.fn.vlaScopes) == 0 {
		return
	}
	n := len(u.fn.vlaScopes) - 1
	if u.fn.vlaScopes[n].saved {
		return
	}
	u.fn.vlaScopes[n].saved = true
	u.fn.vlaScopes[n].token = u.fn.cur.Ptr.StackSave()
}

// pushVLAScope and popVLAScope bracket a C block. The pop is a no-op unless
// something in the block allocated, which is why the save is not written on
// the way in: most blocks declare no such array, and an unconditional save
// and restore in every one of them would pin X29 in every function.
func (u *unit) pushVLAScope() {
	if u.fn != nil {
		u.fn.vlaScopes = append(u.fn.vlaScopes, vlaScope{})
	}
}

func (u *unit) popVLAScope() {
	if u.fn == nil || len(u.fn.vlaScopes) == 0 {
		return
	}
	n := len(u.fn.vlaScopes) - 1
	s := u.fn.vlaScopes[n]
	u.fn.vlaScopes = u.fn.vlaScopes[:n]
	if s.saved && u.fn.cur != nil {
		u.fn.cur.Ptr.StackRestore(s.token)
	}
}

// vlaScope is one block's stack mark.
type vlaScope struct {
	saved bool
	token ir.Ptr
}

// vlaBytes is the size of a variably modified object, for sizeof.
//
// §6.5.3.4p2: sizeof applied to one is evaluated at run time, and its value
// is the size the object actually has — the byte count computed where it was
// declared, not a fresh reading of the length expression.
func (u *unit) vlaBytes(e ast.Expr) (ir.Value, bool) {
	id, ok := stripParens(e).(*ast.Ident)
	if !ok {
		return nil, false
	}
	st := u.lookup(u.name(id))
	if st == nil || st.vla == nil {
		return nil, false
	}
	return st.vla.bytes, true
}

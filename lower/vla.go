package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Lowering for C99 variable-length arrays (VLAs, §6.7.6.2).
//
// A variable length is evaluated once, where its declaration is reached, and
// the byte size of the array type it gives is recorded against that type.
// Everything that scales by an element's size -- indexing, pointer
// arithmetic, pointer difference, sizeof -- asks elemBytes, which answers
// with the recorded size for a variably sized type and the constant one
// otherwise. So `double m[r][c]` indexes rows c*8 bytes apart, and so does a
// `double (*p)[c]` pointing into it.
//
// The object itself is allocated with alloca, and the enclosing block saves
// and restores the stack pointer to give the space back.

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
	// A refusal still binds the name. The declaration was diagnosed and no
	// object file will be written, so what the slot holds does not matter;
	// what matters is that the uses below it do not each report a second
	// time that the variable they name was never declared.
	refuse := func(what string) bool {
		u.unsupported(it, what)
		u.bind(name, &storage{kind: stLocal, typ: t, addr: u.slot(types.Typ(types.Long), name)})
		return true
	}
	if it.Init != nil {
		// §6.7.6.2p5 forbids it, and the analyzer says so. Nothing to
		// lower either way.
		return true
	}
	if !u.evalVLATypes(it.Decl, t) {
		return refuse("this variably modified declarator")
	}
	bytes, ok := u.dynSize(t)
	if !ok {
		return refuse("this variably modified declarator")
	}
	_, align := u.sizeAlign(innermostElem(t))
	b := u.fn.cur

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

// innermostElem is the element type under every array dimension.
func innermostElem(t types.Type) types.Type {
	for {
		a := types.AsArray(t)
		if a == nil {
			return t
		}
		t = a.Elem
	}
}

// evalVLATypes evaluates the variable lengths a declarator gives t, in the
// order they are written, and records the size of every variably sized
// array type among them. It reports false for a shape it cannot pair up.
//
// The declarator nests the other way round from the type: in `m[r][c]` the
// outermost ArrayDeclarator carries c, the innermost dimension. So the
// lengths read from the outside in, reversed, are the type's arrays read
// from the top down -- through pointers too, `(*p)[c]` being a pointer to
// an array of c.
func (u *unit) evalVLATypes(d ast.Declarator, t types.Type) bool {
	lens := arrayLens(d)
	var arrays []*types.Array
	for cur := t; ; {
		if a := types.AsArray(cur); a != nil {
			arrays = append(arrays, a)
			cur = a.Elem
			continue
		}
		if p := types.AsPointer(cur); p != nil {
			cur = p.Elem
			continue
		}
		break
	}
	if len(arrays) != len(lens) {
		return false
	}
	counts := make([]ir.I64, len(arrays))
	for i, a := range arrays {
		if a.Form != types.VLA {
			continue
		}
		e := lens[len(lens)-1-i]
		if e == nil {
			return false
		}
		n := u.rvalue(e)
		switch x := n.(type) {
		case ir.I64:
			counts[i] = x
		case ir.I32:
			// Widened as the written type says: `int a[n]` with n
			// unsigned must not sign-extend a length above 2^31.
			counts[i] = u.widen(x, u.typeOf(e))
		default:
			return false
		}
	}
	if u.fn.vlaSize == nil {
		u.fn.vlaSize = map[*types.Array]ir.I64{}
	}
	// Innermost first, so each size is there for the one above it.
	for i := len(arrays) - 1; i >= 0; i-- {
		a := arrays[i]
		if a.Form != types.VLA {
			continue
		}
		b := u.fn.cur
		u.fn.vlaSize[a] = b.I64.Mul(counts[i], u.elemBytes(a.Elem))
	}
	return true
}

// arrayLens is the length expression of every array declarator, from the
// outside in; nil for one with none.
func arrayLens(d ast.Declarator) []ast.Expr {
	var out []ast.Expr
	for d != nil {
		switch x := d.(type) {
		case *ast.ArrayDeclarator:
			out = append(out, x.Len)
			d = x.Inner
		case *ast.ParenDeclarator:
			d = x.Inner
		case *ast.PtrDeclarator:
			d = x.Inner
		case *ast.BlockPtrDeclarator:
			d = x.Inner
		default:
			return out
		}
	}
	return out
}

// isVariablyModified reports whether t has a variable length anywhere under
// its pointers and arrays.
func isVariablyModified(t types.Type) bool {
	for {
		if a := types.AsArray(t); a != nil {
			if a.Form == types.VLA {
				return true
			}
			t = a.Elem
			continue
		}
		if p := types.AsPointer(t); p != nil {
			t = p.Elem
			continue
		}
		return false
	}
}

// isVariablySized reports whether a value of t has a size known only at run
// time: a variable-length array, or a fixed array of them.
func isVariablySized(t types.Type) bool {
	return isVLA(t) || (types.IsArray(t) && hasVariableExtent(t))
}

// dynSize is the run-time byte size of a variably sized type, or false for a
// type that is not one or whose length this function never evaluated.
func (u *unit) dynSize(t types.Type) (ir.I64, bool) {
	a := types.AsArray(t)
	if a == nil || u.fn == nil {
		return ir.I64{}, false
	}
	if a.Form == types.VLA {
		v, ok := u.fn.vlaSize[a]
		return v, ok
	}
	if a.Form == types.FixedArray && hasVariableExtent(a.Elem) {
		inner, ok := u.dynSize(a.Elem)
		if !ok {
			return ir.I64{}, false
		}
		b := u.fn.cur
		return b.I64.Mul(b.I64.Const(a.Len), inner), true
	}
	return ir.I64{}, false
}

// elemBytes is the size of t as an i64 to scale by: the run-time size of a
// variably sized type, and the constant size of any other.
func (u *unit) elemBytes(t types.Type) ir.I64 {
	if v, ok := u.dynSize(t); ok {
		return v
	}
	size, _ := u.sizeAlign(t)
	return u.fn.cur.I64.Const(int64(size))
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
	if id, ok := stripParens(e).(*ast.Ident); ok {
		if st := u.lookup(u.name(id)); st != nil && st.vla != nil {
			return st.vla.bytes, true
		}
	}
	// A row of one, m[i], or anything else whose type's size was
	// recorded where it was declared.
	if v, ok := u.dynSize(u.typeOf(e)); ok {
		return v, true
	}
	return nil, false
}

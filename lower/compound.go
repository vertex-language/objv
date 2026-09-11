package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// §6.5.2.5's compound literal.
//
// `(struct Vec){ 1, 2 }` is an unnamed *object*, not a value: it has a
// lifetime, it has an address, and it is an lvalue. Which lifetime is the
// scope it was written in — automatic inside a function, static at file
// scope — and that is the whole of the difference between the two halves
// below. The initializer is the same one a declaration would have taken, so
// both halves hand it to the same walk.
//
// It is an lvalue and not a temporary, which is what makes
// `&(struct S){1, 2}` and `(int[]){1, 2, 3}[i]` mean something. Inside a
// function the object is re-initialized every time control reaches the
// literal; the storage is the same each time, which is what "the enclosing
// block" means.

// compoundLit is a compound literal where a value is wanted. An aggregate is
// held by address in this package, so that *is* the value; a scalar one is
// loaded.
func (u *unit) compoundLit(e *ast.CompoundLit, t types.Type) ir.Value {
	addr, at, ok := u.compoundAddr(e)
	if !ok {
		return nil
	}
	if isAggregate(at) {
		return addr
	}
	return u.loadFrom(addr, at)
}

// compoundAddr is the object a compound literal names.
func (u *unit) compoundAddr(e *ast.CompoundLit) (ir.Ptr, types.Type, bool) {
	t := u.typeOf(e)
	if t == nil {
		u.errorf(e, "internal: no type recorded for a compound literal")
		return ir.Ptr{}, nil, false
	}
	if u.fn == nil {
		// At file scope there is no frame and no instruction stream. §6.5.2.5p5
		// gives the object static storage duration, and what a file-scope
		// initializer can do with it is take its address — which is
		// compoundConst's job, not this one's.
		u.errorf(e, "internal: a compound literal wanted a frame at file scope")
		return ir.Ptr{}, nil, false
	}
	slot := u.slot(t, "compound")
	u.initLocal(slot, t, e.Init)
	return slot, t, true
}

// compoundGlobal emits the object a compound literal at file scope is.
func (u *unit) compoundGlobal(e *ast.CompoundLit, t types.Type) (ir.Symbol, bool) {
	f, ok := u.ftype(t)
	if !ok {
		u.unsupported(e, "a compound literal of type "+t.String()+" at file scope")
		return nil, false
	}
	init, ok := u.constInit(e.Init, t)
	if !ok {
		u.unsupported(e, "this compound literal at file scope")
		return nil, false
	}
	g := u.mod.Global(u.sym(u.uniq("compound")), ir.RW, f).Internal()
	_, align := u.sizeAlign(t)
	g.Align(align)
	g.Init(init)
	return g, true
}

// compoundConst is a compound literal in a file-scope initializer, where what
// is wanted is not its value but the address of the object it is.
//
//	static struct S *p = &(struct S){ 1, 2 };
func (u *unit) compoundConst(e *ast.CompoundLit) (ir.Init, bool) {
	t := u.typeOf(e)
	if t == nil {
		return ir.Init{}, false
	}
	sym, ok := u.compoundGlobal(e, t)
	if !ok {
		return ir.Init{}, false
	}
	return ir.RelocInit(sym), true
}

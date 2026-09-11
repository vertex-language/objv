package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// __block variables.
//
// A block captures a copy, which is what makes a capture const and a block
// cheap. `__block` asks for the other thing: one variable, shared by the
// function and by every block that captured it, and still shared after the
// block has outlived the frame the variable was declared in.
//
// It is arranged by moving the variable out of the frame and into a
// structure of its own — runtime.BlockByref — that the frame and the block
// both point at. Every access goes through that structure's `forwarding`
// field rather than to it directly, and that indirection is the whole
// mechanism: while the structure is on the stack, forwarding points at
// itself; when _Block_copy moves it to the heap, the stack copy's forwarding
// is rewritten to the heap one, and both frames go on reading one object.
//
//	    __block int n = 5;              n++ inside a block becomes
//	                                    byref->forwarding->n += 1
//	 ┌──────────────┐
//	 │ isa      = 0 │  the structure, in the frame that declared n
//	 │ forwarding ──┼──▶ itself, until _Block_copy says otherwise
//	 │ flags    = 0 │
//	 │ size    = 32 │
//	 │ n        = 5 │
//	 └──────────────┘
//
// The block literal captures the *address* of the structure, and its copy
// helper hands it to the runtime with BLOCK_FIELD_IS_BYREF, which is what
// moves it to the heap when the block is copied.

// byref is a __block variable: where its structure is, and where the
// variable sits inside it.
type byref struct {
	slot ir.Ptr     // the structure, in the declaring frame
	typ  types.Type // the variable's own type
	off  int64      // the variable's offset in the structure
	size int64      // the structure's total size, which it records
}

// byrefLayout decides a __block variable's structure: which fields it has,
// where the variable sits, and how big the whole thing is.
//
// The size is rounded up to a pointer, which is what clang writes: a
// structure holding one int is 32 bytes and not 28. The runtime copies
// exactly what the size field says.
func (u *unit) byrefLayout(t types.Type) (fields []runtime.Field, off, size int64, helpers bool) {
	fields = runtime.BlockByref
	helpers = types.IsObjectPointer(t) || types.IsBlock(t)
	if helpers {
		fields = runtime.BlockByrefWithHelpers
	}
	off = u.abi.SizeOf(fields)
	vsize, valign := u.sizeAlign(t)
	if vsize == 0 {
		vsize = 1
	}
	off = alignUp(off, int64(valign))
	size = alignUp(off+int64(vsize), u.abi.PtrBytes)
	return fields, off, size, helpers
}

// declareByref builds the structure a __block variable lives in and binds the
// name to it.
func (u *unit) declareByref(name string, t types.Type, it *ast.InitDeclarator) *byref {
	fields, off, size, helpers := u.byrefLayout(t)

	b := &byref{typ: t, off: off, size: size}
	b.slot = u.fn.entry.Ptr.Alloc(uint64(size), uint64(u.abi.PtrBytes))
	u.fn.entry.Name(b.slot, name+"_byref")

	cur := u.fn.cur
	at := func(field string) ir.Ptr {
		o, ok := u.abi.OffsetOf(fields, field)
		if !ok || o == 0 {
			return b.slot
		}
		return cur.Ptr.Add(b.slot, cur.I64.Const(o))
	}
	// isa is null: a byref is not an object, whatever its first word looks
	// like. forwarding points at the structure itself, which is what makes
	// an access correct before anything has copied it anywhere.
	cur.Ptr.Store(cur.Ptr.Const(), at("isa"))
	cur.Ptr.Store(b.slot, at("forwarding"))

	flags := runtime.BlockFlag(0)
	if helpers {
		flags = runtime.BlockByrefHasCopyDispose | u.byrefLayoutFlag(t)
	}
	cur.I32.Store(cur.I32.Const(int64(int32(flags))), at("flags"))
	cur.I32.Store(cur.I32.Const(size), at("size"))
	if helpers {
		cur.Ptr.Store(cur.Ptr.GetAddr(u.byrefCopyHelper(off)), at("byref_keep"))
		cur.Ptr.Store(cur.Ptr.GetAddr(u.byrefDisposeHelper(off)), at("byref_destroy"))

		// And the variable itself is nil, before anything reads it.
		//
		// §5.6 says a __strong location starts empty, and a store into one
		// releases what it replaced — so a structure whose payload was
		// never written holds a stack pattern that the first assignment
		// hands to objc_release. An ordinary __strong local gets the same
		// store where it is declared; this one is in a structure and was
		// being missed.
		payload := b.slot
		if off != 0 {
			payload = cur.Ptr.Add(b.slot, cur.I64.Const(off))
		}
		cur.Ptr.Store(cur.Ptr.Const(), payload)
	}

	u.bind(name, &storage{kind: stByref, typ: t, addr: b.slot, byref: b})
	u.fn.byrefs = append(u.fn.byrefs, b.slot)
	return b
}

// byrefLayoutFlag is the high nibble of a byref's flags: what the runtime
// should understand the variable to be.
//
// Unretained under manual reference counting, which is the historical answer
// and the reason `__block id` was how a block broke a retain cycle before
// there was __weak. Strong under ARC, where a __block object is owned by the
// structure like any other __strong location.
func (u *unit) byrefLayoutFlag(t types.Type) runtime.BlockFlag {
	switch {
	case u.isWeak(t):
		return runtime.BlockByrefLayoutWeak
	case u.isStrong(t):
		return runtime.BlockByrefLayoutStrong
	}
	return runtime.BlockByrefLayoutUnretained
}

// byrefAddr is where a __block variable actually is: through forwarding,
// every time. The load is not an optimization to remove — after a
// _Block_copy the stack structure's forwarding names the heap one, and an
// access that skipped it would read a copy nobody else writes to.
func (u *unit) byrefAddr(b *byref, slot ir.Ptr) ir.Ptr {
	cur := u.fn.cur
	off, _ := u.abi.OffsetOf(runtime.BlockByref, "forwarding")
	fwd := cur.Ptr.Load(cur.Ptr.Add(slot, cur.I64.Const(off)))
	if b.off == 0 {
		return fwd
	}
	return cur.Ptr.Add(fwd, cur.I64.Const(b.off))
}

// releaseByrefs hands every __block structure in this function back to the
// runtime, which is what frees the heap copy a _Block_copy made. On a
// structure nothing copied it does nothing, so the call is unconditional and
// the compiler does not have to know which happened.
func (u *unit) releaseByrefs() {
	if !u.at() || len(u.fn.byrefs) == 0 {
		return
	}
	sig := ir.NewSig()
	sig.Param(ir.TypePtr)
	sig.Param(ir.TypeI32)
	dispose := u.extern(runtime.BlockObjectDispose, sig)
	for i := len(u.fn.byrefs) - 1; i >= 0; i-- {
		u.fn.cur.Call(dispose, u.fn.byrefs[i], u.fn.cur.I32.Const(runtime.BlockFieldByref))
	}
}

// byrefCopyHelper and byrefDisposeHelper are the structure's own helpers,
// which the runtime calls when it moves one to the heap and when the heap
// copy dies. They exist only for a variable holding something the runtime
// has to hand over rather than copy.
//
// The flag is BLOCK_FIELD_IS_OBJECT or'd with BLOCK_BYREF_CALLER — 0x83 —
// which tells the runtime the caller is the byref machinery and not a
// block's copy helper. They are interned by the variable's offset because
// that is all they depend on: one helper serves every byref with an object
// there.
func (u *unit) byrefCopyHelper(off int64) ir.Symbol {
	name := runtime.BlockByrefCopySymbol(int(off))
	if sym, ok := u.byrefHelpers[name]; ok {
		return sym
	}
	fn := u.mod.Func(u.sym(name))
	fn.Internal()
	dst := fn.ParamPtr("dst")
	src := fn.ParamPtr("src")

	prev := u.fn
	u.fn = &fnState{fn: fn, ret: types.Typ(types.Void), labels: map[string]*ir.Block{}}
	defer func() { u.fn = prev }()
	b := fn.Entry()
	u.fn.entry, u.fn.cur = b, b

	sig := ir.NewSig()
	sig.Param(ir.TypePtr)
	sig.Param(ir.TypePtr)
	sig.Param(ir.TypeI32)
	b.Call(u.extern(runtime.BlockObjectAssign, sig),
		b.Ptr.Add(dst, b.I64.Const(off)),
		b.Ptr.Load(b.Ptr.Add(src, b.I64.Const(off))),
		b.I32.Const(runtime.BlockFieldObject|runtime.BlockByrefCaller))
	b.Return()
	u.byrefHelpers[name] = fn
	return fn
}

func (u *unit) byrefDisposeHelper(off int64) ir.Symbol {
	name := runtime.BlockByrefDisposeSymbol(int(off))
	if sym, ok := u.byrefHelpers[name]; ok {
		return sym
	}
	fn := u.mod.Func(u.sym(name))
	fn.Internal()
	src := fn.ParamPtr("src")

	prev := u.fn
	u.fn = &fnState{fn: fn, ret: types.Typ(types.Void), labels: map[string]*ir.Block{}}
	defer func() { u.fn = prev }()
	b := fn.Entry()
	u.fn.entry, u.fn.cur = b, b

	sig := ir.NewSig()
	sig.Param(ir.TypePtr)
	sig.Param(ir.TypeI32)
	b.Call(u.extern(runtime.BlockObjectDispose, sig),
		b.Ptr.Load(b.Ptr.Add(src, b.I64.Const(off))),
		b.I32.Const(runtime.BlockFieldObject|runtime.BlockByrefCaller))
	b.Return()
	u.byrefHelpers[name] = fn
	return fn
}

// refreshByref re-reads the address of a __block variable named by an
// identifier, and is the answer to the one thing the forwarding indirection
// is easy to get wrong.
//
// Every access goes through the structure's forwarding field, and the value
// of that field changes under an assignment whose right-hand side copies a
// block: `fact = ^{ … fact … }` retains the block, retaining a block copies
// it to the heap, and a block that captured this variable takes the
// structure with it. An address read before that names the stack copy, which
// from then on is the one nobody reads.
//
// Only an identifier, because only an identifier can be re-resolved for
// free: `a[i++]` would evaluate its subscript twice.
func (u *unit) refreshByref(e ast.Expr, addr ir.Ptr) ir.Ptr {
	id, ok := stripParens(e).(*ast.Ident)
	if !ok {
		return addr
	}
	st := u.lookup(u.name(id))
	if st == nil || st.kind != stByref || st.byref == nil {
		return addr
	}
	return u.byrefAddr(st.byref, st.addr)
}

// initByref writes a __block variable's initializer.
//
// The structure's address is taken after the value and not before it, for
// refreshByref's reason: `__block Handler h = ^{ … h … };` retains the block,
// retaining a block copies it to the heap, and the block captured this very
// structure — so an address read first names the copy nobody will read again.
//
// Only for something held in a register. An aggregate initializer writes
// through the address as it goes, so there is no "after" to take it at, and
// an aggregate is not something a retain can move.
func (u *unit) initByref(b *byref, t types.Type, init ast.Expr) {
	if isAggregate(t) || !objectValued(t) || (u.arcOn() && u.isWeak(t)) {
		u.initLocal(u.byrefAddr(b, b.slot), t, init)
		return
	}
	v, owned := u.rvalueOwned(init)
	if v == nil {
		return
	}
	v = u.convert(v, u.typeOf(init), t)
	if u.arcOn() && u.isStrong(t) && !owned {
		v = u.retain(v, t)
	}
	u.storeTo(u.byrefAddr(b, b.slot), v, t)
}

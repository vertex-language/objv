package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// Accessor synthesis for properties (§4.8).
//
// Synthesizes getter and setter implementations for stored properties not explicitly written:
//   - Direct ivar load/store for assign/scalars and nonatomic strong
//   - objc_getProperty / objc_setProperty runtime calls for atomic or copy object properties

// synthesizeAccessors emits the accessors a class implementation did not
// write, for every property the analyzer gave storage to.
func (u *unit) synthesizeAccessors(k *types.Class, written map[string]bool) {
	for _, p := range k.Properties {
		if p.Ivar == "" || p.Dynamic || p.Has(types.PropClass) {
			continue
		}
		if !written[p.Getter] {
			u.synthesizeGetter(k, p)
		}
		if !p.Has(types.PropReadonly) && !written[p.Setter] {
			u.synthesizeSetter(k, p)
		}
	}
}

// accessor builds the function a synthesized accessor is: self, _cmd, and
// the value a setter takes. It also records the method, without which the
// class publishes no such selector and the runtime cannot find the body.
//
// The unit's function state is swapped for the duration, exactly as a
// block's invoke function swaps it: this is a whole function being emitted
// while another declaration is being walked.
func (u *unit) accessor(k *types.Class, sel string, value types.Type) (fn *ir.Func, self, cmd, sret ir.Ptr, val ir.Value, ok bool) {
	m := k.Lookup(sel, false)
	if m == nil {
		return nil, ir.Ptr{}, ir.Ptr{}, ir.Ptr{}, nil, false
	}
	fn = u.mod.Func(u.sym(runtime.MethodSymbol(k.Name, "", sel, false))).Internal()

	// A struct-valued property is read and written like any other value
	// this size: the result comes back through storage the caller supplied,
	// and the argument arrives as a pointer to the caller's copy. agg.go
	// has the rules; what is different here is only that no source wrote
	// the parameter list.
	if isIndirectResult(m.Ret) {
		t, okAgg := u.aggType(m.Ret)
		if !okAgg {
			u.unsupported(nil, "a synthesized getter returning "+m.Ret.String())
			return nil, ir.Ptr{}, ir.Ptr{}, ir.Ptr{}, nil, false
		}
		sret = fn.ParamPtr("__ret", ir.SRet(t))
	}
	self = fn.ParamPtr("self")
	cmd = fn.ParamPtr("_cmd")
	if value != nil {
		if isAggregate(value) {
			t, okAgg := u.aggType(value)
			if !okAgg {
				u.unsupported(nil, "a synthesized setter taking "+value.String())
				return nil, ir.Ptr{}, ir.Ptr{}, ir.Ptr{}, nil, false
			}
			val = fn.ParamPtr("value", ir.ByVal(t))
		} else {
			r, okReg := u.reg(value)
			if !okReg {
				u.unsupported(nil, "a synthesized setter taking "+value.String())
				return nil, ir.Ptr{}, ir.Ptr{}, ir.Ptr{}, nil, false
			}
			val = addParam(fn, r, "value", u.narrowAttrs(value)...)
		}
	}
	if !types.IsVoid(m.Ret) && !isIndirectResult(m.Ret) {
		r, okReg := u.reg(m.Ret)
		if !okReg {
			u.unsupported(nil, "a synthesized getter returning "+m.Ret.String())
			return nil, ir.Ptr{}, ir.Ptr{}, ir.Ptr{}, nil, false
		}
		setReturn(fn, r)
	}

	u.methodFns = append(u.methodFns, methodFn{class: k, sig: m, fn: fn})
	return fn, self, cmd, sret, val, true
}

// enterAccessor makes fn the function being emitted and returns what puts the
// enclosing one back.
func (u *unit) enterAccessor(fn *ir.Func, ret types.Type, k *types.Class) func() {
	prev, prevScope := u.fn, u.scope
	u.fn = &fnState{fn: fn, ret: ret, labels: map[string]*ir.Block{}, class: k}
	u.scope = u.top
	leave := u.enterFunc(fn)
	entry := fn.Entry()
	u.fn.entry, u.fn.cur = entry, entry
	return func() {
		leave()
		u.fn, u.scope = prev, prevScope
	}
}

// synthesizeGetter is `- (T)name { return self->_name; }`, or the runtime
// call an atomic object property needs instead.
func (u *unit) synthesizeGetter(k *types.Class, p *types.Property) {
	fn, self, cmd, sret, _, ok := u.accessor(k, p.Getter, nil)
	if !ok {
		return
	}
	leave := u.enterAccessor(fn, p.Type, k)
	defer leave()
	u.fn.sret = sret
	b := u.fn.cur

	off := u.ivarOffset(k, p.Ivar)
	if isIndirectResult(p.Type) {
		// The result is written into the caller's storage, and the function
		// returns nothing: what would come back is already where the caller
		// will read it.
		u.copyAggregate(sret, b.Ptr.Add(self, off), p.Type)
		b.Return()
		return
	}
	if u.isWeak(p.Type) {
		// A weak getter reads through the runtime, which hands back a value
		// it has autoreleased — the object may go away between the read and
		// the caller's use, and only the runtime knows.
		// Read at +1 and handed back through the return handshake, as
		// clang's weak getter does: the caller that retains the result
		// claims this reference and nothing reaches a pool.
		v, ok := u.loadWeakRetained(b.Ptr.Add(self, off)).(ir.Ptr)
		if !ok {
			return
		}
		sig := ir.NewSig()
		sig.Param(ir.TypePtr).Ret(ir.TypePtr)
		b.TailCall(u.extern(runtime.AutoreleaseReturnValue, sig), v)
		return
	}
	if u.atomicObject(p) {
		sig := ir.NewSig()
		sig.Param(ir.TypePtr)
		sig.Param(ir.TypePtr)
		sig.Param(ir.TypeI64)
		sig.Param(ir.TypeI32)
		sig.Ret(ir.TypePtr)
		res := b.Call(u.extern(runtime.GetProperty, sig), self, cmd, off, b.I32.Const(1))
		if res.Len() == 0 {
			return
		}
		b.Return(res.Value(0))
		return
	}
	v := u.loadFrom(b.Ptr.Add(self, off), p.Type)
	if v == nil {
		return
	}
	b.Return(v)
}

// synthesizeSetter is `- (void)setName:(T)v { self->_name = v; }`, or the
// runtime call a property that owns its value needs instead.
func (u *unit) synthesizeSetter(k *types.Class, p *types.Property) {
	fn, self, cmd, _, val, ok := u.accessor(k, p.Setter, p.Type)
	if !ok {
		return
	}
	leave := u.enterAccessor(fn, types.Typ(types.Void), k)
	defer leave()
	b := u.fn.cur

	off := u.ivarOffset(k, p.Ivar)
	if isAggregate(p.Type) {
		src, okPtr := val.(ir.Ptr)
		if !okPtr {
			return
		}
		u.copyAggregate(b.Ptr.Add(self, off), src, p.Type)
		b.Return()
		return
	}
	if u.isWeak(p.Type) {
		u.storeWeak(b.Ptr.Add(self, off), val)
		b.Return()
		return
	}
	if copies, owns := u.ownsValue(p); owns {
		sig := ir.NewSig()
		sig.Param(ir.TypePtr)
		sig.Param(ir.TypePtr)
		sig.Param(ir.TypePtr)
		sig.Param(ir.TypeI64)
		name := runtime.SetPropertySymbol(!p.Has(types.PropNonatomic), copies)
		b.Call(u.extern(name, sig), self, cmd, val, off)
		b.Return()
		return
	}
	u.storeTo(b.Ptr.Add(self, off), val, p.Type)
	b.Return()
}

// ivarOffset loads the variable the runtime writes when it realizes the
// class. It is the non-fragile ABI in one instruction, and it is why a
// synthesized accessor is correct after a superclass grows a member.
func (u *unit) ivarOffset(k *types.Class, ivar string) ir.I64 {
	_, owner := k.FindIvar(ivar)
	name := k.Name
	if owner != nil {
		name = owner.Name
	}
	b := u.fn.cur
	return b.I64.SLoad32(b.Ptr.GetAddr(u.ivarOffsetSymbol(name, ivar)))
}

// atomicObject reports whether a getter has to go through the runtime: an
// object read that a setter must not interleave with.
func (u *unit) atomicObject(p *types.Property) bool {
	return objectValued(p.Type) && !p.Has(types.PropNonatomic)
}

// ownsValue reports whether the setter hands the value to the runtime, and
// whether the runtime should copy it rather than retain it. A property that
// only assigns does neither.
func (u *unit) ownsValue(p *types.Property) (copies, owns bool) {
	if !objectValued(p.Type) {
		return false, false
	}
	switch {
	case p.Has(types.PropCopy):
		return true, true
	case p.Has(types.PropRetain), p.Has(types.PropStrong):
		return false, true
	}
	return false, false
}

// objectValued reports whether ARC manages a value of t: an object pointer
// or a block. Not Class: clang never retains or releases a class object --
// a class need not implement retain at all, and a root class of the
// program's own does not, so a retain sent to one aborts in forwarding.
func objectValued(t types.Type) bool {
	return (types.IsObjectPointer(t) && !types.IsClassType(t)) || types.IsBlock(t)
}

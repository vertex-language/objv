package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// The accessors nobody wrote.
//
// §4.8: a property with neither @synthesize nor @dynamic still has an
// instance variable and a pair of accessors — the modern runtime synthesizes
// by default, and a program that writes `@property (copy) NSString *name;`
// and nothing else expects `-name` and `-setName:` to exist. The analyzer
// creates the *declarations*, so that a send to one typechecks; this creates
// the bodies, without which the send reaches the runtime and fails with
// "unrecognized selector".
//
// Most of the bodies are one load or one store. The interesting ones are a
// call, and which call is the whole of what a property's attributes mean:
//
//	nonatomic, assign    load and store through the ivar's offset
//	nonatomic, copy      objc_setProperty_nonatomic_copy
//	atomic, retain       objc_getProperty, objc_setProperty_atomic
//
// An atomic *scalar* is stored directly all the same: a word-sized store is
// already indivisible, and clang emits the same thing. Only an object needs
// the runtime, because reading a pointer and retaining it have to happen
// without a setter running in between, and the runtime owns that lock.

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
func (u *unit) accessor(k *types.Class, sel string, value types.Type) (fn *ir.Func, self, cmd ir.Ptr, val ir.Value, ok bool) {
	m := k.Lookup(sel, false)
	if m == nil {
		return nil, ir.Ptr{}, ir.Ptr{}, nil, false
	}
	fn = u.mod.Func(u.sym(runtime.MethodSymbol(k.Name, "", sel, false))).Internal()

	self = fn.ParamPtr("self")
	cmd = fn.ParamPtr("_cmd")
	if value != nil {
		r, okReg := u.reg(value)
		if !okReg {
			u.unsupported(nil, "a synthesized setter taking "+value.String())
			return nil, ir.Ptr{}, ir.Ptr{}, nil, false
		}
		val = addParam(fn, r, "value")
	}
	if !types.IsVoid(m.Ret) {
		r, okReg := u.reg(m.Ret)
		if !okReg {
			u.unsupported(nil, "a synthesized getter returning "+m.Ret.String())
			return nil, ir.Ptr{}, ir.Ptr{}, nil, false
		}
		setReturn(fn, r)
	}

	u.methodFns = append(u.methodFns, methodFn{class: k, sig: m, fn: fn})
	return fn, self, cmd, val, true
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
	fn, self, cmd, _, ok := u.accessor(k, p.Getter, nil)
	if !ok {
		return
	}
	leave := u.enterAccessor(fn, p.Type, k)
	defer leave()
	b := u.fn.cur

	off := u.ivarOffset(k, p.Ivar)
	if u.isWeak(p.Type) {
		// A weak getter reads through the runtime, which hands back a value
		// it has autoreleased — the object may go away between the read and
		// the caller's use, and only the runtime knows.
		v := u.loadWeak(b.Ptr.Add(self, off))
		if v == nil {
			return
		}
		b.Return(v)
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
	fn, self, cmd, val, ok := u.accessor(k, p.Setter, p.Type)
	if !ok {
		return
	}
	leave := u.enterAccessor(fn, types.Typ(types.Void), k)
	defer leave()
	b := u.fn.cur

	off := u.ivarOffset(k, p.Ivar)
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

func objectValued(t types.Type) bool { return types.IsObjectPointer(t) || types.IsBlock(t) }

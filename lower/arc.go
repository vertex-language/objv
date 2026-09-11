package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// Automatic reference counting: the retains and releases the program did not
// write.
//
// ARC is not a garbage collector and not a rewrite. It is a set of rules
// about where *ownership* changes, and every one of the calls below is
// placed at a point where it does. The analyzer decided the ownership —
// every object variable has a lifetime, most programs write none, and §5.6's
// default is __strong — and this places the operations that keep it true.
//
// Two facts do all the work.
//
// The first is that an object rvalue is either *owned* — the expression
// produced it at +1, and somebody has to release it — or *borrowed*, valid
// only for as long as whatever is holding it holds it. Which one is decided
// by the selector's name, and the naming convention is normative: alloc,
// copy, init, mutableCopy and new return an object the caller owns, and
// every other method returns one it does not. runtime.FamilyOf is that rule.
//
// The second is that an owned value nobody takes has to be released at the
// end of the full expression. So every owned value is registered as it is
// produced, and a context that *takes* ownership — initializing a __strong
// variable, assigning to one, returning from a method that returns +1 —
// takes it back off the register instead. What is left at the end of the
// statement is what nothing wanted.
//
// That is the whole mechanism. `[[Box alloc] init]` registers alloc's +1,
// init consumes its receiver and registers its own, and the declaration it
// initializes takes that one — so the object is retained once, by the
// variable, and released once, when the variable goes out of scope.
//
// What is not here: __weak, which is refused rather than approximated,
// because a zeroing weak reference is the runtime's side table and not a
// call this could place; and the optimizations clang applies to the pairs it
// can prove redundant, which cost instructions and not correctness.

// arcOn reports whether ownership operations are emitted at all.
func (u *unit) arcOn() bool { return u.arc && u.fn != nil && u.at() }

// owns registers a value the current full expression produced at +1.
func (u *unit) owns(v ir.Value) {
	if u.arcOn() && v != nil {
		u.fn.temps = append(u.fn.temps, v)
	}
}

// takeOwned removes a value from the register and reports whether it was
// there. The caller is claiming the +1 it carries.
func (u *unit) takeOwned(v ir.Value) bool {
	if !u.arcOn() || v == nil {
		return false
	}
	for i := len(u.fn.temps) - 1; i >= 0; i-- {
		if u.fn.temps[i] == v {
			u.fn.temps = append(u.fn.temps[:i], u.fn.temps[i+1:]...)
			return true
		}
	}
	return false
}

// rvalueOwned evaluates an expression for a context that takes ownership of
// the result, and says whether it got one. A borrowed value has to be
// retained by the caller; an owned one is already the caller's.
func (u *unit) rvalueOwned(e ast.Expr) (ir.Value, bool) {
	v := u.rvalue(e)
	if v == nil || !u.arcOn() || !objectValued(u.typeOf(e)) {
		return v, false
	}
	return v, u.takeOwned(v)
}

// releaseTemps ends a full expression: everything still registered is
// something nothing took, and §7.4's end of the statement is where it goes.
func (u *unit) releaseTemps() {
	if u.fn == nil || len(u.fn.temps) == 0 {
		return
	}
	temps := u.fn.temps
	u.fn.temps = nil
	if !u.at() {
		return
	}
	for i := len(temps) - 1; i >= 0; i-- {
		u.release(temps[i])
	}
}

// ---- the runtime's four calls ----

func (u *unit) retain(v ir.Value) ir.Value {
	p, ok := v.(ir.Ptr)
	if !ok || !u.arcOn() {
		return v
	}
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Ret(ir.TypePtr)
	res := u.fn.cur.Call(u.extern(runtime.Retain, sig), p)
	if res.Len() == 0 {
		return v
	}
	return res.Value(0)
}

func (u *unit) release(v ir.Value) {
	p, ok := v.(ir.Ptr)
	if !ok || !u.arcOn() {
		return
	}
	sig := ir.NewSig()
	sig.Param(ir.TypePtr)
	u.fn.cur.Call(u.extern(runtime.Release, sig), p)
}

// autoreleaseReturn hands a +1 value back at +0, which is what a method
// outside the four retaining families returns.
//
// objc_autoreleaseReturnValue rather than objc_autorelease: the two differ
// in what the *caller* may do with the result, and the pair is what lets a
// caller's objc_retainAutoreleasedReturnValue skip the pool entirely.
func (u *unit) autoreleaseReturn(v ir.Value) ir.Value {
	p, ok := v.(ir.Ptr)
	if !ok || !u.arcOn() {
		return v
	}
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Ret(ir.TypePtr)
	res := u.fn.cur.Call(u.extern(runtime.AutoreleaseReturnValue, sig), p)
	if res.Len() == 0 {
		return v
	}
	return res.Value(0)
}

// ---- where ownership changes ----

// isStrong reports whether a location of this type owns what it holds.
func (u *unit) isStrong(t types.Type) bool {
	return u.arc && objectValued(t) && types.LifetimeOf(t) == types.LifeStrong
}

// initStrong writes the first value into a __strong location, which holds
// nothing yet: there is no old value to release, only a new one to own.
func (u *unit) initStrong(addr ir.Ptr, t types.Type, init ast.Expr) {
	v, owned := u.rvalueOwned(init)
	if v == nil {
		return
	}
	v = u.convert(v, u.typeOf(init), t)
	if !owned {
		v = u.retain(v)
	}
	u.storeTo(addr, v, t)
}

// storeStrong replaces what a __strong location holds: the new value is
// owned, the old one is let go.
//
// The store comes before the release, which is not a detail. `x = x` and
// `a.b = a.b` reach here with the same object on both sides, and releasing
// first can free what is about to be stored.
func (u *unit) storeStrong(addr ir.Ptr, t types.Type, v ir.Value, owned bool) ir.Value {
	if !owned {
		v = u.retain(v)
	}
	old := u.loadFrom(addr, t)
	u.storeTo(addr, v, t)
	u.release(old)
	return v
}

// releaseStrongLocal is what a __strong variable's scope ending does.
func (u *unit) releaseStrongLocal(addr ir.Ptr, t types.Type) {
	if !u.at() {
		return
	}
	u.release(u.loadFrom(addr, t))
}

// returnValue applies §the caller's convention to a returned object.
//
// A method in one of the retaining families returns +1 and the caller
// releases it; every other method returns +0, which for a value this frame
// owns means handing it to the autorelease pool on the way out — the frame
// is about to release everything it holds, this value included.
func (u *unit) returnObject(v ir.Value, owned bool) ir.Value {
	if !u.arcOn() || !objectValued(u.fn.ret) {
		return v
	}
	if u.fn.retainedReturn {
		if !owned {
			v = u.retain(v)
		}
		return v
	}
	if owned {
		return u.autoreleaseReturn(v)
	}
	// A borrowed value: whatever holds it may be released before the caller
	// looks, and the frame's own cleanups run between here and there.
	return u.autoreleaseReturn(u.retain(v))
}

// strongLocal is a variable whose scope ending lets go of what it holds, and
// weak says how: a __strong one is released, a __weak one unregistered.
type strongLocal struct {
	addr ir.Ptr
	typ  types.Type
	weak bool
}

// pushARCScope and popARCScope bracket a C block, which is where a __strong
// local's ownership ends.
func (u *unit) pushARCScope() {
	if u.fn != nil {
		u.fn.strongs = append(u.fn.strongs, nil)
	}
}

func (u *unit) popARCScope() {
	if u.fn == nil || len(u.fn.strongs) == 0 {
		return
	}
	n := len(u.fn.strongs) - 1
	for i := len(u.fn.strongs[n]) - 1; i >= 0; i-- {
		u.endLocal(u.fn.strongs[n][i])
	}
	u.fn.strongs = u.fn.strongs[:n]
}

// noteStrongLocal records a variable whose scope will release it.
func (u *unit) noteStrongLocal(addr ir.Ptr, t types.Type) {
	if u.fn == nil || len(u.fn.strongs) == 0 {
		return
	}
	n := len(u.fn.strongs) - 1
	u.fn.strongs[n] = append(u.fn.strongs[n], strongLocal{addr: addr, typ: t})
}

// releaseAllStrong releases every __strong local in every open scope, which
// is what leaving the function does.
func (u *unit) releaseAllStrong() {
	if u.fn == nil {
		return
	}
	for i := len(u.fn.strongs) - 1; i >= 0; i-- {
		for j := len(u.fn.strongs[i]) - 1; j >= 0; j-- {
			u.endLocal(u.fn.strongs[i][j])
		}
	}
}

// consumingSelf reports whether this assignment is `self = …` inside a
// method that owns its receiver.
//
// §ARC gives an initializer the +1 it was called with and expects the +1 it
// returns to be the same one — `self = [super init];` is the line every
// initializer is written on, and what it does is take ownership, not replace
// it. Releasing what self held would release the object being initialized.
func (u *unit) consumingSelf(lhs ast.Expr) bool {
	if u.fn == nil || !u.fn.consumesSelf {
		return false
	}
	id, ok := stripParens(lhs).(*ast.Ident)
	return ok && u.name(id) == "self"
}

// returningConsumedSelf reports whether a return statement hands back the
// receiver of a method that owns it.
//
// `- (instancetype)init { … return self; }` is the shape, and the +1 it
// returns is the +1 it was called with: `self = [super init]` took it, and
// nothing since has let it go. Retaining it again would leave the object
// with a count nothing ever brings down.
func (u *unit) returningConsumedSelf(e ast.Expr) bool {
	if u.fn == nil || !u.fn.consumesSelf {
		return false
	}
	id, ok := stripParens(e).(*ast.Ident)
	return ok && u.name(id) == "self"
}

// ---- what a class owes its instance variables ----

// emitCxxDestruct writes the method the runtime calls as an object is
// deallocated, which releases every __strong instance variable.
//
// It is named `.cxx_destruct`, which is not a name a program can write and
// is exactly what makes it safe: objc4 looks the selector up on the class
// during dealloc and calls it if it is there. C++ named it; Objective-C
// borrowed the hook, because the question is the same one — an object is
// going away and its members have to be let go.
//
// Without it a class under ARC leaks everything it holds. The retains are
// emitted where the ivars are assigned, and nothing would ever balance them.
func (u *unit) emitCxxDestruct(k *types.Class) {
	if !u.arc {
		return
	}
	var owned []types.Ivar
	for _, iv := range k.Ivars {
		if !objectValued(iv.Type) {
			continue
		}
		switch types.LifetimeOf(iv.Type) {
		case types.LifeStrong, types.LifeWeak:
			owned = append(owned, iv)
		}
	}
	if len(owned) == 0 {
		return
	}

	sel := runtime.CxxDestructSelector
	fn := u.mod.Func(u.sym(runtime.MethodSymbol(k.Name, "", sel, false))).Internal()
	self := fn.ParamPtr("self")
	fn.ParamPtr("_cmd")
	u.methodFns = append(u.methodFns, methodFn{class: k, fn: fn, sig: &types.Method{
		Sel: sel, Ret: types.Typ(types.Void), Owner: k.Name,
	}})
	u.hasCxxDestruct[k] = true

	prev, prevScope := u.fn, u.scope
	u.fn = &fnState{fn: fn, ret: types.Typ(types.Void), labels: map[string]*ir.Block{}, class: k}
	u.scope = u.top
	defer func() { u.fn, u.scope = prev, prevScope }()
	b := fn.Entry()
	u.fn.entry, u.fn.cur = b, b

	// objc_storeStrong(&ivar, nil) rather than a bare release: it is one
	// call, it writes the null back, and it is what clang emits — an object
	// whose ivar is read during its own dealloc reads nil rather than a
	// pointer to something freed.
	sig := ir.NewSig()
	sig.Param(ir.TypePtr)
	sig.Param(ir.TypePtr)
	store := u.extern(runtime.StoreStrong, sig)
	for _, iv := range owned {
		off := u.ivarOffset(k, iv.Name)
		if u.isWeak(iv.Type) {
			// A weak ivar is unregistered rather than released: the object
			// it names is not this one's to let go of, and what has to go
			// away is the entry in the runtime's table.
			u.destroyWeak(b.Ptr.Add(self, off))
			continue
		}
		b.Call(store, b.Ptr.Add(self, off), b.Ptr.Const())
	}
	b.Return()
}

// dealloc under ARC ends with [super dealloc], which the program did not
// write and may not write: §ARC forbids sending it and inserts it here.
//
// Without it a user-written -dealloc overrides NSObject's and never reaches
// object_dispose, so the object is never freed and .cxx_destruct — which the
// runtime calls from there — never runs. The class's own log line prints and
// everything it holds leaks, which is a failure that looks like success.
func (u *unit) arcSuperDealloc(k *types.Class) {
	if !u.arcOn() || k == nil || k.Super == nil || u.fn.self == (ir.Ptr{}) {
		return
	}
	// The two-word structure a super send goes through: the receiver, and
	// the class the method was compiled in. objc_msgSendSuper2 finds the
	// superclass itself, which is what keeps a category attached later
	// visible.
	b := u.fn.cur
	st := u.fn.entry.Ptr.Alloc(uint64(2*u.abi.PtrBytes), uint64(u.abi.PtrBytes))
	u.fn.entry.Name(st, "super_dealloc")
	b.Ptr.Store(u.fn.self, st)
	b.Ptr.Store(u.superRef(k.Name), b.Ptr.Add(st, b.I64.Const(u.abi.PtrBytes)))
	u.send(st, true, "dealloc", nil, types.Typ(types.Void), nil)
}

// ---- __weak ----
//
// A weak reference does not keep its object alive and goes to nil when the
// object does. Neither half is something the compiler can arrange on its
// own: the runtime keeps a side table of every weak reference to an object
// and walks it during dealloc, so what lower emits is four calls and no
// stores.
//
// They are placed at the same four points every ownership question is:
// where the variable comes into being, where it is read, where it is
// written, and where its scope ends.

// isWeak reports whether a location of this type is a zeroing weak
// reference.
func (u *unit) isWeak(t types.Type) bool {
	return u.arc && objectValued(t) && types.LifetimeOf(t) == types.LifeWeak
}

// initWeak registers a new weak reference with the runtime. The slot is
// uninitialized before this — objc_storeWeak would read the old value out of
// it — which is why there are two calls and not one.
func (u *unit) initWeak(addr ir.Ptr, v ir.Value) {
	p, _ := v.(ir.Ptr)
	if v == nil {
		p = u.fn.cur.Ptr.Const()
	}
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypePtr)
	u.fn.cur.Call(u.extern(runtime.InitWeak, sig), addr, p)
}

// storeWeak replaces what a weak reference names, unregistering the old
// object and registering the new.
func (u *unit) storeWeak(addr ir.Ptr, v ir.Value) ir.Value {
	p, ok := v.(ir.Ptr)
	if !ok {
		return v
	}
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypePtr)
	res := u.fn.cur.Call(u.extern(runtime.StoreWeak, sig), addr, p)
	if res.Len() == 0 {
		return v
	}
	return res.Value(0)
}

// loadWeak reads one. It cannot be a load: between the read and the use the
// object may be deallocated, so the runtime hands back a value it has
// autoreleased and the caller may hold for the rest of the statement.
func (u *unit) loadWeak(addr ir.Ptr) ir.Value {
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Ret(ir.TypePtr)
	res := u.fn.cur.Call(u.extern(runtime.LoadWeak, sig), addr)
	if res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// destroyWeak unregisters a weak reference whose storage is going away.
func (u *unit) destroyWeak(addr ir.Ptr) {
	sig := ir.NewSig()
	sig.Param(ir.TypePtr)
	u.fn.cur.Call(u.extern(runtime.DestroyWeak, sig), addr)
}

// noteWeakLocal records a weak variable, which its scope unregisters.
func (u *unit) noteWeakLocal(addr ir.Ptr, t types.Type) {
	if u.fn == nil || len(u.fn.strongs) == 0 {
		return
	}
	n := len(u.fn.strongs) - 1
	u.fn.strongs[n] = append(u.fn.strongs[n], strongLocal{addr: addr, typ: t, weak: true})
}

// endLocal is what a variable's scope ending does to what it holds.
func (u *unit) endLocal(l strongLocal) {
	if !u.at() {
		return
	}
	if l.weak {
		u.destroyWeak(l.addr)
		return
	}
	u.releaseStrongLocal(l.addr, l.typ)
}

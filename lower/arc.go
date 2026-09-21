package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// ARC lowering places retain and release calls based on ownership rules:
//   - Object rvalues are either owned (+1 from alloc/copy/init/new per runtime.FamilyOf) or borrowed.
//   - Owned temporaries not consumed by an assignment or return are released at the end of the full expression.
//   - __strong local variables are released when their scope exits.

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
	if u.fn != nil && len(u.fn.recTemps) > 0 {
		recs := u.fn.recTemps
		u.fn.recTemps = nil
		for i := len(recs) - 1; i >= 0; i-- {
			u.destroyRecord(recs[i].addr, recs[i].typ)
		}
	}
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

// tempMark is where the owned temporaries stand now, for a branch that has
// to let go of what it made before it joins the other: a value created on
// one path of `a && [b c]` cannot be released after the paths meet.
func (u *unit) tempMark() int {
	if u.fn == nil {
		return 0
	}
	return len(u.fn.temps)
}

// releaseTempsSince releases the temporaries registered since mark.
func (u *unit) releaseTempsSince(mark int) {
	if u.fn == nil || len(u.fn.temps) <= mark {
		return
	}
	if mark < 0 {
		mark = 0
	}
	temps := append([]ir.Value(nil), u.fn.temps[mark:]...)
	u.fn.temps = u.fn.temps[:mark]
	if !u.at() {
		return
	}
	for i := len(temps) - 1; i >= 0; i-- {
		u.release(temps[i])
	}
}

// claimResult is what ARC does with an object a call hands back at +0 and
// the program uses: it is retained on the spot -- which, straight after the
// call, is objc_retainAutoreleasedReturnValue and the return handshake -- and
// released where the full expression ends, unless something takes it first.
// clang does exactly this for a result passed on as an argument, a receiver
// or a setter's value, not only one stored into a variable, and that is what
// keeps such results out of the autorelease pool.
func (u *unit) claimResult(e ast.Expr, v ir.Value) ir.Value {
	if v == nil || !u.arcOn() || !objectValued(u.typeOf(e)) {
		return v
	}
	for _, t := range u.fn.temps {
		if t == v {
			return v // already +1: new, alloc, copy, init
		}
	}
	switch x := stripParens(e).(type) {
	case *ast.MessageExpr, *ast.CallExpr, *ast.BoxedExpr, *ast.ArrayLit, *ast.DictLit:
	case *ast.MemberExpr:
		if u.info.Props[x] == nil {
			return v
		}
	default:
		return v
	}
	if u.isWeak(u.typeOf(e)) {
		return v
	}
	r := u.retain(v, u.typeOf(e))
	u.owns(r)
	return r
}

// rvalueUnclaimed is an expression's value at +0, for a consumer that uses it
// without owning it -- a __bridge cast, an __unsafe_unretained store. The
// call it may be is not claimed (clang does not claim it either): the result
// stays wherever its callee left it, which for a +0 return is the pool.
func (u *unit) rvalueUnclaimed(e ast.Expr) ir.Value {
	return u.rvalueExpr(stripParens(e))
}

// autoreleasingValue is the value an __autoreleasing location is given. It
// has to outlive the statement -- `*error = [NSError …]` is read by a caller
// after this frame is gone -- so the pool is made to hold it:
// objc_autorelease for a +1 the expression produced, objc_retainAutorelease
// for anything borrowed.
func (u *unit) autoreleasingValue(e ast.Expr, t types.Type) ir.Value {
	v, owned := u.rvalueOwned(e)
	if v == nil {
		return nil
	}
	v = u.convert(v, u.typeOf(e), t)
	p, ok := v.(ir.Ptr)
	if !ok || !u.arcOn() {
		return v
	}
	name := runtime.RetainAutorelease
	if owned {
		name = runtime.Autorelease
	}
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Ret(ir.TypePtr)
	res := u.fn.cur.Call(u.extern(name, sig), p)
	if res.Len() == 0 {
		return v
	}
	return res.Value(0)
}

// isAutoreleasing and isUnsafe name the two ownerships that neither retain
// what they are given nor release it when they go.
func (u *unit) isAutoreleasing(t types.Type) bool {
	return u.arc && objectValued(t) && types.LifetimeOf(t) == types.LifeAutoreleasing
}

func (u *unit) isUnsafe(t types.Type) bool {
	return u.arc && objectValued(t) && types.LifetimeOf(t) == types.LifeUnsafeUnretained
}

// nonOwningValue is the value an __autoreleasing or __unsafe_unretained
// location is given, or false for any other location.
func (u *unit) nonOwningValue(e ast.Expr, t types.Type) (ir.Value, bool) {
	switch {
	case !u.arcOn():
		return nil, false
	case u.isAutoreleasing(t):
		return u.autoreleasingValue(e, t), true
	case u.isUnsafe(t):
		v := u.rvalueUnclaimed(e)
		if v == nil {
			return nil, true
		}
		return u.convert(v, u.typeOf(e), t), true
	}
	return nil, false
}

// strongParam is a parameter's type as the body sees it. Under ARC an object
// parameter with no ownership of its own is __strong (§4.3.3): the callee
// retains what it was lent on entry and releases it on the way out, so that
// assigning the parameter -- `o = [Obj new]` -- releases what it held
// without taking the caller's reference with it.
func (u *unit) strongParam(t types.Type, v ir.Value) types.Type {
	if !u.arcOn() || !objectValued(t) || types.LifetimeOf(t) != types.LifeNone {
		return t
	}
	return types.WithLifetime(t, types.LifeStrong)
}

// paramValue is what a parameter's slot starts with: the argument, retained
// when the slot is a strong one.
func (u *unit) paramValue(t types.Type, v ir.Value) ir.Value {
	if u.isStrong(t) {
		return u.retain(v, t)
	}
	return v
}

// heldForStatement evaluates the object a statement works on for its whole
// length -- the collection of a for-in, the lock of @synchronized -- and
// under ARC keeps it alive until the statement is left by any path: it is
// held in a strong local of the innermost ARC scope, which the caller opens
// around the statement. A temporary, `for (x in [a items])`, would otherwise
// be released where the first statement of the body ends.
func (u *unit) heldForStatement(e ast.Expr) (ir.Ptr, bool) {
	t := u.typeOf(e)
	if !u.arcOn() || !objectValued(t) {
		p, ok := u.rvalue(e).(ir.Ptr)
		u.releaseTemps()
		return p, ok
	}
	v, owned := u.rvalueOwned(e)
	p, ok := v.(ir.Ptr)
	if !ok {
		return ir.Ptr{}, false
	}
	if !owned {
		p, _ = u.retain(p, t).(ir.Ptr)
	}
	u.releaseTemps()
	slot := u.slot(t, "")
	u.storeTo(slot, p, t)
	u.noteStrongLocal(slot, t)
	return p, true
}

// ---- the runtime's four calls ----

// retain takes a reference. t is the static type of v, which decides which
// of the runtime's two retains it is: a block literal lives in the frame that
// wrote it, so retaining one has to copy it to the heap first, and
// objc_retainBlock is the call that does. A nil t means "not a block", for
// the callers that have no type to hand.
func (u *unit) retain(v ir.Value, t types.Type) ir.Value {
	p, ok := v.(ir.Ptr)
	if !ok || !u.arcOn() {
		return v
	}
	name := runtime.Retain
	if t != nil && types.IsBlock(t) {
		name = runtime.RetainBlock
	} else if call := u.justCalled(p); call != nil {
		// The result of the call just made, at +0: the other half of the
		// return handshake. The marker after the call tells the callee's
		// objc_autoreleaseReturnValue that this retain is coming, and the
		// object skips the autorelease pool altogether -- so it is freed
		// when this frame lets go of it, not when some pool drains, and a
		// call with no pool in place leaks nothing.
		call.Meta(ir.Attached(ir.AttachObjCReturnMarker))
		name = runtime.RetainAutoreleasedReturnValue
	}
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Ret(ir.TypePtr)
	res := u.fn.cur.Call(u.extern(name, sig), p)
	if res.Len() == 0 {
		return v
	}
	return res.Value(0)
}

// justCalled is the call instruction whose result p is, when that call is the
// last thing emitted in the current block -- nothing may run between the
// callee's return and the retain that claims its result.
func (u *unit) justCalled(p ir.Ptr) *ir.Inst {
	d := p.Def()
	if d == nil || u.fn == nil || u.fn.cur == nil {
		return nil
	}
	// Inside a region that unwinds, the call is an invoke and its result
	// is the first thing its continuation block has: a parameter, with
	// nothing emitted before the retain.
	if inv, ok := u.fn.invokes[u.fn.cur]; ok && u.fn.cur.Last() == nil {
		for _, bp := range u.fn.cur.Params() {
			if bp == d {
				return inv
			}
		}
	}
	in := d.Inst()
	if in == nil || in != u.fn.cur.Last() {
		return nil
	}
	switch in.Op().Verb {
	case ir.VCall, ir.VCallInd:
		return in
	}
	return nil
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
		v = u.retain(v, t)
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
		v = u.retain(v, t)
	}
	u.replaceStrong(addr, t, v)
	return v
}

// replaceStrong is storeStrong's second half, for the caller that has to do
// something between the retain and the store: a __block variable's address
// is only valid after the retain, since retaining a block moves the
// structure it captured. See refreshByref.
func (u *unit) replaceStrong(addr ir.Ptr, t types.Type, v ir.Value) {
	old := u.loadFrom(addr, t)
	u.storeTo(addr, v, t)
	u.release(old)
}

// releaseStrongLocal is what a __strong variable's scope ending does.
func (u *unit) releaseStrongLocal(addr ir.Ptr, t types.Type) {
	if !u.at() {
		return
	}
	if u.isStrongArray(t) {
		u.releaseStrongArray(addr, t)
		return
	}
	if u.isARCRecord(t) {
		u.destroyRecord(addr, t)
		return
	}
	u.release(u.loadFrom(addr, t))
}

// isStrongArray reports whether t is a fixed array, of any rank, whose
// elements are __strong objects.
func (u *unit) isStrongArray(t types.Type) bool {
	a := types.AsArray(t)
	if a == nil || !u.arc || a.Form != types.FixedArray {
		return false
	}
	if types.IsArray(a.Elem) {
		return u.isStrongArray(a.Elem)
	}
	return u.isStrong(a.Elem)
}

// releaseStrongArray releases every element of a strong array, last first,
// with a loop: the count is the array's size in pointers, whatever its rank.
func (u *unit) releaseStrongArray(addr ir.Ptr, t types.Type) {
	size, _ := u.sizeAlign(t)
	n := int64(size) / u.abi.PtrBytes
	if n == 0 {
		return
	}
	i := u.fn.entry.Ptr.Alloc(8, 8)
	b := u.fn.cur
	b.I64.Store(b.I64.Const(n), i)
	cond, body, done := u.block("strong_array.cond"), u.block("strong_array.body"), u.block("strong_array.done")
	b.Br(cond.To())
	u.fn.cur = cond
	left := cond.I64.Load(i)
	cond.BrIf(cond.I64.Eq(left, cond.I64.Const(0)), done.To(), body.To())
	u.fn.cur = body
	k := body.I64.Sub(body.I64.Load(i), body.I64.Const(1))
	body.I64.Store(k, i)
	elem := body.Ptr.Load(body.Ptr.Add(addr, body.I64.Mul(k, body.I64.Const(u.abi.PtrBytes))))
	u.release(elem)
	u.fn.cur.Br(cond.To())
	u.fn.cur = done
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
			v = u.retain(v, u.fn.ret)
		}
		return v
	}
	// +1 through the frame's cleanups -- a borrowed value is retained, since
	// whatever holds it may be released before the caller looks -- and
	// autoreleased as the very last thing, by tail call, which is what lets
	// the caller's objc_retainAutoreleasedReturnValue claim it: the runtime
	// reads the marker at *its* caller's return address, and after a tail
	// call that is this function's caller. See returnValue.
	if !owned {
		v = u.retain(v, u.fn.ret)
	}
	u.fn.autoreleaseOnReturn = true
	return v
}

// strongLocal is a variable whose scope ending lets go of what it holds, and
// weak says how: a __strong one is released, a __weak one unregistered.
type strongLocal struct {
	addr  ir.Ptr
	typ   types.Type
	weak  bool
	byref *byref // a __block variable's structure; see endByref
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
	// -dealloc is an instance method, so the search starts above the
	// class rather than above the metaclass.
	b.Ptr.Store(u.superRef(k.Name, false), b.Ptr.Add(st, b.I64.Const(u.abi.PtrBytes)))
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

// copyWeak makes dst a new weak reference to whatever src refers to; dst is
// uninitialized before.
func (u *unit) copyWeak(dst, src ir.Ptr) {
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Param(ir.TypePtr)
	u.fn.cur.Call(u.extern(runtime.CopyWeak, sig), dst, src)
}

// loadWeakRetained reads a weak reference at +1: the object, retained, or nil
// if it has gone.
func (u *unit) loadWeakRetained(addr ir.Ptr) ir.Value {
	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Ret(ir.TypePtr)
	res := u.fn.cur.Call(u.extern(runtime.LoadWeakRetained, sig), addr)
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
	if l.byref != nil {
		u.endByref(l.byref)
		return
	}
	u.releaseStrongLocal(l.addr, l.typ)
}

// A writeback is §ARC 4.3.2's out-parameter: the temporary a call was handed,
// and the variable whose value it stands in for.
type writeback struct {
	tmp ir.Ptr
	dst ir.Ptr
	typ types.Type
}

// arcOutArg is pass-by-writeback, and it is what every NSError** in Cocoa
// depends on.
//
// `NSError *err = nil; [thing doIt:&err]` hands the callee the address of a
// __strong variable, and what the callee writes through it is autoreleased:
// nobody retained it. But the variable is one this frame releases where its
// scope ends, so handing over its real address leaves the two disagreeing
// about who owns the object — one release against no retain, which is a
// crash at the caller's scope end rather than at the call.
//
// So the callee is handed a temporary instead, seeded with what the variable
// holds, and whatever it left there is copied back afterwards with a retain.
// The parameter is `T * __autoreleasing *` and the argument is `T __strong *`;
// the temporary is where the two conventions meet.
func (u *unit) arcOutArg(e ast.Expr, param types.Type) (ir.Value, *writeback) {
	if !u.arcOn() {
		return nil, nil
	}
	un, ok := stripParens(e).(*ast.UnaryExpr)
	if !ok || un.Op != token.AND {
		return nil, nil
	}
	p := types.AsPointer(types.Unqualify(param))
	if p == nil || !objectValued(p.Elem) {
		return nil, nil
	}
	// An explicit __autoreleasing or __unsafe_unretained on the pointee says
	// the caller already knows: only a location this frame will release
	// needs the detour.
	inner := u.typeOf(un.X)
	if !u.isStrong(inner) {
		return nil, nil
	}
	addr, _ := u.lvalue(un.X)
	if addr == nil {
		return nil, nil
	}

	tmp := u.slot(inner, "")
	u.fn.cur.Ptr.Store(u.fn.cur.Ptr.Load(*addr), tmp)
	return tmp, &writeback{tmp: tmp, dst: *addr, typ: inner}
}

// applyWritebacks copies every out-parameter temporary back into the variable
// it stood in for. Nothing was retained on the way in, so the store is the
// ordinary one for a __strong location: retain the new, release the old.
func (u *unit) applyWritebacks(wbs []*writeback) {
	for _, w := range wbs {
		if !u.at() {
			return
		}
		u.storeStrong(w.dst, w.typ, u.fn.cur.Ptr.Load(w.tmp), false)
	}
}

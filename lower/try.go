package lower

// §7.2's @try, @catch and @finally, and the type-info a @catch names.
//
// # The shape
//
// A @try is a region of the function whose calls carry a second edge. Every
// call inside one becomes an invoke to the region's landing pad (§G3), and
// the pad is where the decision is made: the personality routine has already
// matched the thrown object against the clauses, and hands the pad a
// selector saying which one. So the pad is a switch, and each arm is a
// @catch body with objc_begin_catch and objc_end_catch around it.
//
// # @finally
//
// The hard part, because a @finally runs on every way out — falling off the
// end of the body, returning through it, breaking out of a loop that crosses
// it, and unwinding past it — and it has to be one copy of the body rather
// than one per exit.
//
// So it is a block reached from all of them, with a slot saying where to go
// afterwards. Every exit stores a code and branches in; the block ends in a
// br_table over the codes. A return stores its value in a slot of its own
// first, because the return itself happens on the other side.
//
// The unwinding exit is clang's rather than C++'s: the pad's last clause is a
// catch-all, so the personality stops at this frame, objc_begin_catch takes
// the object, the @finally runs, and objc_exception_rethrow puts it back on
// its way. A cleanup clause and a resume would be the C++ spelling and would
// need a second copy of the @finally body — resume takes the exception object
// of a pad that dominates it (§19.5), and a block every exit reaches is
// dominated by none of them.
//
// # What is not written
//
// A goto out of a @try that has a @finally. The destination table can hold
// any number of exits and a break or a continue reaches one through it, but
// a goto's target is a label that may not have been lowered yet, and its
// depth in the enclosing @try stack is not known where the goto stands.
// Refused by name rather than lowered without the @finally.

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// A tryScope is one open @try, for the statements lowering inside it.
type tryScope struct {
	// fin is the @finally's single copy, or nil when there is none.
	fin *ir.Block

	// dest says which exit to take once fin has run, and exits is what
	// each code means. Code zero is always the statement after the whole
	// @try.
	dest  ir.Ptr
	exits []*ir.Block

	// rethrow is set on the path that entered fin by unwinding, and is
	// what makes the last thing fin does a rethrow instead of a branch.
	rethrow ir.Ptr

	// after is the statement following the whole @try, made on the first
	// branch to it. A @try whose every arm throws or returns has no path
	// out at all, and both a block nothing reaches and a block with no
	// terminator are §19.2's fault rather than dead code a backend drops.
	after *ir.Block
}

// tryAfter is the block the statement after the @try goes in.
func (u *unit) tryAfter(sc *tryScope) *ir.Block {
	if sc.after == nil {
		sc.after = u.block("try.done")
		if len(sc.exits) > 0 {
			sc.exits[0] = sc.after
		}
	}
	return sc.after
}

// tryStmt lowers @try.
func (u *unit) tryStmt(s *ast.TryStmt) {
	if !u.at() {
		return
	}
	u.needPersonality()

	sc := &tryScope{}
	if s.Finally != nil {
		sc.fin = u.block("finally")
		sc.dest = u.fn.entry.Ptr.Alloc(4, 4)
		sc.rethrow = u.fn.entry.Ptr.Alloc(4, 4)
		sc.exits = []*ir.Block{nil} // code zero is after, made on demand
		u.fn.cur.I32.Store(u.fn.cur.I32.Const(0), sc.dest)
		u.fn.cur.I32.Store(u.fn.cur.I32.Const(0), sc.rethrow)
	}

	// The clauses, in the order they were written. A @finally adds a
	// catch-all after them, which is what makes the personality stop here
	// for an exception no @catch claimed.
	var clauses []ir.PadClause
	for _, cl := range s.Catches {
		clauses = append(clauses, ir.Catch(u.catchTypeInfo(cl)))
	}
	if s.Finally != nil {
		clauses = append(clauses, ir.Catch(nil))
	}
	if len(clauses) == 0 {
		// The analyzer already reported it; there is nothing to protect.
		u.stmt(s.Body)
		return
	}

	// The body. Its pad is built only if something in it can throw — a
	// @try around statements that make no call is a @try nothing can
	// unwind out of, and the clauses hanging off it are unreachable.
	u.fn.tries = append(u.fn.tries, sc)
	h := u.openRegion(func() *ir.Block { return u.fn.fn.Pad(u.padLabel("try"), clauses...) })
	u.stmt(s.Body)
	u.closeRegion()
	if u.at() {
		u.leaveTry(sc, u.tryAfter(sc))
	}

	if h.pad != nil {
		u.dispatchClauses(sc, s, h.pad)
	}

	u.fn.tries = u.fn.tries[:len(u.fn.tries)-1]
	if s.Finally != nil {
		u.finallyBody(sc, s.Finally)
	}
	u.fn.cur = sc.after
}

// dispatchClauses fills the pad: one arm per clause, chosen by the selector.
//
// A clause's number is its position among the pad's catch clauses (§G3), so
// the comparisons are against 1, 2, 3. No type identity is decided here,
// because the personality already decided it — which is the whole point of
// the table the pad's clauses became.
func (u *unit) dispatchClauses(sc *tryScope, s *ast.TryStmt, pad *ir.Block) {
	n := len(s.Catches)
	if s.Finally != nil {
		n++
	}
	arms := make([]*ir.Block, n)
	for i := range arms {
		arms[i] = u.block("catch")
	}
	u.fn.cur = pad
	for i := 0; i < n-1; i++ {
		next := u.block("catch.test")
		hit := u.fn.cur.I32.Eq(pad.Sel(), u.fn.cur.I32.Const(int64(i+1)))
		u.fn.cur.BrIf(hit, arms[i].To(), next.To())
		u.fn.cur = next
	}
	// The last arm needs no test: the personality entered this pad, so one
	// clause matched, and every other has been ruled out.
	u.fn.cur.Br(arms[n-1].To())

	for i, cl := range s.Catches {
		u.fn.cur = arms[i]
		u.catchBody(sc, cl, pad.Exn())
	}
	if s.Finally != nil {
		// The catch-all arm is the @finally's: take the object so the
		// runtime knows it is being handled, say so, and run the block.
		u.fn.cur = arms[n-1]
		u.fn.cur.Call(u.beginCatch(), pad.Exn())
		u.fn.cur.I32.Store(u.fn.cur.I32.Const(1), sc.rethrow)
		u.leaveTry(sc, u.tryAfter(sc))
	}
}

// catchBody lowers one @catch clause, with the object bound to its parameter.
func (u *unit) catchBody(sc *tryScope, cl *ast.CatchClause, exn ir.Ptr) {
	u.push()
	obj := u.fn.cur.Call(u.beginCatch(), exn).Ptr(0)
	if cl.Param != nil {
		if name := u.name(declName(cl.Param.Decl)); name != "" {
			if t := u.typeOf(cl.Param); t != nil {
				// Not a strong local: the object belongs to the runtime
				// between begin_catch and end_catch, and this frame
				// neither took a reference nor owes one back.
				slot := u.slot(t, name)
				u.fn.cur.Ptr.Store(obj, slot)
				u.bind(name, &storage{kind: stLocal, typ: t, addr: slot})
			}
		}
	}

	// A throw out of a @catch body still has to end the clause it is
	// leaving, which is what this pad is for.
	u.openRegion(u.escapePad("catch.esc", func() { u.fn.cur.Call(u.endCatch()) }))
	u.stmt(cl.Body)
	u.closeRegion()
	u.pop()

	if u.at() {
		u.fn.cur.Call(u.endCatch())
		u.leaveTry(sc, u.tryAfter(sc))
	}
}

// finallyBody lowers the @finally's one copy, and the dispatch that follows
// it.
func (u *unit) finallyBody(sc *tryScope, f *ast.FinallyClause) {
	// A throw out of the @finally itself leaves through this pad, giving
	// back the object the catch-all arm took if there was one.
	esc := u.escapePad("finally.esc", func() {
		end := u.block("finally.esc.end")
		done := u.block("finally.esc.done")
		held := u.fn.cur.I32.Ne(u.fn.cur.I32.Load(sc.rethrow), u.fn.cur.I32.Const(0))
		u.fn.cur.BrIf(held, end.To(), done.To())
		end.Call(u.endCatch())
		end.Br(done.To())
		u.fn.cur = done
	})

	u.fn.cur = sc.fin
	u.openRegion(esc)
	u.stmt(f.Body)
	u.closeRegion()

	if u.at() {
		rethrow := u.block("finally.rethrow")
		dispatch := u.block("finally.to")
		flag := u.fn.cur.I32.Ne(u.fn.cur.I32.Load(sc.rethrow), u.fn.cur.I32.Const(0))
		u.fn.cur.BrIf(flag, rethrow.To(), dispatch.To())

		// objc_exception_rethrow puts back the object objc_begin_catch
		// took, and does not return. An invoke where a @try encloses this
		// one: the exception is leaving this region, not this function.
		u.fn.cur = rethrow
		u.callMaybeUnwind(u.extern(runtime.ExceptionRethrow, ir.NewSig()))
		u.endThrow()

		u.fn.cur = dispatch
		u.dispatchExits(sc, dispatch)
	}
}

// escapePad is the pad a region unwinds through: it runs whatever the region
// owes and puts the exception back on its way.
//
// A catch-all clause and a rethrow rather than a cleanup clause and a
// resume, and the reason is §G3's resume: it hands control back to the
// unwinder, which steps past the frame that called it. An enclosing @try in
// the same function would never see the exception. objc_exception_rethrow is
// an ordinary call, and an ordinary call inside a @try is an invoke — which
// is how clang compiles the same statement, and why the Objective-C runtime
// has a rethrow at all.
//
// The pad's body is emitted here, before the region it protects: nothing in
// it depends on the region, and the enclosing handler it rethrows into is
// the one open right now.
func (u *unit) escapePad(label string, cleanup func()) func() *ir.Block {
	return func() *ir.Block {
		u.needPersonality()
		pad := u.fn.fn.Pad(u.padLabel(label), ir.Catch(nil))
		saved := u.fn.cur
		u.fn.cur = pad
		cleanup()
		u.fn.cur.Call(u.beginCatch(), pad.Exn())
		u.callMaybeUnwind(u.extern(runtime.ExceptionRethrow, ir.NewSig()))
		u.endThrow()
		u.fn.cur = saved
		return pad
	}
}

// dispatchExits ends the @finally with the branch its destination slot asks
// for.
func (u *unit) dispatchExits(sc *tryScope, at *ir.Block) {
	// Code zero is always the statement after the @try, and it is the
	// table's default, so reaching here is reaching it.
	u.tryAfter(sc)
	if len(sc.exits) == 1 {
		at.Br(sc.exits[0].To())
		u.fn.cur = nil
		return
	}
	targets := make([]ir.BlockTarget, len(sc.exits)-1)
	for i, b := range sc.exits[1:] {
		targets[i] = b.To()
	}
	// Code zero is the fall-through, which is the table's default: a
	// br_table indexes from zero, so the exits after it are the entries and
	// the one before them is where an index of zero has to land.
	at.BrTable(at.I32.Sub(at.I32.Load(sc.dest), at.I32.Const(1)), targets, sc.exits[0].To())
	u.fn.cur = nil
}

// leaveTry ends the open path by going to target, through the @finally if
// there is one.
func (u *unit) leaveTry(sc *tryScope, target *ir.Block) {
	if sc.fin == nil {
		u.fn.cur.Br(target.To())
		u.fn.cur = nil
		return
	}
	u.fn.cur.I32.Store(u.fn.cur.I32.Const(int64(sc.exitCode(target))), sc.dest)
	u.fn.cur.Br(sc.fin.To())
	u.fn.cur = nil
}

// exitCode is the destination code for target, assigned on first use.
func (sc *tryScope) exitCode(target *ir.Block) int {
	for i, b := range sc.exits {
		if b == target {
			return i
		}
	}
	sc.exits = append(sc.exits, target)
	return len(sc.exits) - 1
}

// leaveTries ends the open path by going to target, running every @finally
// between here and the scope that owns target.
//
// depth is how many @try scopes were open where target was created. The ones
// above it are the ones being left, innermost first, and each hands control
// to the next by way of its own destination slot.
func (u *unit) leaveTries(target *ir.Block, depth int) bool {
	open := u.fn.tries[depth:]
	if len(open) == 0 {
		return false
	}
	// Nothing to run is not nothing to do: a @try with no @finally still
	// ends the path the ordinary way, so only the ones with a block in
	// them take part.
	var chain []*tryScope
	for _, sc := range open {
		if sc.fin != nil {
			chain = append(chain, sc)
		}
	}
	if len(chain) == 0 {
		return false
	}
	for i := len(chain) - 1; i >= 0; i-- {
		to := target
		if i+1 < len(chain) {
			to = chain[i+1].fin
		}
		u.fn.cur.I32.Store(u.fn.cur.I32.Const(int64(chain[i].exitCode(to))), chain[i].dest)
	}
	u.fn.cur.Br(chain[0].fin.To())
	u.fn.cur = nil
	return true
}

// insideFinally reports whether any open @try has a @finally, which is what
// a goto out of one would have to run and cannot.
func (u *unit) insideFinally(depth int) bool {
	for _, sc := range u.fn.tries[depth:] {
		if sc.fin != nil {
			return true
		}
	}
	return false
}

// ---- the type-info a @catch names --------------------------------------

// catchTypeInfo is the global the personality compares a thrown object
// against, or nil for a clause that catches everything.
func (u *unit) catchTypeInfo(cl *ast.CatchClause) ir.Symbol {
	if cl.Param == nil {
		return nil // @catch (...)
	}
	t := u.typeOf(cl.Param)
	class := catchClassName(t)
	if class == "" {
		// @catch (id x), which matches any Objective-C object. The runtime
		// publishes one type-info for it.
		return u.ehTypeImport(runtime.EHTypeID)
	}
	if u.implementsClass(class) {
		return u.ehTypeDef(class)
	}
	return u.ehTypeImport(runtime.EHTypeSymbol(class))
}

// catchClassName is the class a @catch parameter names, or "" for id, for a
// block, and for anything else that is not one class in particular.
func catchClassName(t types.Type) string {
	obj := types.AsObject(t)
	if obj == nil || obj.Base == nil {
		return ""
	}
	return obj.Base.Name
}

// implementsClass reports whether this unit has the @implementation.
func (u *unit) implementsClass(name string) bool {
	for _, k := range u.impls {
		if k.Name == name {
			return true
		}
	}
	return false
}

func (u *unit) ehTypeImport(name string) ir.Symbol {
	return u.classSymbol(name)
}

// ehTypeDef emits the type-info object for a class this unit implements.
//
// Three words, and the first is the odd one: Itanium's vtable pointer names
// the first virtual function rather than the header two slots above it, so
// the initializer is the vtable plus sixteen. It is global rather than
// internal because the @catch may be in another image than the
// @implementation.
func (u *unit) ehTypeDef(class string) ir.Symbol {
	name := runtime.EHTypeSymbol(class)
	if s, ok := u.ehTypes[name]; ok {
		return s
	}
	vtable := u.classSymbol(runtime.EHTypeVTable)
	g := u.mod.Global(u.sym(name), ir.RO,
		u.metaType("objc_typeinfo", runtime.EHType).FType()).
		Export().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(
			ir.RelocInit(vtable).Plus(ir.Int(runtime.EHTypeVTableOffset)),
			ir.RelocInit(u.classNameString(class)),
			ir.RelocInit(u.classSymbol(runtime.ClassSymbol(class))),
		))
	u.ehTypes[name] = g
	return g
}

// ---- small helpers ------------------------------------------------------

func (u *unit) beginCatch() ir.Callee {
	return u.extern(runtime.BeginCatch, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr))
}

func (u *unit) endCatch() ir.Callee {
	return u.extern(runtime.EndCatch, ir.NewSig())
}

// needPersonality names the routine that reads this function's tables. Every
// pad in a function shares one, which is what §G3 declares per function.
func (u *unit) needPersonality() {
	if u.fn.fn.PersonalityFn() != nil {
		return
	}
	u.fn.fn.Personality(u.extern(runtime.Personality, ir.NewSig()))
}

// padLabel is a unique label for a pad block.
func (u *unit) padLabel(prefix string) string {
	u.fn.nblocks++
	// The same spelling u.block uses: a VIR label is an identifier, and a
	// dot is not part of one.
	out := make([]byte, 0, len(prefix)+4)
	for i := 0; i < len(prefix); i++ {
		if prefix[i] == '.' {
			out = append(out, '_')
			continue
		}
		out = append(out, prefix[i])
	}
	return string(out) + "_" + itoa(u.fn.nblocks)
}

// jumpTo records a break or continue target with the @try depth it was made
// at, so that a jump to it later knows how many @finally blocks it crosses.
func (u *unit) jumpTo(b *ir.Block) jumpTarget {
	return jumpTarget{blk: b, tries: len(u.fn.tries)}
}

// jumpOut ends the open path at a break or continue target.
func (u *unit) jumpOut(t jumpTarget) {
	if u.leaveTries(t.blk, t.tries) {
		return
	}
	u.fn.cur.Br(t.blk.To())
	u.fn.cur = nil
}

// ---- calls that may not come back ---------------------------------------

// results is what a call produced, whichever form it took.
//
// An invoke has no result registers of its own — §14 binds them to the
// trailing parameters of its normal edge, because a register the terminator
// defined would have to dominate the unwind edge too, and on that edge no
// call completed. So the two forms hand back different things, and this is
// the shape they have in common.
type results struct{ vals []ir.Value }

func (r results) Len() int { return len(r.vals) }

func (r results) Value(i int) ir.Value {
	if i < 0 || i >= len(r.vals) {
		return nil
	}
	return r.vals[i]
}

func (r results) Ptr(i int) ir.Ptr {
	p, _ := r.Value(i).(ir.Ptr)
	return p
}

// A handler is the pad an open region unwinds to, built on first use.
//
// On first use and not with the region, because a region with no call in it
// unwinds nowhere: its pad would be a block no path from the entry reaches,
// which §19.2 rejects — and rightly, since the clauses hanging off it are
// unreachable too.
type handler struct {
	pad   *ir.Block
	build func() *ir.Block
}

// handler is the pad a call unwinds to here, or nil outside every @try.
func (u *unit) handler() *ir.Block {
	n := len(u.fn.handlers)
	if n == 0 {
		return nil
	}
	h := u.fn.handlers[n-1]
	if h.pad == nil {
		// A pad's own body may unwind, into whatever encloses the region
		// rather than into itself, so it is built with this entry off the
		// stack.
		saved := u.fn.handlers
		u.fn.handlers = saved[:n-1]
		h.pad = h.build()
		u.fn.handlers = saved
	}
	return h.pad
}

// openRegion makes the pad a region will unwind to the handler for the
// statements lowered until closeRegion.
func (u *unit) openRegion(build func() *ir.Block) *handler {
	h := &handler{build: build}
	u.fn.handlers = append(u.fn.handlers, h)
	return h
}

func (u *unit) closeRegion() {
	u.fn.handlers = u.fn.handlers[:len(u.fn.handlers)-1]
}

// callMaybeUnwind emits fn(args), as an invoke when a handler is open.
func (u *unit) callMaybeUnwind(callee ir.Callee, args ...ir.Value) results {
	pad := u.handler()
	if pad == nil || callee == nil {
		return wrapResults(u.fn.cur.Call(callee, args...))
	}
	cont := u.unwindCont(callee.Signature())
	u.fn.cur.Invoke(callee, args, cont.To(), pad)
	u.fn.cur = cont
	return blockResults(cont)
}

// callIndMaybeUnwind is callMaybeUnwind through a pointer.
func (u *unit) callIndMaybeUnwind(p ir.Ptr, t *ir.Type, args ...ir.Value) results {
	pad := u.handler()
	if pad == nil || t == nil {
		return wrapResults(u.fn.cur.CallInd(p, t, args...))
	}
	cont := u.unwindCont(t.Sig())
	u.fn.cur.InvokeInd(p, t, args, cont.To(), pad)
	u.fn.cur = cont
	return blockResults(cont)
}

// unwindCont is an invoke's normal target: a block whose parameters are the
// call's results and nothing else.
func (u *unit) unwindCont(sig *ir.Sig) *ir.Block {
	b := u.block("call.cont")
	if sig != nil {
		for i, r := range sig.Rets() {
			b.Param(r.Type, "r"+itoa(i))
		}
	}
	return b
}

func blockResults(b *ir.Block) results {
	ps := b.Params()
	out := results{vals: make([]ir.Value, len(ps))}
	for i, p := range ps {
		out.vals[i] = ir.Wrap(p)
	}
	return out
}

func wrapResults(r ir.Results) results {
	out := results{vals: make([]ir.Value, r.Len())}
	for i := range out.vals {
		out.vals[i] = r.Value(i)
	}
	return out
}

// finishReturn ends a return: the releases this frame owes, and the return
// itself — or the detour through every open @finally, which has to run
// before either.
//
// The detour gets a block of its own rather than sharing one, because what
// the releases are depends on which scopes this particular return is
// leaving, and that is known here and nowhere later.
func (u *unit) finishReturn(v ir.Value) {
	if !u.insideFinally(0) {
		u.releaseAllStrong()
		u.releaseByrefs()
		u.releasePools()
		u.returnValue(v)
		return
	}

	// The value is computed here and returned on the far side, so it waits
	// in a slot: the @finally runs in between and may do anything.
	var slot ir.Ptr
	if v != nil {
		slot = u.returnSlot(v)
		u.storeReg(v, slot)
	}
	exit := u.block("return.after_finally")
	u.leaveTries(exit, 0)

	u.fn.cur = exit
	u.releaseAllStrong()
	u.releaseByrefs()
	u.releasePools()
	if v == nil {
		u.returnValue(nil)
		return
	}
	u.returnValue(u.loadReg(v, slot))
}

func (u *unit) returnValue(v ir.Value) {
	if v == nil {
		u.fn.cur.Return()
	} else {
		u.fn.cur.Return(v)
	}
	u.fn.cur = nil
}

// storeReg parks a register value in a slot; loadReg takes it back with the
// type the caller says it had. There is no source type left to consult here —
// the value has already been converted to what the signature returns — so
// the switch is over the register type itself.
//
// An i1 travels as an i32. There is no one-bit store, and widening it is
// exactly what a one-bit value in memory is anywhere else.
func (u *unit) storeReg(v ir.Value, p ir.Ptr) {
	b := u.fn.cur
	switch val := v.(type) {
	case ir.I1:
		b.I32.Store(b.I32.ZExtI1(val), p)
	case ir.I32:
		b.I32.Store(val, p)
	case ir.I64:
		b.I64.Store(val, p)
	case ir.F32:
		b.F32.Store(val, p)
	case ir.F64:
		b.F64.Store(val, p)
	case ir.Ptr:
		b.Ptr.Store(val, p)
	}
}

func (u *unit) loadReg(like ir.Value, p ir.Ptr) ir.Value {
	b := u.fn.cur
	switch like.(type) {
	case ir.I1:
		return b.I32.Ne(b.I32.Load(p), b.I32.Const(0))
	case ir.I32:
		return b.I32.Load(p)
	case ir.I64:
		return b.I64.Load(p)
	case ir.F32:
		return b.F32.Load(p)
	case ir.F64:
		return b.F64.Load(p)
	}
	return b.Ptr.Load(p)
}

// returnSlot is where a return crossing a @finally parks its value. One per
// function: two returns of one type want one slot, and a slot is eight bytes.
func (u *unit) returnSlot(ir.Value) ir.Ptr {
	if u.fn.retSlot != (ir.Ptr{}) {
		return u.fn.retSlot
	}
	u.fn.retSlot = u.fn.entry.Ptr.Alloc(16, 16)
	return u.fn.retSlot
}

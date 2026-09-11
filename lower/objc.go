package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// The Objective-C lowering: what a message send, an instance variable and
// the two block statements become.

// message lowers §6.3's send.
//
// A send is a call to objc_msgSend with the receiver and the selector in
// front of the arguments. Which msgSend is the target's question, and
// runtime.Send answers it: the trampolines differ in how they forward a
// return value, and calling the wrong one corrupts it rather than failing to
// link.
func (u *unit) message(e *ast.MessageExpr, t types.Type) ir.Value {
	m := u.info.Sends[e]

	recv, super := u.receiver(e)
	if recv == nil {
		return nil
	}
	sel := u.selectorOf(e)
	if sel == "" {
		return nil
	}

	var params []types.Param
	if m != nil {
		params = m.Params
	}
	var args []ir.Value
	var wbs []*writeback
	i := 0
	for _, a := range e.Args {
		for _, v := range a.Vals {
			if i < len(params) {
				// §ARC 4.3.2's out-parameter — [x doIt:&error]. See arc.go.
				if out, wb := u.arcOutArg(v, params[i].Type); wb != nil {
					args = append(args, out)
					wbs = append(wbs, wb)
					i++
					continue
				}
			}
			val := u.rvalue(v)
			if val == nil {
				u.internal(v, "an argument of this message")
				return nil
			}
			at := u.typeOf(v)
			if i < len(params) {
				val = u.convert(val, at, params[i].Type)
			} else {
				// Past the declared parameters is the var-tail, where
				// §6.5.2.2's promotions are all the callee can expect --
				// and where an aggregate has no declaration to hang byval
				// on, so there is nothing to say about how it travels.
				if types.IsRecord(at) {
					u.unsupported(v, "a struct or union in a variadic argument")
					return nil
				}
				val = u.defaultPromote(val, at)
			}
			args = append(args, val)
			i++
		}
	}

	ret := t
	if m != nil {
		ret = m.Ret
	}
	variadic := m != nil && m.Variadic
	v := u.sendWith(*recv, super, sel, args, params, variadic, ret, e)
	u.applyWritebacks(wbs)

	// §ARC's naming convention, applied. A method in one of the retaining
	// families hands back an object the caller owns, and init hands back
	// the one it was given — which is why `[[Box alloc] init]` is one +1
	// and not two. runtime.FamilyOf is the rule; arc.go is the register.
	if u.arcOn() {
		fam := runtime.FamilyOf(sel)
		if fam.ConsumesSelf() {
			u.takeOwned(*recv)
		}
		if fam.ReturnsRetained() && objectValued(ret) {
			u.owns(v)
		}
	}
	return v
}

// send emits one message: the call every Objective-C construct that means a
// message goes through.
//
// A send is a call to objc_msgSend with the receiver and the selector in
// front of the arguments. Which msgSend is the target's question, and
// runtime.Send answers it: the trampolines differ in how they forward a
// return value, and calling the wrong one corrupts it rather than failing to
// link.
func (u *unit) send(recv ir.Value, super bool, sel string, args []ir.Value,
	ret types.Type, at ast.Node) ir.Value {
	return u.sendWith(recv, super, sel, args, nil, false, ret, at)
}

// sendWith is send with the method's declared parameters, which an aggregate
// argument needs: a struct is a pointer at this level whatever the
// convention does with it, and only the declaration says which pointer it is.
func (u *unit) sendWith(recv ir.Value, super bool, sel string, args []ir.Value,
	params []types.Param, variadic bool, ret types.Type, at ast.Node) ir.Value {

	selp := u.selectorRef(sel)
	if selp == nil {
		return nil
	}

	// A result the caller supplies storage for goes in front of the
	// receiver, which is where both conventions want it: on x86-64
	// objc_msgSend_stret takes the pointer in RDI and self in RSI, and on
	// AArch64 the pointer travels in X8 and self stays in X0. Writing it as
	// §19.13's sret on the first parameter says the one thing, and each
	// backend puts it where its own convention has it.
	var out ir.Ptr
	all := []ir.Value{recv, selp}
	if isIndirectResult(ret) {
		out = u.aggResult(ret)
		all = append([]ir.Value{out}, all...)
	}
	for j, a := range args {
		if j < len(params) && isAggregate(params[j].Type) {
			copied, ok := u.aggArg(a, params[j].Type, at)
			if !ok {
				return nil
			}
			a = copied
		}
		all = append(all, a)
	}

	name := u.abi.Send(u.arch, ret, u.model, super)

	// The signature is the arguments' own. objc_msgSend forwards whatever
	// it was handed, so the call site describes the *method* rather than
	// the trampoline — which is why the same entry point is imported with
	// one signature and called with many.
	// How many of the arguments the *method* declared. Everything past that
	// is the var-tail, and saying so is not decoration: Apple's AArch64
	// passes a variadic argument on the stack where a fixed one goes in a
	// register, so a send to stringWithFormat: described as fixed puts every
	// format argument where the callee does not look.
	fixed := len(all)
	if variadic {
		fixed = len(all) - (len(args) - len(params))
	}

	sig := ir.NewSig()
	for i, a := range all {
		if i == fixed {
			sig.Variadic()
			break
		}
		r, ok := u.regOfValue(a)
		if !ok {
			u.errorf(at, "internal: %T is not a register value in a send to %s", a, sel)
			return nil
		}
		switch {
		case i == 0 && out != (ir.Ptr{}):
			t, _ := u.aggType(ret)
			sig.Param(r, ir.SRet(t))
		default:
			j := i - 2
			if out != (ir.Ptr{}) {
				j--
			}
			if j >= 0 && j < len(params) && isAggregate(params[j].Type) {
				t, _ := u.aggType(params[j].Type)
				sig.Param(r, ir.ByVal(t))
				continue
			}
			sig.Param(r)
		}
	}
	hasRet := false
	if ret != nil && !types.IsVoid(ret) && out == (ir.Ptr{}) {
		if r, ok := u.reg(ret); ok {
			sig.Ret(r)
			hasRet = true
		}
	}

	// The send goes through a *pointer* to the trampoline, typed with this
	// call site's own signature.
	//
	// One objc_msgSend has as many signatures as there are methods: it
	// forwards whatever it was handed, and the caller and the method have
	// to agree about the registers. A direct call would fix one signature
	// on the imported symbol and mis-call every other method through it,
	// so the address is taken once and named per signature — which is what
	// clang does too, by casting the function pointer at each call.
	fnTy := u.sendType(sig)
	imp := u.extern(name, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Variadic().Ret(ir.TypePtr))
	fp := u.fn.cur.Ptr.GetAddr(imp)
	res := u.callIndMaybeUnwind(fp, fnTy, all...)
	if out != (ir.Ptr{}) {
		return out
	}
	if !hasRet || res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// sendToClass is a send to a class object named by name — what a literal
// that means `[NSNumber numberWithInt:]` needs and no syntax wrote.
func (u *unit) sendToClass(class, sel string, args []ir.Value, ret types.Type, at ast.Node) ir.Value {
	return u.send(u.classRef(class), false, sel, args, ret, at)
}

// sendType names a send's signature, once per distinct shape.
func (u *unit) sendType(sig *ir.Sig) *ir.Type { return u.namedFuncType("msgsig", sig) }

// namedFuncType interns a function type by its shape.
//
// An indirect call needs a *named* type in VIR, and two call sites with the
// same registers are the same type: naming by shape is what keeps a module
// from growing one type declaration per call.
func (u *unit) namedFuncType(prefix string, sig *ir.Sig) *ir.Type {
	key := prefix
	if sig.IsVariadic() {
		key += "_var"
	}
	for _, p := range sig.Params() {
		key += "_" + p.Type.String()
		// The attributes are part of the shape, and so is the type each
		// names. Two signatures of three pointers are two *different*
		// signatures when one says the first is the caller's storage for a
		// struct result — and two sret signatures are different when the
		// structs are, because the backend classifies the type. Naming them
		// alike would call one through the other's type.
		for _, a := range p.Attrs {
			key += "_" + a.String()
			if at := a.Type(); at != nil {
				key += "_" + at.Name()
			}
		}
	}
	for _, r := range sig.Rets() {
		key += "_r" + r.Type.String()
	}
	if t := u.mod.LookupType(key); t != nil {
		return t
	}
	return u.mod.FuncType(key, sig)
}

// receiver lowers what the send goes to, and says whether it was `super`.
//
// A super send does not take the superclass: objc_msgSendSuper2 takes a
// two-word structure of the receiver and the class the method was compiled
// in, and finds the superclass itself — which is what lets a category
// attached to the superclass afterwards still be found.
func (u *unit) receiver(e *ast.MessageExpr) (*ir.Value, bool) {
	return u.recvOf(e.Recv)
}

// recvOf lowers one receiver expression. Dot syntax needs the same three
// answers a send does — super, a class name, an ordinary value — so the
// judgement lives here and not in the shape of the syntax that asked.
func (u *unit) recvOf(x ast.Expr) (*ir.Value, bool) {
	switch r := x.(type) {
	case *ast.SuperExpr:
		if u.fn.class == nil {
			return nil, false
		}
		b := u.fn.cur
		// The slot is in the entry block, which §19.6 requires and which is
		// why the entry block is kept open; the stores are here, where the
		// send is.
		st := u.fn.entry.Ptr.Alloc(uint64(2*u.abi.PtrBytes), uint64(u.abi.PtrBytes))
		u.fn.entry.Name(st, "super")
		b.Ptr.Store(u.fn.self, st)
		cls := u.superRef(u.fn.class.Name, u.fn.classMethod)
		b.Ptr.Store(cls, b.Ptr.Add(st, b.I64.Const(u.abi.PtrBytes)))
		var v ir.Value = st
		return &v, true

	case *ast.ClassExpr:
		v := u.classRef(u.name(r.Name))
		var val ir.Value = v
		return &val, false
	}

	// A bare class name is the class object, which is a class reference
	// rather than a load of a variable.
	if id, ok := stripParens(x).(*ast.Ident); ok {
		if o := types.AsObject(u.typeOf(x)); o != nil && o.Meta && o.Base != nil {
			if u.lookup(u.name(id)) == nil {
				v := u.classRef(o.Base.Name)
				var val ir.Value = v
				return &val, false
			}
		}
	}
	v := u.rvalue(x)
	if v == nil {
		return nil, false
	}
	return &v, false
}

// selectorOf is the selector a send names, as one string.
func (u *unit) selectorOf(e *ast.MessageExpr) string {
	if e.Sel != nil {
		return u.name(e.Sel)
	}
	var pieces []string
	for _, a := range e.Args {
		pieces = append(pieces, u.name(a.Sel))
	}
	return types.Selector(pieces, true)
}

func (u *unit) selectorText(e *ast.SelectorExpr) string {
	var pieces []string
	keyword := false
	for _, p := range e.Parts {
		pieces = append(pieces, u.name(p.Name))
		if p.Colon.IsValid() {
			keyword = true
		}
	}
	return types.Selector(pieces, keyword)
}

// regOfValue is the register type a value already has.
//
// It answers about the concrete register types and nothing else. A value it
// does not recognize is a lowering bug — a *I64 handed over where an I64 was
// meant, say — so the caller reports rather than quietly emitting nothing.
func (u *unit) regOfValue(v ir.Value) (ir.RegType, bool) {
	switch v.(type) {
	case ir.I1:
		return ir.TypeI1, true
	case ir.I32:
		return ir.TypeI32, true
	case ir.I64:
		return ir.TypeI64, true
	case ir.F32:
		return ir.TypeF32, true
	case ir.F64:
		return ir.TypeF64, true
	case ir.Ptr:
		return ir.TypePtr, true
	}
	return 0, false
}

// ---- the references the runtime rewrites ----

// selectorRef loads the SEL for a selector.
//
// The reference is a global the runtime rewrites at load: what the compiler
// writes is a pointer to the name, and what is there afterwards is the
// unique selector. One per selector per translation unit, which is what
// makes a send two instructions rather than a lookup.
func (u *unit) selectorRef(sel string) ir.Value {
	if sel == "" {
		return nil
	}
	sym, ok := u.selRefs[sel]
	if !ok {
		name := u.methodName(sel)
		g := u.mod.Global(u.sym(u.uniq(runtime.SelectorRefPrefix)), ir.RW, u.ptrFType()).
			Internal().
			Section(u.abi.Name(runtime.SecSelectorRefs)).
			Align(uint64(u.abi.PtrBytes)).
			Init(ir.RelocInit(name))
		sym = g
		u.selRefs[sel] = sym
	}
	return u.fn.cur.Ptr.Load(u.fn.cur.Ptr.GetAddr(sym))
}

// classRef loads the class object for a class the unit mentions.
func (u *unit) classRef(class string) ir.Ptr {
	sym, ok := u.classRefs[class]
	if !ok {
		g := u.mod.Global(u.sym(u.uniq(runtime.ClassRefPrefix)), ir.RW, u.ptrFType()).
			Internal().
			Section(u.abi.Name(runtime.SecClassRefs)).
			Align(uint64(u.abi.PtrBytes)).
			Init(ir.RelocInit(u.classSymbol(runtime.ClassSymbol(class))))
		sym = g
		u.classRefs[class] = sym
	}
	return u.fn.cur.Ptr.Load(u.fn.cur.Ptr.GetAddr(sym))
}

// superRef loads the object a super send starts its search above.
//
// The class for an instance method and the *metaclass* for a class method,
// which is not a refinement: objc_msgSendSuper2 takes one step up from what
// it is given, and a class method's next implementation is on the
// superclass's metaclass. Handed the class instead, the search starts among
// the instance methods of the superclass and finds either the wrong method
// or nothing at all.
func (u *unit) superRef(class string, meta bool) ir.Ptr {
	symbol := runtime.ClassSymbol(class)
	key := class
	if meta {
		symbol = runtime.MetaclassSymbol(class)
		key = "+" + class
	}
	sym, ok := u.superRefs[key]
	if !ok {
		g := u.mod.Global(u.sym(u.uniq(runtime.SuperRefPrefix)), ir.RW, u.ptrFType()).
			Internal().
			Section(u.abi.Name(runtime.SecSuperRefs)).
			Align(uint64(u.abi.PtrBytes)).
			Init(ir.RelocInit(u.classSymbol(symbol)))
		sym = g
		u.superRefs[key] = sym
	}
	return u.fn.cur.Ptr.Load(u.fn.cur.Ptr.GetAddr(sym))
}

// classSymbol is a class or metaclass object by its symbol name, imported if
// this unit does not define it.
//
// The same map holds both, and holds a definition once it exists: a
// reference to a class this unit implements and the definition of it have to
// be one symbol, or the reference points at an import of a name the module
// also defines.
func (u *unit) classSymbol(symbol string) ir.Symbol {
	if s, ok := u.classSyms[symbol]; ok {
		return s
	}
	s := ir.Symbol(u.mod.ImportGlobal(u.sym(symbol), u.ptrFType()))
	u.classSyms[symbol] = s
	return s
}

// methodName interns a selector's characters, in the section the linker
// merges them across images.
func (u *unit) methodName(sel string) ir.Symbol {
	return u.cstringIn(sel, runtime.SecMethodNames, runtime.MethodNameLabel)
}

// classNameString interns a class or protocol name.
func (u *unit) classNameString(name string) ir.Symbol {
	return u.cstringIn(name, runtime.SecClassNames, runtime.ClassNameLabel)
}

// methodTypeString interns a method's type encoding.
func (u *unit) methodTypeString(s string) ir.Symbol {
	return u.cstringIn(s, runtime.SecMethodTypes, runtime.MethodTypeLabel)
}

// cstringIn interns one NUL-terminated string in a section, once per unit.
func (u *unit) cstringIn(s string, sec runtime.Section, label string) ir.Symbol {
	key := label + "\x00" + s
	if sym, ok := u.strs[key]; ok {
		return sym
	}
	g := u.mod.Global(u.sym(u.uniq(label)), ir.RO,
		ir.Array(uint64(len(s)+1), ir.StoreI8.FType())).
		Internal().
		Section(u.abi.Name(sec)).
		Init(ir.Str(s))
	u.strs[key] = g
	return g
}

// cstring is an ordinary C string constant, for @encode and for a string
// literal.
func (u *unit) cstring(s string) ir.Value {
	return u.fn.cur.Ptr.GetAddr(u.cstringSym(s))
}

// cstringSym is the object a plain string literal is, interned by content.
// It is separate from cstring because a file-scope initializer names the
// symbol and has no block to take its address in.
func (u *unit) cstringSym(s string) ir.Symbol {
	sym, ok := u.cstrs[s]
	if !ok {
		g := u.mod.Global(u.sym(u.uniq("str")), ir.RO,
			ir.Array(uint64(len(s)+1), ir.StoreI8.FType())).
			Internal().
			Init(ir.Str(s))
		sym = g
		u.cstrs[s] = sym
	}
	return sym
}

// stringSymbol is the object a string literal names, whichever kind it is.
func (u *unit) stringSymbol(s *ast.StringLit) ir.Symbol {
	if s.Object {
		return nil // a constant NSString needs a block; see constantString
	}
	val := analyzer.DecodeString(u.src, s, u.model, func(string) {})
	if size, _ := u.sizeAlign(val.Elem); size != 1 {
		return nil
	}
	b := make([]byte, 0, len(val.Data))
	for _, c := range val.Data[:len(val.Data)-1] {
		b = append(b, byte(c))
	}
	return u.cstringSym(string(b))
}

// stringLit lowers a string literal.
//
// A plain literal is an array of characters in read-only memory; an @"…"
// literal is a string object, which is a different thing in a different
// section and is emitted by constantString.
func (u *unit) stringLit(e *ast.StringLit) ir.Value {
	if e.Object {
		return u.constantString(e)
	}
	val := analyzer.DecodeString(u.src, e, u.model, func(string) {})
	if size, _ := u.sizeAlign(val.Elem); size == 1 {
		b := make([]byte, 0, len(val.Data))
		for _, c := range val.Data[:len(val.Data)-1] {
			b = append(b, byte(c))
		}
		return u.cstring(string(b))
	}
	return u.wideString(val)
}

// wideString is a L"…", u"…" or U"…" literal: an array of code units whose
// width is the element type's, and which therefore cannot be written as a
// byte string.
func (u *unit) wideString(val analyzer.StringValue) ir.Value {
	_, align := u.sizeAlign(val.Elem)
	st, ok := u.store(val.Elem)
	if !ok {
		return nil
	}
	key := "wide\x00" + val.Elem.String() + "\x00"
	items := make([]ir.Init, 0, len(val.Data))
	for _, c := range val.Data {
		items = append(items, ir.Lit(ir.Int(int64(c))))
		key += string(rune(c))
	}
	sym, ok2 := u.strs[key]
	if !ok2 {
		g := u.mod.Global(u.sym(u.uniq("wstr")), ir.RO,
			ir.Array(uint64(len(items)), st.FType())).
			Internal().
			Align(uint64(align)).
			Init(ir.List(items...))
		sym = g
		u.strs[key] = sym
	}
	return u.fn.cur.Ptr.GetAddr(sym)
}

// ---- instance variables ----

// ivarAddr is the address of an instance variable of self.
func (u *unit) ivarAddr(class, ivar string) *ir.Ptr {
	if u.fn == nil || u.fn.class == nil {
		return nil
	}
	p := u.ivarAddrOf(u.fn.self, class, ivar)
	return &p
}

// ivarAddrOf is the address of one instance variable of an object.
//
// The offset is loaded rather than added as a constant, and that load is the
// whole of the non-fragile ABI: the runtime writes the variable when it
// realizes the class, so a superclass that grew a member moves this one
// without the code being rebuilt.
func (u *unit) ivarAddrOf(obj ir.Ptr, class, ivar string) ir.Ptr {
	b := u.fn.cur
	sym := u.ivarOffsetSymbol(class, ivar)
	off := b.I64.SLoad32(b.Ptr.GetAddr(sym))
	return b.Ptr.Add(obj, off)
}

// ivarOffsetSymbol is one instance variable's offset variable.
//
// A method body reads it before the metadata pass writes it, so the same
// symbol has to come back either way: imported for a class another image
// defines, and the definition itself for one this unit implements.
func (u *unit) ivarOffsetSymbol(class, ivar string) ir.Symbol {
	name := runtime.IvarOffsetSymbol(class, ivar)
	if s, ok := u.ivarSyms[name]; ok {
		return s
	}
	s := ir.Symbol(u.mod.ImportGlobal(u.sym(name), ir.StoreI32.FType()))
	u.ivarSyms[name] = s
	return s
}

// ---- the two block statements ----

// autoreleasePool lowers §7.3's @autoreleasepool.
//
// It is a push, the body, and a pop of the token the push returned. The pop
// has to happen on every path out, which is why the token is kept on the
// function state: a `return` inside the block pops it before returning.
func (u *unit) autoreleasePool(s *ast.AutoreleasePoolStmt) {
	if !u.at() {
		return
	}
	push := u.extern(runtime.AutoreleasePoolPush, ir.NewSig().Ret(ir.TypePtr))
	pop := u.extern(runtime.AutoreleasePoolPop, ir.NewSig().Param(ir.TypePtr))

	tok := u.fn.cur.Call(push).Ptr(0)
	// The token is parked in a slot as well as kept on the function state:
	// the pad that pops it on the unwinding path is not dominated by the
	// block the push is in, so it cannot read the value directly.
	slot := u.fn.entry.Ptr.Alloc(uint64(u.abi.PtrBytes), uint64(u.abi.PtrBytes))
	u.fn.cur.Ptr.Store(tok, slot)

	u.fn.pools = append(u.fn.pools, tok)
	u.openRegion(u.escapePad("pool.esc", func() {
		u.fn.cur.Call(pop, u.fn.cur.Ptr.Load(slot))
	}))
	u.stmt(s.Body)
	u.closeRegion()
	u.fn.pools = u.fn.pools[:len(u.fn.pools)-1]
	if u.at() {
		u.fn.cur.Call(pop, tok)
	}
}

// releasePools pops every open autorelease pool, innermost first. A return
// from inside one leaves the block, and leaving it drains it.
func (u *unit) releasePools() {
	if !u.at() || len(u.fn.pools) == 0 {
		return
	}
	pop := u.extern(runtime.AutoreleasePoolPop, ir.NewSig().Param(ir.TypePtr))
	for i := len(u.fn.pools) - 1; i >= 0; i-- {
		u.fn.cur.Call(pop, u.fn.pools[i])
	}
}

// synchronized lowers §7.3's @synchronized: a lock, the body, an unlock.
//
// The unlock happens on every path out, the unwinding one included — a lock
// a thrown exception left held is a deadlock rather than a leak. See try.go
// for what the pad is.
func (u *unit) synchronized(s *ast.SyncStmt) {
	if !u.at() {
		return
	}
	obj := u.rvalue(s.X)
	p, ok := obj.(ir.Ptr)
	if !ok {
		return
	}
	enter := u.extern(runtime.SyncEnter, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypeI32))
	exit := u.extern(runtime.SyncExit, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypeI32))

	// Through a slot, for the reason autoreleasePool states: the pad is
	// reached only by an unwind edge, and the block the lock was taken in
	// does not dominate it.
	slot := u.fn.entry.Ptr.Alloc(uint64(u.abi.PtrBytes), uint64(u.abi.PtrBytes))
	u.fn.cur.Ptr.Store(p, slot)
	u.fn.cur.Call(enter, p)

	u.openRegion(u.escapePad("sync.esc", func() {
		u.fn.cur.Call(exit, u.fn.cur.Ptr.Load(slot))
	}))
	u.stmt(s.Body)
	u.closeRegion()
	if u.at() {
		u.fn.cur.Call(exit, p)
	}
}

// throwStmt lowers `@throw obj;` and the bare `@throw;` (§7.2).
//
// Neither call returns, and the block has to end somewhere: a trap says the
// path stops here without claiming the call fell through. The bare form
// re-raises the object the @catch around it is holding, which is what the
// runtime's rethrow does with no argument at all — objc_begin_catch already
// told it which object that is.
//
// A throw inside a @try is a call with an unwind edge like any other: the
// @catch clauses of the very @try it stands in are entitled to see it.
func (u *unit) throwStmt(s *ast.ThrowStmt) {
	if !u.at() {
		return
	}
	if s.X == nil {
		u.callMaybeUnwind(u.extern(runtime.ExceptionRethrow, ir.NewSig()))
		u.endThrow()
		return
	}
	v := u.rvalue(s.X)
	p, ok := v.(ir.Ptr)
	if !ok {
		return
	}
	throw := u.extern(runtime.ExceptionThrow, ir.NewSig().Param(ir.TypePtr))
	u.callMaybeUnwind(throw, p)
	u.endThrow()
}

// endThrow closes the path a throw left open. With a handler in scope the
// call was an invoke, so lowering is now in its normal edge — a block
// nothing reaches, which still has to end.
func (u *unit) endThrow() {
	if !u.at() {
		return
	}
	u.fn.cur.Trap()
	u.fn.cur = nil
}

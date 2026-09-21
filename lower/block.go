package lower

import (
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// Block literals lower into a descriptor, an invoke function, optional copy/dispose
// helpers, and a block structure containing captured variables.
// Global blocks (no captures) are emitted as constants in __const.
// Stack blocks are allocated on the stack with captures from analyzer.Info.Captures.

// blockCapture is one capture with its place in the literal decided.
type blockCapture struct {
	analyzer.Capture
	off int64

	// field is the runtime's BLOCK_FIELD_* code for what the helpers do
	// with it, and zero for a capture the helpers do not touch. A captured
	// int is copied and forgotten; a captured object is retained by the
	// copy helper and released by the dispose helper, which is what keeps
	// it alive for as long as the block is.
	field int64
}

// blockParts is everything a literal produces except the literal itself.
type blockParts struct {
	caps   []blockCapture
	size   int64
	invoke *ir.Func
	desc   ir.Symbol
	flags  runtime.BlockFlag
}

// blockLit lowers ^{ … } where a value is wanted.
func (u *unit) blockLit(e *ast.BlockLit, t types.Type) ir.Value {
	p, ok := u.blockParts(e, t)
	if !ok {
		return nil
	}
	if len(p.caps) == 0 {
		return u.fn.cur.Ptr.GetAddr(u.globalBlock(p))
	}
	return u.stackBlock(e, p)
}

// blockConst is ^{ … } in a file-scope initializer.
//
// Only a literal that captured nothing can be one, and only one can be: a
// global block is a constant, and there is nothing at file scope for a
// literal to capture.
func (u *unit) blockConst(e *ast.BlockLit, t types.Type) (ir.Init, bool) {
	p, ok := u.blockParts(e, t)
	if !ok || len(p.caps) > 0 {
		return ir.Init{}, false
	}
	return ir.RelocInit(u.globalBlock(p)), true
}

// blockParts compiles the body and emits the descriptor.
func (u *unit) blockParts(e *ast.BlockLit, t types.Type) (blockParts, bool) {
	bt, _ := types.Unqualify(t).(*types.Block)
	if bt == nil || bt.Sig == nil {
		u.errorf(e, "internal: a block literal whose type is not a block type")
		return blockParts{}, false
	}
	caps, size, ok := u.placeCaptures(e)
	if !ok {
		return blockParts{}, false
	}

	name := u.blockName()
	invoke := u.blockInvoke(e, bt.Sig, caps, name)
	if invoke == nil {
		return blockParts{}, false
	}

	flags := runtime.BlockHasSignature
	if hasHelpers(caps) {
		flags |= runtime.BlockHasCopyDispose
	}
	if len(caps) == 0 {
		flags |= runtime.BlockIsGlobal
	}
	return blockParts{
		caps:   caps,
		size:   size,
		invoke: invoke,
		desc:   u.blockDescriptor(bt.Sig, caps, size, name),
		flags:  flags,
	}, true
}

// placeCaptures assigns each capture an offset and reports the literal's
// total size.
//
// The size is not rounded up at the end, which is what clang writes into the
// descriptor: a block capturing one int is 36 bytes, not 40. The runtime
// copies exactly that many.
func (u *unit) placeCaptures(e *ast.BlockLit) ([]blockCapture, int64, bool) {
	off := u.abi.SizeOf(runtime.BlockLiteral)
	var out []blockCapture
	for _, c := range u.info.Captures[e] {
		if c.Block {
			// A __block variable is not copied into the literal. What the
			// literal holds is a pointer to the structure the variable
			// lives in, so that the function and every block that captured
			// it reach one variable. See byref.go.
			off = alignUp(off, u.abi.PtrBytes)
			out = append(out, blockCapture{Capture: c, off: off,
				field: runtime.BlockFieldByref})
			off += u.abi.PtrBytes
			continue
		}
		size, align := u.sizeAlign(c.Type)
		if size == 0 {
			size = 1
		}
		off = alignUp(off, int64(align))
		field := blockField(c.Type)
		if u.isWeak(c.Type) {
			field = weakCaptureField
		}
		out = append(out, blockCapture{Capture: c, off: off, field: field})
		off += int64(size)
	}
	return out, off, true
}

// weakCaptureField marks a __weak capture. It is not a BLOCK_FIELD_* code
// and never reaches the runtime: the field is a weak reference of its own,
// made with objc_copyWeak from the captured variable, copied with
// objc_copyWeak and destroyed with objc_destroyWeak -- which is how clang
// keeps `__weak Owner *w = o; o.action = ^{ … w … };` from being a cycle.
const weakCaptureField = -1

// blockField is what a copy helper does with a capture of this type.
func blockField(t types.Type) int64 {
	switch {
	case types.IsObjectPointer(t):
		return runtime.BlockFieldObject
	case types.IsBlock(t):
		return runtime.BlockFieldBlock
	}
	return 0
}

func hasHelpers(caps []blockCapture) bool {
	for _, c := range caps {
		if c.field != 0 {
			return true
		}
	}
	return false
}

func alignUp(n, to int64) int64 {
	if to <= 1 {
		return n
	}
	return (n + to - 1) / to * to
}

// blockName is the middle of every symbol this literal produces.
//
// The shape is clang's — ___main_block_invoke, and _2 on the second literal
// in the same function — because a backtrace through a block is read by
// people, and the enclosing function's name is the only thing in it that
// says where the block was written.
func (u *unit) blockName() string {
	fn := u.blockBase
	if fn == "" {
		fn = "anon"
	}
	u.blockSeq++
	if u.blockSeq > 1 {
		return fn + "\x00" + itoa(u.blockSeq)
	}
	return fn
}

// enterFunc starts a function's block numbering and returns the function
// that restores the enclosing one's. Every symbol a literal produces is
// named after the function it was written in, counted from one.
func (u *unit) enterFunc(fn *ir.Func) func() {
	base, seq := u.blockBase, u.blockSeq
	u.blockBase, u.blockSeq = strings.TrimPrefix(fn.Name(), u.symPrefix), 0
	return func() { u.blockBase, u.blockSeq = base, seq }
}

// blockSym applies a name to one of the labels in runtime/block.go. The
// number a second literal carries goes on the end of the whole symbol, not
// in the middle of it, which is where clang puts it.
func blockSym(label func(string) string, name string) string {
	base, suffix, found := strings.Cut(name, "\x00")
	if !found {
		return label(base)
	}
	return label(base) + "_" + suffix
}

// ---- the invoke function ----

// blockInvoke compiles the literal's body into a function of its own.
//
// The signature is the block's with one parameter in front: the block
// itself. Every capture is bound as a local whose storage is inside that
// block, so the body reads a capture exactly as it reads any other variable
// and nothing below this has to know the difference.
func (u *unit) blockInvoke(e *ast.BlockLit, sig *types.Func, caps []blockCapture, name string) *ir.Func {
	// The signature rules are a function's, which sigOf already states: an
	// aggregate parameter travels as a byval pointer and an aggregate
	// result as storage the caller supplied. This asks the same question
	// and builds a function rather than a signature with the answer.
	if _, why := u.sigOf(sig); why != "" {
		u.unsupported(e, "a block with "+why)
		return nil
	}
	fn := u.mod.Func(u.sym(blockSym(runtime.BlockInvokeSymbol, name)))
	fn.Internal()

	var class *types.Class
	if u.fn != nil {
		class = u.fn.class
	}
	prevFn, prevScope := u.fn, u.scope
	u.fn = &fnState{fn: fn, ret: sig.Ret, labels: map[string]*ir.Block{}, class: class}

	// A block body sees file scope and its own names. It does not see the
	// enclosing function's locals: what it reached for is in caps, and
	// leaving the outer scope visible would let a name resolve to a slot in
	// a frame this function does not have.
	//
	// The names that are not locals come with it, though — a static local
	// and an enumeration constant are declared inside the function and live
	// outside the frame. See globalsVisible.
	outer := u.globalsVisible()
	u.scope = u.top
	u.push()
	for name, st := range outer {
		u.bind(name, st)
	}
	prevName := u.funcNameText
	// clang's spelling, without the underscores the symbol carries: what
	// __func__ says inside a block is the invoke function's own name.
	u.funcNameText = strings.ReplaceAll(blockSym(runtime.BlockInvokeSymbol, name), "__", "")
	u.bindFuncName()
	defer func() {
		u.funcNameText = prevName
		u.pop()
		u.fn, u.scope = prevFn, prevScope
	}()

	// The result's storage before everything — §19.13 wants sret first —
	// and then the block, which invoke takes in front of every declared
	// parameter.
	if isIndirectResult(sig.Ret) {
		t, _ := u.aggType(sig.Ret)
		u.fn.sret = fn.ParamPtr("__ret", ir.SRet(t))
	}
	self := addParam(fn, ir.TypePtr, "block")
	values := make([]ir.Value, len(sig.Params))
	for i, p := range sig.Params {
		if isAggregate(p.Type) {
			t, _ := u.aggType(p.Type)
			values[i] = fn.ParamPtr(paramNameAt(e, i, p), ir.ByVal(t))
			continue
		}
		r, _ := u.reg(p.Type)
		values[i] = addParam(fn, r, paramNameAt(e, i, p))
	}
	if sig.Variadic {
		// ^(int n, ...) is variadic like any function: va_start needs to
		// know, and so do the callers, which callBlock tells.
		fn.Variadic()
	}
	if !types.IsVoid(sig.Ret) && !isIndirectResult(sig.Ret) {
		r, _ := u.reg(sig.Ret)
		setReturn(fn, r)
	}

	// Allocations in entry and the body in a block of its own, for the
	// reason buildBody gives: §19.6 admits alloc in the entry block only,
	// and a slot is wanted at points the entry block has long since
	// branched past.
	entry := fn.Entry()
	body := fn.Block("body")
	u.fn.entry, u.fn.cur = entry, body
	defer func() { entry.Br(body.To()) }()

	blk, _ := self.(ir.Ptr)
	// A strong parameter's scope, as a function's: see strongParam.
	u.pushARCScope()
	for i, p := range sig.Params {
		n := paramNameAt(e, i, p)
		if isAggregate(p.Type) {
			// Already storage this function owns: the convention says the
			// caller copied it, so a second copy is a copy of a copy.
			addr, _ := values[i].(ir.Ptr)
			u.bind(n, &storage{kind: stLocal, typ: p.Type, addr: addr})
			if u.isARCRecord(p.Type) {
				u.noteStrongLocal(addr, p.Type)
			}
			continue
		}
		pt := u.strongParam(p.Type, values[i])
		slot := u.slot(pt, n+"_addr")
		u.storeTo(slot, u.paramValue(pt, values[i]), pt)
		u.bind(n, &storage{kind: stLocal, typ: pt, addr: slot})
		if u.isStrong(pt) {
			u.noteStrongLocal(slot, pt)
		}
	}
	// The captures. Each is a local whose address is inside the block, so a
	// read of one is the ordinary load of a local and costs one add.
	for _, c := range caps {
		addr := u.fn.cur.Ptr.Add(blk, u.fn.cur.I64.Const(c.off))
		u.fn.cur.Name(addr, c.Name+"_addr")
		if c.Block {
			// The block holds a pointer to the structure, so the structure
			// is one load away and every access goes through its
			// forwarding field from there — exactly as in the frame that
			// declared it.
			_, off, _, _ := u.byrefLayout(c.Type)
			slot := u.fn.cur.Ptr.Load(addr)
			u.bind(c.Name, &storage{kind: stByref, typ: c.Type, addr: slot,
				byref: &byref{typ: c.Type, off: off}})
			continue
		}
		u.bind(c.Name, &storage{kind: stLocal, typ: c.Type, addr: addr})
		if c.Name == "self" && class != nil {
			if p, ok := u.loadFrom(addr, c.Type).(ir.Ptr); ok {
				u.fn.self = p
			}
		}
	}
	// §4.5's bare instance variables are as visible inside a block in a
	// method as outside it — through the self the block captured.
	if class != nil && u.fn.self != (ir.Ptr{}) {
		u.bindIvars(class)
	}

	if e.Body != nil {
		u.stmt(e.Body)
	}
	u.popARCScope()
	if u.at() {
		if types.IsVoid(sig.Ret) {
			u.fn.cur.Return()
		} else {
			u.fn.cur.Trap()
		}
	}
	return fn
}

// paramNameAt is the name the literal gave a parameter, or one this makes up.
func paramNameAt(e *ast.BlockLit, i int, p types.Param) string {
	if p.Name != "" {
		return p.Name
	}
	return "p" + itoa(i)
}

// ---- the data ----

// blockDescriptor emits the descriptor: the literal's size, the helpers if
// there are any, and the invoke function's @encode signature.
func (u *unit) blockDescriptor(sig *types.Func, caps []blockCapture, size int64, name string) ir.Symbol {
	fields, init := runtime.BlockDescriptor, []ir.Init{
		ir.Lit(ir.Int(0)),
		ir.Lit(ir.Int(size)),
	}
	typeName := "block_descriptor"
	if hasHelpers(caps) {
		fields, typeName = runtime.BlockDescriptorWithHelpers, "block_descriptor_2"
		init = append(init,
			ir.RelocInit(u.blockCopyHelper(caps, name)),
			ir.RelocInit(u.blockDisposeHelper(caps, name)))
	}
	// The signature, and a null layout. The extended layout the flags could
	// point at describes which captures are objects, and the runtime reads
	// it only when the flag for it is set, which this does not set: the
	// copy helper already says what has to be retained.
	init = append(init,
		ir.RelocInit(u.cstringSym(u.abi.BlockTypes(sig.Ret, sig.Params, u.model))),
		ir.Lit(ir.Int(0)))

	descName := blockSym(func(n string) string { return runtime.BlockDescriptorLabel + "_" + n }, name)
	return u.mod.Global(u.sym(descName), ir.RO,
		u.metaType(typeName, fields).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecBlockConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(init...))
}

// globalBlock is a literal that captured nothing: the whole structure is a
// constant, because there is nothing about it that differs between two
// executions of the statement that wrote it.
func (u *unit) globalBlock(p blockParts) ir.Symbol {
	return u.mod.Global(u.sym(u.uniq(runtime.BlockLiteralLabel)), ir.RO,
		u.metaType("block_literal", runtime.BlockLiteral).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecBlockConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(
			ir.RelocInit(u.blockClass(runtime.GlobalBlockClass)),
			ir.Lit(ir.Int(int64(p.flags))),
			ir.Lit(ir.Int(0)),
			ir.RelocInit(p.invoke),
			ir.RelocInit(p.desc)))
}

// stackBlock builds a literal in the frame and yields its address.
func (u *unit) stackBlock(e *ast.BlockLit, p blockParts) ir.Value {
	caps, size, invoke, desc, flags := p.caps, p.size, p.invoke, p.desc, p.flags

	b := u.fn.cur
	lit := u.fn.entry.Ptr.Alloc(uint64(size), uint64(u.abi.PtrBytes))
	u.fn.entry.Name(lit, "block")

	at := func(off int64) ir.Ptr {
		if off == 0 {
			return lit
		}
		return b.Ptr.Add(lit, b.I64.Const(off))
	}
	put := func(field string, v ir.Value) {
		off, ok := u.abi.OffsetOf(runtime.BlockLiteral, field)
		if !ok {
			u.errorf(e, "internal: a block literal has no "+field+" field")
			return
		}
		switch val := v.(type) {
		case ir.Ptr:
			b.Ptr.Store(val, at(off))
		case ir.I32:
			b.I32.Store(val, at(off))
		}
	}
	put("isa", b.Ptr.GetAddr(u.blockClass(runtime.StackBlockClass)))
	put("flags", b.I32.Const(int64(int32(flags))))
	put("reserved", b.I32.Const(0))
	put("invoke", b.Ptr.GetAddr(invoke))
	put("descriptor", b.Ptr.GetAddr(desc))

	for _, c := range caps {
		st := u.lookup(c.Name)
		if st == nil {
			u.errorf(e, "internal: the capture '"+c.Name+"' is not in scope")
			return nil
		}
		// A __block capture is the structure's address, not the variable's
		// value: that is the whole point of it.
		if c.Block {
			b.Ptr.Store(st.addr, at(c.off))
			continue
		}
		// An aggregate is captured by copying its bytes into the literal,
		// which is what "by value" means for something no register holds.
		if isAggregate(c.Type) {
			if src, ok := u.capturedAddr(st); ok {
				u.copyAggregate(at(c.off), src, c.Type)
				continue
			}
			u.errorf(e, "internal: the capture '"+c.Name+"' has no address")
			return nil
		}
		if c.field == weakCaptureField {
			src, ok := u.capturedAddr(st)
			if !ok {
				u.errorf(e, "internal: the capture '"+c.Name+"' has no address")
				return nil
			}
			u.copyWeak(at(c.off), src)
			// The field is registered with the runtime until the literal
			// dies, which for one on the stack is its scope's end.
			u.noteWeakLocal(at(c.off), c.Type)
			continue
		}
		v := u.loadFrom(st.addr, c.Type)
		if v == nil {
			return nil
		}
		u.storeTo(at(c.off), v, c.Type)
	}
	return lit
}

// blockClass imports one of the two objects a literal's isa points at. They
// are data symbols in libSystem, not classes this compiler declares.
func (u *unit) blockClass(name string) ir.Symbol {
	if sym, ok := u.classSyms[name]; ok {
		return sym
	}
	sym := u.mod.ImportGlobal(u.sym(name), u.ptrFType())
	u.classSyms[name] = sym
	return sym
}

// ---- the helpers ----

// blockCopyHelper is the function the runtime calls when a block is copied
// to the heap: one _Block_object_assign per capture it has to keep alive.
//
//	void copy(void *dst, void *src) {
//	    _Block_object_assign(dst + off, *(void **)(src + off), FIELD);
//	}
//
// The destination is an *address* and the source a *value*, which is not
// symmetrical and is what the runtime's signature says: the helper is
// assigning into the new block, and the runtime needs the address to write.
func (u *unit) blockCopyHelper(caps []blockCapture, name string) ir.Symbol {
	fn := u.mod.Func(u.sym(blockSym(runtime.BlockCopySymbol, name)))
	fn.Internal()
	dst := fn.ParamPtr("dst")
	src := fn.ParamPtr("src")

	prev := u.fn
	u.fn = &fnState{fn: fn, ret: types.Typ(types.Void), labels: map[string]*ir.Block{}}
	defer func() { u.fn = prev }()
	entry := fn.Entry()
	u.fn.entry, u.fn.cur = entry, entry
	b := entry

	asig := ir.NewSig()
	asig.Param(ir.TypePtr)
	asig.Param(ir.TypePtr)
	asig.Param(ir.TypeI32)
	assign := u.extern(runtime.BlockObjectAssign, asig)
	for _, c := range caps {
		if c.field == 0 {
			continue
		}
		if c.field == weakCaptureField {
			u.copyWeak(b.Ptr.Add(dst, b.I64.Const(c.off)), b.Ptr.Add(src, b.I64.Const(c.off)))
			continue
		}
		b.Call(assign,
			b.Ptr.Add(dst, b.I64.Const(c.off)),
			b.Ptr.Load(b.Ptr.Add(src, b.I64.Const(c.off))),
			b.I32.Const(c.field))
	}
	b.Return()
	return fn
}

// blockDisposeHelper is what the runtime calls when a copied block dies.
func (u *unit) blockDisposeHelper(caps []blockCapture, name string) ir.Symbol {
	fn := u.mod.Func(u.sym(blockSym(runtime.BlockDisposeSymbol, name)))
	fn.Internal()
	src := fn.ParamPtr("src")

	prev := u.fn
	u.fn = &fnState{fn: fn, ret: types.Typ(types.Void), labels: map[string]*ir.Block{}}
	defer func() { u.fn = prev }()
	entry := fn.Entry()
	u.fn.entry, u.fn.cur = entry, entry
	b := entry

	dsig := ir.NewSig()
	dsig.Param(ir.TypePtr)
	dsig.Param(ir.TypeI32)
	dispose := u.extern(runtime.BlockObjectDispose, dsig)
	for _, c := range caps {
		if c.field == 0 {
			continue
		}
		if c.field == weakCaptureField {
			u.destroyWeak(b.Ptr.Add(src, b.I64.Const(c.off)))
			continue
		}
		b.Call(dispose,
			b.Ptr.Load(b.Ptr.Add(src, b.I64.Const(c.off))),
			b.I32.Const(c.field))
	}
	b.Return()
	return fn
}

// ---- calling one ----

// callBlock lowers `b(args…)`: load invoke out of the block and call it with
// the block in front. Nothing about which block it is is known here, and
// nothing needs to be — the layout is the ABI, and invoke is at a fixed
// offset in every block there has ever been.
func (u *unit) callBlock(e *ast.CallExpr, bt *types.Block) ir.Value {
	sig := bt.Sig
	if sig == nil {
		u.errorf(e, "internal: a block type with no signature")
		return nil
	}
	recv := u.rvalue(e.Fun)
	p, ok := recv.(ir.Ptr)
	if !ok {
		return nil
	}
	// The result's storage in front of the block, which is where §19.13
	// wants sret and where every convention puts the hidden pointer.
	var out ir.Ptr
	args := []ir.Value{p}
	if isIndirectResult(sig.Ret) {
		out = u.aggResult(sig.Ret)
		args = []ir.Value{out, p}
	}
	fixed := -1 // how many of args precede the var-tail
	var wbs []*writeback
	for i, a := range e.Args {
		if i == len(sig.Params) {
			fixed = len(args)
		}
		if i < len(sig.Params) {
			// §ARC 4.3.2's out-parameter, as for a function: `blk(&err)`
			// hands the block a temporary and copies it back after.
			if v, wb := u.arcOutArg(a, sig.Params[i].Type); wb != nil {
				args = append(args, v)
				wbs = append(wbs, wb)
				continue
			}
		}
		v := u.rvalue(a)
		if v == nil {
			return nil
		}
		at := u.typeOf(a)
		if i < len(sig.Params) {
			v = u.convert(v, at, sig.Params[i].Type)
			if isAggregate(sig.Params[i].Type) {
				copied, ok := u.aggArg(v, sig.Params[i].Type, a)
				if !ok {
					return nil
				}
				v = copied
			}
		} else {
			if types.IsRecord(at) {
				words, ok := u.variadicAggregate(v, at, a)
				if !ok {
					return nil
				}
				args = append(args, words...)
				continue
			}
			v = u.defaultPromote(v, at)
		}
		args = append(args, v)
	}

	b := u.fn.cur
	off, ok := u.abi.OffsetOf(runtime.BlockLiteral, "invoke")
	if !ok {
		u.errorf(e, "internal: a block literal has no invoke field")
		return nil
	}
	invoke := b.Ptr.Load(b.Ptr.Add(p, b.I64.Const(off)))

	isig := ir.NewSig()
	lead := 1 // the block itself, which invoke takes in front
	if out != (ir.Ptr{}) {
		lead = 2
	}
	if fixed < 0 {
		fixed = len(args)
	}
	for i, a := range args {
		// The var-tail is not described: on Apple's arm64 it goes on the
		// stack where a fixed argument would be in a register, and the
		// signature saying where it starts is what puts it there.
		if sig.Variadic && i == fixed {
			isig.Variadic()
			break
		}
		r, ok := u.regOfValue(a)
		if !ok {
			u.errorf(e, "internal: an argument to a block is not a register value")
			return nil
		}
		if i == 0 && out != (ir.Ptr{}) {
			t, _ := u.aggType(sig.Ret)
			isig.Param(r, ir.SRet(t))
			continue
		}
		if j := i - lead; j >= 0 && j < len(sig.Params) && isAggregate(sig.Params[j].Type) {
			t, _ := u.aggType(sig.Params[j].Type)
			isig.Param(r, ir.ByVal(t))
			continue
		}
		isig.Param(r)
	}
	hasRet := false
	if !types.IsVoid(sig.Ret) && out == (ir.Ptr{}) {
		r, ok := u.reg(sig.Ret)
		if !ok {
			u.unsupported(e, "a block returning "+sig.Ret.String())
			return nil
		}
		isig.Ret(r)
		hasRet = true
	}
	res := u.callIndMaybeUnwind(invoke, u.namedFuncType("blocksig", isig), args...)
	u.applyWritebacks(wbs)
	if out != (ir.Ptr{}) {
		return out
	}
	if !hasRet || res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// capturedAddr is where a capture's value is in the enclosing frame. It is
// the slot itself for an ordinary local, and the variable inside the
// structure for one that was itself captured by an enclosing block.
func (u *unit) capturedAddr(st *storage) (ir.Ptr, bool) {
	switch st.kind {
	case stLocal:
		return st.addr, true
	case stByref:
		return u.byrefAddr(st.byref, st.addr), true
	}
	return ir.Ptr{}, false
}

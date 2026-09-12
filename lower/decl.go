package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// fnState is one function or method being built.
type fnState struct {
	fn    *ir.Func
	entry *ir.Block
	cur   *ir.Block // where instructions go now; nil once the path ended

	ret types.Type

	// labels are the blocks a goto may reach, created on first mention so
	// that a forward goto and its label meet.
	labels map[string]*ir.Block

	// breaks and continues are the innermost loop's or switch's targets,
	// each with the number of @try scopes that were open where it was
	// made: leaving through one runs its @finally, and that is the count
	// that says how many.
	breaks    []jumpTarget
	continues []jumpTarget

	// cases is the block each open switch gave each of its labels. A label
	// is not always at the top of the body, so it is found by lookup where
	// it stands rather than by walking for it.
	cases []map[ast.Stmt]*ir.Block

	// self is the receiver, inside a method. Every instance variable
	// access and every implicit send starts here.
	self  ir.Ptr
	class *types.Class

	// sret is the storage the caller supplied for a result this function
	// returns by writing rather than by value. A return statement copies
	// into it; see agg.go.
	sret ir.Ptr

	// vlaScopes are the open C blocks' stack marks, innermost last: a block
	// that allocated a variably modified array saves the stack pointer on
	// its way in and restores it on its way out. See vla.go.
	vlaScopes []vlaScope

	// pools are the autorelease pool tokens of the enclosing
	// @autoreleasepool blocks, innermost last.
	pools []ir.Ptr

	// tries are the open @try statements, innermost last, and handlers the
	// pad block a call inside the region being lowered unwinds to. They are
	// two stacks rather than one because they do not nest alike: a @try
	// stays open across its own @catch and @finally bodies, and each of
	// those has a pad of its own. See try.go.
	tries    []*tryScope
	handlers []*handler

	// retSlot is where a return crossing a @finally parks its value while
	// the block runs. One per function, made on first need.
	retSlot ir.Ptr

	// byrefs are the __block structures this function declared, which are
	// handed back to the runtime on every path out. See byref.go.
	byrefs []ir.Ptr

	// temps are the objects the full expression being lowered produced at
	// +1 and nothing has claimed. See arc.go.
	temps []ir.Value

	// strongs are the __strong locals in scope, innermost scope last, each
	// released where its scope ends.
	strongs [][]strongLocal

	// retainedReturn says this method returns +1, and consumesSelf that it
	// takes ownership of its receiver. Both are what the selector's family
	// says; only an init method does the second.
	retainedReturn bool
	consumesSelf   bool

	// classMethod says this is a + method, which decides what a super send
	// starts its search above: the metaclass and not the class. See
	// superRef.
	classMethod bool

	nblocks int
}

// A jumpTarget is where a break or a continue goes, and how deep in the
// @try stack the statement that owns it stands.
type jumpTarget struct {
	blk   *ir.Block
	tries int
}

// block makes a fresh block with a readable label.
//
// A VIR label is an identifier, so the readable `for.cond` shape a reader
// expects is spelled with an underscore. The counter keeps two loops in one
// function from claiming the same label.
func (u *unit) block(prefix string) *ir.Block {
	u.fn.nblocks++
	label := make([]byte, 0, len(prefix)+4)
	for i := 0; i < len(prefix); i++ {
		if prefix[i] == '.' {
			label = append(label, '_')
			continue
		}
		label = append(label, prefix[i])
	}
	return u.fn.fn.Block(string(label) + "_" + itoa(u.fn.nblocks))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// at reports whether there is an open path to emit into. A statement after a
// `return` has none, and emitting into a terminated block is an error the IR
// would report rather than a program anyone wrote.
func (u *unit) at() bool { return u.fn != nil && u.fn.cur != nil }

// ---- pass one: declare ----

// declareFile enters every file-scope name, so that a definition may call a
// function declared below it.
//
// A function this unit defines is created here as a definition, not an
// import: the two are one symbol, and a call written before the definition
// has to reach the same one the definition fills in.
func (u *unit) declareFile() {
	for _, d := range u.fileScope() {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Name == nil {
			continue
		}
		name := u.linkName(u.name(fd.Name), fd)
		// A definition this unit will not emit is not a definition to
		// declare: a symbol created here and never given a body is a
		// module the verifier rejects. See notEmitted.
		if u.notEmitted(name, fd) {
			u.omitted[name] = true
			continue
		}
		u.defines[name] = true
	}

	// And which file-scope *objects* it defines, for the same reason. A
	// header writes `extern int counter;` and the file below it writes
	// `int counter = 0;`, and whichever is read first decides whether the
	// module defines the name or imports it — so the answer is settled
	// before either is read.
	//
	// §6.9.2's tentative definition counts: `int counter;` at file scope
	// with no initializer defines the object too, and a unit with both that
	// and an extern declaration still defines one.
	for _, d := range u.fileScope() {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		sp := u.specs(g)
		if sp.storage == token.TYPEDEF || sp.storage == token.EXTERN {
			continue
		}
		for _, it := range g.List {
			name := u.name(declName(it.Decl))
			t := u.typeOf(it)
			if name == "" || t == nil {
				continue
			}
			if types.Unqualify(t).Kind() == types.FuncKind {
				continue // a prototype, which declareFuncName settles
			}
			u.definesVar[name] = true
		}
	}
	for _, d := range u.file.Decls {
		if impl, ok := d.(*ast.ClassImplDecl); ok {
			if k := u.classNamed(u.name(impl.Name)); k != nil {
				u.declareClass(k)
				u.declareIvars(k)
			}
		}
	}
	for _, d := range u.fileScope() {
		switch d := d.(type) {
		case *ast.GenDecl:
			u.declareGen(d)
		case *ast.FuncDecl:
			u.declareFunc(d)
		}
	}
}

// fileScope is every declaration at file scope, including the ones written
// inside an @interface, an @implementation, a category or a protocol.
//
// A C declaration between two methods is not a member of the class: §4.4
// puts it at file scope like any other, and a `static` there is the ordinary
// way to give a class one variable rather than one per instance —
//
//	@implementation Calculator
//	static NSUInteger gCalls = 0;
//	+ (void)initialize { gCalls++; }
//	@end
//
// — which is how +initialize is written wherever it is written at all. The
// definitions were already emitted from inside the walk over members; what
// was missing was the declaration, so a method body referring to one reached
// lowering with nothing to refer to.
//
// The same holds inside an *@interface*, and AppKit depends on it. NSApp is
// declared between the class's first line and its first property:
//
//	@interface NSApplication : NSResponder <…>
//	APPKIT_EXTERN __kindof NSApplication *NSApp;
//	@property (class, readonly, strong) __kindof NSApplication *sharedApplication;
//
// A class body holds methods, properties and instance variables (§4); a C
// declaration in one is at file scope wherever it stands, and `[NSApp
// activateIgnoringOtherApps:YES]` is in every AppKit program's main.
func (u *unit) fileScope() []ast.Decl {
	out := make([]ast.Decl, 0, len(u.file.Decls))
	add := func(members []ast.Decl) {
		for _, m := range members {
			switch m.(type) {
			case *ast.GenDecl, *ast.FuncDecl:
				out = append(out, m)
			}
		}
	}
	for _, d := range u.file.Decls {
		out = append(out, d)
		switch d := d.(type) {
		case *ast.ClassInterfaceDecl:
			add(d.Members)
		case *ast.ClassImplDecl:
			add(d.Members)
		case *ast.CategoryDecl:
			add(d.Members)
		case *ast.CategoryImplDecl:
			add(d.Members)
		case *ast.ProtocolDecl:
			add(d.Members)
		}
	}
	return out
}

func (u *unit) declareGen(d *ast.GenDecl) {
	sp := u.specs(d)
	for _, it := range d.List {
		name := u.name(declName(it.Decl))
		if name == "" {
			continue
		}
		t := u.typeOf(it)
		if t == nil {
			continue
		}
		if sp.storage == token.TYPEDEF {
			continue
		}
		if types.Unqualify(t).Kind() == types.FuncKind {
			if !u.checkOverloadLinkage(name, it, sp.storage == token.STATIC) {
				continue
			}
			u.declareFuncName(name, t, it)
			continue
		}
		u.declareGlobalVar(name, t, sp, it)
	}
}

// specs is the storage class and the facts about a declaration lowering
// needs. Analysis already validated them; this only reads them back.
type declSpec struct {
	storage token.Kind
	block   bool
}

func (u *unit) specs(d *ast.GenDecl) declSpec {
	var sp declSpec
	for _, s := range d.Specs {
		ks, ok := s.(*ast.KeywordSpec)
		if !ok {
			continue
		}
		switch ks.Kind {
		case token.TYPEDEF, token.EXTERN, token.STATIC, token.AUTO, token.REGISTER:
			sp.storage = ks.Kind
		case token.BLOCK:
			sp.block = true
		}
	}
	return sp
}

func (u *unit) declareGlobalVar(name string, t types.Type, sp declSpec, at ast.Node) {
	if u.lookup(name) != nil {
		return
	}
	f, ok := u.ftype(t)
	if !ok {
		// Same rule as a function declaration: <dispatch/queue.h> declares
		// _dispatch_main_q as a struct dispatch_queue_s, which is opaque
		// and has no VIR type, and every Objective-C program on Darwin
		// reads that declaration and no program reads that object.
		u.undescribed[name] = "a global of type " + t.String()
		return
	}

	if !u.definesVar[name] {
		u.top.names[name] = &storage{kind: stGlobal, typ: t,
			imp: &pendingImport{sym: u.sym(name), ftyp: f}}
		return
	}
	g := u.mod.Global(u.sym(name), ir.RW, f)
	if sp.storage == token.STATIC {
		g.Internal()
	} else {
		g.Export()
	}
	_, align := u.sizeAlign(t)
	g.Align(align)
	u.top.names[name] = &storage{kind: stGlobal, typ: t, sym: g}
}

// bindFuncName gives §6.4.2.2's predefined identifier its storage.
//
// __func__ is not a macro. It is *declared*, at the top of every function
// body, as `static const char __func__[] = "name"` — so it is a read-only
// array like any other string literal, and the three spellings gcc gives it
// share one. Every logging macro in every project expands to it.
//
// __PRETTY_FUNCTION__ is the same string here and a signature under clang.
// Spelling a C declaration back out is a renderer this package does not have
// and would have to agree with character for character to be worth anything.
func (u *unit) bindFuncName() {
	if u.funcNameText == "" {
		return
	}
	t := types.Qualify(&types.Array{
		Elem: types.Typ(types.Char),
		Form: types.FixedArray,
		Len:  int64(len(u.funcNameText)) + 1,
	}, types.QConst)
	sym := u.cstringSym(u.funcNameText)
	for _, name := range [...]string{"__func__", "__FUNCTION__", "__PRETTY_FUNCTION__"} {
		u.bind(name, &storage{kind: stGlobal, typ: t, sym: sym})
	}
}

// funcFor is the ir.Func a name denotes in this unit, made on first ask.
func (u *unit) funcFor(name string) *ir.Func {
	if fn := u.funcs[name]; fn != nil {
		return fn
	}
	fn := u.mod.Func(u.sym(name))
	u.funcs[name] = fn
	return fn
}

func (u *unit) declareFunc(d *ast.FuncDecl) {
	t := u.typeOf(d)
	if t == nil || d.Name == nil {
		return
	}
	// A name whose definition this unit omitted gets nothing at all --
	// not even the import a prototype would otherwise produce, which
	// would name a symbol no object file has. See notEmitted.
	if u.omitted[u.linkName(u.name(d.Name), d)] {
		return
	}
	if !u.checkOverloadLinkage(u.name(d.Name), d, u.isStatic(d)) {
		return
	}
	u.declareFuncName(u.name(d.Name), t, d)
}

func (u *unit) declareFuncName(name string, t types.Type, at ast.Node) {
	// An overloaded name is several functions and needs several symbols.
	// See overload.go; the plain name stays with the first of them, so a
	// declaration that carries the attribute and has no siblings is
	// unchanged.
	name = u.linkName(name, at)
	if u.lookup(name) != nil {
		return
	}
	ft, ok := types.Unqualify(t).(*types.Func)
	if !ok {
		return
	}
	sig, why := u.sigOf(ft)
	if why != "" {
		// A declaration is not a demand. Darwin's headers declare a great
		// many functions this compiler cannot call -- <_stdlib.h> has
		// div(), which returns a struct by value -- and a program that
		// never calls one wants nothing from it. Keep the reason and
		// report it if a use turns up. See undescribed.
		u.undescribed[name] = why
		return
	}
	if u.defines[name] {
		fn := u.mod.Func(u.sym(name))
		u.funcs[name] = fn
		u.top.names[name] = &storage{kind: stFunc, typ: t, sym: fn}
		return
	}
	u.top.names[name] = &storage{kind: stFunc, typ: t,
		imp: &pendingImport{sym: u.sym(name), sig: sig}}
}

// signatureLowerable reports whether every type in a signature has a
// representation, naming the first that does not.
func (u *unit) signatureLowerable(ft *types.Func, at ast.Node) bool {
	if isIndirectResult(ft.Ret) {
		if _, ok := u.aggType(ft.Ret); !ok {
			u.unsupported(at, "a return type of "+ft.Ret.String())
			return false
		}
	} else if !types.IsVoid(ft.Ret) {
		if _, ok := u.reg(ft.Ret); !ok {
			u.unsupported(at, "a return type of "+ft.Ret.String())
			return false
		}
	}
	for _, p := range ft.Params {
		if isAggregate(p.Type) {
			if _, ok := u.aggType(p.Type); !ok {
				u.unsupported(at, "a parameter of type "+p.Type.String())
				return false
			}
			continue
		}
		if types.IsVoid(p.Type) {
			continue // `f(void)`, which declares no parameter at all
		}
		if _, ok := u.reg(p.Type); !ok {
			u.unsupported(at, "a parameter of type "+p.Type.String())
			return false
		}
	}
	return true
}

// A funcSig is what a function's signature produced: its parameter values and
// the storage a result too large to return in registers is written through.
type funcSig struct {
	values []ir.Value
	sret   ir.Ptr
}

// funcSignature gives fn its parameters and its result, once.
//
// Apart from the body because a *call* has to see the signature before the
// body is lowered — a function defined further down the file is called from
// one lowered above it — and because parameters may only be added to an
// ir.Func before its entry block exists. So every definition in the unit gets
// its signature in one pass and its body in the next, and this is what the
// first pass runs. See defineFile.
func (u *unit) funcSignature(fn *ir.Func, ft *types.Func, names []*ast.Ident, at ast.Node) (funcSig, bool) {
	if sig, ok := u.sigs[fn]; ok {
		return sig, true
	}
	var sig funcSig
	// Every type is checked before any parameter is added, because adding
	// one is not undoable: a signature that failed halfway is retried the
	// next time the function is reached — nothing caches a failure — and
	// the second attempt appends a second result pointer beside the first.
	// What came out was the IR builder's complaint about *that*, on a line
	// of <arm/_types.h>, instead of the diagnostic naming the type this
	// package cannot lower.
	if !u.signatureLowerable(ft, at) {
		return sig, false
	}

	// The result's storage, before every parameter: such a function does not
	// return a value at all, it writes one through the pointer its caller
	// handed it.
	if isIndirectResult(ft.Ret) {
		t, _ := u.aggType(ft.Ret)
		sig.sret = fn.ParamPtr("__ret", ir.SRet(t))
	}
	for i, p := range ft.Params {
		name := declParamName(u, names, i, p)
		if isAggregate(p.Type) {
			t, _ := u.aggType(p.Type)
			sig.values = append(sig.values, fn.ParamPtr(name, ir.ByVal(t)))
			continue
		}
		r, _ := u.reg(p.Type)
		sig.values = append(sig.values, addParam(fn, r, name))
	}
	if ft.Variadic {
		// The var-tail is part of the signature and not only of the call:
		// va_start needs to know this function has one, and a call to it
		// needs to know which arguments the declaration named.
		fn.Variadic()
	}
	if !types.IsVoid(ft.Ret) && !isIndirectResult(ft.Ret) {
		r, ok := u.reg(ft.Ret)
		if !ok {
			u.unsupported(at, "a return type of "+ft.Ret.String())
			return sig, false
		}
		setReturn(fn, r)
	}
	u.sigs[fn] = sig
	return sig, true
}

// declParamName is the name a parameter is known by: the definition's, where
// there is one, and the declaration's otherwise.
func declParamName(u *unit, names []*ast.Ident, i int, p types.Param) string {
	if i < len(names) && names[i] != nil {
		return u.name(names[i])
	}
	return p.Name
}

// sigOf builds a signature. A parameter or a return value that is not held
// in a register needs the target's classification rules — which struct goes
// in which registers, and which goes in memory — and objv does not have them
// yet, so a function that takes or returns one is refused rather than
// mis-called.
//
// The refusal is a *reason*, not a diagnostic, because the same signature is
// built for a declaration and for a call and only one of those is worth
// reporting. See describeSig.
func (u *unit) sigOf(ft *types.Func) (*ir.Sig, string) {
	sig := ir.NewSig()
	// A result the caller supplies storage for comes first, which is
	// §19.13's rule and is also every convention's: the hidden pointer
	// precedes the real arguments wherever it travels in the argument
	// sequence at all.
	if isIndirectResult(ft.Ret) {
		t, ok := u.aggType(ft.Ret)
		if !ok {
			return nil, "a return type of " + ft.Ret.String()
		}
		sig.Param(ir.TypePtr, ir.SRet(t))
	}
	for _, p := range ft.Params {
		if isAggregate(p.Type) {
			t, ok := u.aggType(p.Type)
			if !ok {
				return nil, "a parameter of type " + p.Type.String()
			}
			sig.Param(ir.TypePtr, ir.ByVal(t))
			continue
		}
		r, ok := u.reg(p.Type)
		if !ok {
			return nil, "a parameter of type " + p.Type.String()
		}
		sig.Param(r)
	}
	if ft.Variadic {
		sig.Variadic()
	}
	if !types.IsVoid(ft.Ret) && !isIndirectResult(ft.Ret) {
		r, ok := u.reg(ft.Ret)
		if !ok {
			return nil, "a return type of " + ft.Ret.String()
		}
		sig.Ret(r)
	}
	return sig, ""
}

// ---- pass two: define ----

func (u *unit) defineFile() {
	// Every definition's signature before any body, so that a call to a
	// function defined further down the file sees the arity it will have.
	// C admits the shape — a prototype above, the definition below — and it
	// is how any file with mutual recursion in it is written.
	for _, d := range u.fileScope() {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Name == nil {
			continue
		}
		t := u.typeOf(fd)
		ft, isFunc := types.Unqualify(t).(*types.Func)
		if !isFunc || u.notEmitted(u.name(fd.Name), fd) {
			continue
		}
		u.funcSignature(u.funcFor(u.name(fd.Name)), ft, paramNames(fd), fd)
	}

	for _, d := range u.file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			u.defineFunc(d)
		case *ast.GenDecl:
			u.defineGlobals(d)
		case *ast.ClassImplDecl:
			u.defineImpl(u.name(d.Name), "", d.Members)
		case *ast.CategoryImplDecl:
			u.defineImpl(u.name(d.Class), u.name(d.Name), d.Members)
		}
	}
}

// defineGlobals gives file-scope objects their initializers.
func (u *unit) defineGlobals(d *ast.GenDecl) {
	sp := u.specs(d)
	if sp.storage == token.TYPEDEF || sp.storage == token.EXTERN {
		return
	}
	for _, it := range d.List {
		if it.Init == nil {
			continue
		}
		name := u.name(declName(it.Decl))
		st := u.lookup(name)
		if st == nil || st.kind != stGlobal {
			continue
		}
		g, ok := st.sym.(*ir.Global)
		if !ok {
			continue
		}
		init, ok := u.constInit(it.Init, st.typ)
		if !ok {
			u.unsupported(it, "this initializer at file scope")
			continue
		}
		g.Init(init)
	}
}

func (u *unit) defineFunc(d *ast.FuncDecl) {
	if d.Body == nil || d.Name == nil {
		return
	}
	t := u.typeOf(d)
	ft, ok := types.Unqualify(t).(*types.Func)
	if !ok {
		return
	}
	name := u.name(d.Name)
	if !u.checkOverloadLinkage(name, d, u.isStatic(d)) {
		return
	}
	// __func__ keeps the name the source wrote; the symbol is what the
	// overload suffix belongs to.
	u.funcNameText = name
	name = u.linkName(name, d)
	if u.notEmitted(name, d) {
		return
	}
	defer func() { u.funcNameText = "" }()
	fn := u.funcFor(name)
	// An inline definition is emitted internal, not exported. §6.7.4p7 says
	// it provides no external definition, so the one unit that wrote
	// `extern inline int f(void);` owns the name -- and every other unit
	// that used the function would collide with it, and with each other,
	// if this exported a second `_f`. C99 does not promise the two are the
	// same function, only that both behave the same way, so a private copy
	// per unit is conforming and is what links.
	if u.isStatic(d) || u.isInlineDefinition(name, d) {
		fn.Internal()
	} else {
		fn.Export()
	}
	u.top.names[name] = &storage{kind: stFunc, typ: t, sym: fn}
	u.noteStaticInit(name, fn)

	u.buildBody(fn, ft, paramNames(d), d.Body, nil, runtime.FamilyNone)
}

// notEmitted reports whether a definition this unit read is one it does not
// owe an object file.
//
// A static function nothing here mentions cannot be called from anywhere, and
// an inline definition provides no external definition (§6.7.4p7), so both are
// emitted only when used. See useset.go for why that is not an optimization:
// Apple's <math.h> and <objc/objc.h> define inline functions in terms of
// builtins objv does not implement, and one #import brings in dozens.
func (u *unit) notEmitted(name string, d *ast.FuncDecl) bool {
	if !u.isStatic(d) && !u.isInlineDefinition(name, d) {
		return false
	}
	if u.used == nil {
		u.used = u.planUsed()
	}
	return !u.used[name]
}

func (u *unit) isStatic(d *ast.FuncDecl) bool {
	for _, s := range d.Specs {
		if ks, ok := s.(*ast.KeywordSpec); ok && ks.Kind == token.STATIC {
			return true
		}
	}
	return false
}

// paramNames is the names the definition gave its parameters, in order.
func paramNames(d *ast.FuncDecl) []*ast.Ident {
	fd := outermostFunc(d.Decl)
	if fd == nil {
		return nil
	}
	var out []*ast.Ident
	for _, p := range fd.Params {
		if p.Decl != nil {
			out = append(out, p.Decl.DeclName())
		} else {
			out = append(out, nil)
		}
	}
	for _, id := range fd.Idents {
		out = append(out, id)
	}
	return out
}

func outermostFunc(d ast.Declarator) *ast.FuncDeclarator {
	var found *ast.FuncDeclarator
	for {
		switch dd := d.(type) {
		case *ast.FuncDeclarator:
			found, d = dd, dd.Inner
		case *ast.PtrDeclarator:
			d = dd.Inner
		case *ast.BlockPtrDeclarator:
			d = dd.Inner
		case *ast.ParenDeclarator:
			d = dd.Inner
		case *ast.ArrayDeclarator:
			d = dd.Inner
		default:
			return found
		}
	}
}

func declName(d ast.Declarator) *ast.Ident {
	if d == nil {
		return nil
	}
	return d.DeclName()
}

// buildBody is the shared body of a function and a method.
//
// A method is a function whose first two parameters are self and _cmd, so
// the only thing that differs is what the parameters are called and what is
// bound in the scope — which is why a method does not get a lowering of its
// own.
func (u *unit) buildBody(fn *ir.Func, ft *types.Func, names []*ast.Ident,
	stmts *ast.CompoundStmt, self *types.Class, fam runtime.Family) {

	prev := u.fn
	u.fn = &fnState{fn: fn, ret: ft.Ret, labels: map[string]*ir.Block{}, class: self,
		classMethod: u.classMethod,
		// What the selector's name promises about what this method returns
		// and what it does with its receiver. See arc.go.
		retainedReturn: u.arc && fam.ReturnsRetained(),
		consumesSelf:   u.arc && fam.ConsumesSelf(),
	}
	leave := u.enterFunc(fn)
	u.push()
	u.bindFuncName()
	defer func() {
		u.pop()
		leave()
		u.fn = prev
	}()

	sig, ok := u.funcSignature(fn, ft, names, stmts)
	if !ok {
		return
	}
	u.fn.sret = sig.sret
	values := sig.values

	// The entry block holds allocations and nothing else, and the body goes
	// in a block of its own.
	//
	// §19.6 admits alloc in the entry block only, and a frame slot is not
	// always wanted at the top: a local declared after a loop, a block
	// literal built after one, a fast enumeration's state — each needs a
	// slot, and by then the entry block would have been terminated by the
	// loop's first branch. Keeping entry open until the body is finished is
	// what makes the two rules compatible, and it costs one branch that
	// every backend folds away.
	entry := fn.Entry()
	body := fn.Block("body")
	u.fn.entry, u.fn.cur = entry, body
	defer func() { entry.Br(body.To()) }()

	// Each parameter is copied into a slot, because a parameter is an
	// ordinary local: it may be assigned, addressed, or captured by a
	// block.
	for i, p := range ft.Params {
		name := declParamName(u, names, i, p)
		// An aggregate parameter is already storage this function owns: the
		// convention says the caller copied it, so the pointer *is* the
		// local and copying it again would be a second copy of a copy.
		if isAggregate(p.Type) {
			addr, _ := values[i].(ir.Ptr)
			u.bind(name, &storage{kind: stLocal, typ: p.Type, addr: addr})
			continue
		}
		// The slot is named apart from the parameter register: they are two
		// registers, and one name printed twice is not a module the text
		// format can read back.
		slot := u.slot(p.Type, name+"_addr")
		u.storeTo(slot, values[i], p.Type)
		u.bind(name, &storage{kind: stLocal, typ: p.Type, addr: slot})

		// self is kept on the function state as well as bound by name:
		// every instance variable access and every implicit send starts
		// from it.
		if self != nil && i == 0 {
			if sp, ok := values[i].(ir.Ptr); ok {
				u.fn.self = sp
			}
		}
	}

	// §4.5: every visible instance variable of the class and its
	// superclasses is a bare name inside a method body. Superclass first,
	// so that a subclass's own variable shadows an inherited one.
	if self != nil {
		u.bindIvars(self)
	}

	u.stmt(stmts)

	// A body that fell off the end still needs a terminator. A void
	// function returns; anything else has already been reported by the
	// analyzer, and a trap is what a program that reaches here does.
	if u.at() {
		if types.IsVoid(ft.Ret) {
			u.releaseAllStrong()
			if fam == runtime.FamilyNone && self != nil && u.deallocating {
				u.arcSuperDealloc(self)
			}
			u.releaseByrefs()
			u.fn.cur.Return()
		} else {
			u.fn.cur.Trap()
		}
	}
}

// bindIvars puts a class's instance variables in scope, superclass first.
func (u *unit) bindIvars(k *types.Class) {
	var chain []*types.Class
	for x := k; x != nil; x = x.Super {
		chain = append(chain, x)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		x := chain[i]
		for j := range x.Ivars {
			iv := &x.Ivars[j]
			u.bind(iv.Name, &storage{
				kind: stIvar, typ: iv.Type, class: x.Name, ivar: iv.Name,
			})
		}
	}
}

// slot allocates a local's storage in the entry block.
//
// Every local gets one. A variable that is addressed, captured by a block,
// or assigned inside a loop needs a place to be, and deciding which locals
// could have avoided one is the optimizer's job.
func (u *unit) slot(t types.Type, name string) ir.Ptr {
	size, align := u.sizeAlign(t)
	if size == 0 {
		size = 1
	}
	p := u.fn.entry.Ptr.Alloc(size, align)
	if name != "" {
		u.fn.entry.Name(p, name)
	}
	return p
}

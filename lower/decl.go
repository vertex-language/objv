package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/ast"
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

	// breaks and continues are the innermost loop's or switch's targets.
	breaks    []*ir.Block
	continues []*ir.Block

	// self is the receiver, inside a method. Every instance variable
	// access and every implicit send starts here.
	self  ir.Ptr
	class *types.Class

	// pools are the autorelease pool tokens of the enclosing
	// @autoreleasepool blocks, innermost last.
	pools []ir.Ptr

	nblocks int
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
	for _, d := range u.file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Name == nil {
			continue
		}
		name := u.name(fd.Name)
		// A definition this unit will not emit is not a definition to
		// declare: a symbol created here and never given a body is a
		// module the verifier rejects. See notEmitted.
		if u.notEmitted(name, fd) {
			u.omitted[name] = true
			continue
		}
		u.defines[name] = true
	}
	for _, d := range u.file.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			u.declareGen(d)
		case *ast.FuncDecl:
			u.declareFunc(d)
		case *ast.ClassImplDecl:
			if k := u.classNamed(u.name(d.Name)); k != nil {
				u.declareClass(k)
				u.declareIvars(k)
			}
		}
	}
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

	if sp.storage == token.EXTERN {
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

func (u *unit) declareFunc(d *ast.FuncDecl) {
	t := u.typeOf(d)
	if t == nil || d.Name == nil {
		return
	}
	// A name whose definition this unit omitted gets nothing at all --
	// not even the import a prototype would otherwise produce, which
	// would name a symbol no object file has. See notEmitted.
	if u.omitted[u.name(d.Name)] {
		return
	}
	u.declareFuncName(u.name(d.Name), t, d)
}

func (u *unit) declareFuncName(name string, t types.Type, at ast.Node) {
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
	for _, p := range ft.Params {
		if isAggregate(p.Type) {
			return nil, "a function taking a struct or union by value"
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
	if !types.IsVoid(ft.Ret) {
		if isAggregate(ft.Ret) {
			return nil, "a function returning a struct or union"
		}
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
	if u.notEmitted(name, d) {
		return
	}
	fn := u.funcs[name]
	if fn == nil {
		fn = u.mod.Func(u.sym(name))
		u.funcs[name] = fn
	}
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

	u.buildBody(fn, ft, paramNames(d), d.Body, nil)
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
	stmts *ast.CompoundStmt, self *types.Class) {

	prev := u.fn
	u.fn = &fnState{fn: fn, ret: ft.Ret, labels: map[string]*ir.Block{}, class: self}
	leave := u.enterFunc(fn)
	u.push()
	defer func() {
		u.pop()
		leave()
		u.fn = prev
	}()

	// The parameters are declared before the entry block, because a block
	// freezes the signature the first time it is written into.
	paramName := func(i int, p types.Param) string {
		if i < len(names) && names[i] != nil {
			return u.name(names[i])
		}
		return p.Name
	}
	var values []ir.Value
	for i, p := range ft.Params {
		r, ok := u.reg(p.Type)
		if !ok {
			u.unsupported(stmts, "a parameter of type "+p.Type.String())
			return
		}
		values = append(values, addParam(fn, r, paramName(i, p)))
	}
	if !types.IsVoid(ft.Ret) {
		r, ok := u.reg(ft.Ret)
		if !ok {
			u.unsupported(stmts, "a return type of "+ft.Ret.String())
			return
		}
		setReturn(fn, r)
	}

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
		name := paramName(i, p)
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

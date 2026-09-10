package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Statements, as control flow.
//
// A statement emits into u.fn.cur and may leave it nil: after a return, a
// break or a goto there is no open path, and the statements that follow one
// are unreachable. Nothing here creates a block to hold them — a block with
// no predecessor is what the IR's verifier would object to, and the source
// that wrote it is what the analyzer already reported.

func (u *unit) stmt(s ast.Stmt) {
	if s == nil {
		return
	}
	switch s := s.(type) {
	case *ast.CompoundStmt:
		u.push()
		for _, item := range s.Items {
			u.stmt(item)
		}
		u.pop()

	case *ast.DeclStmt:
		u.localDecl(s.D)

	case *ast.ExprStmt:
		u.rvalue(s.X)

	case *ast.EmptyStmt, *ast.BadStmt:

	case *ast.ReturnStmt:
		u.returnStmt(s)

	case *ast.IfStmt:
		u.ifStmt(s)

	case *ast.WhileStmt:
		u.whileStmt(s)

	case *ast.DoStmt:
		u.doStmt(s)

	case *ast.ForStmt:
		u.forStmt(s)

	case *ast.SwitchStmt:
		u.switchStmt(s)

	case *ast.CaseStmt:
		// Reached only outside a switch, which the analyzer reported.
		u.stmt(s.Stmt)

	case *ast.BreakStmt:
		if !u.at() {
			return
		}
		if n := len(u.fn.breaks); n > 0 {
			u.fn.cur.Br(u.fn.breaks[n-1].To())
			u.fn.cur = nil
		}

	case *ast.ContinueStmt:
		if !u.at() {
			return
		}
		if n := len(u.fn.continues); n > 0 {
			u.fn.cur.Br(u.fn.continues[n-1].To())
			u.fn.cur = nil
		}

	case *ast.LabeledStmt:
		blk := u.labelBlock(u.name(s.Label))
		if u.at() {
			u.fn.cur.Br(blk.To())
		}
		u.fn.cur = blk
		u.stmt(s.Stmt)

	case *ast.GotoStmt:
		if !u.at() {
			return
		}
		u.fn.cur.Br(u.labelBlock(u.name(s.Label)).To())
		u.fn.cur = nil

	case *ast.AutoreleasePoolStmt:
		u.autoreleasePool(s)

	case *ast.SyncStmt:
		u.synchronized(s)

	case *ast.ForInStmt:
		u.forIn(s)

	case *ast.ThrowStmt:
		u.throwStmt(s)

	case *ast.TryStmt:
		u.unsupported(s, "@try")

	case *ast.AsmStmt:
		u.unsupported(s, "inline assembly")

	default:
		u.unsupported(s, "this statement")
	}
}

// labelBlock is the block a label names, made on first mention so that a
// forward goto and its label meet.
func (u *unit) labelBlock(name string) *ir.Block {
	if b, ok := u.fn.labels[name]; ok {
		return b
	}
	b := u.block("label." + name)
	u.fn.labels[name] = b
	return b
}

// localDecl gives a block-scope declaration its storage and its initializer.
func (u *unit) localDecl(d ast.Decl) {
	g, ok := d.(*ast.GenDecl)
	if !ok {
		return
	}
	sp := u.specs(g)
	if sp.storage == token.TYPEDEF {
		return
	}
	for _, it := range g.List {
		name := u.name(declName(it.Decl))
		t := u.typeOf(it)
		if name == "" || t == nil {
			continue
		}
		if sp.storage == token.EXTERN {
			u.declareGlobalVar(name, t, sp, it)
			continue
		}
		if sp.storage == token.STATIC {
			// A static local is a global with a name nobody else can
			// write: one object, initialized once, that keeps its value
			// across calls.
			sym := u.staticLocal(name, t, it)
			u.bind(name, &storage{kind: stGlobal, typ: t, sym: sym})
			continue
		}
		slot := u.slot(t, name)
		u.bind(name, &storage{kind: stLocal, typ: t, addr: slot})
		if it.Init == nil {
			continue
		}
		u.initLocal(slot, t, it.Init)
	}
}

// staticLocal emits the global a static local is.
//
// Its initializer is a constant expression like any other static object's
// (§6.7.9p4), so it is folded here and not stored on entry: the object is
// initialized once, before the program runs, and a function that is never
// called still has it.
func (u *unit) staticLocal(name string, t types.Type, it *ast.InitDeclarator) ir.Symbol {
	f, ok := u.ftype(t)
	if !ok {
		u.unsupported(it, "a static local of type "+t.String())
		return nil
	}
	g := u.mod.Global(u.sym(u.uniq("static."+name)), ir.RW, f).Internal()
	_, align := u.sizeAlign(t)
	g.Align(align)
	if it.Init != nil {
		init, ok := u.constInit(it.Init, t)
		if !ok {
			u.unsupported(it, "this initializer on a static local")
			return g
		}
		g.Init(init)
	}
	return g
}

func (u *unit) returnStmt(s *ast.ReturnStmt) {
	if !u.at() {
		return
	}
	if s.Result == nil {
		u.releasePools()
		u.fn.cur.Return()
		u.fn.cur = nil
		return
	}
	v := u.rvalue(s.Result)
	if v == nil {
		return
	}
	v = u.convert(v, u.typeOf(s.Result), u.fn.ret)
	u.releasePools()
	u.fn.cur.Return(v)
	u.fn.cur = nil
}

func (u *unit) ifStmt(s *ast.IfStmt) {
	c := u.truth(s.Cond)
	if c == nil {
		return
	}
	thenB := u.block("if.then")
	done := u.block("if.done")
	elseB := done
	if s.Else != nil {
		elseB = u.block("if.else")
	}
	u.fn.cur.BrIf(*c, thenB.To(), elseB.To())

	u.fn.cur = thenB
	u.stmt(s.Then)
	if u.at() {
		u.fn.cur.Br(done.To())
	}
	if s.Else != nil {
		u.fn.cur = elseB
		u.stmt(s.Else)
		if u.at() {
			u.fn.cur.Br(done.To())
		}
	}
	u.fn.cur = done
}

func (u *unit) whileStmt(s *ast.WhileStmt) {
	cond, body, done := u.block("while.cond"), u.block("while.body"), u.block("while.done")
	u.fn.cur.Br(cond.To())

	u.fn.cur = cond
	c := u.truth(s.Cond)
	if c == nil {
		return
	}
	u.fn.cur.BrIf(*c, body.To(), done.To())

	u.fn.cur = body
	u.loop(done, cond, s.Body)
	u.fn.cur = done
}

func (u *unit) doStmt(s *ast.DoStmt) {
	body, cond, done := u.block("do.body"), u.block("do.cond"), u.block("do.done")
	u.fn.cur.Br(body.To())

	u.fn.cur = body
	u.loop(done, cond, s.Body)

	u.fn.cur = cond
	c := u.truth(s.Cond)
	if c == nil {
		return
	}
	u.fn.cur.BrIf(*c, body.To(), done.To())
	u.fn.cur = done
}

func (u *unit) forStmt(s *ast.ForStmt) {
	u.push()
	defer u.pop()

	switch init := s.Init.(type) {
	case ast.Decl:
		u.localDecl(init)
	case ast.Expr:
		u.rvalue(init)
	}

	cond, body, post, done :=
		u.block("for.cond"), u.block("for.body"), u.block("for.post"), u.block("for.done")
	u.fn.cur.Br(cond.To())

	u.fn.cur = cond
	if s.Cond != nil {
		c := u.truth(s.Cond)
		if c == nil {
			return
		}
		u.fn.cur.BrIf(*c, body.To(), done.To())
	} else {
		u.fn.cur.Br(body.To())
	}

	u.fn.cur = body
	u.loop(done, post, s.Body)

	u.fn.cur = post
	if s.Post != nil {
		u.rvalue(s.Post)
	}
	if u.at() {
		u.fn.cur.Br(cond.To())
	}
	u.fn.cur = done
}

// loop runs a loop body with its break and continue targets in force.
func (u *unit) loop(brk, cont *ir.Block, body ast.Stmt) {
	u.fn.breaks = append(u.fn.breaks, brk)
	u.fn.continues = append(u.fn.continues, cont)
	u.stmt(body)
	u.fn.breaks = u.fn.breaks[:len(u.fn.breaks)-1]
	u.fn.continues = u.fn.continues[:len(u.fn.continues)-1]
	if u.at() {
		u.fn.cur.Br(cont.To())
	}
}

// switchStmt lowers a switch as a chain of comparisons over the case values.
//
// The cases are collected before anything is emitted, because a case label
// may appear anywhere inside the body — which is what makes Duff's device
// legal and what a lowering that walked the body linearly would get wrong.
func (u *unit) switchStmt(s *ast.SwitchStmt) {
	v := u.rvalue(s.Cond)
	if v == nil {
		return
	}
	sel, ok := v.(ir.I32)
	if !ok {
		w := u.convert(v, u.typeOf(s.Cond), types.Typ(types.Int))
		if sel, ok = w.(ir.I32); !ok {
			u.unsupported(s, "a switch on this type")
			return
		}
	}

	body, ok := s.Body.(*ast.CompoundStmt)
	if !ok {
		u.unsupported(s, "a switch whose body is not a block")
		return
	}
	cases := collectCases(body)
	done := u.block("switch.done")

	blocks := make(map[ast.Stmt]*ir.Block, len(cases))
	var targets []ir.BlockTarget
	var values []int64
	dflt := done
	for _, c := range cases {
		blk := u.block("switch.case")
		blocks[c] = blk
		if c.Kind == token.DEFAULT {
			dflt = blk
			continue
		}
		val, ok := u.info.Consts[c.Value]
		if !ok {
			if val, ok = u.foldInt(c.Value); !ok {
				u.unsupported(c, "a case value that is not constant")
				return
			}
		}
		values = append(values, val)
		targets = append(targets, blk.To())
	}

	// VIR's br_table is dense: it selects by index. A C switch is sparse,
	// so the values are compared one at a time — correct for every shape,
	// and the place a denser lowering would go if the profile ever asked
	// for one.
	cur := u.fn.cur
	for i, val := range values {
		next := u.block("switch.test")
		cur.BrIf(cur.I32.Eq(sel, cur.I32.Const(val)), targets[i], next.To())
		cur = next
	}
	cur.Br(dflt.To())

	// The body is walked once, in order: a case label opens its block and
	// the statements between two labels fall through to the next.
	u.fn.breaks = append(u.fn.breaks, done)
	u.fn.cur = nil
	for _, item := range body.Items {
		if c, ok := item.(*ast.CaseStmt); ok {
			blk := blocks[c]
			if u.at() {
				u.fn.cur.Br(blk.To())
			}
			u.fn.cur = blk
			u.stmt(c.Stmt)
			continue
		}
		u.stmt(item)
	}
	u.fn.breaks = u.fn.breaks[:len(u.fn.breaks)-1]
	if u.at() {
		u.fn.cur.Br(done.To())
	}
	u.fn.cur = done
}

// collectCases finds every case and default label in a switch body, in
// order. It does not descend into a nested switch, whose labels belong to
// that one.
func collectCases(body *ast.CompoundStmt) []*ast.CaseStmt {
	var out []*ast.CaseStmt
	var walk func(ast.Stmt)
	walk = func(s ast.Stmt) {
		switch s := s.(type) {
		case *ast.CaseStmt:
			out = append(out, s)
			walk(s.Stmt)
		case *ast.CompoundStmt:
			for _, item := range s.Items {
				walk(item)
			}
		case *ast.LabeledStmt:
			walk(s.Stmt)
		case *ast.IfStmt:
			walk(s.Then)
			walk(s.Else)
		case *ast.WhileStmt:
			walk(s.Body)
		case *ast.DoStmt:
			walk(s.Body)
		case *ast.ForStmt:
			walk(s.Body)
		}
	}
	for _, item := range body.Items {
		walk(item)
	}
	return out
}

package analyzer

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Statements: the structure checks, and the Objective-C statements of §7.

func (c *checker) checkStmt(s ast.Stmt, pushScope bool) {
	switch s := s.(type) {
	case nil, *ast.BadStmt:
		return

	case *ast.CompoundStmt:
		if pushScope {
			c.push()
			defer c.pop()
		}
		for _, item := range s.Items {
			c.checkStmt(item, true)
		}

	case *ast.DeclStmt:
		c.checkDecl(s.D, false)

	case *ast.ExprStmt:
		c.expr(s.X)

	case *ast.EmptyStmt:

	case *ast.LabeledStmt:
		name := c.name(s.Label)
		if prev := c.labels[name]; prev != nil {
			c.report(s.Label, "duplicate label '"+name+"'")
		} else {
			c.labels[name] = s
		}
		c.checkStmt(s.Stmt, true)

	case *ast.CaseStmt:
		if c.switchD == 0 {
			c.report(s, "'"+s.Kind.String()+"' outside a switch statement")
		}
		if s.Value != nil {
			c.requireConst(s.Value, "case value")
		}
		c.checkStmt(s.Stmt, true)

	case *ast.IfStmt:
		c.push()
		c.requireScalar(s.Cond, c.rvalue(s.Cond), "if")
		c.checkStmt(s.Then, true)
		c.checkStmt(s.Else, true)
		c.pop()

	case *ast.SwitchStmt:
		c.push()
		t := c.rvalue(s.Cond)
		if t != nil && !types.IsInteger(t) {
			c.report(s.Cond, "switch condition is "+t.String()+", which is not an integer type")
		}
		c.switchD++
		c.checkStmt(s.Body, true)
		c.switchD--
		c.pop()

	case *ast.WhileStmt:
		c.push()
		c.requireScalar(s.Cond, c.rvalue(s.Cond), "while")
		c.loopD++
		c.checkStmt(s.Body, true)
		c.loopD--
		c.pop()

	case *ast.DoStmt:
		c.loopD++
		c.checkStmt(s.Body, true)
		c.loopD--
		c.requireScalar(s.Cond, c.rvalue(s.Cond), "while")

	case *ast.ForStmt:
		c.push()
		switch init := s.Init.(type) {
		case ast.Decl:
			c.checkDecl(init, false)
		case ast.Expr:
			c.expr(init)
		}
		if s.Cond != nil {
			c.requireScalar(s.Cond, c.rvalue(s.Cond), "for")
		}
		c.expr(s.Post)
		c.loopD++
		c.checkStmt(s.Body, true)
		c.loopD--
		c.pop()

	case *ast.GotoStmt:
		c.gotos = append(c.gotos, s)

	case *ast.ContinueStmt:
		if c.loopD == 0 {
			c.report(s, "'continue' outside a loop")
		}

	case *ast.BreakStmt:
		if c.loopD == 0 && c.switchD == 0 {
			c.report(s, "'break' outside a loop or switch")
		}

	case *ast.ReturnStmt:
		c.checkReturn(s)

	case *ast.AsmStmt:
		// §7.4's interior is a balanced token sequence whose shape is the
		// target's. Nothing here reads it.

	// ---- Objective-C ----

	case *ast.ForInStmt:
		c.checkForIn(s)

	case *ast.TryStmt:
		c.checkTry(s)

	case *ast.ThrowStmt:
		c.checkThrow(s)

	case *ast.SyncStmt:
		t := c.rvalue(s.X)
		if t != nil && !types.IsObjCObject(t) {
			c.report(s.X, "@synchronized takes an object as its lock, not "+t.String())
		}
		c.checkStmt(s.Body, true)

	case *ast.AutoreleasePoolStmt:
		c.checkStmt(s.Body, true)
	}
}

// checkReturn is C11 §6.8.6.4, and §6.9's block return-type inference.
func (c *checker) checkReturn(s *ast.ReturnStmt) {
	got := c.rvalue(s.Result)

	if c.fnRet == nil {
		// A block whose return type was not written: every return
		// contributes, and they have to agree.
		if s.Result == nil {
			return
		}
		if c.inferred == nil {
			c.inferred = got
			return
		}
		if got != nil && !types.Compatible(c.inferred, got) &&
			!(types.IsObjCObject(c.inferred) && types.IsObjCObject(got)) {
			c.report(s, "the block returns "+c.inferred.String()+
				" elsewhere and "+got.String()+" here; write the return type")
		}
		return
	}
	if types.IsVoid(c.fnRet) {
		// `return f();` where f returns void. §6.8.6.4p1 forbids it and
		// clang accepts it, with a warning, as the extension every C
		// codebase that wraps a void function in another one relies on --
		// <simd/math.h> among them:
		//
		//	static inline void __tg_sincos(simd_float4 x, …) {
		//	  return _simd_sincos_f4(x, sinp, cosp);
		//	}
		//
		// Nothing is returned either way, so there is nothing to get
		// wrong; what would be wrong is refusing to read the header.
		if s.Result != nil && !types.IsVoid(got) {
			c.report(s, "returning a value from something whose return type is void")
		}
		return
	}
	if s.Result == nil {
		c.report(s, "returning nothing from something that returns "+c.fnRet.String())
		return
	}
	c.checkStackBlockReturn(s.Result)
	c.checkAssign(s.Result, c.fnRet, got, "returning")
}

// checkStackBlockReturn is the one thing about blocks that manual retain and
// release cannot be trusted with.
//
// A block literal is an object in the frame that wrote it (§6.9), so
// returning one hands back an address that stops being a block the moment
// the function returns. Under ARC the compiler copies it to the heap on the
// way out and the program is correct; under manual retain and release
// nothing does, and what the caller gets is whatever the next call writes
// over that frame. The failure is at the caller, a long way from the return,
// and the program usually runs for a while first.
//
// clang makes this an error rather than a warning, and so does this: the
// fix is `[^{ … } copy]`, which the program has to write itself.
func (c *checker) checkStackBlockReturn(e ast.Expr) {
	if c.arc() || e == nil {
		return
	}
	// Through parentheses and casts, which change nothing about where the
	// object is. A conditional yielding one on either arm is the same
	// mistake written twice.
	switch x := stripParens(e).(type) {
	case *ast.BlockLit:
		c.report(x, "returning a block that lives on the local stack; "+
			"send it -copy, or compile with -fobjc-arc")
	case *ast.CastExpr:
		c.checkStackBlockReturn(x.X)
	case *ast.CondExpr:
		c.checkStackBlockReturn(x.Then)
		c.checkStackBlockReturn(x.Else)
	}
}

// checkForIn is §7.1's fast enumeration.
//
// The collection is sent -countByEnumeratingWithState:objects:count:, which
// is what NSFastEnumeration declares, so it has to be an object. The loop
// variable is an object pointer, and in the declaration form it is declared
// in a scope of its own.
func (c *checker) checkForIn(s *ast.ForInStmt) {
	c.push()
	defer c.pop()

	var elem types.Type
	switch {
	case s.Decl != nil:
		sp := types.BuildSpecs(c.unit, s.Specs, c)
		t, id := types.BuildDeclarator(c.unit, sp.Type, s.Decl, false, c)
		t = c.ownership(t, false)
		elem = t
		if id != nil {
			c.declare(id, &symbol{kind: symObject, typ: t, node: s})
			c.info.Types[s] = t
		}
	case s.X != nil:
		elem = c.expr(s.X)
	}
	if elem != nil && !types.IsObjCObject(elem) {
		c.report(s, "the loop variable of a fast enumeration is an object pointer, not "+
			elem.String())
	}

	coll := c.rvalue(s.Coll)
	if coll != nil && !types.IsObjCObject(coll) {
		c.report(s.Coll, "fast enumeration takes an object conforming to "+
			"NSFastEnumeration, not "+coll.String())
	}
	c.selector("countByEnumeratingWithState:objects:count:")

	c.loopD++
	c.checkStmt(s.Body, true)
	c.loopD--
}

// checkTry is §7.2's @try.
func (c *checker) checkTry(s *ast.TryStmt) {
	if len(s.Catches) == 0 && s.Finally == nil {
		c.report(s, "@try requires at least one @catch or a @finally")
	}
	c.checkStmt(s.Body, true)

	seenAll := ast.Node(nil)
	for _, cl := range s.Catches {
		c.push()
		if seenAll != nil {
			// A clause after @catch (...) can never run: the one before it
			// catches everything.
			c.warn(cl, "this @catch is unreachable; the @catch (...) before it catches everything")
		}
		if cl.Ellipsis.IsValid() {
			seenAll = cl
		} else if cl.Param != nil {
			sp := types.BuildSpecs(c.unit, cl.Param.Specs, c)
			t, id := types.BuildDeclarator(c.unit, sp.Type, cl.Param.Decl, true, c)
			if !types.IsObjCObject(t) {
				c.report(cl.Param, "@catch takes an object pointer, not "+t.String())
			}
			if id != nil {
				c.declare(id, &symbol{kind: symObject, typ: t, node: cl.Param})
				c.info.Types[cl.Param] = t
			}
		}
		c.inCatch++
		c.checkStmt(cl.Body, false)
		c.inCatch--
		c.pop()
	}
	if s.Finally != nil {
		c.checkStmt(s.Finally.Body, true)
	}
}

// checkThrow is §7.2's @throw. The bare form rethrows what is being handled,
// and is valid only inside a @catch.
func (c *checker) checkThrow(s *ast.ThrowStmt) {
	if s.X == nil {
		if c.inCatch == 0 {
			c.report(s, "'@throw' with no expression rethrows the exception being handled; "+
				"it is valid only inside a @catch")
		}
		return
	}
	t := c.rvalue(s.X)
	if t != nil && !types.IsObjCObject(t) {
		c.report(s.X, "@throw takes an object, not "+t.String())
	}
}

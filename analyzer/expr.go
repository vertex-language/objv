package analyzer

import (
	"strconv"
	"strings"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Expression typing, and the constraints that need it.
//
// The rule this file works to is that a nil type means "not known", and
// nothing is reported about an operand whose type is not known. A checker
// that guesses produces a diagnostic about code that is correct, which is
// worse than saying nothing: the user cannot fix it, and learns to ignore the
// compiler.

// expr types an expression and reports what it can about it.
//
// The type returned is the expression's own — an array is still an array, a
// function still a function. Contexts that perform C11 §6.3.2.1's
// conversions call rvalue instead.
func (c *checker) expr(e ast.Expr) types.Type {
	t := c.expr1(e)
	if t != nil && e != nil {
		c.info.Types[e] = t
	}
	return t
}

func (c *checker) expr1(e ast.Expr) types.Type {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *ast.BadExpr:
		return nil

	case *ast.ParenExpr:
		return c.expr(e.X)

	case *ast.BasicLit:
		return c.literalType(e)

	case *ast.StringLit:
		return c.stringType(e)

	case *ast.Ident:
		return c.identType(e)

	case *ast.GenericExpr:
		return c.genericType(e)

	case *ast.IndexExpr:
		return c.indexType(e)

	case *ast.CallExpr:
		return c.callType(e)

	case *ast.MemberExpr:
		return c.memberType(e)

	case *ast.IncDecExpr:
		// §6.5.2.4 and §6.5.3.1 both want a modifiable lvalue, which is the
		// same requirement an assignment has and is checked the same way:
		// `x++` is `x = x + 1` in everything but the value it yields.
		t := c.expr(e.X)
		c.requireScalar(e, t, e.Op.String())
		if t != nil {
			c.checkModifiable(e.X, t)
		}
		return t

	case *ast.CompoundLit:
		t := c.completeArray(c.typeName(e.Type), e.Init)
		c.info.Types[e.Type] = t
		c.expr(e.Init)
		return t

	case *ast.UnaryExpr:
		return c.unaryType(e)

	case *ast.SizeofExpr:
		if e.Type != nil {
			t := c.typeName(e.Type)
			if o := types.Unqualify(t); o.Kind() == types.ObjectKind {
				// The non-fragile runtime decides an instance's size when
				// the program loads, so there is no number here to give.
				c.report(e, "invalid application of 'sizeof' to the interface type "+
					o.String()+" under the non-fragile runtime")
			}
		} else {
			c.expr(e.X)
		}
		return c.sizeType()

	case *ast.AlignofExpr:
		if e.Type != nil {
			c.typeName(e.Type)
		} else {
			c.quietType(e.X)
		}
		return c.sizeType()

	case *ast.CastExpr:
		return c.castType(e)

	case *ast.BinaryExpr:
		return c.binaryType(e)

	case *ast.CondExpr:
		return c.condType(e)

	case *ast.AssignExpr:
		return c.assignType(e)

	case *ast.StmtExpr:
		return c.stmtExprType(e)

	case *ast.InitList:
		for _, it := range e.Items {
			for _, d := range it.Designators {
				if ix, ok := d.(*ast.IndexDesignator); ok {
					c.expr(ix.Index)
				}
			}
			c.expr(it.Value)
		}
		return nil

	// ---- Objective-C ----

	case *ast.MessageExpr:
		return c.messageType(e)

	case *ast.SuperExpr:
		// §6.3 makes super a receiver and nothing else. Reaching it here
		// means it was written where a value was wanted.
		c.report(e, "'super' is a receiver, not an expression")
		return nil

	case *ast.ClassExpr:
		// A class name with type arguments, which is a receiver too.
		return c.classReceiver(e)

	case *ast.SelectorExpr:
		return c.selectorExprType(e)

	case *ast.ProtocolExpr:
		if e.Name != nil {
			p := c.protocol(c.name(e.Name))
			if !p.Complete {
				c.report(e.Name, "cannot find protocol declaration for '"+p.Name+"'")
			}
		}
		return types.NewObject(c.class("Protocol"))

	case *ast.EncodeExpr:
		// §6.4: the runtime's type string for a type, as a character array.
		// Its length is the encoding's, which lower produces; the type is a
		// char array either way.
		if e.Type != nil {
			c.typeName(e.Type)
		}
		return &types.Array{Elem: types.Typ(types.Char), Form: types.IncompleteArray}

	case *ast.BoxedExpr:
		return c.boxedType(e)

	case *ast.ArrayLit:
		for _, el := range e.Elems {
			c.requireObject(el, c.rvalue(el), "an array literal element")
		}
		return c.cocoaClass(e, "NSArray")

	case *ast.DictLit:
		for _, kv := range e.Pairs {
			c.requireObject(kv.Key, c.rvalue(kv.Key), "a dictionary literal key")
			c.requireObject(kv.Value, c.rvalue(kv.Value), "a dictionary literal value")
		}
		return c.cocoaClass(e, "NSDictionary")

	case *ast.BlockLit:
		return c.blockType(e)

	case *ast.AvailabilityExpr:
		// §6.10: an ordinary primary expression whose value is a boolean.
		return types.Typ(types.Int)
	}
	return nil
}

// rvalue is expr followed by §6.3.2.1's conversions: an array becomes a
// pointer to its first element, a function a pointer to itself.
func (c *checker) rvalue(e ast.Expr) types.Type {
	t := c.expr(e)
	if t == nil {
		return nil
	}
	return types.Decay(t)
}

func (c *checker) sizeType() types.Type { return c.model.SizeType() }

// stmtExprType checks §6.1's statement expression. The value is the last
// statement's, and only an expression statement has one.
func (c *checker) stmtExprType(e *ast.StmtExpr) types.Type {
	if e.Body == nil {
		return types.Typ(types.Void)
	}
	c.push()
	defer c.pop()
	var last types.Type = types.Typ(types.Void)
	for i, item := range e.Body.Items {
		if es, ok := item.(*ast.ExprStmt); ok && i == len(e.Body.Items)-1 {
			last = types.Decay(c.expr(es.X))
			continue
		}
		c.checkStmt(item, true)
	}
	return last
}

// ---- leaves ----

func (c *checker) literalType(n *ast.BasicLit) types.Type {
	text := string(c.unit.Slice(n.Lo, n.Hi))
	rep := func(msg string) { c.report(n, msg) }
	switch n.Kind {
	case token.INT_LIT:
		return DecodeIntConst(text, c.model, rep).Type
	case token.FLOAT_LIT:
		_, t := DecodeFloatConst(text, rep)
		return t
	case token.CHAR_LIT:
		return DecodeCharConst(text, c.model, rep).Type
	case token.BOOL_LIT:
		// §2.3: what YES and NO expand to. BOOL is a typedef from
		// <objc/objc.h>, so the type is whatever that unit declared it as.
		if s := c.lookup("BOOL"); s != nil && s.kind == symTypedef {
			return s.typ
		}
		return types.Typ(types.SChar)
	}
	return nil
}

// stringType is §6.1's StringLiteralSequence. A sequence that begins with
// @"…" denotes a string object; a plain one is an array of characters.
func (c *checker) stringType(n *ast.StringLit) types.Type {
	if n.Object {
		return c.cocoaClass(n, "NSString")
	}
	var sv StringValue
	c.reportOnce(n, func(rep func(string)) {
		sv = DecodeString(c.unit, n, c.model, rep)
	})
	if sv.Elem == nil {
		return nil
	}
	return &types.Array{Elem: sv.Elem, Form: types.FixedArray, Len: int64(len(sv.Data))}
}

// cocoaClass is the type of a literal whose class the language names but the
// compiler does not define: NSString for @"…", NSNumber for @42, NSArray and
// NSDictionary for the collection literals.
//
// The class has to have been declared. A literal is a message send to it —
// +stringWithUTF8String:, +numberWithInt:, +arrayWithObjects:count: — and a
// send to a class this unit has never heard of is a send to nothing, which is
// exactly what clang says about the same program.
func (c *checker) cocoaClass(at ast.Node, name string) types.Type {
	k := c.class(name)
	if !k.Complete {
		c.report(at, "cannot build a literal: the definition of class '"+name+
			"' is not available; import <Foundation/Foundation.h>")
	}
	return types.NewObject(k)
}

// identType resolves an ordinary identifier and reports one that is not
// declared. C11 removed the implicit declaration, so a name with nothing
// behind it is a constraint violation here rather than a link failure later.
func (c *checker) identType(id *ast.Ident) types.Type {
	name := c.name(id)
	if name == "" {
		return nil
	}
	if s := c.lookup(name); s != nil {
		return s.typ
	}
	// A bare class name in expression position is the class object, which is
	// what `[NSString class]` and `NSString.alloc` both start from.
	if k, ok := c.classes[name]; ok {
		return c.classObjectType(k)
	}
	// A builtin is the compiler's own function, and builtin.go says which
	// ones objv has. One it does not have is named here rather than left
	// untyped, because the phases below cannot name it.
	if t := builtinType(name); t != nil {
		return t
	}
	if c.quiet > 0 {
		return nil
	}
	if IsCompilerBuiltin(name) {
		if !c.undeclared[name] {
			c.undeclared[name] = true
			c.report(id, "objv does not implement '"+name+"'")
		}
		return nil
	}
	if !c.undeclared[name] {
		c.undeclared[name] = true
		c.report(id, "'"+name+"' is undeclared")
	}
	return nil
}

// classObjectType is the type of a class name used as a value: a pointer to
// the class object, which is what a class-method send goes to.
func (c *checker) classObjectType(k *types.Class) types.Type {
	return &types.Pointer{Elem: &types.Object{Base: k, Meta: true}}
}

// IsCompilerBuiltin reports whether a name belongs to the compiler rather
// than to any declaration. A program cannot declare one — every spelling is
// reserved — so a name with this shape and nothing behind it is a builtin
// objv has not implemented, which is a different thing to say than that the
// program used an undeclared identifier.
func IsCompilerBuiltin(name string) bool {
	return strings.HasPrefix(name, "__builtin_") ||
		strings.HasPrefix(name, "__sync_") ||
		strings.HasPrefix(name, "__c11_atomic_") ||
		strings.HasPrefix(name, "__atomic_")
}

// genericType is C11 §6.5.1.1: the controlling expression is not evaluated,
// its type selects an association, and the result is that association's
// value.
func (c *checker) genericType(e *ast.GenericExpr) types.Type {
	ctrl := c.rvalue(e.Ctrl)
	var chosen, dflt types.Type
	found := false
	for _, a := range e.Assocs {
		if a.Type == nil {
			dflt = c.expr(a.Value)
			continue
		}
		at := c.typeName(a.Type)
		t := c.expr(a.Value)
		if !found && ctrl != nil && at != nil &&
			types.Compatible(types.Unqualify(at), types.Unqualify(ctrl)) {
			chosen, found = t, true
		}
	}
	if found {
		return chosen
	}
	return dflt
}

// ---- postfix ----

// indexType covers both operators §6.2 overloads onto the bracket: C's
// subscript, and Objective-C's, which is a message send.
func (c *checker) indexType(e *ast.IndexExpr) types.Type {
	x, i := c.rvalue(e.X), c.rvalue(e.Index)
	if o := types.AsObject(x); o != nil {
		return c.subscriptSend(e, x, i, false)
	}
	switch {
	case x != nil && types.IsPointer(x):
		if i != nil && !types.IsInteger(i) {
			c.report(e, "array subscript is "+i.String()+", which is not an integer type")
		}
		return types.AsPointer(x).Elem
	case i != nil && types.IsPointer(i):
		return types.AsPointer(i).Elem
	case x != nil && i != nil:
		c.report(e, "cannot subscript a value of type "+x.String())
	}
	return nil
}

// subscriptSend resolves §6.2's object subscripting: an integral subscript
// is objectAtIndexedSubscript:, an object subscript is
// objectForKeyedSubscript:, and the assigning forms are the setters.
//
// Which one it is comes from the subscript's type, and the method has to
// exist on the receiver — the syntax is sugar for a send, and a send to a
// method nobody declared is what the runtime would fail on.
func (c *checker) subscriptSend(e *ast.IndexExpr, recv, index types.Type, assigning bool) types.Type {
	keyed := index != nil && types.IsObjectPointer(index)
	if index != nil && !keyed && !types.IsInteger(index) {
		c.report(e, "an object subscript is an integer or an object pointer, not "+index.String())
		return nil
	}
	sel := "objectAtIndexedSubscript:"
	if keyed {
		sel = "objectForKeyedSubscript:"
	}
	if assigning {
		sel = "setObject:atIndexedSubscript:"
		if keyed {
			sel = "setObject:forKeyedSubscript:"
		}
	}
	c.selector(sel)
	m := c.lookupMethod(recv, sel, false)
	if m == nil {
		if o := types.AsObject(recv); o != nil && o.Base != nil && o.Base.Complete {
			c.report(e, "'"+o.Base.Name+"' does not implement '"+sel+
				"', which subscripting requires")
		}
		return nil
	}
	if assigning {
		if len(m.Params) > 0 {
			return c.substTypeArgs(m.Params[0].Type, recv)
		}
		return types.Typ(types.Void)
	}
	return c.substInstancetype(c.substTypeArgs(m.Ret, recv), recv)
}

// callType checks C11 §6.5.2.2, and calling a block, which is the same
// production and a different thing.
func (c *checker) callType(e *ast.CallExpr) types.Type {
	fnT := c.rvalue(e.Fun)
	// The one call whose value is known here. Recorded so that lower emits
	// the answer rather than its own conservative one, and so that a header
	// that puts it in a constant context gets the same answer twice.
	if v, ok := c.evalInt(e); ok {
		c.info.Consts[e] = v
	}
	args := make([]types.Type, len(e.Args))
	for i, a := range e.Args {
		args[i] = c.rvalue(a)
	}
	if fnT == nil {
		return nil
	}
	if b := types.AsBlock(fnT); b != nil {
		return c.checkArgs(e, b.Sig, args, "block")
	}
	ft := types.AsFunc(fnT)
	if ft == nil {
		if p := types.AsPointer(fnT); p != nil {
			ft = types.AsFunc(p.Elem)
		}
	}
	if ft == nil {
		c.report(e.Fun, "called object is "+fnT.String()+
			", which is not a function, a block, or a pointer to one")
		return nil
	}
	return c.checkArgs(e, ft, args, "function")
}

func (c *checker) checkArgs(e *ast.CallExpr, ft *types.Func, args []types.Type, what string) types.Type {
	if !ft.Proto {
		return ft.Ret
	}
	np := len(ft.Params)
	if np == 1 && types.IsVoid(ft.Params[0].Type) {
		np = 0
	}
	switch {
	case len(args) < np:
		c.report(e, "too few arguments to "+what+": "+plural(np, "argument")+
			" expected, "+strconv.Itoa(len(args))+" given")
		return ft.Ret
	case len(args) > np && !ft.Variadic:
		c.report(e, "too many arguments to "+what+": "+plural(np, "argument")+
			" expected, "+strconv.Itoa(len(args))+" given")
		return ft.Ret
	}
	for i := 0; i < np && i < len(args); i++ {
		c.checkAssign(e.Args[i], types.AdjustParam(ft.Params[i].Type), args[i],
			"passing argument "+strconv.Itoa(i+1))
	}
	return ft.Ret
}

// memberType resolves the three things §6.2 overloads onto the dot: a
// structure member, a property access on an object, and a class property on
// a class name.
func (c *checker) memberType(e *ast.MemberExpr) types.Type {
	x := c.expr(e.X)
	name := c.name(e.Sel)
	if x == nil {
		return nil
	}

	// Property dot syntax. `obj.name` is a send of the property's getter,
	// and the tree keeps the dot — the rewrite is lower's, and a diagnostic
	// about code the user did not write would be worse than none.
	if o := types.AsObject(x); o != nil && e.Op == token.PERIOD {
		return c.propertyAccess(e, x, o, name)
	}

	base := x
	if e.Op == token.ARROW {
		p := types.AsPointer(types.Decay(x))
		if p == nil {
			c.report(e, "'->' applied to "+x.String()+", which is not a pointer")
			return nil
		}
		base = p.Elem
	} else if types.IsPointer(types.Unqualify(x)) {
		c.report(e, "'.' applied to "+x.String()+"; use '->'")
		return nil
	}

	// An instance variable, reached through self or another object pointer.
	// The base here is what the pointer points at, which is the interface
	// type itself rather than another pointer to one.
	if o, ok := types.Unqualify(base).(*types.Object); ok {
		return c.ivarAccess(e, o, name)
	}
	rec := types.AsRecord(base)
	if rec == nil {
		c.report(e, "member reference base is "+x.String()+", which is not a structure or union")
		return nil
	}
	if !rec.Complete {
		c.report(e, "member access into incomplete type "+rec.String())
		return nil
	}
	t, ok := findMember(rec, name)
	if !ok {
		c.report(e.Sel, "no member named '"+name+"' in "+rec.String())
		return nil
	}
	// A member of a qualified object is qualified: `const struct S s; s.x`
	// is a const int.
	return types.Qualify(t, types.QualsOf(base))
}

// propertyAccess resolves dot syntax on an object pointer.
func (c *checker) propertyAccess(e *ast.MemberExpr, recv types.Type, o *types.Object, name string) types.Type {
	class := o.Meta
	if p := c.findProperty(o, name); p != nil {
		if p.Has(types.PropClass) != class {
			// A class property is reached through the class, an instance
			// property through an instance.
			if class {
				c.report(e, "property '"+name+"' is an instance property")
			} else {
				c.report(e, "property '"+name+"' is a class property")
			}
		}
		c.info.Props[e] = p
		c.selector(p.Getter)
		return c.substInstancetype(c.substTypeArgs(p.Type, recv), recv)
	}
	// Dot syntax also reaches a bare getter: `view.superview` where the
	// method exists and no @property declares it. clang admits it, and the
	// send it becomes is the same one.
	if m := c.lookupMethod(recv, name, class); m != nil && len(m.Params) == 0 {
		c.selector(name)
		return c.substInstancetype(m.Ret, recv)
	}
	if o.Base == nil {
		// A property on id: the runtime decides, and this compiler cannot.
		c.warn(e.Sel, "property '"+name+"' on 'id' is resolved at run time")
		return types.ID()
	}
	c.report(e.Sel, "no property or getter named '"+name+"' on '"+o.String()+"'")
	return nil
}

// ivarAccess resolves `self->_count` and `obj->_count`, and §4.5's
// visibility.
func (c *checker) ivarAccess(e *ast.MemberExpr, o *types.Object, name string) types.Type {
	if o.Base == nil {
		c.report(e.Sel, "cannot access an instance variable through 'id'; "+
			"the class is not known here")
		return nil
	}
	iv, owner := o.Base.FindIvar(name)
	if iv == nil {
		c.report(e.Sel, "'"+o.Base.Name+"' has no instance variable named '"+name+"'")
		return nil
	}
	c.checkIvarAccess(e, iv, owner)
	return iv.Type
}

// checkIvarAccess enforces §4.5's visibility.
func (c *checker) checkIvarAccess(at ast.Node, iv *types.Ivar, owner *types.Class) {
	switch iv.Vis {
	case types.VisPublic, types.VisPackage:
		return
	case types.VisProtected:
		if c.self != nil && c.self.IsSubclassOf(owner) {
			return
		}
		c.report(at, "instance variable '"+iv.Name+"' is @protected; "+
			"it is visible to '"+owner.Name+"' and its subclasses")
	case types.VisPrivate:
		if c.self == owner {
			return
		}
		c.report(at, "instance variable '"+iv.Name+"' is @private to '"+owner.Name+"'")
	}
}

func findMember(r *types.Record, name string) (types.Type, bool) {
	for _, f := range r.Fields {
		if f.Name == name {
			return f.Type, true
		}
	}
	// An anonymous member's members belong to the record containing it.
	for _, f := range r.Fields {
		if f.Name != "" {
			continue
		}
		if inner := types.AsRecord(f.Type); inner != nil {
			if t, ok := findMember(inner, name); ok {
				return t, true
			}
		}
	}
	return nil, false
}

// ---- unary, cast, binary ----

func (c *checker) unaryType(e *ast.UnaryExpr) types.Type {
	switch e.Op {
	case token.AND:
		t := c.expr(e.X)
		if t == nil {
			return nil
		}
		return &types.Pointer{Elem: t}

	case token.MUL:
		t := c.rvalue(e.X)
		if t == nil {
			return nil
		}
		if types.IsBlock(t) {
			// §5.7: a block pointer may not be dereferenced. It points at a
			// closure with a layout the language does not expose.
			c.report(e, "a block pointer cannot be dereferenced")
			return nil
		}
		p := types.AsPointer(t)
		if p == nil {
			c.report(e, "cannot dereference "+t.String())
			return nil
		}
		if types.IsVoid(p.Elem) {
			c.report(e, "cannot dereference a pointer to void")
			return nil
		}
		return p.Elem

	case token.NOT:
		t := c.rvalue(e.X)
		c.requireScalar(e, t, "!")
		return types.Typ(types.Int)

	case token.ADD, token.SUB:
		t := c.rvalue(e.X)
		if t == nil {
			return nil
		}
		if !types.IsArithmetic(t) {
			c.report(e, "invalid operand to unary '"+e.Op.String()+"': "+t.String())
			return nil
		}
		return c.model.Promote(t)

	case token.TILDE:
		t := c.rvalue(e.X)
		if t == nil {
			return nil
		}
		if !types.IsInteger(t) {
			c.report(e, "invalid operand to '~': "+t.String())
			return nil
		}
		return c.model.Promote(t)

	case token.INC, token.DEC:
		t := c.expr(e.X)
		c.requireScalar(e, t, e.Op.String())
		return t
	}
	return nil
}

// castType checks §6.5's cast, including the three bridge casts.
func (c *checker) castType(e *ast.CastExpr) types.Type {
	t := c.typeName(e.Type)
	src := c.rvalue(e.X)
	if t == nil {
		return nil
	}
	if e.IsBridge() {
		c.checkBridgeCast(e, t, src)
		return t
	}
	if src != nil && c.arc() && types.Bridge(t, src) == types.BridgeNeeded {
		// Under ARC a conversion that crosses the boundary has to say what
		// happens to the ownership of the value, and there are three
		// answers. Which one is the program's to give.
		c.report(e, "cast between "+src.String()+" and "+t.String()+
			" requires a bridge cast under ARC: __bridge to transfer nothing, "+
			"__bridge_retained to hand ownership out, __bridge_transfer to take it in")
	}
	return t
}

func (c *checker) binaryType(e *ast.BinaryExpr) types.Type {
	x, y := c.rvalue(e.X), c.rvalue(e.Y)
	if e.Op == token.COMMA {
		return y
	}
	if x == nil || y == nil {
		return nil
	}

	switch e.Op {
	case token.LAND, token.LOR:
		c.requireScalar(e.X, x, e.Op.String())
		c.requireScalar(e.Y, y, e.Op.String())
		return types.Typ(types.Int)

	case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
		c.checkComparison(e, x, y)
		return types.Typ(types.Int)

	case token.ADD, token.SUB:
		// Pointer arithmetic, which an object pointer is not admitted to:
		// the modern runtime forbids it, and that is what leaves the
		// subscript operator free to be a message send (§6.2).
		px, py := types.IsPointer(x) && !types.IsObjectPointer(x), types.IsPointer(y) && !types.IsObjectPointer(y)
		switch {
		case types.IsObjectPointer(x) || types.IsObjectPointer(y):
			c.report(e, "arithmetic on an object pointer is not allowed")
			return nil
		case px && types.IsInteger(y):
			return x
		case types.IsInteger(x) && py && e.Op == token.ADD:
			return y
		case px && py && e.Op == token.SUB:
			return c.model.PtrDiffType()
		case types.IsArithmetic(x) && types.IsArithmetic(y):
			return c.model.Usual(x, y)
		}
		c.report(e, "invalid operands to '"+e.Op.String()+"': "+x.String()+" and "+y.String())
		return nil

	case token.MUL, token.QUO:
		if types.IsArithmetic(x) && types.IsArithmetic(y) {
			return c.model.Usual(x, y)
		}
	case token.REM, token.AND, token.OR, token.XOR, token.SHL, token.SHR:
		if types.IsInteger(x) && types.IsInteger(y) {
			if e.Op == token.SHL || e.Op == token.SHR {
				return c.model.Promote(x) // the left operand's type wins
			}
			return c.model.Usual(x, y)
		}
	}
	c.report(e, "invalid operands to '"+e.Op.String()+"': "+x.String()+" and "+y.String())
	return nil
}

func (c *checker) checkComparison(e *ast.BinaryExpr, x, y types.Type) {
	switch {
	case types.IsArithmetic(x) && types.IsArithmetic(y):
	case types.IsObjCObject(x) && types.IsObjCObject(y):
		// Two object pointers compare by identity, whatever their classes.
	case types.IsObjCObject(x) && c.isNullConst(e.Y), types.IsObjCObject(y) && c.isNullConst(e.X):
	case types.IsPointer(x) && types.IsPointer(y):
	case types.IsPointer(x) && c.isNullConst(e.Y), types.IsPointer(y) && c.isNullConst(e.X):
	default:
		c.report(e, "invalid operands to '"+e.Op.String()+"': "+x.String()+" and "+y.String())
	}
}

func (c *checker) condType(e *ast.CondExpr) types.Type {
	cond := c.rvalue(e.Cond)
	c.requireScalar(e.Cond, cond, "?:")
	t, f := c.rvalue(e.Then), c.rvalue(e.Else)
	if t == nil || f == nil {
		return nil
	}
	switch {
	case types.IsArithmetic(t) && types.IsArithmetic(f):
		return c.model.Usual(t, f)
	case types.IsVoid(t) && types.IsVoid(f):
		return types.Typ(types.Void)
	case types.IsObjCObject(t) && c.isNullConst(e.Else):
		return t
	case types.IsObjCObject(f) && c.isNullConst(e.Then):
		return f
	case types.IsObjCObject(t) && types.IsObjCObject(f):
		// Two object pointers: the common type is the nearest class both
		// descend from, and id where there is none.
		return c.commonObject(t, f)
	case types.Compatible(types.Unqualify(t), types.Unqualify(f)):
		return t
	case types.IsPointer(t) && c.isNullConst(e.Else):
		return t
	case types.IsPointer(f) && c.isNullConst(e.Then):
		return f
	}
	c.report(e, "the two arms of '?:' have incompatible types "+t.String()+" and "+f.String())
	return nil
}

// commonObject is the type two object pointers have in common: the nearest
// class both descend from, or id.
func (c *checker) commonObject(a, b types.Type) types.Type {
	ao, bo := types.AsObject(a), types.AsObject(b)
	if ao == nil || bo == nil || ao.Base == nil || bo.Base == nil {
		return types.ID()
	}
	for k := ao.Base; k != nil; k = k.Super {
		if bo.Base.IsSubclassOf(k) {
			return types.NewObject(k)
		}
	}
	return types.ID()
}

func (c *checker) assignType(e *ast.AssignExpr) types.Type {
	// An assignment to a subscript is the setter, not the getter: §6.2
	// makes `d[k] = v` a send of setObject:forKeyedSubscript:.
	if ix, ok := stripParens(e.Lhs).(*ast.IndexExpr); ok && e.Op == token.ASSIGN {
		recv := c.rvalue(ix.X)
		if types.IsObjectPointer(recv) {
			index := c.rvalue(ix.Index)
			// The type this bracket gets is the setter's parameter, not the
			// getter's return: the subscript is a store here, and the two
			// selectors are unrelated methods that need not agree. Recording
			// it is this path's job because it is the one path that types an
			// IndexExpr without going through expr.
			if t := c.subscriptSend(ix, recv, index, true); t != nil {
				c.info.Types[ix] = t
			}
			return c.rvalue(e.Rhs)
		}
	}
	lhs := c.expr(e.Lhs)
	rhs := c.rvalue(e.Rhs)
	if lhs == nil {
		return nil
	}
	c.checkModifiable(e.Lhs, lhs)
	if e.Op == token.ASSIGN {
		c.checkAssign(e.Rhs, lhs, rhs, "assigning")
		return types.Unqualify(lhs)
	}
	// A compound assignment is the binary operator followed by a simple
	// assignment, and its operands obey the operator's rules.
	if rhs != nil && !types.IsScalar(lhs) {
		c.report(e, "invalid operand to '"+e.Op.String()+"': "+lhs.String())
	}
	return types.Unqualify(lhs)
}

// checkModifiable reports an assignment to something that cannot be assigned
// to: a const object, an array, or an object the language declares read-only.
func (c *checker) checkModifiable(at ast.Node, t types.Type) {
	if e, ok := at.(ast.Expr); ok {
		if c.checkCaptured(e) {
			return
		}
		if what := notLvalue(e); what != "" {
			c.report(e, "cannot assign to "+what+": it is not an lvalue")
			return
		}
	}
	switch {
	case types.QualsOf(t)&types.QConst != 0:
		c.report(at, "cannot assign to a const "+types.Unqualify(t).String())
	case types.IsArray(t):
		c.report(at, "cannot assign to an array")
	}
}

// checkCaptured reports an assignment to a variable the enclosing block
// captured, and says what to do about it.
//
// §6.9: a capture is a copy, made where the literal was written and const
// inside the body. Assigning to one would compile to a store into the block
// literal that the enclosing function never sees, which is a program that
// runs and is wrong — the reason the language makes it a constraint
// violation rather than leaving it to mean something. __block is the way to
// ask for the variable itself.
func (c *checker) checkCaptured(e ast.Expr) bool {
	id, ok := stripParens(e).(*ast.Ident)
	if !ok || len(c.blocks) == 0 {
		return false
	}
	name := c.name(id)
	b := c.blocks[len(c.blocks)-1]
	if !b.seen[name] {
		return false
	}
	for _, cap := range c.info.Captures[b.lit] {
		if cap.Name != name || cap.Block {
			continue
		}
		c.report(e, "cannot assign to '"+name+
			"': a block captures a copy; declare it __block to share it")
		return true
	}
	return false
}

// notLvalue names the expression form when it is certainly not an lvalue,
// and returns "" when it is one or when this cannot tell.
//
// §6.3.2.1p1: an lvalue designates an object. A function call does not —
// C++ made a call returning a class an lvalue and C did not — so `f().x = 1`
// assigns to storage that has no name and no life after the statement. The
// list is deliberately the certain cases only: reporting a real lvalue as
// not one is worse than saying nothing, because the user cannot fix it.
func notLvalue(e ast.Expr) string {
	switch e := stripParens(e).(type) {
	case *ast.CallExpr:
		return "the result of a call"
	case *ast.MessageExpr:
		return "the result of a message send"
	case *ast.CastExpr:
		return "the result of a cast"
	case *ast.BinaryExpr, *ast.CondExpr, *ast.IncDecExpr, *ast.SizeofExpr:
		return "the result of an operator"
	case *ast.BasicLit:
		return "a constant"
	case *ast.MemberExpr:
		// A member of an lvalue is an lvalue and a member through a
		// pointer always is; a member of something that is not is not.
		if e.Op == token.ARROW {
			return ""
		}
		if what := notLvalue(e.X); what != "" {
			return "a member of " + what
		}
	}
	return ""
}

func stripParens(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// checkAssign applies §6.5.16.1's constraints and says which one failed.
func (c *checker) checkAssign(at ast.Node, dst, src types.Type, what string) {
	if dst == nil || src == nil {
		return
	}
	switch types.Assignable(dst, src, c.isNullConst(exprOf(at))) {
	case types.AssignOK:
	case types.AssignDiscardsQuals:
		c.report(at, what+" "+src.String()+" to "+dst.String()+" discards qualifiers")
	case types.AssignIntPointer:
		c.report(at, what+" between "+dst.String()+" and "+src.String()+
			" without a cast: one is a pointer and the other an integer")
	case types.AssignObjCUnrelated:
		c.report(at, what+" "+src.String()+" to "+dst.String()+
			": the classes are unrelated"+downcastHint(dst, src))
	case types.AssignObjCProtocol:
		c.report(at, what+" "+src.String()+" to "+dst.String()+
			": it is not known to conform to "+missingProtocols(dst, src))
	case types.AssignObjCTypeArgs:
		c.report(at, what+" "+src.String()+" to "+dst.String()+
			": the type arguments differ, and the parameter is invariant")
	case types.AssignPointerMismatch:
		c.report(at, what+" "+src.String()+" to "+dst.String()+" without a cast")
	default:
		c.report(at, what+" "+src.String()+" to "+dst.String()+" is not allowed")
	}
}

// downcastHint names the cast a program probably meant, where the relation
// is the right one backwards.
func downcastHint(dst, src types.Type) string {
	d, s := types.AsObject(dst), types.AsObject(src)
	if d == nil || s == nil || d.Base == nil || s.Base == nil {
		return ""
	}
	if d.Base.IsSubclassOf(s.Base) {
		return "; '" + d.Base.Name + "' is a subclass of '" + s.Base.Name +
			"', so this narrows and needs a cast"
	}
	return ""
}

func missingProtocols(dst, src types.Type) string {
	d, s := types.AsObject(dst), types.AsObject(src)
	if d == nil || s == nil {
		return "the required protocols"
	}
	var missing []string
	for _, p := range d.Protocols {
		if !s.Conforms(p) {
			missing = append(missing, "'"+p.Name+"'")
		}
	}
	if len(missing) == 0 {
		return "the required protocols"
	}
	return strings.Join(missing, ", ")
}

func exprOf(n ast.Node) ast.Expr {
	e, _ := n.(ast.Expr)
	return e
}

// isNullConst reports whether e is a null pointer constant: an integer
// constant expression of value zero, or such a constant cast to void *.
func (c *checker) isNullConst(e ast.Expr) bool {
	if e == nil {
		return false
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return c.isNullConst(e.X)
	case *ast.CastExpr:
		t := c.info.Types[e.Type]
		if p := types.AsPointer(t); p != nil && types.IsVoid(p.Elem) {
			return c.isNullConst(e.X)
		}
		return false
	}
	v, ok := c.evalInt(e)
	return ok && v == 0
}

func (c *checker) requireScalar(at ast.Node, t types.Type, what string) {
	if t == nil {
		return
	}
	if !types.IsScalar(types.Decay(t)) {
		c.report(at, "operand of '"+what+"' is "+t.String()+", which is not a scalar type")
	}
}

// requireObject reports a non-object where §6.8's literals take one: every
// element of an array literal and every key and value of a dictionary
// literal is sent a message.
func (c *checker) requireObject(at ast.Node, t types.Type, what string) {
	if t == nil {
		return
	}
	if !types.IsObjCObject(t) {
		c.report(at, what+" is "+t.String()+
			", which is not an object; box it with @(…)")
	}
}

// reportOnce runs f with a reporter that reports at most one diagnostic at n.
func (c *checker) reportOnce(n ast.Node, f func(report func(string))) {
	done := false
	f(func(msg string) {
		if !done {
			done = true
			c.report(n, msg)
		}
	})
}

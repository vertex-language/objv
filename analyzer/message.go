package analyzer

import (
	"strconv"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Message sends (§6.3), which is where the language's dynamism meets what a
// compiler can check.
//
// What is checkable is the shape: the receiver's static type says which
// class or protocol to look the selector up in, and if a method is found its
// signature says how many arguments there are and what they are. What is not
// checkable is a send to `id` — the runtime resolves it, and the compiler's
// honest answer is to say what it does not know rather than guess.

func (c *checker) messageType(e *ast.MessageExpr) types.Type {
	recv, super := c.receiverType(e)
	sel, args := c.selectorOf(e)
	c.selector(sel)

	if recv == nil || sel == "" {
		return nil
	}
	if c.arc() && c.checkARCSelector(e, sel) {
		return types.ID()
	}
	m := c.resolveSend(e, recv, sel, super)
	c.info.Sends[e] = m
	if m == nil {
		return c.unresolvedSend(e, recv, sel)
	}
	if m.Unavailable {
		c.report(e, "'"+sel+"' is unavailable on '"+m.Owner+"'")
	}
	c.checkSendArgs(e, m, args)
	return c.substInstancetype(c.substTypeArgs(m.Ret, recv), recv)
}

// receiverType types the receiver and says whether it was `super`.
//
// super is not a value: it is the same object as self, with the lookup
// starting one class higher. So its type is self's and its meaning is a
// different search, which is what the second return is for.
func (c *checker) receiverType(e *ast.MessageExpr) (types.Type, bool) {
	switch r := e.Recv.(type) {
	case *ast.SuperExpr:
		if c.self == nil {
			c.report(r, "'super' is valid only inside a method")
			return nil, false
		}
		if c.self.Super == nil {
			c.report(r, "'"+c.self.Name+"' is a root class and has no superclass")
			return nil, false
		}
		if c.meth != nil && c.meth.Class {
			return c.classObjectType(c.self.Super), true
		}
		return types.NewObject(c.self.Super), true

	case *ast.ClassExpr:
		return c.classReceiver(r), false
	}
	return c.rvalue(e.Recv), false
}

// classReceiver types §6.3's `ClassName TypeArgumentList` receiver.
func (c *checker) classReceiver(e *ast.ClassExpr) types.Type {
	k := c.class(c.name(e.Name))
	o := &types.Object{Base: k, Meta: true}
	if e.TypeArgs != nil {
		c.checkTypeArgs(&ast.ObjectType{Name: e.Name, TypeArgs: e.TypeArgs}, k)
		for _, a := range e.TypeArgs.Args {
			o.Args = append(o.Args, c.typeName(a))
		}
	}
	return &types.Pointer{Elem: o}
}

// selectorOf builds the selector and types the arguments.
func (c *checker) selectorOf(e *ast.MessageExpr) (string, []types.Type) {
	if e.Sel != nil {
		return c.name(e.Sel), nil
	}
	var pieces []string
	var args []types.Type
	for _, a := range e.Args {
		pieces = append(pieces, c.name(a.Sel))
		for _, v := range a.Vals {
			args = append(args, c.rvalue(v))
		}
	}
	return types.Selector(pieces, true), args
}

// resolveSend looks a selector up against the receiver's static type.
func (c *checker) resolveSend(e *ast.MessageExpr, recv types.Type, sel string, super bool) *types.Method {
	o := types.AsObject(recv)
	if o == nil {
		if types.AsTypeParam(recv) != nil {
			// §5.5's type parameter, erased. `[[array firstObject] foo]` on
			// an unspecialized NSArray sends to ObjectType, which is `id`
			// by the time there is a value: the runtime resolves it, and so
			// does this — which is to say, it does not.
			return nil
		}
		if types.IsBlock(recv) {
			// A block is an object: its first word is an isa, which is why
			// `[^{ … } copy]` is how a block is moved to the heap without
			// ARC, and why a block can be put in an NSArray. Which class it
			// is belongs to libSystem and is not in any header, so the send
			// is the one to an id — resolved by the runtime, unresolvable
			// here, and typed id.
			return nil
		}
		c.report(e, "receiver is "+recv.String()+", which is not an object pointer")
		return nil
	}
	return c.lookupMethod(recv, sel, o.Meta)
}

// lookupMethod searches what the receiver's type knows: the class and its
// superclasses, the protocols it adopts, and the protocols a qualified id
// names.
func (c *checker) lookupMethod(recv types.Type, sel string, class bool) *types.Method {
	o := types.AsObject(recv)
	if o == nil {
		return nil
	}
	if o.Base != nil {
		if m := o.Base.Lookup(sel, class); m != nil {
			return m
		}
		// A class object also responds to what NSObject declares as an
		// instance method — +alloc's receiver is a class and -class's is
		// not — but only the metaclass chain is searched for class methods,
		// which Lookup already did.
	}
	for _, p := range o.Protocols {
		if m := p.Lookup(sel, class); m != nil {
			return m
		}
	}
	return nil
}

// unresolvedSend reports a send whose method was not found, or says nothing
// where nothing can be said.
func (c *checker) unresolvedSend(e *ast.MessageExpr, recv types.Type, sel string) types.Type {
	o := types.AsObject(recv)
	switch {
	case o == nil && (types.IsBlock(recv) || types.AsTypeParam(recv) != nil):
		// A send to a block or to an erased type parameter, which the
		// runtime resolves like any send to id.
		return types.ID()

	case o == nil:
		return nil

	case o.Base != nil && o.Base.Complete:
		what := "instance method"
		if o.Meta {
			what = "class method"
		}
		c.report(e, "no "+what+" '"+sel+"' on '"+o.Base.Name+"'")
		if alt := c.similarSelector(o.Base, sel, o.Meta); alt != "" {
			c.note(e, "did you mean '"+alt+"'?")
		}
		return types.ID()

	case o.Base != nil:
		// The class is a forward declaration: the interface is not here to
		// look in.
		c.report(e, "receiver '"+o.Base.Name+"' is a forward declaration; "+
			"import the header that defines it")
		return types.ID()

	case len(o.Protocols) > 0:
		c.report(e, "no method '"+sel+"' declared by "+o.String())
		return types.ID()
	}

	// A send to id. The runtime resolves it, and so may this program — but
	// a selector nothing in the translation unit declares is almost always
	// a misspelling, and it is the one thing worth saying.
	if !c.knownSelector(sel) {
		c.warn(e, "no method '"+sel+"' is declared anywhere in this unit; "+
			"the send is resolved at run time")
	}
	return types.ID()
}

// knownSelector reports whether any class or protocol in the unit declares
// the selector.
func (c *checker) knownSelector(sel string) bool {
	for _, k := range c.classOrder {
		for _, m := range k.Methods {
			if m.Sel == sel {
				return true
			}
		}
	}
	for _, p := range c.protocolOrder {
		for _, m := range p.Methods {
			if m.Sel == sel {
				return true
			}
		}
	}
	return false
}

// similarSelector finds a selector on the receiver that differs from the one
// written only by case, which is the misspelling that actually happens.
func (c *checker) similarSelector(k *types.Class, sel string, class bool) string {
	want := lower(sel)
	for x := k; x != nil; x = x.Super {
		for _, m := range x.Methods {
			if m.Class == class && m.Sel != sel && lower(m.Sel) == want {
				return m.Sel
			}
		}
	}
	return ""
}

func lower(s string) string {
	b := []byte(s)
	for i, ch := range b {
		if ch >= 'A' && ch <= 'Z' {
			b[i] = ch + ('a' - 'A')
		}
	}
	return string(b)
}

// checkSendArgs checks the arguments against the method's parameters.
//
// The count cannot disagree — the selector is built from the colons, so a
// send with the wrong number of arguments names a different selector and was
// never going to resolve to this method. What is checked is the types, and
// the trailing arguments a variadic method takes.
func (c *checker) checkSendArgs(e *ast.MessageExpr, m *types.Method, args []types.Type) {
	i := 0
	for _, a := range e.Args {
		for _, v := range a.Vals {
			if i >= len(m.Params) {
				if !m.Variadic {
					c.report(v, "too many arguments to '"+m.Sel+"': "+
						plural(len(m.Params), "argument")+" expected")
					return
				}
				i++
				continue
			}
			if i < len(args) && args[i] != nil {
				want := c.substTypeArgs(m.Params[i].Type, c.info.Types[e.Recv])
				c.checkAssign(v, types.AdjustParam(want), args[i],
					"passing argument "+strconv.Itoa(i+1)+" of '"+m.Sel+"'")
			}
			i++
		}
	}
}

// substInstancetype resolves §5.4's instancetype to what the receiver is.
//
// It is the reason `[[NSMutableString alloc] init]` is an NSMutableString
// rather than an id: -init is declared on NSObject and returns instancetype,
// and the receiver decides what that means at each send.
func (c *checker) substInstancetype(t types.Type, recv types.Type) types.Type {
	o := types.AsObject(t)
	if o == nil || !o.Instancetype {
		return t
	}
	r := types.AsObject(recv)
	if r == nil {
		return types.ID()
	}
	if r.Base == nil {
		return types.ID()
	}
	// A class-method send yields an instance of the class, not the class.
	return types.NewObject(r.Base, r.Protocols...)
}

// checkARCSelector refuses the memory-management messages ARC owns.
//
// Under ARC the compiler inserts retains and releases where the ownership
// rules say they belong; a program that also sends them would be
// double-counting, and there is no way to reconcile the two. -dealloc is
// refused as a send for the same reason and allowed as an override, which is
// where a class still frees what it owns by hand.
// It reports whether the send was refused, so that nothing further is said
// about a send that is not going to happen.
func (c *checker) checkARCSelector(e *ast.MessageExpr, sel string) bool {
	switch sel {
	case "retain", "release", "autorelease", "retainCount":
		c.report(e, "ARC forbids sending '"+sel+"'; it manages the retain count")
		return true
	case "dealloc":
		c.report(e, "ARC forbids sending 'dealloc' explicitly")
		return true
	}
	return false
}

// substTypeArgs replaces a generic class's type parameters with the
// arguments the receiver was specialized with.
//
// It is what makes `arr[0]` an NSString rather than an ObjectType:
// -objectAtIndexedSubscript: is declared on NSArray<ObjectType> and returns
// the parameter, and the receiver says what the parameter is. Where the
// receiver is unspecialized there is nothing to substitute and the parameter
// erases to its bound, which is what the runtime does too.
func (c *checker) substTypeArgs(t types.Type, recv types.Type) types.Type {
	o := types.AsObject(recv)
	if o == nil || o.Base == nil || len(o.Args) == 0 || t == nil {
		return t
	}
	return substParams(t, o.Base.TypeParams, o.Args)
}

func substParams(t types.Type, params []*types.TypeParam, args []types.Type) types.Type {
	if t == nil {
		return nil
	}
	if p := types.AsTypeParam(t); p != nil {
		for i, q := range params {
			if q == p && i < len(args) {
				// The qualifiers written on the parameter stay: a
				// `__weak T` is weak whatever T turns out to be.
				return carryQuals(t, args[i])
			}
		}
		return t
	}
	switch u := types.Unqualify(t).(type) {
	case *types.Pointer:
		if e := substParams(u.Elem, params, args); e != u.Elem {
			return carryQuals(t, &types.Pointer{Elem: e})
		}
	case *types.Object:
		if len(u.Args) == 0 {
			return t
		}
		out := &types.Object{Base: u.Base, Meta: u.Meta,
			Instancetype: u.Instancetype, Protocols: u.Protocols}
		changed := false
		for _, a := range u.Args {
			s := substParams(a, params, args)
			changed = changed || s != a
			out.Args = append(out.Args, s)
		}
		if changed {
			return carryQuals(t, out)
		}
	case *types.Block:
		// A block's signature may name the parameter too, which is what
		// every enumeration method's completion handler does.
		sig := &types.Func{Ret: substParams(u.Sig.Ret, params, args),
			Variadic: u.Sig.Variadic, Proto: u.Sig.Proto}
		changed := sig.Ret != u.Sig.Ret
		for _, p := range u.Sig.Params {
			s := substParams(p.Type, params, args)
			changed = changed || s != p.Type
			sig.Params = append(sig.Params, types.Param{Name: p.Name, Type: s})
		}
		if changed {
			return carryQuals(t, &types.Block{Sig: sig})
		}
	}
	return t
}

// carryQuals gives a substituted type the qualifiers the parameter carried.
func carryQuals(from, to types.Type) types.Type {
	to = types.Qualify(to, types.QualsOf(from))
	to = types.WithLifetime(to, types.LifetimeOf(from))
	return types.WithNullability(to, types.NullabilityOf(from))
}

// selectorExprType is §6.4's @selector, whose value is a SEL and whose
// operand is a name rather than an expression.
func (c *checker) selectorExprType(e *ast.SelectorExpr) types.Type {
	var pieces []string
	keyword := false
	for _, p := range e.Parts {
		pieces = append(pieces, c.name(p.Name))
		if p.Colon.IsValid() {
			keyword = true
		}
	}
	sel := types.Selector(pieces, keyword)
	c.selector(sel)
	if sel != "" && !c.knownSelector(sel) {
		// A selector nothing declares is a string the runtime will look up
		// and fail to find. clang warns here too, and for the same reason.
		c.warn(e, "no method '"+sel+"' is declared in this unit")
	}
	if s := c.lookup("SEL"); s != nil && s.kind == symTypedef {
		return s.typ
	}
	return types.ID()
}

// boxedType is §6.8's boxed expression: @42, @'c', @YES, @(expr).
//
// What class it builds is the operand's business — a number goes to
// NSNumber, a C string to NSString — so the operand is typed first and the
// class chosen from it.
func (c *checker) boxedType(e *ast.BoxedExpr) types.Type {
	t := c.rvalue(e.X)
	if t == nil {
		return nil
	}
	switch {
	case types.IsArithmetic(t):
		return c.cocoaClass(e, "NSNumber")
	case types.IsObjCObject(t):
		// @(anObject) is the object itself, which is what boxing an
		// expression of object type means.
		return t
	}
	if p := types.AsPointer(t); p != nil {
		u := types.Unqualify(p.Elem)
		if b, ok := u.(*types.Basic); ok && (b.K == types.Char || b.K == types.SChar || b.K == types.UChar) {
			// A C string boxes to an NSString.
			return c.cocoaClass(e, "NSString")
		}
	}
	if r := types.AsRecord(t); r != nil {
		// A struct boxes to an NSValue, which is what @(CGPoint) is for.
		return c.cocoaClass(e, "NSValue")
	}
	c.report(e, "cannot box a value of type "+t.String())
	return nil
}

// blockType checks §6.9's block literal and gives it its type.
//
// The body is checked in a scope of its own with the parameters declared,
// and the return type is the one written or the one the returns agree on —
// void where there are none, which is §6.9's rule.
func (c *checker) blockType(e *ast.BlockLit) types.Type {
	sig := &types.Func{Proto: true, Variadic: e.Ellipsis.IsValid()}
	if e.Type != nil {
		sig.Ret = c.typeName(e.Type)
	}

	c.push()
	// Everything the body looks up from here is either the block's own or a
	// capture, and the depth this scope sits at is what tells them apart.
	c.blocks = append(c.blocks, &blockScope{lit: e, base: len(c.scopes) - 1, seen: map[string]bool{}})
	defer func() { c.blocks = c.blocks[:len(c.blocks)-1] }()
	for _, p := range e.Params {
		sp := types.BuildSpecs(c.unit, p.Specs, c)
		t, id := types.BuildDeclarator(c.unit, sp.Type, p.Decl, true, c)
		t = types.AdjustParam(t)
		if id != nil {
			c.declare(id, &symbol{kind: symObject, typ: t, node: p})
		}
		if types.IsVoid(t) && len(e.Params) == 1 && id == nil {
			continue // (void): no parameters
		}
		sig.Params = append(sig.Params, types.Param{Name: c.name(id), Type: t})
		c.info.Types[p] = t
	}

	// A block returns from itself, not from the function containing it, so
	// the return type in force is the block's for the length of its body.
	prevRet, prevInferred := c.fnRet, c.inferred
	c.fnRet, c.inferred = sig.Ret, nil
	c.declareFuncNameText(e, "<block>")
	if e.Body != nil {
		c.checkStmt(e.Body, false)
	}
	if sig.Ret == nil {
		// §6.9: inferred from the return statements, and void where there
		// are none.
		sig.Ret = c.inferred
		if sig.Ret == nil {
			sig.Ret = types.Typ(types.Void)
		}
	}
	c.fnRet, c.inferred = prevRet, prevInferred
	c.pop()

	return &types.Block{Sig: sig}
}

package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Dot syntax and object subscripting: the two places where an expression
// that looks like C is a message.
//
// Neither is an lvalue. `obj.name = x` is a call to setName:, so it has no
// address, cannot be the operand of &, and reads and writes through two
// different selectors — which is why they are handled here as a pair of
// sends rather than in lvalue.go as an address.

// objcRef is an access that is really a send: the receiver, and the two
// selectors that read and write it.
type objcRef struct {
	recv  ir.Value
	super bool

	get string
	set string

	// index is the subscript, for the object-subscript forms. It is a
	// second argument to both selectors and is evaluated once, before
	// either send — which matters for `a[i++] += 1`.
	index ir.Value

	typ types.Type // what the getter returns
	at  ast.Node
}

// propertyRef resolves dot syntax the analyzer recorded as a property.
func (u *unit) propertyRef(e *ast.MemberExpr) *objcRef {
	p := u.info.Props[e]
	if p == nil {
		return nil
	}
	recv, super := u.recvOf(e.X)
	if recv == nil {
		return nil
	}
	return &objcRef{recv: *recv, super: super, get: p.Getter, set: p.Setter,
		typ: p.Type, at: e}
}

// subscriptRef resolves `a[i]` where a is an object.
//
// Which pair of selectors it is depends on the *index*, not on the receiver:
// an integral index is indexed subscripting and anything else is keyed, and
// a class that implements neither is an error the analyzer already gave.
func (u *unit) subscriptRef(e *ast.IndexExpr, t types.Type) *objcRef {
	if !types.IsObjectPointer(u.typeOf(e.X)) {
		return nil
	}
	recv := u.rvalue(e.X)
	idx := u.rvalue(e.Index)
	if recv == nil || idx == nil {
		return nil
	}
	get, set := "objectForKeyedSubscript:", "setObject:forKeyedSubscript:"
	if types.IsInteger(u.typeOf(e.Index)) {
		get, set = "objectAtIndexedSubscript:", "setObject:atIndexedSubscript:"
		n := u.toI64(idx)
		if n == nil {
			return nil
		}
		idx = *n
	}
	return &objcRef{recv: recv, get: get, set: set, index: idx, typ: t, at: e}
}

// load reads through the getter.
func (r *objcRef) load(u *unit) ir.Value {
	var args []ir.Value
	if r.index != nil {
		args = append(args, r.index)
	}
	return u.send(r.recv, r.super, r.get, args, r.typ, r.at)
}

// store writes through the setter and yields the value stored, which is what
// an assignment expression's own value is.
//
// The setter's argument order is not the getter's: setObject:atIndexedSubscript:
// takes the object first and the index second, where the getter took only the
// index. That is the selector's shape and not a choice this makes.
func (r *objcRef) store(u *unit, v ir.Value) ir.Value {
	args := []ir.Value{v}
	if r.index != nil {
		args = append(args, r.index)
	}
	u.send(r.recv, r.super, r.set, args, types.Typ(types.Void), r.at)
	return v
}

// objcAssign lowers an assignment whose left side is a property or an object
// subscript. It reports whether it handled the expression.
func (u *unit) objcAssign(e *ast.AssignExpr, t types.Type) (ir.Value, bool) {
	var ref *objcRef
	switch lhs := stripParens(e.Lhs).(type) {
	case *ast.MemberExpr:
		ref = u.propertyRef(lhs)
	case *ast.IndexExpr:
		ref = u.subscriptRef(lhs, u.typeOf(lhs))
	}
	if ref == nil {
		return nil, false
	}
	if ref.set == "" {
		u.errorf(e, "this property is read-only")
		return nil, true
	}

	if e.Op == token.ASSIGN {
		v := u.rvalue(e.Rhs)
		if v == nil {
			return nil, true
		}
		return ref.store(u, u.convert(v, u.typeOf(e.Rhs), ref.typ)), true
	}

	// A compound assignment is the getter, the operator, and the setter.
	// The receiver and the index were evaluated once, above, which is the
	// whole reason this goes through objcRef rather than lowering the
	// syntax twice.
	old := ref.load(u)
	rhs := u.rvalue(e.Rhs)
	if old == nil || rhs == nil {
		return nil, true
	}
	res := u.arith(compoundOp(e.Op), old, u.convert(rhs, u.typeOf(e.Rhs), ref.typ),
		u.signed(ref.typ))
	if res == nil {
		u.unsupported(e, "this compound assignment to a property")
		return nil, true
	}
	return ref.store(u, u.convert(res, ref.typ, t)), true
}

// objcIncDec lowers ++ and -- on a property or an object subscript.
func (u *unit) objcIncDec(x ast.Expr, op token.Kind, post bool) (ir.Value, bool) {
	var ref *objcRef
	switch e := stripParens(x).(type) {
	case *ast.MemberExpr:
		ref = u.propertyRef(e)
	case *ast.IndexExpr:
		ref = u.subscriptRef(e, u.typeOf(e))
	}
	if ref == nil {
		return nil, false
	}
	old := ref.load(u)
	if old == nil {
		return nil, true
	}
	one := u.constOf(1, ref.typ)
	arith := token.ADD
	if op == token.DEC {
		arith = token.SUB
	}
	next := u.arith(arith, old, one, u.signed(ref.typ))
	if next == nil {
		u.unsupported(x, "++ or -- on a property of type "+ref.typ.String())
		return nil, true
	}
	ref.store(u, next)
	if post {
		return old, true
	}
	return next, true
}

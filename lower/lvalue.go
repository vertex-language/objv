package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Addresses.
//
// lvalue answers where an object is, and what type is there. The two go
// together because the address alone does not say how wide the object is,
// and every caller that has an address wants to load or store through it.
//
// A nil address means "already reported".

func (u *unit) lvalue(e ast.Expr) (*ir.Ptr, types.Type) {
	if e == nil || !u.at() {
		return nil, nil
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return u.lvalue(e.X)

	case *ast.Ident:
		return u.identAddr(e)

	case *ast.UnaryExpr:
		if e.Op == token.MUL {
			v := u.rvalue(e.X)
			p, ok := v.(ir.Ptr)
			if !ok {
				return nil, nil
			}
			t := u.typeOf(e)
			return &p, t
		}

	case *ast.IndexExpr:
		return u.indexAddr(e)

	case *ast.MemberExpr:
		return u.memberAddr(e)

	case *ast.StringLit:
		v := u.stringLit(e)
		if p, ok := v.(ir.Ptr); ok {
			return &p, u.typeOf(e)
		}
		return nil, nil
	}

	// An aggregate rvalue has an address, because that is what an aggregate
	// is here: a call or a send that returns a struct wrote it into storage
	// the caller allocated, and the value of the expression is that
	// storage. §6.5.2.3 does not make it an lvalue — `f().x = 1` is still
	// not assignable, which the analyzer says — but `f().x` has to read
	// from somewhere, and this is where.
	if t := u.typeOf(e); isAggregate(t) {
		if p, ok := u.rvalue(e).(ir.Ptr); ok {
			return &p, t
		}
		return nil, nil
	}
	u.unsupported(e, "taking the address of this expression")
	return nil, nil
}

func (u *unit) identAddr(id *ast.Ident) (*ir.Ptr, types.Type) {
	name := u.name(id)
	st := u.lookup(name)
	if st == nil {
		u.errorf(id, "internal: '"+name+"' reached lowering undeclared")
		return nil, nil
	}
	switch st.kind {
	case stLocal:
		p := st.addr
		return &p, st.typ
	case stGlobal, stFunc:
		p := u.fn.cur.Ptr.GetAddr(u.symOf(st))
		return &p, st.typ
	case stIvar:
		p := u.ivarAddr(st.class, st.ivar)
		return p, st.typ
	}
	u.errorf(id, "'"+name+"' has no address")
	return nil, nil
}

// indexAddr is a[i], for a pointer or an array. The object-subscript form is
// a message send and never reaches here.
func (u *unit) indexAddr(e *ast.IndexExpr) (*ir.Ptr, types.Type) {
	xt, it := u.typeOf(e.X), u.typeOf(e.Index)
	base, idx := e.X, e.Index
	bt := xt
	if types.IsInteger(xt) {
		base, idx, bt = e.Index, e.X, it
	}

	var p ir.Ptr
	var elem types.Type
	switch {
	case types.IsArray(bt):
		addr, at := u.lvalue(base)
		if addr == nil {
			return nil, nil
		}
		p, elem = *addr, types.AsArray(at).Elem
	case types.IsPointer(bt):
		v := u.rvalue(base)
		ptr, ok := v.(ir.Ptr)
		if !ok {
			return nil, nil
		}
		p, elem = ptr, types.AsPointer(bt).Elem
	default:
		return nil, nil
	}

	n := u.rvalue(idx)
	off := u.toI64(n)
	if off == nil {
		return nil, nil
	}
	size, _ := u.sizeAlign(elem)
	b := u.fn.cur
	at := b.Ptr.Add(p, b.I64.Mul(*off, b.I64.Const(int64(size))))
	return &at, elem
}

// memberAddr is x.m, p->m, and an instance variable reached through either.
func (u *unit) memberAddr(e *ast.MemberExpr) (*ir.Ptr, types.Type) {
	// Property dot syntax is a message send, and a send has no address:
	// reading and writing one go through prop.go, and anything that got
	// here wanted the address of a value that is not in memory.
	if p := u.info.Props[e]; p != nil {
		u.errorf(e, "a property is not an lvalue: %s is read through %s",
			p.Name, p.Getter)
		return nil, nil
	}

	name := u.name(e.Sel)
	xt := u.typeOf(e.X)

	// An object pointer: the member is an instance variable, and its offset
	// is a global the runtime writes.
	if o := types.AsObject(xt); o != nil {
		recv := u.rvalue(e.X)
		p, ok := recv.(ir.Ptr)
		if !ok || o.Base == nil {
			return nil, nil
		}
		iv, owner := o.Base.FindIvar(name)
		if iv == nil {
			return nil, nil
		}
		addr := u.ivarAddrOf(p, owner.Name, name)
		return &addr, iv.Type
	}

	var base ir.Ptr
	var rec *types.Record
	if e.Op == token.ARROW {
		v := u.rvalue(e.X)
		p, ok := v.(ir.Ptr)
		if !ok {
			return nil, nil
		}
		base = p
		if pt := types.AsPointer(xt); pt != nil {
			rec = types.AsRecord(pt.Elem)
		}
	} else {
		addr, at := u.lvalue(e.X)
		if addr == nil {
			return nil, nil
		}
		base, rec = *addr, types.AsRecord(at)
	}
	if rec == nil {
		return nil, nil
	}
	off, ok := u.model.Offsetof(rec, name)
	if !ok {
		// A bit-field has no address, and an anonymous member's member is
		// reached through the offsets of each step — which the model
		// already adds up when it can.
		u.unsupported(e, "reading a bit-field")
		return nil, nil
	}
	b := u.fn.cur
	at := b.Ptr.Add(base, b.I64.Const(off))
	ft, _ := findField(rec, name)
	return &at, ft
}

func findField(r *types.Record, name string) (types.Type, bool) {
	for _, f := range r.Fields {
		if f.Name == name {
			return f.Type, true
		}
	}
	for _, f := range r.Fields {
		if f.Name != "" {
			continue
		}
		if inner := types.AsRecord(f.Type); inner != nil {
			if t, ok := findField(inner, name); ok {
				return t, true
			}
		}
	}
	return nil, false
}

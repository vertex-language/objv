package lower

// §7.17's read-modify-write operations, which objv's own <stdatomic.h> is
// written in terms of.
//
// They are not ordinary builtins because they have no signature: the type is
// the pointee of the first argument, and one name serves every width. So the
// analyzer types them from the operand (see AtomicBuiltins) and this picks
// the VIR verb from the same place.
//
// Every one of them is emitted sequentially consistent. C11 lets a program
// ask for less and objv's header already discards the request — the
// _explicit forms evaluate their ordering argument and then hand it to the
// plain one — so the ordering is the strongest rather than a guess at what
// was meant. Strengthening is always correct and never surprising; weakening
// is neither.

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// atomicBuiltin lowers one of them, and reports whether the name was one.
func (u *unit) atomicBuiltin(name string, e *ast.CallExpr) (ir.Value, bool) {
	op, ok := analyzer.AtomicBuiltins[name]
	if !ok {
		return nil, false
	}
	if op == "cas" {
		return u.atomicCas(e), true
	}
	if len(e.Args) != 2 {
		u.errorf(e, "internal: %s takes two arguments", name)
		return nil, true
	}
	addr, elem, ok := u.atomicTarget(e, e.Args[0])
	if !ok {
		return nil, true
	}
	v := u.convert(u.rvalue(e.Args[1]), u.typeOf(e.Args[1]), elem)
	if v == nil {
		return nil, true
	}
	b := u.fn.cur
	switch x := v.(type) {
	case ir.I32:
		return u.atomicRmw32(op, x, addr, elem), true
	case ir.I64:
		switch op {
		case "xchg":
			return b.I64.AtomicRmwXchg(x, addr, ir.SeqCst), true
		case "add":
			return b.I64.AtomicRmwAdd(x, addr, ir.SeqCst), true
		case "sub":
			return b.I64.AtomicRmwSub(x, addr, ir.SeqCst), true
		case "and":
			return b.I64.AtomicRmwAnd(x, addr, ir.SeqCst), true
		case "or":
			return b.I64.AtomicRmwOr(x, addr, ir.SeqCst), true
		case "xor":
			return b.I64.AtomicRmwXor(x, addr, ir.SeqCst), true
		}
	}
	u.unsupported(e, "an atomic operation on "+elem.String())
	return nil, true
}

// atomicRmw32 is the i32 half, which is also where the sub-word widths live:
// a one- or two-byte atomic is an i32 operation naming its access width.
func (u *unit) atomicRmw32(op string, v ir.I32, addr ir.Ptr, elem types.Type) ir.Value {
	b := u.fn.cur
	size, _ := u.sizeAlign(elem)
	switch size {
	case 1:
		switch op {
		case "xchg":
			return b.I32.AtomicRmwXchg8(v, addr, ir.SeqCst)
		case "add":
			return b.I32.AtomicRmwAdd8(v, addr, ir.SeqCst)
		case "sub":
			return b.I32.AtomicRmwSub8(v, addr, ir.SeqCst)
		case "and":
			return b.I32.AtomicRmwAnd8(v, addr, ir.SeqCst)
		case "or":
			return b.I32.AtomicRmwOr8(v, addr, ir.SeqCst)
		case "xor":
			return b.I32.AtomicRmwXor8(v, addr, ir.SeqCst)
		}
	case 2:
		switch op {
		case "xchg":
			return b.I32.AtomicRmwXchg16(v, addr, ir.SeqCst)
		case "add":
			return b.I32.AtomicRmwAdd16(v, addr, ir.SeqCst)
		case "sub":
			return b.I32.AtomicRmwSub16(v, addr, ir.SeqCst)
		case "and":
			return b.I32.AtomicRmwAnd16(v, addr, ir.SeqCst)
		case "or":
			return b.I32.AtomicRmwOr16(v, addr, ir.SeqCst)
		case "xor":
			return b.I32.AtomicRmwXor16(v, addr, ir.SeqCst)
		}
	}
	switch op {
	case "xchg":
		return b.I32.AtomicRmwXchg(v, addr, ir.SeqCst)
	case "add":
		return b.I32.AtomicRmwAdd(v, addr, ir.SeqCst)
	case "sub":
		return b.I32.AtomicRmwSub(v, addr, ir.SeqCst)
	case "and":
		return b.I32.AtomicRmwAnd(v, addr, ir.SeqCst)
	case "or":
		return b.I32.AtomicRmwOr(v, addr, ir.SeqCst)
	}
	return b.I32.AtomicRmwXor(v, addr, ir.SeqCst)
}

// atomicCas is §7.17.7.4, whose contract has two halves: it reports whether
// the exchange happened, and on failure it writes the value it actually
// found through the expected pointer — so a retry loop does not have to read
// the object again, and would race if it did.
func (u *unit) atomicCas(e *ast.CallExpr) ir.Value {
	if len(e.Args) != 3 {
		u.errorf(e, "internal: a compare-exchange takes three arguments")
		return nil
	}
	addr, elem, ok := u.atomicTarget(e, e.Args[0])
	if !ok {
		return nil
	}
	expAddr, _ := u.lvalueOf(e.Args[1])
	if expAddr == nil {
		u.unsupported(e, "a compare-exchange whose expected value has no address")
		return nil
	}
	want := u.loadFrom(*expAddr, elem)
	next := u.convert(u.rvalue(e.Args[2]), u.typeOf(e.Args[2]), elem)
	if want == nil || next == nil {
		return nil
	}

	b := u.fn.cur
	var found, hit ir.Value
	switch w := want.(type) {
	case ir.I32:
		n, _ := next.(ir.I32)
		got := b.I32.AtomicCas(w, n, addr, ir.SeqCst, ir.SeqCst)
		found, hit = got, b.I32.Eq(got, w)
	case ir.I64:
		n, _ := next.(ir.I64)
		got := b.I64.AtomicCas(w, n, addr, ir.SeqCst, ir.SeqCst)
		found, hit = got, b.I64.Eq(got, w)
	default:
		u.unsupported(e, "a compare-exchange on "+elem.String())
		return nil
	}

	// The write-back. Unconditional: on success the value found is the
	// value expected, so storing it changes nothing, and a branch to avoid
	// a store of the same bytes would cost more than it saved.
	u.storeTo(*expAddr, found, elem)
	return hit
}

// atomicTarget is the object an atomic operation names: its address and the
// type it holds.
func (u *unit) atomicTarget(e ast.Node, arg ast.Expr) (ir.Ptr, types.Type, bool) {
	pt := types.AsPointer(types.Unqualify(u.typeOf(arg)))
	if pt == nil {
		u.errorf(e, "internal: an atomic operation's object is not a pointer")
		return ir.Ptr{}, nil, false
	}
	v := u.rvalue(arg)
	p, ok := v.(ir.Ptr)
	if !ok {
		return ir.Ptr{}, nil, false
	}
	return p, types.Unqualify(pt.Elem), true
}

// lvalueOf is lvalue over an expression that is a pointer *to* the place —
// `&expected` was written by the caller, so what arrives is the address and
// not the object.
func (u *unit) lvalueOf(arg ast.Expr) (*ir.Ptr, types.Type) {
	pt := types.AsPointer(types.Unqualify(u.typeOf(arg)))
	if pt == nil {
		return nil, nil
	}
	v := u.rvalue(arg)
	p, ok := v.(ir.Ptr)
	if !ok {
		return nil, nil
	}
	return &p, types.Unqualify(pt.Elem)
}

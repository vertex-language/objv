package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// __builtin_add_overflow, _sub_ and _mul_: the exact mathematical result of
// two integers of any types, stored into the third argument's pointee
// truncated to its type, and whether it did not fit.
//
// The arithmetic is done in a working type wide enough to hold both
// operands and the result type exactly -- signed if any of the three is,
// and a bit wider when a signed type has to hold an unsigned one of the
// same width. There the IR's overflow predicates say whether the operation
// itself overflowed, and a round trip through the result type says whether
// what it produced fits. A working type of 65 bits -- int64_t with uint64_t,
// or signed operands into a uint64_t -- is done in two 64-bit words instead:
// see wideOverflow.

func (u *unit) overflowBuiltin(op builtinOp, name string, e *ast.CallExpr) ir.Value {
	if len(e.Args) != 3 {
		u.errorf(e, "'"+name+"' takes two operands and a pointer to the result")
		return nil
	}
	at, bt := types.Unqualify(u.typeOf(e.Args[0])), types.Unqualify(u.typeOf(e.Args[1]))
	pt := types.AsPointer(types.Unqualify(u.typeOf(e.Args[2])))
	if pt == nil || !types.IsInteger(at) || !types.IsInteger(bt) || !types.IsInteger(types.Unqualify(pt.Elem)) {
		u.errorf(e, "'"+name+"' takes two integers and a pointer to an integer")
		return nil
	}
	rt := types.Unqualify(pt.Elem)

	wa, sa := u.model.IntBits(at)
	wb, sb := u.model.IntBits(bt)
	wr, sr := u.model.IntBits(rt)
	signed := sa || sb || sr
	w := wa
	if wb > w {
		w = wb
	}
	if wr > w {
		w = wr
	}
	if signed && ((!sa && wa == w) || (!sb && wb == w) || (!sr && wr == w)) {
		w++
	}
	if w > 64 {
		return u.wideOverflow(op, e, at, bt, rt, sa, sb, sr)
	}
	var et types.Type
	switch {
	case w <= 32 && signed:
		et = types.Typ(types.Int)
	case w <= 32:
		et = types.Typ(types.UInt)
	case signed:
		et = types.Typ(types.LongLong)
	default:
		et = types.Typ(types.ULongLong)
	}

	a := u.rvalue(e.Args[0])
	bv := u.rvalue(e.Args[1])
	dst, ok := u.rvalue(e.Args[2]).(ir.Ptr)
	if a == nil || bv == nil || !ok {
		return nil
	}
	a = u.convert(a, at, et)
	bv = u.convert(bv, bt, et)

	b := u.fn.cur
	var res ir.Value
	var over ir.I1
	switch x := a.(type) {
	case ir.I32:
		y, _ := bv.(ir.I32)
		switch op {
		case bAddO:
			res = b.I32.Add(x, y)
			over = pick(signed, b.I32.SAddO(x, y), b.I32.UAddO(x, y))
		case bSubO:
			res = b.I32.Sub(x, y)
			over = pick(signed, b.I32.SSubO(x, y), b.I32.ULt(x, y))
		default:
			res = b.I32.Mul(x, y)
			over = pick(signed, b.I32.SMulO(x, y), b.I32.UMulO(x, y))
		}
	case ir.I64:
		y, _ := bv.(ir.I64)
		switch op {
		case bAddO:
			res = b.I64.Add(x, y)
			over = pick(signed, b.I64.SAddO(x, y), b.I64.UAddO(x, y))
		case bSubO:
			res = b.I64.Sub(x, y)
			over = pick(signed, b.I64.SSubO(x, y), b.I64.ULt(x, y))
		default:
			res = b.I64.Mul(x, y)
			over = pick(signed, b.I64.SMulO(x, y), b.I64.UMulO(x, y))
		}
	default:
		u.internal(e, "the operands of '"+name+"'")
		return nil
	}

	// Into the result type, and back: a value that changes on the way is
	// one the result type cannot hold.
	stored := u.convert(res, et, rt)
	back := u.convert(stored, rt, et)
	var changed ir.I1
	switch r := res.(type) {
	case ir.I32:
		changed = b.I32.Ne(back.(ir.I32), r)
	case ir.I64:
		changed = b.I64.Ne(back.(ir.I64), r)
	}
	u.storeTo(dst, stored, rt)
	return b.I32.Or(b.I32.ZExtI1(over), b.I32.ZExtI1(changed))
}

func pick(signed bool, s, un ir.I1) ir.I1 {
	if signed {
		return s
	}
	return un
}

// wideOverflow is the checked operation on operands that need 65 bits
// between them, as a 128-bit value in two i64 words: the low word, and the
// high one that sign- or zero-extends it. Add and subtract carry between
// them; multiply is the low 128 bits of the product, which for operands of
// at most 65 bits never wraps into a value that looks as if it fits. It
// fits the result type when truncating the low word to it and extending
// back gives both words again.
func (u *unit) wideOverflow(op builtinOp, e *ast.CallExpr, at, bt, rt types.Type, sa, sb, sr bool) ir.Value {
	wide := func(x ast.Expr, t types.Type, signed bool) (lo, hi ir.I64, ok bool) {
		v := u.rvalue(x)
		if v == nil {
			return lo, hi, false
		}
		w := types.Typ(types.ULongLong)
		if signed {
			w = types.Typ(types.LongLong)
		}
		lo, ok = u.convert(v, t, w).(ir.I64)
		if !ok {
			return lo, hi, false
		}
		b := u.fn.cur
		if signed {
			return lo, b.I64.SShr(lo, b.I64.Const(63)), true
		}
		return lo, b.I64.Const(0), true
	}
	alo, ahi, ok1 := wide(e.Args[0], at, sa)
	blo, bhi, ok2 := wide(e.Args[1], bt, sb)
	dst, ok3 := u.rvalue(e.Args[2]).(ir.Ptr)
	if !ok1 || !ok2 || !ok3 {
		return nil
	}
	b := u.fn.cur
	var lo, hi ir.I64
	switch op {
	case bAddO:
		lo = b.I64.Add(alo, blo)
		carry := b.I64.ZExtI1(b.I64.ULt(lo, alo))
		hi = b.I64.Add(b.I64.Add(ahi, bhi), carry)
	case bSubO:
		lo = b.I64.Sub(alo, blo)
		borrow := b.I64.ZExtI1(b.I64.ULt(alo, blo))
		hi = b.I64.Sub(b.I64.Sub(ahi, bhi), borrow)
	default:
		lo = b.I64.Mul(alo, blo)
		hi = b.I64.Add(b.I64.UMulHi(alo, blo),
			b.I64.Add(b.I64.Mul(ahi, blo), b.I64.Mul(alo, bhi)))
	}
	wt := types.Typ(types.ULongLong)
	if sr {
		wt = types.Typ(types.LongLong)
	}
	stored := u.convert(lo, types.Typ(types.ULongLong), rt)
	back, _ := u.convert(stored, rt, wt).(ir.I64)
	wantHi := b.I64.Const(0)
	if sr {
		wantHi = b.I64.SShr(back, b.I64.Const(63))
	}
	fits := b.I1.And(b.I64.Eq(back, lo), b.I64.Eq(hi, wantHi))
	u.storeTo(dst, stored, rt)
	return b.I32.ZExtI1(b.I1.Not(fits))
}

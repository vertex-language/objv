package lower

import (
	"math/big"
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// __int128 and unsigned __int128.
//
// VIR has no 128-bit register, so a 128-bit integer is held the way a struct
// is: by address, in 16 bytes aligned to 16, as two words -- lo at 0 and hi
// at 8, little-endian like the target. That is also exactly how Apple's arm64
// convention passes and returns one: in two consecutive registers, x0/x1 for
// a result, with no rounding to an even register, and entirely on the stack
// once fewer than two are left -- the treatment a 16-byte struct of two
// words gets. So a parameter, a result, a local, a copy and a global are the
// struct machinery's, and this file is only the arithmetic: carries across
// the words, UMulHi for the product's high half, Select for shifts by a
// count only known at run time, and compiler-rt's __divti3 family, which
// takes and returns the same pairs, for division and the conversions to and
// from floating point.

// isWide reports whether t is one of the two 128-bit integer types.
func isWide(t types.Type) bool {
	if t == nil {
		return false
	}
	switch types.Unqualify(t).Kind() {
	case types.Int128, types.UInt128:
		return true
	}
	return false
}

// wideType is the VIR type a 128-bit integer lives in.
func (u *unit) wideType() *ir.Type {
	if t := u.mod.LookupType("int128"); t != nil {
		return t
	}
	t := u.mod.Struct("int128")
	t.Internal()
	t.FieldAt("lo", ir.StoreI64.FType(), 0)
	t.FieldAt("hi", ir.StoreI64.FType(), 8)
	t.Align(16)
	return t
}

// wideParts loads a 128-bit value's two words.
func (u *unit) wideParts(p ir.Ptr) (lo, hi ir.I64) {
	b := u.fn.cur
	return b.I64.Load(p), b.I64.Load(b.Ptr.Add(p, b.I64.Const(8)))
}

// wideMake stores two words into a new 128-bit temporary.
func (u *unit) wideMake(lo, hi ir.I64) ir.Ptr {
	p := u.slot(types.Typ(types.Int128), "wide")
	b := u.fn.cur
	b.I64.Store(lo, p)
	b.I64.Store(hi, b.Ptr.Add(p, b.I64.Const(8)))
	return p
}

// convertWide is convert where either side is 128 bits wide.
func (u *unit) convertWide(v ir.Value, from, to types.Type) ir.Value {
	b := u.fn.cur
	if isWide(to) && isWide(from) {
		return v // the same bits, read as the other signedness
	}
	if isWide(to) {
		switch x := v.(type) {
		case ir.F32, ir.F64:
			name := "__fixdfti"
			if !u.signed(to) {
				name = "__fixunsdfti"
			}
			if f, ok := x.(ir.F32); ok {
				name = strings.Replace(name, "df", "sf", 1)
				return u.wideCall(name, []ir.Value{f}, []ir.RegType{ir.TypeF32})
			}
			return u.wideCall(name, []ir.Value{x}, []ir.RegType{ir.TypeF64})
		case ir.Ptr:
			if from != nil && types.IsPointer(from) {
				return u.wideMake(b.Ptr.Diff(x, b.Ptr.Const()), b.I64.Const(0))
			}
			return v
		}
		lo := u.toI64Signed(v, from)
		if lo == nil {
			return nil
		}
		hi := b.I64.Const(0)
		if from == nil || u.signed(from) {
			hi = b.I64.SShr(*lo, b.I64.Const(63))
		}
		return u.wideMake(*lo, hi)
	}
	// From 128 bits.
	p, ok := v.(ir.Ptr)
	if !ok {
		return v
	}
	lo, hi := u.wideParts(p)
	ut := types.Unqualify(to)
	switch {
	case ut.Kind() == types.Bool:
		return b.I32.ZExtI1(b.I64.Ne(b.I64.Or(lo, hi), b.I64.Const(0)))
	case types.IsFloat(ut):
		signed := u.signed(from)
		r, _ := u.reg(to)
		var name string
		switch {
		case r == ir.TypeF32 && signed:
			name = "__floattisf"
		case r == ir.TypeF32:
			name = "__floatuntisf"
		case signed:
			name = "__floattidf"
		default:
			name = "__floatuntidf"
		}
		sig := ir.NewSig()
		sig.Param(ir.TypePtr, ir.ByVal(u.wideType())).Ret(r)
		res := u.fn.cur.Call(u.extern(name, sig), p)
		if res.Len() == 0 {
			return nil
		}
		return res.Value(0)
	case types.IsPointer(ut):
		return b.Ptr.Add(b.Ptr.Const(), lo)
	}
	// An integer: the low word, then the ordinary conversion from a 64-bit
	// one, which narrows and normalizes.
	return u.convert(lo, types.Typ(types.ULongLong), to)
}

// toI64Signed widens an integer register value to i64 by its type's sign.
func (u *unit) toI64Signed(v ir.Value, from types.Type) *ir.I64 {
	b := u.fn.cur
	switch x := v.(type) {
	case ir.I64:
		return &x
	case ir.I32:
		y := u.widen(x, from)
		return &y
	case ir.I1:
		y := b.I64.ZExtI1(x)
		return &y
	}
	return nil
}

// wideCall calls a compiler-rt routine that returns a 128-bit integer.
func (u *unit) wideCall(name string, args []ir.Value, regs []ir.RegType) ir.Value {
	out := u.slot(types.Typ(types.Int128), "wide")
	sig := ir.NewSig()
	sig.Param(ir.TypePtr, ir.SRet(u.wideType()))
	for _, r := range regs {
		if r == ir.TypePtr {
			sig.Param(ir.TypePtr, ir.ByVal(u.wideType()))
			continue
		}
		sig.Param(r)
	}
	u.fn.cur.Call(u.extern(name, sig), append([]ir.Value{out}, args...)...)
	return out
}

// wideBinaryExpr lowers a binary operator one of whose operands is 128 bits.
func (u *unit) wideBinaryExpr(e *ast.BinaryExpr, xt, yt types.Type) ir.Value {
	x, y := u.rvalue(e.X), u.rvalue(e.Y)
	if x == nil || y == nil {
		return nil
	}
	if e.Op == token.SHL || e.Op == token.SHR {
		ct := u.model.Promote(xt)
		if !isWide(ct) {
			// A narrow value shifted by a wide count: the count's low
			// word is all a valid count can be.
			n := u.convert(y, yt, types.Typ(types.Int))
			return u.arith(e.Op, u.convert(x, xt, ct), u.matchShiftWidth(u.convert(x, xt, ct), n), u.signed(ct))
		}
		n := u.convert(y, yt, types.Typ(types.LongLong))
		np, _ := n.(ir.I64)
		xp, _ := u.convert(x, xt, ct).(ir.Ptr)
		return u.wideShift(e.Op, xp, np, u.signed(ct))
	}
	ct := u.model.Usual(xt, yt)
	x, y = u.convert(x, xt, ct), u.convert(y, yt, ct)
	if !isWide(ct) {
		return u.arith(e.Op, x, y, u.signed(ct)) // a double, say
	}
	xp, ok1 := x.(ir.Ptr)
	yp, ok2 := y.(ir.Ptr)
	if !ok1 || !ok2 {
		return nil
	}
	return u.wideArith(e.Op, xp, yp, u.signed(ct), e)
}

// wideArith is one operator on two 128-bit values: a new 128-bit value, or
// an int for a comparison.
func (u *unit) wideArith(op token.Kind, x, y ir.Ptr, signed bool, at ast.Node) ir.Value {
	b := u.fn.cur
	alo, ahi := u.wideParts(x)
	blo, bhi := u.wideParts(y)
	switch op {
	case token.ADD:
		lo := b.I64.Add(alo, blo)
		return u.wideMake(lo, b.I64.Add(b.I64.Add(ahi, bhi), b.I64.ZExtI1(b.I64.ULt(lo, alo))))
	case token.SUB:
		lo := b.I64.Sub(alo, blo)
		return u.wideMake(lo, b.I64.Sub(b.I64.Sub(ahi, bhi), b.I64.ZExtI1(b.I64.ULt(alo, blo))))
	case token.MUL:
		hi := b.I64.Add(b.I64.UMulHi(alo, blo), b.I64.Add(b.I64.Mul(ahi, blo), b.I64.Mul(alo, bhi)))
		return u.wideMake(b.I64.Mul(alo, blo), hi)
	case token.AND:
		return u.wideMake(b.I64.And(alo, blo), b.I64.And(ahi, bhi))
	case token.OR:
		return u.wideMake(b.I64.Or(alo, blo), b.I64.Or(ahi, bhi))
	case token.XOR:
		return u.wideMake(b.I64.Xor(alo, blo), b.I64.Xor(ahi, bhi))
	case token.QUO, token.REM:
		name := map[bool]map[token.Kind]string{
			true:  {token.QUO: "__divti3", token.REM: "__modti3"},
			false: {token.QUO: "__udivti3", token.REM: "__umodti3"},
		}[signed][op]
		return u.wideCall(name, []ir.Value{x, y}, []ir.RegType{ir.TypePtr, ir.TypePtr})
	case token.EQL, token.NEQ:
		eq := b.I1.And(b.I64.Eq(alo, blo), b.I64.Eq(ahi, bhi))
		if op == token.NEQ {
			eq = b.I1.Not(eq)
		}
		return b.I32.ZExtI1(eq)
	case token.LSS, token.GTR, token.LEQ, token.GEQ:
		// The high words decide, signed or not; equal high words leave it
		// to the low ones, always unsigned.
		if op == token.GTR || op == token.LEQ {
			alo, ahi, blo, bhi = blo, bhi, alo, ahi
		}
		hiLess := b.I64.ULt(ahi, bhi)
		if signed {
			hiLess = b.I64.SLt(ahi, bhi)
		}
		less := b.I1.Or(hiLess, b.I1.And(b.I64.Eq(ahi, bhi), b.I64.ULt(alo, blo)))
		if op == token.LEQ || op == token.GEQ {
			less = b.I1.Not(less) // a <= b is !(b < a)
		}
		return b.I32.ZExtI1(less)
	}
	u.unsupported(at, "the operator "+op.String()+" on a 128-bit integer")
	return nil
}

// wideShift shifts a 128-bit value by a count known at run time. Every
// count from 0 to 127 is one of three shapes -- zero, under 64, 64 and over
// -- and Select picks among them without a branch.
func (u *unit) wideShift(op token.Kind, x ir.Ptr, n ir.I64, signed bool) ir.Value {
	b := u.fn.cur
	lo, hi := u.wideParts(x)
	c64 := b.I64.Const(64)
	n = b.I64.And(n, b.I64.Const(127))
	small := b.I64.ULt(n, c64)
	zero := b.I64.Eq(n, b.I64.Const(0))
	m := b.I64.And(n, b.I64.Const(63)) // the count within a word
	back := b.I64.Sub(c64, m)          // 64-m, only used when m != 0
	backSafe := b.I64.And(back, b.I64.Const(63))
	if op == token.SHL {
		carry := b.I64.Select(zero, b.I64.Const(0), b.I64.UShr(lo, backSafe))
		nlo := b.I64.Select(small, b.I64.Shl(lo, m), b.I64.Const(0))
		nhi := b.I64.Select(small, b.I64.Or(b.I64.Shl(hi, m), carry), b.I64.Shl(lo, m))
		return u.wideMake(nlo, nhi)
	}
	fill := b.I64.Const(0)
	shr := b.I64.UShr(hi, m)
	if signed {
		fill = b.I64.SShr(hi, b.I64.Const(63))
		shr = b.I64.SShr(hi, m)
	}
	carry := b.I64.Select(zero, b.I64.Const(0), b.I64.Shl(hi, backSafe))
	nlo := b.I64.Select(small, b.I64.Or(b.I64.UShr(lo, m), carry), shr)
	nhi := b.I64.Select(small, shr, fill)
	return u.wideMake(nlo, nhi)
}

// wideUnary is -x and ~x.
func (u *unit) wideUnary(op token.Kind, x ir.Ptr) ir.Value {
	b := u.fn.cur
	lo, hi := u.wideParts(x)
	nlo, nhi := b.I64.Not(lo), b.I64.Not(hi)
	if op == token.TILDE {
		return u.wideMake(nlo, nhi)
	}
	one := b.I64.Const(1)
	rlo := b.I64.Add(nlo, one)
	return u.wideMake(rlo, b.I64.Add(nhi, b.I64.ZExtI1(b.I64.Eq(rlo, b.I64.Const(0)))))
}

// wideTruth is a 128-bit value tested against zero.
func (u *unit) wideTruth(x ir.Ptr) *ir.I1 {
	b := u.fn.cur
	lo, hi := u.wideParts(x)
	c := b.I64.Ne(b.I64.Or(lo, hi), b.I64.Const(0))
	return &c
}

// foldWide evaluates a constant expression whose value may need 128 bits --
// `(unsigned __int128)1 << 100` in a global's initializer -- which the
// analyzer's 64-bit evaluator cannot hold. Every step is wrapped to its
// type's width and sign, as the run-time operation would be.
func (u *unit) foldWide(e ast.Expr) (*big.Int, bool) {
	t := u.typeOf(e)
	if !isWide(t) {
		if v, ok := u.info.Consts[e]; ok {
			return u.wrapTo(big.NewInt(v), t), true
		}
		if v, ok := u.foldInt(e); ok {
			return u.wrapTo(big.NewInt(v), t), true
		}
	}
	switch x := stripParens(e).(type) {
	case *ast.CastExpr:
		v, ok := u.foldWide(x.X)
		if !ok {
			return nil, false
		}
		return u.wrapTo(v, t), true
	case *ast.UnaryExpr:
		v, ok := u.foldWide(x.X)
		if !ok {
			return nil, false
		}
		switch x.Op {
		case token.SUB:
			return u.wrapTo(new(big.Int).Neg(v), t), true
		case token.TILDE:
			return u.wrapTo(new(big.Int).Not(v), t), true
		case token.ADD:
			return v, true
		}
	case *ast.BinaryExpr:
		a, ok1 := u.foldWide(x.X)
		b, ok2 := u.foldWide(x.Y)
		if !ok1 || !ok2 {
			return nil, false
		}
		r := new(big.Int)
		switch x.Op {
		case token.ADD:
			r.Add(a, b)
		case token.SUB:
			r.Sub(a, b)
		case token.MUL:
			r.Mul(a, b)
		case token.AND:
			r.And(a, b)
		case token.OR:
			r.Or(a, b)
		case token.XOR:
			r.Xor(a, b)
		case token.SHL:
			r.Lsh(a, uint(b.Uint64()&127))
		case token.SHR:
			r.Rsh(a, uint(b.Uint64()&127))
		case token.QUO, token.REM:
			if b.Sign() == 0 {
				return nil, false
			}
			if x.Op == token.QUO {
				r.Quo(a, b)
			} else {
				r.Rem(a, b)
			}
		default:
			return nil, false
		}
		return u.wrapTo(r, t), true
	}
	return nil, false
}

// wrapTo reduces v to what an integer of type t holds: its width's worth of
// bits, read as its sign says.
func (u *unit) wrapTo(v *big.Int, t types.Type) *big.Int {
	bits, signed := u.model.IntBits(types.Unqualify(t))
	if bits == 0 {
		return v
	}
	mod := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	r := new(big.Int).Mod(v, mod)
	if signed && r.Bit(int(bits)-1) == 1 {
		r.Sub(r, mod)
	}
	return r
}

// wideWords splits a 128-bit value into its two words.
func wideWords(v *big.Int) (lo, hi int64) {
	mod := new(big.Int).Lsh(big.NewInt(1), 128)
	w := new(big.Int).Mod(v, mod)
	mask := new(big.Int).SetUint64(^uint64(0))
	lo = int64(new(big.Int).And(w, mask).Uint64())
	hi = int64(new(big.Int).Rsh(w, 64).Uint64())
	return lo, hi
}

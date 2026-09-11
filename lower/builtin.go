package lower

import (
	"math"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// The compiler's own functions.
//
// A builtin is not a call. `__builtin_fabs(x)` names no symbol and there is
// no `_fabs` in any object this compiler writes: the name is a way of asking
// the compiler for one instruction, and what arrives here is the instruction.
// That is why this file is a switch over verbs rather than a table of runtime
// entry points — every entry below is a VIR operation that already exists,
// and the correspondence is the reason the analyzer's table holds these names
// and not others. A builtin with no VIR verb behind it would have to become a
// call to a libm function of the same name, which is a decision worth making
// when something needs it rather than in advance.
//
// analyzer/builtin.go types these; the two lists are held together by
// TestBuiltinsAreLowered.

type builtinOp uint8

const (
	bAbs builtinOp = iota
	bSqrt
	bCeil
	bFloor
	bTrunc
	bCopySign
	bMin
	bMax
	bFMA
	bInf
	bClz
	bCtz
	bPopcnt
	bParity
	bBswap
	bBswap16
	bExpect
	bTrap
	bConstantP
	bVaStart
	bVaEnd
	bVaCopy
	bVaArgRef
)

// builtinOps is the name-to-verb map, built the same way the analyzer builds
// its signatures so that the two stay the same shape: a float builtin comes
// in three widths whose suffix is part of the name, and a bit builtin in
// three whose suffix names the argument's width.
var builtinOps = func() map[string]builtinOp {
	m := map[string]builtinOp{}
	for _, sfx := range []string{"", "f", "l"} {
		m["__builtin_fabs"+sfx] = bAbs
		m["__builtin_sqrt"+sfx] = bSqrt
		m["__builtin_ceil"+sfx] = bCeil
		m["__builtin_floor"+sfx] = bFloor
		m["__builtin_trunc"+sfx] = bTrunc
		m["__builtin_copysign"+sfx] = bCopySign
		m["__builtin_fmin"+sfx] = bMin
		m["__builtin_fmax"+sfx] = bMax
		m["__builtin_fma"+sfx] = bFMA
		m["__builtin_inf"+sfx] = bInf
		m["__builtin_huge_val"+sfx] = bInf
	}
	for _, sfx := range []string{"", "l", "ll"} {
		m["__builtin_clz"+sfx] = bClz
		m["__builtin_ctz"+sfx] = bCtz
		m["__builtin_popcount"+sfx] = bPopcnt
		m["__builtin_parity"+sfx] = bParity
	}
	m["__builtin_bswap16"] = bBswap16
	m["__builtin_bswap32"] = bBswap
	m["__builtin_bswap64"] = bBswap
	m["__builtin_expect"] = bExpect
	m["__builtin_unreachable"] = bTrap
	m["__builtin_trap"] = bTrap
	m["__builtin_constant_p"] = bConstantP
	m["__builtin_va_start"] = bVaStart
	m["__builtin_va_end"] = bVaEnd
	m["__builtin_va_copy"] = bVaCopy
	m["__builtin_va_arg_ref"] = bVaArgRef
	return m
}()

// builtinCall lowers a call to one of the compiler's own functions, and
// reports whether the name was one.
//
// A name that is not is not an error here: the analyzer already reported it,
// and returning false lets the ordinary call path run for anything that only
// looks like a builtin.
func (u *unit) builtinCall(name string, e *ast.CallExpr) (ir.Value, bool) {
	op, ok := builtinOps[name]
	if !ok {
		return nil, false
	}
	b := u.fn.cur

	switch op {
	case bConstantP:
		// "Is this a compile-time constant?" — answered no, always, and
		// the argument is not evaluated, which is the half of the contract
		// that matters: every use of it in a header is
		//
		//	if (__builtin_constant_p(x)) return CONSTANT_FOLD(x);
		//	else return runtime_version(x);
		//
		// and both arms are correct code. Answering no takes the arm that
		// is correct for a value the compiler does not know, which is the
		// arm that is correct for every value.
		//
		// The analyzer answers it outright when the operand folds, in the
		// one constant evaluator this compiler has; what reaches here is
		// the operand it could not fold, whose answer is no.
		if v, ok := u.foldInt(e); ok {
			return b.I32.Const(v), true
		}
		return b.I32.Const(0), true

	case bVaStart, bVaEnd, bVaCopy, bVaArgRef:
		return u.variadicBuiltin(op, e)

	case bTrap:
		// __builtin_unreachable and __builtin_trap become the same
		// instruction. C says reaching an unreachable is undefined and a
		// compiler may assume it does not happen; VIR has no verb for that
		// on purpose — "there is no unreachable, which is undefined
		// behaviour under a friendlier name" — so the path ends in a trap,
		// which is a conforming implementation of undefined and a far
		// better one to debug.
		b.Trap()
		u.fn.cur = nil
		return nil, true
	}

	fn := types.AsFunc(u.typeOf(e.Fun))
	if fn == nil {
		return nil, true
	}
	args := make([]ir.Value, 0, len(e.Args))
	for i, a := range e.Args {
		v := u.rvalue(a)
		if v == nil {
			return nil, true
		}
		if i < len(fn.Params) {
			v = u.convert(v, u.typeOf(a), fn.Params[i].Type)
		}
		args = append(args, v)
	}
	if len(args) != len(fn.Params) {
		u.errorf(e, "internal: '"+name+"' was checked with "+itoa(len(fn.Params))+
			" parameters and lowered with "+itoa(len(args))+" arguments")
		return nil, true
	}

	if op == bExpect {
		// The second argument is the value the first is expected to have.
		// VIR carries no branch weights, so there is nowhere to put it and
		// the result is the first argument — which is what the builtin's
		// value is defined to be in any case.
		return args[0], true
	}

	var out ir.Value
	switch len(args) {
	case 0:
		out = u.builtinConst(op, fn.Ret)
	case 1:
		out = u.builtin1(op, args[0])
	case 2:
		out = u.builtin2(op, args[0], args[1])
	case 3:
		out = u.builtin3(op, args[0], args[1], args[2])
	}
	if out == nil {
		u.unsupported(e, "'"+name+"' on "+fn.Ret.String())
	}
	return out, true
}

// builtinConst is a builtin that takes nothing: the two spellings of
// infinity, whose width is the return type's.
func (u *unit) builtinConst(op builtinOp, ret types.Type) ir.Value {
	if op != bInf {
		return nil
	}
	b := u.fn.cur
	r, ok := u.reg(ret)
	if !ok {
		return nil
	}
	switch r {
	case ir.TypeF32:
		return b.F32.Const(math.Inf(1))
	case ir.TypeF64:
		return b.F64.Const(math.Inf(1))
	case ir.TypeF80:
		return b.F80().Const(math.Inf(1))
	}
	return nil
}

// builtin1 is the one-argument builtins. The dispatch is on the value's own
// type rather than on the name's suffix, which is what makes `long double`
// come out right on both targets without this file knowing which: the model
// already decided whether it is f80 or f64, and the value says so.
func (u *unit) builtin1(op builtinOp, a ir.Value) ir.Value {
	b := u.fn.cur
	switch a := a.(type) {
	case ir.F32:
		switch op {
		case bAbs:
			return b.F32.Abs(a)
		case bSqrt:
			return b.F32.Sqrt(a)
		case bCeil:
			return b.F32.Ceil(a)
		case bFloor:
			return b.F32.Floor(a)
		case bTrunc:
			return b.F32.Trunc(a)
		}
	case ir.F64:
		switch op {
		case bAbs:
			return b.F64.Abs(a)
		case bSqrt:
			return b.F64.Sqrt(a)
		case bCeil:
			return b.F64.Ceil(a)
		case bFloor:
			return b.F64.Floor(a)
		case bTrunc:
			return b.F64.Trunc(a)
		}
	case ir.F80:
		switch op {
		case bAbs:
			return b.F80().Abs(a)
		case bSqrt:
			return b.F80().Sqrt(a)
		case bCeil:
			return b.F80().Ceil(a)
		case bFloor:
			return b.F80().Floor(a)
		case bTrunc:
			return b.F80().Trunc(a)
		}
	case ir.I32:
		switch op {
		case bClz:
			return b.I32.Clz(a)
		case bCtz:
			return b.I32.Ctz(a)
		case bPopcnt:
			return b.I32.Popcnt(a)
		case bParity:
			// Parity is the low bit of the population count: an odd number
			// of set bits is odd parity.
			return b.I32.And(b.I32.Popcnt(a), b.I32.Const(1))
		case bBswap:
			return b.I32.Bswap(a)
		case bBswap16:
			// VIR has no i16, and a 16-bit value is held zero-extended in
			// an i32. Reversing all four bytes puts the two that matter in
			// the top half, so the shift is part of the operation and not
			// a conversion after it.
			return b.I32.UShr(b.I32.Bswap(a), b.I32.Const(16))
		}
	case ir.I64:
		// The ll-suffixed bit builtins take a 64-bit argument and return an
		// int: the count of bits in a 64-bit word still fits in 32.
		switch op {
		case bClz:
			return b.I32.WrapI64(b.I64.Clz(a))
		case bCtz:
			return b.I32.WrapI64(b.I64.Ctz(a))
		case bPopcnt:
			return b.I32.WrapI64(b.I64.Popcnt(a))
		case bParity:
			return b.I32.WrapI64(b.I64.And(b.I64.Popcnt(a), b.I64.Const(1)))
		case bBswap:
			return b.I64.Bswap(a)
		}
	}
	return nil
}

// builtin2 is the two-argument float builtins.
//
// fmin and fmax are MinNum and MaxNum, not Minimum and Maximum: C's fmin
// returns the other operand when one is a NaN, and VIR spells that pair
// MinNum/MaxNum. Minimum and Maximum propagate the NaN, which is IEEE 754's
// minimum/maximum and a different function.
func (u *unit) builtin2(op builtinOp, x, y ir.Value) ir.Value {
	b := u.fn.cur
	switch a := x.(type) {
	case ir.F32:
		c, ok := y.(ir.F32)
		if !ok {
			return nil
		}
		switch op {
		case bCopySign:
			return b.F32.CopySign(a, c)
		case bMin:
			return b.F32.MinNum(a, c)
		case bMax:
			return b.F32.MaxNum(a, c)
		}
	case ir.F64:
		c, ok := y.(ir.F64)
		if !ok {
			return nil
		}
		switch op {
		case bCopySign:
			return b.F64.CopySign(a, c)
		case bMin:
			return b.F64.MinNum(a, c)
		case bMax:
			return b.F64.MaxNum(a, c)
		}
	case ir.F80:
		c, ok := y.(ir.F80)
		if !ok {
			return nil
		}
		switch op {
		case bCopySign:
			return b.F80().CopySign(a, c)
		case bMin:
			return b.F80().MinNum(a, c)
		case bMax:
			return b.F80().MaxNum(a, c)
		}
	}
	return nil
}

// builtin3 is fma, which is the whole reason the builtin exists: a*b+c with
// one rounding rather than two is not something a program can write.
func (u *unit) builtin3(op builtinOp, x, y, z ir.Value) ir.Value {
	if op != bFMA {
		return nil
	}
	b := u.fn.cur
	switch a := x.(type) {
	case ir.F32:
		c, ok1 := y.(ir.F32)
		d, ok2 := z.(ir.F32)
		if ok1 && ok2 {
			return b.F32.FMA(a, c, d)
		}
	case ir.F64:
		c, ok1 := y.(ir.F64)
		d, ok2 := z.(ir.F64)
		if ok1 && ok2 {
			return b.F64.FMA(a, c, d)
		}
	case ir.F80:
		c, ok1 := y.(ir.F80)
		d, ok2 := z.(ir.F80)
		if ok1 && ok2 {
			return b.F80().FMA(a, c, d)
		}
	}
	return nil
}

// §7.16's variadic machinery.
//
// objv's own <stdarg.h> writes the four macros in terms of these builtins,
// taking the *address* of the list rather than the list itself:
//
//	#define va_start(ap, last) __builtin_va_start(&(ap))
//	#define va_arg(ap, type)   (*(type *)__builtin_va_arg_ref(&(ap), (type *)0))
//
// which is what lets one signature serve whatever shape __builtin_va_list
// has on the target — a pointer on Darwin's AArch64, four fields on SysV
// x86-64. The second argument of va_arg_ref is a null pointer that exists
// only to carry the type: C has no other way to hand a type to something
// that is not a keyword, and VIR's va_arg_ref wants one.
//
// The `last` argument of va_start is dropped. It names the last fixed
// parameter, which mattered to a compiler that computed the tail's address
// from it; VIR's va_start knows where this function's own parameters ended.
func (u *unit) variadicBuiltin(op builtinOp, e *ast.CallExpr) (ir.Value, bool) {
	b := u.fn.cur
	ap, ok := u.listArg(e, 0)
	if !ok {
		return nil, true
	}
	switch op {
	case bVaStart:
		b.VaStart(ap)
		return nil, true
	case bVaEnd:
		b.VaEnd(ap)
		return nil, true
	case bVaCopy:
		src, ok := u.listArg(e, 1)
		if !ok {
			return nil, true
		}
		b.VaCopy(ap, src)
		return nil, true
	}

	// va_arg_ref: the type is the second argument's pointee, and the
	// result is the argument's address in the list.
	if len(e.Args) < 2 {
		u.errorf(e, "internal: __builtin_va_arg_ref takes a list and a type")
		return nil, true
	}
	pt := types.AsPointer(types.Unqualify(u.typeOf(e.Args[1])))
	if pt == nil {
		u.errorf(e, "internal: __builtin_va_arg_ref's second argument is not a pointer")
		return nil, true
	}
	t, ok := u.vaArgType(pt.Elem)
	if !ok {
		u.unsupported(e, "va_arg of type "+pt.Elem.String())
		return nil, true
	}
	return b.Ptr.VaArgRef(ap, t), true
}

// listArg evaluates one `&ap` argument.
func (u *unit) listArg(e *ast.CallExpr, i int) (ir.Ptr, bool) {
	if i >= len(e.Args) {
		u.errorf(e, "internal: a variadic builtin was called with too few arguments")
		return ir.Ptr{}, false
	}
	p, ok := u.rvalue(e.Args[i]).(ir.Ptr)
	if !ok {
		u.errorf(e.Args[i], "internal: a va_list argument is not an address")
		return ir.Ptr{}, false
	}
	return p, true
}

// vaArgType names the type one variadic argument has, which VIR's va_arg_ref
// wants named rather than described: the ABI knowledge it needs to advance
// the list is the same knowledge byval demands, and it reads it off the type.
func (u *unit) vaArgType(t types.Type) (*ir.Type, bool) {
	if r, ok := types.Unqualify(t).(*types.Record); ok {
		return u.recordType(r)
	}
	f, ok := u.ftype(t)
	if !ok {
		return nil, false
	}
	return u.namedFType("vaarg", f), true
}

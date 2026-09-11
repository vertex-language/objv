package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Expressions, as values.
//
// Every expression here answers with an ir.Value or with nil, and nil means
// "already reported": either the analyzer said something about this tree, or
// this package said it could not lower it. Nothing invents a value to keep
// going, because a made-up value becomes a real instruction.

// rvalue lowers an expression to its value.
func (u *unit) rvalue(e ast.Expr) ir.Value {
	if e == nil || !u.at() {
		return nil
	}
	t := u.typeOf(e)

	switch e := e.(type) {
	case *ast.ParenExpr:
		return u.rvalue(e.X)

	case *ast.BasicLit:
		return u.literal(e, t)

	case *ast.StringLit:
		return u.stringLit(e)

	case *ast.Ident:
		// An enumeration constant has no storage: what it is worth is what
		// the analyzer recorded, and there is nothing to look up.
		if v, ok := u.info.Consts[e]; ok {
			return u.constOf(v, t)
		}
		return u.identValue(e, t)

	case *ast.UnaryExpr:
		return u.unary(e, t)

	case *ast.BinaryExpr:
		return u.binary(e, t)

	case *ast.AssignExpr:
		return u.assign(e, t)

	case *ast.CondExpr:
		return u.conditional(e, t)

	case *ast.CallExpr:
		return u.call(e, t)

	case *ast.CompoundLit:
		return u.compoundLit(e, t)

	case *ast.CastExpr:
		return u.cast(e, t)

	case *ast.SizeofExpr:
		if v, ok := u.info.Consts[e]; ok {
			return u.constOf(v, t)
		}
		if e.Type != nil {
			st := u.typeOf(e.Type)
			if hasVariableExtent(st) || isVLA(st) {
				u.unsupported(e, "sizeof applied to a variably modified type name")
				return nil
			}
			size, _ := u.sizeAlign(st)
			return u.constOf(int64(size), t)
		}
		if inner := u.typeOf(e.X); inner != nil {
			// §6.5.3.4p2: applied to a variably modified object, sizeof is
			// evaluated where it is written and yields the size that object
			// actually has. sizeAlign would answer with a pointer's width,
			// which is what an array of n elements is not.
			if isVLA(inner) {
				if v, ok := u.vlaBytes(e.X); ok {
					return u.convert(v, u.model.SizeType(), t)
				}
				u.unsupported(e, "sizeof applied to this variably modified object")
				return nil
			}
			size, _ := u.sizeAlign(inner)
			return u.constOf(int64(size), t)
		}
		return nil

	case *ast.AlignofExpr:
		if e.Type != nil {
			_, align := u.sizeAlign(u.typeOf(e.Type))
			return u.constOf(int64(align), t)
		}
		return nil

	case *ast.IncDecExpr:
		if v, done := u.objcIncDec(e.X, e.Op, true); done {
			return v
		}
		if m, ok := stripParens(e.X).(*ast.MemberExpr); ok {
			if base, bf, isBF := u.bitFieldMember(m); isBF {
				return u.incDecBitField(base, bf, e.Op, true)
			}
		}
		return u.incDec(e.X, e.Op, t, true)

	case *ast.MemberExpr:
		// Dot syntax on an object is the getter, not a member.
		if r := u.propertyRef(e); r != nil {
			return r.load(u)
		}
		// A bit-field has no address, so it is not read through one.
		if base, bf, ok := u.bitFieldMember(e); ok {
			return u.loadBitField(base, bf)
		}
		return u.lvalueRead(e)

	case *ast.IndexExpr:
		if r := u.subscriptRef(e, t); r != nil {
			return r.load(u)
		}
		return u.lvalueRead(e)

	// ---- Objective-C ----

	case *ast.MessageExpr:
		return u.message(e, t)

	case *ast.SelectorExpr:
		return u.selectorRef(u.selectorText(e))

	case *ast.BoxedExpr:
		return u.boxed(e, t)

	case *ast.ArrayLit:
		return u.arrayLit(e, t)

	case *ast.DictLit:
		return u.dictLit(e, t)

	case *ast.BlockLit:
		return u.blockLit(e, t)

	case *ast.ProtocolExpr:
		// @protocol(X) is the address of the protocol object, which this
		// unit defines whether or not anything else mentions it.
		p := u.protocolNamed(u.name(e.Name))
		if p == nil {
			u.errorf(e, "no protocol named %s", u.name(e.Name))
			return nil
		}
		return u.fn.cur.Ptr.GetAddr(u.protocolSym(p))

	case *ast.EncodeExpr:
		if e.Type != nil {
			return u.cstring(u.abi.Encode(u.typeOf(e.Type), u.model))
		}
		return nil

	case *ast.AvailabilityExpr:
		return u.availability(e)

	case *ast.GenericExpr:
		return u.generic(e)

	case *ast.StmtExpr:
		return u.stmtExpr(e, t)
	}
	u.unsupported(e, "this expression")
	return nil
}

// literal lowers a constant.
func (u *unit) literal(e *ast.BasicLit, t types.Type) ir.Value {
	switch e.Kind {
	case token.INT_LIT, token.CHAR_LIT, token.BOOL_LIT:
		v, ok := u.foldInt(e)
		if !ok {
			return nil
		}
		return u.constOf(v, t)
	case token.FLOAT_LIT:
		text := string(u.src.Slice(e.Lo, e.Hi))
		f, _ := analyzer.DecodeFloatConst(text, func(string) {})
		if r, _ := u.reg(t); r == ir.TypeF32 {
			return u.fn.cur.F32.Const(float64(float32(f)))
		}
		return u.fn.cur.F64.Const(f)
	}
	return nil
}

// foldInt reads an integer constant's value.
func (u *unit) foldInt(e ast.Expr) (int64, bool) {
	if v, ok := u.info.Consts[e]; ok {
		return v, true
	}
	lit, ok := e.(*ast.BasicLit)
	if !ok {
		return 0, false
	}
	text := string(u.src.Slice(lit.Lo, lit.Hi))
	switch lit.Kind {
	case token.INT_LIT:
		return int64(analyzer.DecodeIntConst(text, u.model, func(string) {}).Value), true
	case token.CHAR_LIT:
		return int64(analyzer.DecodeCharConst(text, u.model, func(string) {}).Value), true
	case token.BOOL_LIT:
		if text == "__objc_yes" {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// foldFloat is a floating constant expression's value.
//
// It is the only floating evaluator the compiler has — the analyzer's folds
// integers, because those are what §6.6 requires somewhere — so it answers
// for the shapes a constant initializer is written in rather than for
// literals alone. `static const CGFloat NSVariableStatusItemLength = -1.0;`
// in <NSStatusBar.h> is a unary minus applied to one, and a compiler that
// only knows literals cannot initialize it.
func (u *unit) foldFloat(e ast.Expr) (float64, bool) {
	switch x := stripParens(e).(type) {
	case *ast.UnaryExpr:
		v, ok := u.foldFloat(x.X)
		if !ok {
			return 0, false
		}
		switch x.Op {
		case token.SUB:
			return -v, true
		case token.ADD:
			return v, true
		}
		return 0, false

	case *ast.CastExpr:
		// A cast to a floating type is the value; to anything else it is a
		// conversion this does not perform.
		if types.IsFloat(u.typeOf(x)) {
			return u.foldFloat(x.X)
		}
		return 0, false

	case *ast.BasicLit:
		if x.Kind == token.FLOAT_LIT {
			text := string(u.src.Slice(x.Lo, x.Hi))
			v, _ := analyzer.DecodeFloatConst(text, func(string) {})
			return v, true
		}
	}
	if v, ok := u.foldInt(e); ok {
		return float64(v), true
	}
	return 0, false
}

// constOf builds a constant of the register type t is held in.
func (u *unit) constOf(v int64, t types.Type) ir.Value {
	b := u.fn.cur
	r, ok := u.reg(t)
	if !ok {
		r = ir.TypeI32
	}
	switch r {
	case ir.TypeI64:
		return b.I64.Const(v)
	case ir.TypeF32:
		return b.F32.Const(float64(v))
	case ir.TypeF64:
		return b.F64.Const(float64(v))
	case ir.TypePtr:
		if v == 0 {
			return b.Ptr.Const()
		}
		return nil
	}
	return b.I32.Const(v)
}

// identValue reads what a name holds.
func (u *unit) identValue(id *ast.Ident, t types.Type) ir.Value {
	name := u.name(id)
	st := u.lookup(name)
	if st == nil {
		if why := u.undescribed[name]; why != "" {
			u.unsupported(id, why)
			return nil
		}
		u.errorf(id, "internal: '"+name+"' reached lowering undeclared")
		return nil
	}
	switch st.kind {
	case stEnum:
		return u.constOf(st.value, t)
	case stFunc:
		return u.fn.cur.Ptr.GetAddr(u.symOf(st))
	case stGlobal:
		p := u.fn.cur.Ptr.GetAddr(u.symOf(st))
		if isAggregate(st.typ) {
			return p
		}
		return u.loadFrom(p, st.typ)
	case stLocal:
		if isAggregate(st.typ) {
			return st.addr
		}
		return u.loadFrom(st.addr, st.typ)
	case stByref:
		p := u.byrefAddr(st.byref, st.addr)
		if isAggregate(st.typ) {
			return p
		}
		return u.loadFrom(p, st.typ)
	case stIvar:
		if u.ivarIsBitField(st) {
			u.unsupported(id, "an instance variable declared as a bit-field")
			return nil
		}
		addr := u.ivarAddr(st.class, st.ivar)
		if addr == nil {
			return nil
		}
		// An aggregate's value is its address — that is what every caller
		// here means by one, because a struct does not fit a register and
		// nothing but the ABI decides how it travels. The four kinds of
		// storage have to agree about it, and the two above already did.
		if isAggregate(st.typ) {
			return *addr
		}
		return u.loadFrom(*addr, st.typ)
	}
	return nil
}

// unary lowers the prefix operators.
func (u *unit) unary(e *ast.UnaryExpr, t types.Type) ir.Value {
	switch e.Op {
	case token.AND:
		addr, _ := u.lvalue(e.X)
		if addr == nil {
			return nil
		}
		return *addr

	case token.MUL:
		p := u.rvalue(e.X)
		ptr, ok := p.(ir.Ptr)
		if !ok {
			return nil
		}
		// Dereferencing a function pointer yields a function designator,
		// which is the same address again: `(*f)(x)` and `f(x)` are the
		// same call, and §6.3.2.1p4 is why nothing is loaded here.
		if isAggregate(t) || types.IsFunc(t) {
			return ptr
		}
		return u.loadFrom(ptr, t)

	case token.ADD:
		return u.rvalue(e.X)

	case token.SUB:
		v := u.rvalue(e.X)
		return u.negate(v)

	case token.TILDE:
		v := u.rvalue(e.X)
		switch x := v.(type) {
		case ir.I32:
			return u.fn.cur.I32.Not(x)
		case ir.I64:
			return u.fn.cur.I64.Not(x)
		}
		return nil

	case token.NOT:
		c := u.truth(e.X)
		if c == nil {
			return nil
		}
		b := u.fn.cur
		return b.I32.ZExtI1(b.I1.Xor(*c, b.I1.Const(true)))

	case token.INC, token.DEC:
		if v, done := u.objcIncDec(e.X, e.Op, false); done {
			return v
		}
		if m, ok := stripParens(e.X).(*ast.MemberExpr); ok {
			if base, bf, isBF := u.bitFieldMember(m); isBF {
				return u.incDecBitField(base, bf, e.Op, false)
			}
		}
		return u.incDec(e.X, e.Op, t, false)
	}
	return nil
}

func (u *unit) negate(v ir.Value) ir.Value {
	b := u.fn.cur
	switch x := v.(type) {
	case ir.I32:
		return b.I32.Sub(b.I32.Const(0), x)
	case ir.I64:
		return b.I64.Sub(b.I64.Const(0), x)
	case ir.F32:
		return b.F32.Neg(x)
	case ir.F64:
		return b.F64.Neg(x)
	}
	return nil
}

// truth lowers an expression to the one-bit value a condition tests.
func (u *unit) truth(e ast.Expr) *ir.I1 {
	v := u.rvalue(e)
	if v == nil {
		return nil
	}
	return u.truthOf(v)
}

// truthOf is truth over a value already in hand, for the caller that needs
// the value as well as the test — `a ?: b` evaluates a once.
func (u *unit) truthOf(v ir.Value) *ir.I1 {
	if v == nil {
		return nil
	}
	b := u.fn.cur
	var c ir.I1
	switch x := v.(type) {
	case ir.I1:
		c = x
	case ir.I32:
		c = b.I32.Ne(x, b.I32.Const(0))
	case ir.I64:
		c = b.I64.Ne(x, b.I64.Const(0))
	case ir.F32:
		c = b.F32.Ne(x, b.F32.Const(0))
	case ir.F64:
		c = b.F64.Ne(x, b.F64.Const(0))
	case ir.Ptr:
		c = b.Ptr.Ne(x, b.Ptr.Const())
	default:
		return nil
	}
	return &c
}

// lvalueRead is the value of an expression that has an address: take it and
// load, unless the type is one that is passed by its address anyway.
func (u *unit) lvalueRead(e ast.Expr) ir.Value {
	addr, at := u.lvalue(e)
	if addr == nil {
		return nil
	}
	if isAggregate(at) {
		return *addr
	}
	return u.loadFrom(*addr, at)
}

// binary lowers the arithmetic, bitwise and relational operators.
func (u *unit) binary(e *ast.BinaryExpr, t types.Type) ir.Value {
	switch e.Op {
	case token.LAND, token.LOR:
		return u.shortCircuit(e, t)
	case token.COMMA:
		u.rvalue(e.X)
		return u.rvalue(e.Y)
	}

	// §6.3.2.1p3: an array operand is a pointer to its first element
	// everywhere but sizeof, &, and a string literal's initializer — none
	// of which is a binary operator. So `a + 2` on a char[8] is pointer
	// arithmetic, and asking the declared type would say it is arithmetic
	// on an array and have nothing to do.
	xt, yt := types.Decay(u.typeOf(e.X)), types.Decay(u.typeOf(e.Y))
	// Pointer arithmetic scales by the element, which is the one place the
	// operands are not converted to a common type first.
	if types.IsPointer(xt) && types.IsInteger(yt) && (e.Op == token.ADD || e.Op == token.SUB) {
		return u.ptrArith(e, xt)
	}
	if types.IsInteger(xt) && types.IsPointer(yt) && e.Op == token.ADD {
		return u.ptrArith(e, yt)
	}

	// A comparison involving a pointer. The other side is a pointer too, or
	// a null pointer constant, and either way it belongs in a pointer
	// register before Ptr.Eq will look at it.
	if types.IsPointer(xt) || types.IsPointer(yt) {
		x := u.pointerOperand(e.X, xt, yt)
		y := u.pointerOperand(e.Y, yt, xt)
		if x == nil || y == nil {
			return nil
		}
		return u.ptrCompareOrDiff(e, x, y, t)
	}

	x, y := u.rvalue(e.X), u.rvalue(e.Y)
	if x == nil || y == nil {
		return nil
	}

	// §6.3.1.8: both operands go to their common type before the operator
	// sees them. A shift is the exception — its operands are promoted
	// separately, and the result has the left operand's type.
	if types.IsArithmetic(xt) && types.IsArithmetic(yt) {
		if e.Op == token.SHL || e.Op == token.SHR {
			ct := u.model.Promote(xt)
			x = u.convert(x, xt, ct)
			y = u.convert(y, yt, u.model.Promote(yt))
			y = u.matchShiftWidth(x, y)
			return u.arith(e.Op, x, y, u.signed(ct))
		}
		ct := u.model.Usual(xt, yt)
		x = u.convert(x, xt, ct)
		y = u.convert(y, yt, ct)
		res := u.arith(e.Op, x, y, u.signed(ct))
		if res == nil {
			u.errorf(e, "internal: no lowering for %s on %s", e.Op, ct)
		}
		return res
	}

	signed := u.signed(xt) || u.signed(yt)
	res := u.arith(e.Op, x, y, signed)
	if res == nil {
		u.errorf(e, "internal: no lowering for %s on %s and %s", e.Op, xt, yt)
	}
	return res
}

// pointerOperand lowers one side of a pointer comparison.
//
// A null pointer constant becomes the null pointer, not an address computed
// by adding zero to it: `p == 0` asks whether p is null, and saying so
// directly is both shorter and what the comparison means. It is recognized
// before the operand is lowered rather than after, so that nothing is
// emitted for the zero at all — which a constant, having no side effects,
// permits.
func (u *unit) pointerOperand(e ast.Expr, from, other types.Type) ir.Value {
	if !types.IsPointer(from) && u.isNullConstant(e) {
		return u.fn.cur.Ptr.Const()
	}
	v := u.rvalue(e)
	if v == nil {
		return nil
	}
	if types.IsPointer(from) {
		return v
	}
	return u.convert(v, from, other)
}

// isNullConstant reports whether an expression is §6.3.2.3p3's null pointer
// constant: an integer constant expression with the value zero, with any
// number of casts to void * or parentheses around it.
func (u *unit) isNullConstant(e ast.Expr) bool {
	switch x := stripParens(e).(type) {
	case *ast.CastExpr:
		return u.isNullConstant(x.X)
	default:
		if n, ok := u.info.Consts[x]; ok {
			return n == 0
		}
		if n, ok := u.foldInt(x); ok {
			return n == 0
		}
	}
	return false
}

// matchShiftWidth puts the shift count in the register the shifted value
// occupies. VIR's shifts take two operands of one type; C's do not, and the
// count is only ever a small number.
func (u *unit) matchShiftWidth(x, n ir.Value) ir.Value {
	b := u.fn.cur
	switch x.(type) {
	case ir.I64:
		switch v := n.(type) {
		case ir.I32:
			return b.I64.SExtI32(v)
		}
	case ir.I32:
		switch v := n.(type) {
		case ir.I64:
			return b.I32.WrapI64(v)
		}
	}
	return n
}

// arith is the operator table over two values of the same register type.
func (u *unit) arith(op token.Kind, x, y ir.Value, signed bool) ir.Value {
	b := u.fn.cur
	switch a := x.(type) {
	case ir.I32:
		c, ok := y.(ir.I32)
		if !ok {
			return nil
		}
		switch op {
		case token.ADD:
			return b.I32.Add(a, c)
		case token.SUB:
			return b.I32.Sub(a, c)
		case token.MUL:
			return b.I32.Mul(a, c)
		case token.QUO:
			if signed {
				return b.I32.SDiv(a, c)
			}
			return b.I32.UDiv(a, c)
		case token.REM:
			if signed {
				return b.I32.SRem(a, c)
			}
			return b.I32.URem(a, c)
		case token.AND:
			return b.I32.And(a, c)
		case token.OR:
			return b.I32.Or(a, c)
		case token.XOR:
			return b.I32.Xor(a, c)
		case token.SHL:
			return b.I32.Shl(a, c)
		case token.SHR:
			if signed {
				return b.I32.SShr(a, c)
			}
			return b.I32.UShr(a, c)
		}
		return u.compare32(op, a, c, signed)
	case ir.I64:
		c, ok := y.(ir.I64)
		if !ok {
			return nil
		}
		switch op {
		case token.ADD:
			return b.I64.Add(a, c)
		case token.SUB:
			return b.I64.Sub(a, c)
		case token.MUL:
			return b.I64.Mul(a, c)
		case token.QUO:
			if signed {
				return b.I64.SDiv(a, c)
			}
			return b.I64.UDiv(a, c)
		case token.REM:
			if signed {
				return b.I64.SRem(a, c)
			}
			return b.I64.URem(a, c)
		case token.AND:
			return b.I64.And(a, c)
		case token.OR:
			return b.I64.Or(a, c)
		case token.XOR:
			return b.I64.Xor(a, c)
		case token.SHL:
			return b.I64.Shl(a, c)
		case token.SHR:
			if signed {
				return b.I64.SShr(a, c)
			}
			return b.I64.UShr(a, c)
		}
		return u.compare64(op, a, c, signed)
	case ir.F64:
		c, ok := y.(ir.F64)
		if !ok {
			return nil
		}
		switch op {
		case token.ADD:
			return b.F64.Add(a, c)
		case token.SUB:
			return b.F64.Sub(a, c)
		case token.MUL:
			return b.F64.Mul(a, c)
		case token.QUO:
			return b.F64.Div(a, c)
		}
		return u.compareF64(op, a, c)
	case ir.F32:
		c, ok := y.(ir.F32)
		if !ok {
			return nil
		}
		switch op {
		case token.ADD:
			return b.F32.Add(a, c)
		case token.SUB:
			return b.F32.Sub(a, c)
		case token.MUL:
			return b.F32.Mul(a, c)
		case token.QUO:
			return b.F32.Div(a, c)
		}
		return u.compareF32(op, a, c)
	}
	return nil
}

// The comparisons. VIR has no Gt or Ge: a greater-than is a less-than with
// the operands the other way round, which is what the spec says and what
// these do.
func (u *unit) compare32(op token.Kind, a, c ir.I32, signed bool) ir.Value {
	b := u.fn.cur
	lt, le := op == token.LSS, op == token.LEQ
	switch op {
	case token.GTR:
		a, c, lt = c, a, true
	case token.GEQ:
		a, c, le = c, a, true
	}
	var r ir.I1
	switch {
	case lt && signed:
		r = b.I32.SLt(a, c)
	case lt:
		r = b.I32.ULt(a, c)
	case le && signed:
		r = b.I32.SLe(a, c)
	case le:
		r = b.I32.ULe(a, c)
	case op == token.EQL:
		r = b.I32.Eq(a, c)
	case op == token.NEQ:
		r = b.I32.Ne(a, c)
	default:
		return nil
	}
	return b.I32.ZExtI1(r)
}

func (u *unit) compare64(op token.Kind, a, c ir.I64, signed bool) ir.Value {
	b := u.fn.cur
	lt, le := op == token.LSS, op == token.LEQ
	switch op {
	case token.GTR:
		a, c, lt = c, a, true
	case token.GEQ:
		a, c, le = c, a, true
	}
	var r ir.I1
	switch {
	case lt && signed:
		r = b.I64.SLt(a, c)
	case lt:
		r = b.I64.ULt(a, c)
	case le && signed:
		r = b.I64.SLe(a, c)
	case le:
		r = b.I64.ULe(a, c)
	case op == token.EQL:
		r = b.I64.Eq(a, c)
	case op == token.NEQ:
		r = b.I64.Ne(a, c)
	default:
		return nil
	}
	return b.I32.ZExtI1(r)
}

func (u *unit) compareF64(op token.Kind, a, c ir.F64) ir.Value {
	b := u.fn.cur
	var r ir.I1
	switch op {
	case token.EQL:
		r = b.F64.Eq(a, c)
	case token.NEQ:
		r = b.F64.Ne(a, c)
	case token.LSS:
		r = b.F64.Lt(a, c)
	case token.GTR:
		r = b.F64.Lt(c, a)
	case token.LEQ:
		r = b.F64.Le(a, c)
	case token.GEQ:
		r = b.F64.Le(c, a)
	default:
		return nil
	}
	return b.I32.ZExtI1(r)
}

func (u *unit) compareF32(op token.Kind, a, c ir.F32) ir.Value {
	b := u.fn.cur
	var r ir.I1
	switch op {
	case token.EQL:
		r = b.F32.Eq(a, c)
	case token.NEQ:
		r = b.F32.Ne(a, c)
	case token.LSS:
		r = b.F32.Lt(a, c)
	case token.GTR:
		r = b.F32.Lt(c, a)
	case token.LEQ:
		r = b.F32.Le(a, c)
	case token.GEQ:
		r = b.F32.Le(c, a)
	default:
		return nil
	}
	return b.I32.ZExtI1(r)
}

// ptrArith scales an integer by the element size and adds it.
func (u *unit) ptrArith(e *ast.BinaryExpr, pt types.Type) ir.Value {
	var pe, ie ast.Expr = e.X, e.Y
	if types.IsInteger(u.typeOf(e.X)) {
		pe, ie = e.Y, e.X
	}
	p := u.rvalue(pe)
	n := u.rvalue(ie)
	ptr, ok := p.(ir.Ptr)
	if !ok || n == nil {
		return nil
	}
	elem := types.AsPointer(pt).Elem
	size, _ := u.sizeAlign(elem)
	b := u.fn.cur
	off := u.toI64(n)
	if off == nil {
		return nil
	}
	scaled := b.I64.Mul(*off, b.I64.Const(int64(size)))
	if e.Op == token.SUB {
		return b.Ptr.Sub(ptr, scaled)
	}
	return b.Ptr.Add(ptr, scaled)
}

// ptrCompareOrDiff handles the operators two pointers admit.
func (u *unit) ptrCompareOrDiff(e *ast.BinaryExpr, x, y ir.Value, t types.Type) ir.Value {
	a, ok1 := x.(ir.Ptr)
	c, ok2 := y.(ir.Ptr)
	if !ok1 || !ok2 {
		return nil
	}
	b := u.fn.cur
	if e.Op == token.SUB {
		diff := b.Ptr.Diff(a, c)
		elem := types.AsPointer(u.typeOf(e.X))
		size := int64(1)
		if elem != nil {
			s, _ := u.sizeAlign(elem.Elem)
			if s > 0 {
				size = int64(s)
			}
		}
		return b.I64.SDiv(diff, b.I64.Const(size))
	}
	var r ir.I1
	switch e.Op {
	case token.EQL:
		r = b.Ptr.Eq(a, c)
	case token.NEQ:
		r = b.Ptr.Ne(a, c)
	case token.LSS:
		r = b.Ptr.Lt(a, c)
	case token.GTR:
		r = b.Ptr.Lt(c, a)
	case token.LEQ:
		r = b.Ptr.Le(a, c)
	case token.GEQ:
		r = b.Ptr.Le(c, a)
	default:
		return nil
	}
	return b.I32.ZExtI1(r)
}

// shortCircuit lowers && and ||, which are the two operators that are
// control flow rather than arithmetic.
func (u *unit) shortCircuit(e *ast.BinaryExpr, t types.Type) ir.Value {
	lhs := u.truth(e.X)
	if lhs == nil {
		return nil
	}
	rhsBlk := u.block("cond.rhs")
	done := u.block("cond.done")
	res := done.ParamI32("v")

	b := u.fn.cur
	zero, one := b.I32.Const(0), b.I32.Const(1)
	if e.Op == token.LAND {
		b.BrIf(*lhs, rhsBlk.To(), done.To(zero))
	} else {
		b.BrIf(*lhs, done.To(one), rhsBlk.To())
	}

	u.fn.cur = rhsBlk
	rhs := u.truth(e.Y)
	if rhs == nil {
		return nil
	}
	rb := u.fn.cur
	rb.Br(done.To(rb.I32.ZExtI1(*rhs)))

	u.fn.cur = done
	return res
}

// conditional lowers ?: as a branch with a block parameter, which is what
// VIR has instead of a phi.
func (u *unit) conditional(e *ast.CondExpr, t types.Type) ir.Value {
	if e.Then == nil {
		return u.binaryConditional(e, t)
	}
	c := u.truth(e.Cond)
	if c == nil {
		return nil
	}
	thenB, elseB, done := u.block("cond.then"), u.block("cond.else"), u.block("cond.done")

	void := types.IsVoid(t)
	var res ir.Value
	if !void {
		r, ok := u.reg(t)
		if !ok {
			u.unsupported(e, "a conditional of type "+t.String())
			return nil
		}
		res = done.Param(r, "v")
	}
	u.fn.cur.BrIf(*c, thenB.To(), elseB.To())

	u.fn.cur = thenB
	tv := u.rvalue(e.Then)
	if u.at() {
		if void {
			u.fn.cur.Br(done.To())
		} else if tv != nil {
			u.fn.cur.Br(done.To(u.convert(tv, u.typeOf(e.Then), t)))
		}
	}
	u.fn.cur = elseB
	ev := u.rvalue(e.Else)
	if u.at() {
		if void {
			u.fn.cur.Br(done.To())
		} else if ev != nil {
			u.fn.cur.Br(done.To(u.convert(ev, u.typeOf(e.Else), t)))
		}
	}
	u.fn.cur = done
	return res
}

// binaryConditional lowers GCC's `a ?: b`.
//
// One evaluation of a, not two. That is the entire difference from
// `a ? a : b` and the entire reason the form exists: a is often a call, and
// `[self cached] ?: [self compute]` must not compute twice when the cache
// hit. So the value is taken once, tested, and handed to the true edge.
func (u *unit) binaryConditional(e *ast.CondExpr, t types.Type) ir.Value {
	v := u.rvalue(e.Cond)
	if v == nil {
		return nil
	}
	ct := u.typeOf(e.Cond)
	c := u.truthOf(v)
	if c == nil {
		return nil
	}
	thenB, elseB, done := u.block("cond.then"), u.block("cond.else"), u.block("cond.done")

	void := types.IsVoid(t)
	var res ir.Value
	if !void {
		r, ok := u.reg(t)
		if !ok {
			u.unsupported(e, "a conditional of type "+t.String())
			return nil
		}
		res = done.Param(r, "v")
	}
	u.fn.cur.BrIf(*c, thenB.To(), elseB.To())

	u.fn.cur = thenB
	if void {
		u.fn.cur.Br(done.To())
	} else {
		u.fn.cur.Br(done.To(u.convert(v, ct, t)))
	}

	u.fn.cur = elseB
	ev := u.rvalue(e.Else)
	if u.at() {
		if void {
			u.fn.cur.Br(done.To())
		} else if ev != nil {
			u.fn.cur.Br(done.To(u.convert(ev, u.typeOf(e.Else), t)))
		}
	}
	u.fn.cur = done
	return res
}

// assign lowers simple and compound assignment.
func (u *unit) assign(e *ast.AssignExpr, t types.Type) ir.Value {
	// Dot syntax and object subscripting assign through a setter, and
	// have no address for the ordinary path below to write to.
	if v, done := u.objcAssign(e, t); done {
		return v
	}

	// A bit-field is not assigned to; its unit is read, its bits replaced,
	// and the unit written back.
	if m, ok := stripParens(e.Lhs).(*ast.MemberExpr); ok {
		if base, bf, isBF := u.bitFieldMember(m); isBF {
			return u.assignBitField(e, base, bf)
		}
	}

	addr, at := u.lvalue(e.Lhs)
	if addr == nil {
		return nil
	}
	if e.Op == token.ASSIGN {
		if isAggregate(at) {
			src := u.rvalue(e.Rhs)
			if p, ok := src.(ir.Ptr); ok {
				u.copyAggregate(*addr, p, at)
			}
			return *addr
		}
		// Under ARC an assignment is where ownership changes: a __strong
		// location lets go of what it held and takes what it is given, and
		// `self = [super init]` in an initializer takes the +1 its
		// superclass produced rather than releasing it. See arc.go.
		if u.arcOn() && (u.isStrong(at) || u.consumingSelf(e.Lhs)) {
			v, owned := u.rvalueOwned(e.Rhs)
			if v == nil {
				return nil
			}
			v = u.convert(v, u.typeOf(e.Rhs), at)
			if !u.isStrong(at) {
				u.storeTo(u.refreshByref(e.Lhs, *addr), v, at) // self, consumed
				return v
			}
			if !owned {
				v = u.retain(v, at)
			}
			// The address after the retain and not before it: retaining a
			// block copies it to the heap, and a block that captured a
			// __block variable takes that variable's structure with it.
			u.replaceStrong(u.refreshByref(e.Lhs, *addr), at, v)
			return v
		}
		v := u.rvalue(e.Rhs)
		if v == nil {
			return nil
		}
		v = u.convert(v, u.typeOf(e.Rhs), at)
		u.storeTo(u.refreshByref(e.Lhs, *addr), v, at)
		return v
	}

	// A compound assignment reads, operates, and writes back.
	old := u.loadFrom(*addr, at)
	rhs := u.rvalue(e.Rhs)
	if old == nil || rhs == nil {
		return nil
	}
	op := compoundOp(e.Op)
	var res ir.Value
	if types.IsPointer(at) {
		elem := types.AsPointer(at).Elem
		size, _ := u.sizeAlign(elem)
		n := u.toI64(rhs)
		p, ok := old.(ir.Ptr)
		if n == nil || !ok {
			return nil
		}
		scaled := u.fn.cur.I64.Mul(*n, u.fn.cur.I64.Const(int64(size)))
		if op == token.SUB {
			res = u.fn.cur.Ptr.Sub(p, scaled)
		} else {
			res = u.fn.cur.Ptr.Add(p, scaled)
		}
	} else {
		res = u.arith(op, old, u.convert(rhs, u.typeOf(e.Rhs), at), u.signed(at))
	}
	if res == nil {
		return nil
	}
	u.storeTo(*addr, res, at)
	return res
}

func compoundOp(k token.Kind) token.Kind {
	switch k {
	case token.ADD_ASSIGN:
		return token.ADD
	case token.SUB_ASSIGN:
		return token.SUB
	case token.MUL_ASSIGN:
		return token.MUL
	case token.QUO_ASSIGN:
		return token.QUO
	case token.REM_ASSIGN:
		return token.REM
	case token.AND_ASSIGN:
		return token.AND
	case token.OR_ASSIGN:
		return token.OR
	case token.XOR_ASSIGN:
		return token.XOR
	case token.SHL_ASSIGN:
		return token.SHL
	case token.SHR_ASSIGN:
		return token.SHR
	}
	return k
}

// incDec lowers ++ and --, in both positions. postfix says the value is the
// one from before the change.
func (u *unit) incDec(x ast.Expr, op token.Kind, t types.Type, postfix bool) ir.Value {
	addr, at := u.lvalue(x)
	if addr == nil {
		return nil
	}
	old := u.loadFrom(*addr, at)
	if old == nil {
		return nil
	}
	b := u.fn.cur
	var next ir.Value
	if types.IsPointer(at) {
		elem := types.AsPointer(at).Elem
		size, _ := u.sizeAlign(elem)
		p, ok := old.(ir.Ptr)
		if !ok {
			return nil
		}
		d := b.I64.Const(int64(size))
		if op == token.DEC {
			next = b.Ptr.Sub(p, d)
		} else {
			next = b.Ptr.Add(p, d)
		}
	} else {
		one := u.constOf(1, at)
		arith := token.ADD
		if op == token.DEC {
			arith = token.SUB
		}
		next = u.arith(arith, old, one, u.signed(at))
	}
	if next == nil {
		return nil
	}
	u.storeTo(*addr, next, at)
	if postfix {
		return old
	}
	return next
}

// call lowers a function call, and a block invocation.
func (u *unit) call(e *ast.CallExpr, t types.Type) ir.Value {
	// A builtin is not a call and never reaches a symbol, so it is answered
	// before anything here looks for one. See builtin.go.
	if id, ok := stripParens(e.Fun).(*ast.Ident); ok {
		if v, handled := u.builtinCall(u.name(id), e); handled {
			return v
		}
	}
	ft := u.typeOf(e.Fun)
	if bt, ok := types.Unqualify(ft).(*types.Block); ok {
		return u.callBlock(e, bt)
	}
	fn := types.AsFunc(ft)
	if fn == nil {
		if p := types.AsPointer(ft); p != nil {
			fn = types.AsFunc(p.Elem)
		}
	}
	if fn == nil {
		return nil
	}

	// The storage an aggregate result is written into, which is also the
	// value of the call: an aggregate is held by address in this package,
	// and the caller's storage is that address.
	var out ir.Ptr
	var args []ir.Value
	if isIndirectResult(fn.Ret) {
		out = u.aggResult(fn.Ret)
		args = append(args, out)
	}
	var wbs []*writeback
	for i, a := range e.Args {
		if i < len(fn.Params) {
			// §ARC 4.3.2's out-parameter, which is handed a temporary and
			// copied back after the call. See arc.go.
			if v, wb := u.arcOutArg(a, fn.Params[i].Type); wb != nil {
				args = append(args, v)
				wbs = append(wbs, wb)
				continue
			}
		}
		v := u.rvalue(a)
		if v == nil {
			u.internal(a, "an argument of this call")
			return nil
		}
		at := u.typeOf(a)
		if i < len(fn.Params) {
			v = u.convert(v, at, fn.Params[i].Type)
			if isAggregate(fn.Params[i].Type) {
				copied, ok := u.aggArg(v, fn.Params[i].Type, a)
				if !ok {
					return nil
				}
				v = copied
			}
		} else {
			if types.IsRecord(at) {
				// A struct in the var-tail. It is legal C, and what the
				// convention does with it is a classification this has no
				// way to state: there is no declared parameter to hang
				// byval on. An *array* is not one of these — §6.3.2.1
				// decayed it to a pointer before it got here.
				u.unsupported(a, "a struct or union in a variadic argument")
				return nil
			}
			v = u.defaultPromote(v, at)
		}
		args = append(args, v)
	}

	// A direct call to a name reaches its symbol; anything else is a call
	// through a pointer, which the IR wants a type for.
	if id, ok := stripParens(e.Fun).(*ast.Ident); ok {
		if st := u.lookup(u.name(id)); st != nil && st.kind == stFunc {
			if callee, ok := u.symOf(st).(ir.Callee); ok {
				res := u.callMaybeUnwind(callee, args...)
				u.applyWritebacks(wbs)
				if out != (ir.Ptr{}) {
					return out
				}
				if types.IsVoid(fn.Ret) || res.Len() == 0 {
					return nil
				}
				return res.Value(0)
			}
		}
	}
	// A call through a pointer: the pointer is a value, and VIR wants the
	// signature stated at the site rather than taken from a symbol.
	//
	// The parameters are the *arguments'* register types and not the
	// declared ones, which is what makes an unprototyped call and a call
	// past a var-tail describe themselves correctly — the conversions
	// above already put each argument in the type the callee expects.
	fp := u.rvalue(e.Fun)
	p, ok := fp.(ir.Ptr)
	if !ok {
		return nil
	}
	sig := ir.NewSig()
	for i, a := range args {
		r, ok := u.regOfValue(a)
		if !ok {
			u.errorf(e, "internal: %T is not a register value in an indirect call", a)
			return nil
		}
		// An aggregate is a pointer here whatever the convention does with
		// it, so only the attribute tells the backend which pointer it is.
		// The leading one is the result's storage, and the rest line up
		// with the declared parameters.
		switch {
		case i == 0 && out != (ir.Ptr{}):
			t, _ := u.aggType(fn.Ret)
			sig.Param(r, ir.SRet(t))
		default:
			j := i
			if out != (ir.Ptr{}) {
				j--
			}
			if j < len(fn.Params) && isAggregate(fn.Params[j].Type) {
				t, _ := u.aggType(fn.Params[j].Type)
				sig.Param(r, ir.ByVal(t))
				continue
			}
			sig.Param(r)
		}
	}
	hasRet := false
	if !types.IsVoid(fn.Ret) && out == (ir.Ptr{}) {
		if r, ok := u.reg(fn.Ret); ok {
			sig.Ret(r)
			hasRet = true
		} else {
			u.unsupported(e, "an indirect call returning "+fn.Ret.String())
			return nil
		}
	}
	res := u.callIndMaybeUnwind(p, u.namedFuncType("fnsig", sig), args...)
	u.applyWritebacks(wbs)
	if out != (ir.Ptr{}) {
		return out
	}
	if !hasRet || res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// cast lowers a conversion the program wrote.
func (u *unit) cast(e *ast.CastExpr, t types.Type) ir.Value {
	// (void *)0 is the null pointer, not an address arrived at by adding
	// zero to one. Every `#define NULL ((void *)0)` in every header is
	// this case.
	if types.IsPointer(t) && u.isNullConstant(e.X) {
		return u.fn.cur.Ptr.Const()
	}
	if e.IsBridge() {
		return u.bridgeCast(e, t)
	}
	v := u.rvalue(e.X)
	if v == nil {
		return nil
	}
	if types.IsVoid(t) {
		return nil
	}
	return u.convert(v, u.typeOf(e.X), t)
}

// bridgeCast lowers §6.5's three bridge keywords, which are casts about
// ownership and about nothing else: the bits are the same pointer either
// way, and what differs is who owes a release.
//
// __bridge moves nothing. __bridge_retained hands ARC's reference *out* —
// the result is +1 and belongs to whatever C code takes it, which is why
// CFBridgingRetain is spelled with it and why the program is expected to
// CFRelease. __bridge_transfer takes a +1 the other way, so the value
// arrives owned and either something claims it or the end of the statement
// releases it.
//
// Getting this wrong is not a leak in one direction and a leak in the other:
// a __bridge_retained lowered as a plain cast leaves ARC still holding a
// reference the program has also handed to CFRelease, which is a double
// release and a crash somewhere else entirely.
func (u *unit) bridgeCast(e *ast.CastExpr, t types.Type) ir.Value {
	v := u.rvalue(e.X)
	if v == nil || types.IsVoid(t) {
		return nil
	}
	v = u.convert(v, u.typeOf(e.X), t)
	if !u.arcOn() {
		// Without ARC there is no ownership for the keyword to describe,
		// which the analyzer has already said.
		return v
	}
	switch e.Op.String() {
	case "__bridge_retained":
		// A reference for the far side, taken without disturbing the one
		// the operand's own storage holds.
		return u.retain(v, u.typeOf(e.X))
	case "__bridge_transfer":
		u.owns(v)
	}
	return v
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

// generic lowers a C11 _Generic selection.
//
// There is nothing to choose here. The controlling expression is not
// evaluated — §6.5.1.1 says so, and it is why `_Generic(*(int *)0, ...)` is
// well defined — and which association it selected is a fact about types
// that the analyzer settled and recorded. This emits that association's
// expression and nothing else: the arms not taken are not lowered, so a
// `_Generic` whose unselected arm would not compile for this type still
// builds, which is the whole reason the construct exists.
func (u *unit) generic(e *ast.GenericExpr) ir.Value {
	sel, ok := u.info.Generics[e]
	if !ok || sel == nil {
		u.internal(e, "this _Generic selection")
		return nil
	}
	return u.rvalue(sel)
}

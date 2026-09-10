package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/types"
)

// Conversions between register types.
//
// The IR is explicit: an i32 does not become an i64 by being used as one.
// Every widening, narrowing and float conversion is an instruction, and this
// is where the source language's implicit conversions become them.

// convert coerces a value of from to the register type to is held in.
func (u *unit) convert(v ir.Value, from, to types.Type) ir.Value {
	if v == nil || to == nil {
		return v
	}
	want, ok := u.reg(to)
	if !ok {
		return v
	}
	b := u.fn.cur
	switch x := v.(type) {
	case ir.I32:
		switch want {
		case ir.TypeI32:
			return x
		case ir.TypeI64:
			if from != nil && !u.signed(from) {
				return b.I64.ZExtI32(x)
			}
			return b.I64.SExtI32(x)
		case ir.TypeF32:
			if from != nil && !u.signed(from) {
				return b.F32.UCvtI32(x)
			}
			return b.F32.SCvtI32(x)
		case ir.TypeF64:
			if from != nil && !u.signed(from) {
				return b.F64.UCvtI32(x)
			}
			return b.F64.SCvtI32(x)
		case ir.TypePtr:
			// An integer becomes a pointer by being the displacement from
			// null, which is the mirror of Ptr.Diff going the other way.
			return b.Ptr.Add(b.Ptr.Const(), u.widen(x, from))
		}
	case ir.I64:
		switch want {
		case ir.TypeI64:
			return x
		case ir.TypeI32:
			return b.I32.WrapI64(x)
		case ir.TypeF32:
			if from != nil && !u.signed(from) {
				return b.F32.UCvtI64(x)
			}
			return b.F32.SCvtI64(x)
		case ir.TypeF64:
			if from != nil && !u.signed(from) {
				return b.F64.UCvtI64(x)
			}
			return b.F64.SCvtI64(x)
		case ir.TypePtr:
			return b.Ptr.Add(b.Ptr.Const(), x)
		}
	case ir.F32:
		switch want {
		case ir.TypeF32:
			return x
		case ir.TypeF64:
			return b.F64.FCvtF32(x)
		case ir.TypeI32:
			if u.signed(to) {
				return b.I32.SCvtF32(x)
			}
			return b.I32.UCvtF32(x)
		case ir.TypeI64:
			if u.signed(to) {
				return b.I64.SCvtF32(x)
			}
			return b.I64.UCvtF32(x)
		}
	case ir.F64:
		switch want {
		case ir.TypeF64:
			return x
		case ir.TypeF32:
			return b.F32.FCvtF64(x)
		case ir.TypeI32:
			if u.signed(to) {
				return b.I32.SCvtF64(x)
			}
			return b.I32.UCvtF64(x)
		case ir.TypeI64:
			if u.signed(to) {
				return b.I64.SCvtF64(x)
			}
			return b.I64.UCvtF64(x)
		}
	case ir.Ptr:
		// A pointer converts to a pointer by doing nothing — VIR has one
		// pointer type — and to an integer by measuring it.
		switch want {
		case ir.TypePtr:
			return x
		case ir.TypeI64:
			return b.Ptr.Diff(x, b.Ptr.Const())
		case ir.TypeI32:
			return b.I32.WrapI64(b.Ptr.Diff(x, b.Ptr.Const()))
		}
	case ir.I1:
		switch want {
		case ir.TypeI32:
			return b.I32.ZExtI1(x)
		case ir.TypeI64:
			return b.I64.ZExtI1(x)
		case ir.TypeI1:
			return x
		case ir.TypePtr:
			return b.Ptr.Add(b.Ptr.Const(), b.I64.ZExtI1(x))
		}
	}
	return v
}

// widen is an i32 as the i64 a pointer displacement takes, sign-extended or
// not according to where it came from.
func (u *unit) widen(x ir.I32, from types.Type) ir.I64 {
	b := u.fn.cur
	if from != nil && !u.signed(from) {
		return b.I64.ZExtI32(x)
	}
	return b.I64.SExtI32(x)
}

// toI64 widens an integer value to the width a pointer offset takes.
func (u *unit) toI64(v ir.Value) *ir.I64 {
	if v == nil {
		return nil
	}
	b := u.fn.cur
	switch x := v.(type) {
	case ir.I64:
		return &x
	case ir.I32:
		y := b.I64.SExtI32(x)
		return &y
	case ir.I1:
		y := b.I64.ZExtI1(x)
		return &y
	}
	return nil
}

// defaultPromote is what a variadic argument gets: a narrow integer becomes
// an int, and a float becomes a double.
func (u *unit) defaultPromote(v ir.Value, from types.Type) ir.Value {
	if v == nil {
		return nil
	}
	if f, ok := v.(ir.F32); ok {
		return u.fn.cur.F64.FCvtF32(f)
	}
	return v
}

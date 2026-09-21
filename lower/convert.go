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
//
// A register holds more bits than a _Bool, a char or a short, so the value
// in it is kept normalized: a _Bool is 0 or 1, and a narrow integer is its
// value extended to 32 bits the way a load of it would. Every conversion
// into one of those types re-establishes that, which is where C's
// truncation to the narrow type happens.
func (u *unit) convert(v ir.Value, from, to types.Type) ir.Value {
	if v == nil || to == nil {
		return v
	}
	if types.Unqualify(to).Kind() == types.Bool && !isBool(from) {
		if c := u.truthOf(v); c != nil {
			return u.fn.cur.I32.ZExtI1(*c)
		}
		return v
	}
	r := u.convertReg(v, from, to)
	if x, ok := r.(ir.I32); ok && !u.fitsIn(from, to) {
		return u.narrow(x, to)
	}
	return r
}

func isBool(t types.Type) bool {
	return t != nil && types.Unqualify(t).Kind() == types.Bool
}

// fitsIn reports whether every value of from is already a normalized value
// of to, so converting needs no truncation. Only a narrow to can fail it.
func (u *unit) fitsIn(from, to types.Type) bool {
	ts, _ := u.model.Sizeof(types.Unqualify(to))
	if ts >= 4 || !types.IsInteger(types.Unqualify(to)) {
		return true
	}
	if from == nil || !types.IsInteger(types.Unqualify(from)) {
		return false
	}
	if isBool(from) {
		return true
	}
	fs, _ := u.model.Sizeof(types.Unqualify(from))
	fsig, tsig := u.signed(from), u.signed(to)
	if fs == ts {
		return fsig == tsig
	}
	// Narrower: an unsigned value fits either way; a signed one only in
	// a signed type.
	return fs < ts && (!fsig || tsig)
}

// narrow truncates an i32 to a 1- or 2-byte integer type and extends it back
// the way that type's loads do.
func (u *unit) narrow(x ir.I32, to types.Type) ir.I32 {
	size, _ := u.model.Sizeof(types.Unqualify(to))
	if size <= 0 || size >= 4 {
		return x
	}
	b := u.fn.cur
	bits := int64(size * 8)
	if u.signed(to) {
		sh := b.I32.Const(32 - bits)
		return b.I32.SShr(b.I32.Shl(x, sh), sh)
	}
	return b.I32.And(x, b.I32.Const(int64(1)<<bits-1))
}

// convertReg is the conversion between register types alone.
func (u *unit) convertReg(v ir.Value, from, to types.Type) ir.Value {
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

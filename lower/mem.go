package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/types"
)

// Loads and stores, and the width question they answer.
//
// VIR holds a narrow integer in an i32 and stores it in one or two bytes, so
// every load has to say which it is doing and every store has to say how much
// of the register to write. Getting it wrong is not a type error the IR would
// catch — an i32 store of a char writes three bytes of whatever was beside
// it — which is why both go through here.

// loadFrom reads a value of t from an address.
//
// An aggregate has no register to be loaded into: its value *is* its address,
// which is what a struct assignment copies and what a struct argument passes.
// Nil comes back for one, and the caller works with the pointer instead.
func (u *unit) loadFrom(p ir.Ptr, t types.Type) ir.Value {
	// A weak reference is not read by loading it: the object may be
	// deallocated between the read and the use, and only the runtime knows.
	// See arc.go.
	if u.isWeak(t) && u.arcOn() {
		return u.loadWeak(p)
	}
	if isAggregate(t) {
		return nil
	}
	b := u.fn.cur
	st, ok := u.store(t)
	if !ok {
		return nil
	}
	reg, ok := u.reg(t)
	if !ok {
		return nil
	}
	signed := u.signed(t)
	switch reg {
	case ir.TypeI32:
		switch st {
		case ir.StoreI8:
			if signed {
				return b.I32.SLoad8(p)
			}
			return b.I32.ULoad8(p)
		case ir.StoreI16:
			if signed {
				return b.I32.SLoad16(p)
			}
			return b.I32.ULoad16(p)
		}
		return b.I32.Load(p)
	case ir.TypeI64:
		switch st {
		case ir.StoreI8:
			if signed {
				return b.I64.SLoad8(p)
			}
			return b.I64.ULoad8(p)
		case ir.StoreI16:
			if signed {
				return b.I64.SLoad16(p)
			}
			return b.I64.ULoad16(p)
		case ir.StoreI32:
			if signed {
				return b.I64.SLoad32(p)
			}
			return b.I64.ULoad32(p)
		}
		return b.I64.Load(p)
	case ir.TypeF32:
		return b.F32.Load(p)
	case ir.TypeF64:
		return b.F64.Load(p)
	}
	return b.Ptr.Load(p)
}

// storeTo writes a value of t to an address.
func (u *unit) storeTo(p ir.Ptr, v ir.Value, t types.Type) {
	if v == nil {
		return
	}
	// And not written by storing: the runtime keeps the table that makes it
	// go to nil.
	if u.isWeak(t) && u.arcOn() {
		u.storeWeak(p, v)
		return
	}
	b := u.fn.cur
	st, _ := u.store(t)
	switch val := v.(type) {
	case ir.I32:
		switch st {
		case ir.StoreI8:
			b.I32.Store8(val, p)
		case ir.StoreI16:
			b.I32.Store16(val, p)
		default:
			b.I32.Store(val, p)
		}
	case ir.I64:
		switch st {
		case ir.StoreI8:
			b.I64.Store8(val, p)
		case ir.StoreI16:
			b.I64.Store16(val, p)
		case ir.StoreI32:
			b.I64.Store32(val, p)
		default:
			b.I64.Store(val, p)
		}
	case ir.F32:
		b.F32.Store(val, p)
	case ir.F64:
		b.F64.Store(val, p)
	case ir.Ptr:
		b.Ptr.Store(val, p)
	case ir.I1:
		b.I32.Store8(b.I32.ZExtI1(val), p)
	}
}

// copyAggregate copies a struct, a union or an array from one address to
// another. It is what a struct assignment is: the language has no operation
// on the members, only on the object.
func (u *unit) copyAggregate(dst, src ir.Ptr, t types.Type) {
	size, _ := u.sizeAlign(t)
	b := u.fn.cur
	b.MemCpy(dst, src, b.I64.Const(int64(size)))
}

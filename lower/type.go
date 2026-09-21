package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/types"
)

// What a source type is held in.
//
// VIR has six register types and no i8 or i16: a narrow integer lives in an
// i32 and reaches memory through the sub-width load and store verbs. So this
// file answers two different questions, and keeping them apart is most of
// the work — `char c` is *stored* in one byte and *held* in an i32, and a
// lowering that conflated the two would either truncate on every assignment
// or read three bytes of whatever was next to it.

// reg is the register type a value of t is held in.
//
// Every object pointer, block and selector is a ptr; every enumeration is
// the integer it is compatible with; a struct or a union has no register
// type at all and is held by address, which is what ok=false says.
func (u *unit) reg(t types.Type) (ir.RegType, bool) {
	if t == nil {
		return 0, false
	}
	u2 := types.Unqualify(t)
	switch u2.Kind() {
	case types.Bool, types.Char, types.SChar, types.UChar,
		types.Short, types.UShort, types.Int, types.UInt:
		return ir.TypeI32, true
	case types.Long, types.ULong, types.LongLong, types.ULongLong:
		if sz, _ := u.model.Sizeof(u2); sz == 4 {
			return ir.TypeI32, true
		}
		return ir.TypeI64, true
	case types.EnumKind:
		e := u2.(*types.Enum)
		return u.reg(types.Typ(e.Underlying()))
	case types.Float:
		return ir.TypeF32, true
	case types.Double:
		return ir.TypeF64, true
	case types.LongDouble:
		// The extended float types are a property of the layout, and the
		// two targets objv emits for do not agree about long double: it is
		// f80 on x86-64 and the same as double on arm64. The model's size
		// is what says which.
		if sz, _ := u.model.Sizeof(u2); sz > 8 {
			return ir.TypeF80, true
		}
		return ir.TypeF64, true
	case types.PointerKind, types.BlockKind, types.TypeParamKind:
		return ir.TypePtr, true
	case types.ArrayKind, types.FuncKind:
		// Both decay to a pointer wherever a value is wanted; reaching here
		// with one means the caller wanted the object itself.
		return ir.TypePtr, true
	}
	return 0, false
}

// isAggregate reports whether a value of t is held by address rather than in
// a register: a struct, a union, or an array.
func isAggregate(t types.Type) bool {
	switch types.Unqualify(t).Kind() {
	case types.StructKind, types.UnionKind, types.ArrayKind:
		return true
	case types.Int128, types.UInt128:
		return true // held by address, as two words: see int128.go
	}
	return false
}

// store is the store type a value of t occupies in memory — which is not the
// register type: a char is an i8 in memory and an i32 in a register.
func (u *unit) store(t types.Type) (ir.StoreType, bool) {
	if t == nil {
		return 0, false
	}
	u2 := types.Unqualify(t)
	if u2.Kind() == types.EnumKind {
		return u.store(types.Typ(u2.(*types.Enum).Underlying()))
	}
	size, ok := u.model.Sizeof(u2)
	if !ok {
		return 0, false
	}
	switch u2.Kind() {
	case types.Float:
		return ir.StoreF32, true
	case types.Double:
		return ir.StoreF64, true
	case types.LongDouble:
		if size > 8 {
			return ir.StoreF80, true
		}
		return ir.StoreF64, true
	case types.PointerKind, types.BlockKind, types.TypeParamKind,
		types.ArrayKind, types.FuncKind:
		return ir.StorePtr, true
	}
	switch size {
	case 1:
		return ir.StoreI8, true
	case 2:
		return ir.StoreI16, true
	case 4:
		return ir.StoreI32, true
	case 8:
		return ir.StoreI64, true
	}
	return 0, false
}

// sizeAlign is a type's size and alignment, with a pointer's as the fallback
// for a type the model cannot measure — which is a type analysis already
// reported on.
func (u *unit) sizeAlign(t types.Type) (uint64, uint64) {
	size, ok := u.model.Sizeof(t)
	if !ok {
		size = u.abi.PtrBytes
	}
	align, ok := u.model.Alignof(t)
	if !ok || align == 0 {
		align = u.abi.PtrBytes
	}
	return uint64(size), uint64(align)
}

// signed reports whether loads of t sign-extend.
func (u *unit) signed(t types.Type) bool {
	u2 := types.Unqualify(t)
	if u2.Kind() == types.Char {
		return u.model.CharSigned
	}
	return types.IsSigned(u2)
}

// ptrFType is the ftype a pointer-sized global holds.
func (u *unit) ptrFType() ir.FType { return ir.StorePtr.FType() }

// intFType is the ftype an integer of this many bytes holds.
func intFType(bytes int64) ir.FType {
	switch bytes {
	case 1:
		return ir.StoreI8.FType()
	case 2:
		return ir.StoreI16.FType()
	case 8:
		return ir.StoreI64.FType()
	}
	return ir.StoreI32.FType()
}

// addParam declares a parameter of the given register type. The IR spells
// each width as its own method, so this is the one place that switch lives.
func addParam(fn *ir.Func, r ir.RegType, name string) ir.Value {
	switch r {
	case ir.TypeI1:
		return fn.ParamI1(name)
	case ir.TypeI32:
		return fn.ParamI32(name)
	case ir.TypeI64:
		return fn.ParamI64(name)
	case ir.TypeF32:
		return fn.ParamF32(name)
	case ir.TypeF64:
		return fn.ParamF64(name)
	case ir.TypeF80:
		return fn.ParamF80(name)
	}
	return fn.ParamPtr(name)
}

// setReturn states the return type.
func setReturn(fn *ir.Func, r ir.RegType) {
	switch r {
	case ir.TypeI1:
		fn.ReturnsI1()
	case ir.TypeI32:
		fn.ReturnsI32()
	case ir.TypeI64:
		fn.ReturnsI64()
	case ir.TypeF32:
		fn.ReturnsF32()
	case ir.TypeF64:
		fn.ReturnsF64()
	case ir.TypeF80:
		fn.ReturnsF80()
	default:
		fn.ReturnsPtr()
	}
}

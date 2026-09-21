package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// Aggregate parameter and return lowering.
//
// Structs and unions are passed via VIR `byval` pointers (copied into local storage
// to preserve pass-by-value semantics) and returned via `sret` pointers.

// aggType is the named VIR type an aggregate parameter or result names in a
// signature. An array is not one: C decays an array parameter to a pointer
// and has no way to return one, so the only aggregate that reaches a call
// boundary is a struct or a union.
func (u *unit) aggType(t types.Type) (*ir.Type, bool) {
	r, ok := types.Unqualify(t).(*types.Record)
	if !ok {
		return nil, false
	}
	return u.recordType(r)
}

// indirectResult reports whether a result of this type is written through a
// pointer the caller supplies rather than returned in registers.
//
// Every aggregate is, as far as VIR is concerned: sret states the type and
// the backend decides whether the bytes actually travel in registers. A
// two-word struct comes back in X0 and X1 on AArch64 and the caller's
// storage is written from them, which is the backend's business and not a
// difference this has to see.
func isIndirectResult(t types.Type) bool {
	return t != nil && isAggregate(t) && !types.IsVoid(t)
}

// aggSlot is frame storage for an aggregate, in the entry block where §19.6
// wants it.
func (u *unit) aggSlot(t types.Type, name string) ir.Ptr { return u.slot(t, name) }

// aggArg is one aggregate argument: a copy the callee owns.
//
// The copy is not optional. A by-value parameter is the callee's own object
// — it may assign to it, take its address, pass it on — and passing the
// caller's object instead would make `f(s)` able to change s. AAPCS64 says
// the same thing from the other side: "the argument is copied to memory
// allocated by the caller".
func (u *unit) aggArg(v ir.Value, t types.Type, at ast.Node) (ir.Value, bool) {
	src, ok := v.(ir.Ptr)
	if !ok {
		u.errorf(at, "internal: an aggregate argument is not an address")
		return nil, false
	}
	dst := u.aggSlot(t, "arg")
	// A struct that owns objects is handed over as a copy the callee
	// owns and destroys: retained here, released there.
	if u.isARCRecord(t) {
		// A temporary -- a call's result, a compound literal -- is moved
		// in: the callee takes what it owns, and nothing is left to
		// destroy here. Anything else is copied.
		if u.takeRecordTemp(src) {
			return src, true
		}
		u.copyConstruct(dst, src, t)
	} else {
		u.copyAggregate(dst, src, t)
	}
	return dst, true
}

// variadicAggregate is a struct or union passed in a var-tail, as the
// arguments that carry it.
//
// Apple's arm64 convention puts every variadic argument in 8-byte stack
// slots: an aggregate of 16 bytes or less is copied into as many slots as
// it fills, and a larger one is copied and its address passed in one slot.
// Handing the backend the aggregate's 8-byte words, each an i64 variadic
// argument, lays the slots out byte for byte the same -- which is what the
// callee's va_arg reads back. Another convention classifies the aggregate
// into registers, and is not written.
func (u *unit) variadicAggregate(v ir.Value, t types.Type, at ast.Node) ([]ir.Value, bool) {
	src, ok := v.(ir.Ptr)
	if !ok {
		u.errorf(at, "internal: an aggregate argument is not an address")
		return nil, false
	}
	size, align := u.sizeAlign(t)
	if u.arch != runtime.ARM64 || align > 8 {
		u.unsupported(at, "a struct or union in a variadic argument")
		return nil, false
	}
	if size > 16 {
		copied, ok := u.aggArg(src, t, at)
		if !ok {
			return nil, false
		}
		return []ir.Value{copied}, true
	}
	// Copied first into a whole number of slots, so the last word's load
	// does not read past the end of the object.
	slots := (int64(size) + 7) / 8
	tmp := u.aggSlot(types.Typ(types.LongLong), "vararg")
	if slots > 1 {
		tmp = u.aggSlot(&types.Array{Elem: types.Typ(types.LongLong), Len: slots}, "vararg")
	}
	b := u.fn.cur
	b.MemSet(tmp, b.I32.Const(0), b.I64.Const(slots*8))
	u.copyAggregate(tmp, src, t)
	var out []ir.Value
	for i := int64(0); i < slots; i++ {
		b := u.fn.cur
		out = append(out, u.loadFrom(b.Ptr.Add(tmp, b.I64.Const(i*8)), types.Typ(types.LongLong)))
	}
	return out, true
}

// aggResult is the storage a call writes its result into, which is also the
// value of the call expression: an aggregate is held by address here, and
// the address of the caller's storage is what every later phase wants.
func (u *unit) aggResult(t types.Type) ir.Ptr {
	p := u.aggSlot(t, "ret")
	// A struct that owns objects comes back owning them, and is a
	// temporary of the full expression: what takes it copies it.
	if u.isARCRecord(t) {
		size, _ := u.sizeAlign(t)
		b := u.fn.cur
		b.MemSet(p, b.I32.Const(0), b.I64.Const(int64(size)))
		u.noteRecordTemp(p, t)
	}
	return p
}

package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
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
func (u *unit) aggSlot(t types.Type, name string) ir.Ptr {
	size, align := u.sizeAlign(t)
	if size == 0 {
		size = 1
	}
	p := u.fn.entry.Ptr.Alloc(size, align)
	if name != "" {
		u.fn.entry.Name(p, name)
	}
	return p
}

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
	u.copyAggregate(dst, src, t)
	return dst, true
}

// aggResult is the storage a call writes its result into, which is also the
// value of the call expression: an aggregate is held by address here, and
// the address of the caller's storage is what every later phase wants.
func (u *unit) aggResult(t types.Type) ir.Ptr { return u.aggSlot(t, "ret") }

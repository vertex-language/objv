package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/types"
)

// Structs and unions across a call boundary.
//
// C passes and returns them by value, and no calling convention passes them
// in one register. What each convention does instead is a classification —
// AAPCS64 asks whether the aggregate is homogeneous, then whether it is
// sixteen bytes or less, then falls back to the caller's copy by reference;
// SysV sorts each eightbyte into INTEGER, SSE or MEMORY — and neither answer
// is derivable from the other.
//
// None of that is here. VIR states the *question* in the signature and the
// backend answers it: `byval` on a pointer parameter says the bytes it points
// at are the argument, and `sret` on the first says the callee writes its
// result through it. What comes out the other side is registers where the
// convention wants registers and memory where it wants memory, per target,
// which is the only way a compiler with two backends gets this right once.
//
// So this file is about what lower still owes:
//
//   - an aggregate is held by address everywhere in this package, so an
//     argument is already a pointer and a result is already storage;
//   - the argument has to be a *copy*, because the callee owns its
//     parameter and may assign to it;
//   - the result's storage is the caller's, allocated before the call.

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

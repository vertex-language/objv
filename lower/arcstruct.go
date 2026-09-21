package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/types"
)

// C structs that own objects (ARC §4.3.5, clang's "non-trivial C structs").
//
// Under ARC a struct may hold __strong and __weak members, and then it is
// not a bag of bytes any more: making one retains what its strong members
// hold and registers its weak ones, and ending one releases and unregisters
// them. clang writes that as helper functions -- __copy_constructor_…,
// __destructor_… -- and this writes the same operations inline, field by
// field:
//
//   - a local, a parameter or a compound literal is destroyed where its
//     scope ends; a call's result is a temporary destroyed where the full
//     expression ends;
//   - initializing one from another copy-constructs (retain), and assigning
//     one copy-assigns (retain the new, release the old);
//   - an argument is a copy the callee owns and destroys, and a returned one
//     is a copy the caller owns -- which on Apple's arm64 still travels in
//     registers like any struct of its size, exactly as clang passes it.

// ownedField is one member a copy or a destruction has to do something
// about: an object pointer, strong or weak, at a byte offset in the record.
type ownedField struct {
	off  int64
	typ  types.Type
	weak bool
}

// isARCRecord reports whether a value of t is a struct that owns objects.
func (u *unit) isARCRecord(t types.Type) bool {
	if !u.arc || u.fn == nil {
		return false
	}
	r, ok := types.Unqualify(t).(*types.Record)
	if !ok || r.Union {
		return false
	}
	return len(u.ownedFields(t, 0, nil)) > 0
}

// ownedFields flattens a type's owned members -- through nested structs and
// fixed arrays -- to their offsets from base.
func (u *unit) ownedFields(t types.Type, base int64, out []ownedField) []ownedField {
	switch x := types.Unqualify(t).(type) {
	case *types.Record:
		if x.Union || !x.Complete {
			return out
		}
		offs, ok := u.model.FieldOffsets(x)
		if !ok {
			return out
		}
		for i, f := range x.Fields {
			if f.BitField {
				continue
			}
			out = u.ownedFields(f.Type, base+offs[i], out)
		}
	case *types.Array:
		if x.Form != types.FixedArray {
			return out
		}
		inner := u.ownedFields(x.Elem, 0, nil)
		if len(inner) == 0 {
			return out
		}
		size, _ := u.sizeAlign(x.Elem)
		for k := int64(0); k < x.Len; k++ {
			for _, f := range inner {
				f.off += base + k*int64(size)
				out = append(out, f)
			}
		}
	default:
		switch {
		case u.isStrong(t):
			out = append(out, ownedField{off: base, typ: t})
		case u.isWeak(t):
			out = append(out, ownedField{off: base, typ: t, weak: true})
		}
	}
	return out
}

func (u *unit) fieldAt(p ir.Ptr, off int64) ir.Ptr {
	if off == 0 {
		return p
	}
	b := u.fn.cur
	return b.Ptr.Add(p, b.I64.Const(off))
}

// copyConstruct makes dst, which holds nothing yet, a copy of src: the bytes,
// then a reference taken for every object a strong member holds and a
// registration for every weak one.
func (u *unit) copyConstruct(dst, src ir.Ptr, t types.Type) {
	u.copyAggregate(dst, src, t)
	for _, f := range u.ownedFields(t, 0, nil) {
		if f.weak {
			u.copyWeak(u.fieldAt(dst, f.off), u.fieldAt(src, f.off))
			continue
		}
		b := u.fn.cur
		addr := u.fieldAt(dst, f.off)
		b.Ptr.Store(u.retain(b.Ptr.Load(addr), nil).(ir.Ptr), addr)
	}
}

// copyAssign replaces what dst holds with a copy of src. The new objects are
// retained before the old ones are let go, so that `a = a` keeps its objects.
func (u *unit) copyAssign(dst, src ir.Ptr, t types.Type) {
	fields := u.ownedFields(t, 0, nil)
	fresh := make([]ir.Value, len(fields))
	old := make([]ir.Value, len(fields))
	for i, f := range fields {
		if f.weak {
			continue
		}
		b := u.fn.cur
		fresh[i] = u.retain(b.Ptr.Load(u.fieldAt(src, f.off)), nil)
		old[i] = b.Ptr.Load(u.fieldAt(dst, f.off))
	}
	for _, f := range fields {
		if f.weak {
			u.destroyWeak(u.fieldAt(dst, f.off))
		}
	}
	u.copyAggregate(dst, src, t)
	for i, f := range fields {
		if f.weak {
			u.copyWeak(u.fieldAt(dst, f.off), u.fieldAt(src, f.off))
			continue
		}
		u.fn.cur.Ptr.Store(fresh[i].(ir.Ptr), u.fieldAt(dst, f.off))
	}
	for i, f := range fields {
		if !f.weak {
			u.release(old[i])
		}
	}
}

// destroyRecord ends one: every strong member released, every weak one
// unregistered.
func (u *unit) destroyRecord(addr ir.Ptr, t types.Type) {
	if !u.at() {
		return
	}
	// First member to last, which is the order clang's __destructor_
	// helpers go in.
	for _, f := range u.ownedFields(t, 0, nil) {
		if f.weak {
			u.destroyWeak(u.fieldAt(addr, f.off))
			continue
		}
		u.release(u.fn.cur.Ptr.Load(u.fieldAt(addr, f.off)))
	}
}

// noteRecordTemp registers a struct temporary -- a call's result -- for
// destruction where the full expression ends.
func (u *unit) noteRecordTemp(addr ir.Ptr, t types.Type) {
	if u.fn != nil && u.isARCRecord(t) {
		u.fn.recTemps = append(u.fn.recTemps, recTemp{addr: addr, typ: t})
	}
}

// takeRecordTemp removes a struct temporary from the full expression's list
// and reports whether it was one: the caller is taking what it owns --
// moving it into an argument or a variable -- rather than copying it.
func (u *unit) takeRecordTemp(addr ir.Ptr) bool {
	if u.fn == nil {
		return false
	}
	for i := len(u.fn.recTemps) - 1; i >= 0; i-- {
		if u.fn.recTemps[i].addr == addr {
			u.fn.recTemps = append(u.fn.recTemps[:i], u.fn.recTemps[i+1:]...)
			return true
		}
	}
	return false
}

type recTemp struct {
	addr ir.Ptr
	typ  types.Type
}

package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Bit-fields.
//
// §6.7.2.1p13 gives a bit-field no address, and that is the whole of why it
// needs a file. Every other member is read by computing an address and
// loading through it; a bit-field is read by loading the *allocation unit*
// it shares with its neighbours and then shifting and masking the bits out
// of it, and written by loading that unit, replacing the bits, and storing
// it back. A read-modify-write, for what the source wrote as an assignment.
//
// Where the bits are is not decided here. §6.7.2.1p11 leaves the allocation
// implementation-defined and the two answers in circulation disagree about
// ordinary structs, so the rule is the target's — types.Model.MSBitfields —
// and one walk computes both the record's size and each field's placement.
// This asks that walk through Model.BitPlaces, so the offsets emitted and
// the size reported cannot drift apart.
//
// The sign matters on the way out and not on the way in. A signed bit-field
// of six bits holds -32..31, so reading one shifts left to put its top bit
// in the sign position and arithmetic-shifts back down; an unsigned one
// masks. Writing either only has to keep the bits that fit.

// bitField is a member that is one: where its unit is, which bits inside it,
// and what the declared type says about the sign.
type bitField struct {
	off    int64 // the unit's byte offset from the record's base
	bitOff int64 // the field's first bit within the unit
	unit   int64 // the unit's width in bytes
	width  int64 // the field's width in bits
	typ    types.Type
}

// bitFieldOf finds a member by name and reports whether it is a bit-field.
//
// The search descends into anonymous members, as §6.7.2.1p13's does, adding
// each step's offset — which is what makes `s.flag` reach a bit-field of an
// anonymous struct inside s.
func (u *unit) bitFieldOf(r *types.Record, name string) (bitField, bool) {
	places, ok := u.model.BitPlaces(r)
	if !ok {
		return bitField{}, false
	}
	for i, f := range r.Fields {
		if f.Name == name && f.BitField {
			return bitField{
				off:    places[i].Off,
				bitOff: places[i].BitOff,
				unit:   places[i].Unit,
				width:  f.Width,
				typ:    f.Type,
			}, true
		}
	}
	offs, ok := u.model.FieldOffsets(r)
	if !ok {
		return bitField{}, false
	}
	for i, f := range r.Fields {
		if f.Name != "" {
			continue
		}
		inner := types.AsRecord(f.Type)
		if inner == nil {
			continue
		}
		if bf, ok := u.bitFieldOf(inner, name); ok {
			bf.off += offs[i]
			return bf, true
		}
	}
	return bitField{}, false
}

// unitAt is the address of the allocation unit a bit-field lives in.
func (u *unit) unitAt(base ir.Ptr, bf bitField) ir.Ptr {
	if bf.off == 0 {
		return base
	}
	b := u.fn.cur
	return b.Ptr.Add(base, b.I64.Const(bf.off))
}

// loadBitField reads the bits out of the unit.
//
// The shift is done at the width the *declared type* needs, not the unit's:
// `unsigned long long x : 40` is forty bits of a value an int cannot hold,
// and §6.3.1.1 leaves it in its declared type rather than promoting it. The
// load zero-extends whatever the unit is, so the field's bits are at
// [bitOff, bitOff+width) either way and one shift pair reaches them.
func (u *unit) loadBitField(base ir.Ptr, bf bitField) ir.Value {
	b := u.fn.cur
	addr := u.unitAt(base, bf)
	if u.wideBitField(bf) {
		v := u.loadUnit64(addr, bf.unit)
		v = b.I64.Shl(v, b.I64.Const(64-bf.bitOff-bf.width))
		if u.signedBitField(bf) {
			v = b.I64.SShr(v, b.I64.Const(64-bf.width))
		} else {
			v = b.I64.UShr(v, b.I64.Const(64-bf.width))
		}
		if u.bitFieldWide(bf) {
			return v
		}
		// The shifting needed sixty-four bits and the value does not: a
		// field at bit 40 is reached only from the whole unit, and
		// `unsigned x : 3` there is still an int.
		return b.I32.WrapI64(v)
	}
	v := u.loadUnit(addr, bf.unit)
	v = b.I32.Shl(v, b.I32.Const(32-bf.bitOff-bf.width))
	if u.signedBitField(bf) {
		return b.I32.SShr(v, b.I32.Const(32-bf.width))
	}
	return b.I32.UShr(v, b.I32.Const(32-bf.width))
}

// storeBitField replaces the field's bits and leaves its neighbours alone,
// which is what makes it a read-modify-write rather than a store.
//
// It writes exactly the unit's bytes. A wider store would be simpler and
// would clobber whatever the layout put after the unit.
func (u *unit) storeBitField(base ir.Ptr, bf bitField, v ir.Value) {
	b := u.fn.cur
	addr := u.unitAt(base, bf)
	mask := bitMask(bf.width)

	if u.wideBitField(bf) {
		p := u.toI64(v)
		if p == nil {
			return
		}
		cur := u.loadUnit64(addr, bf.unit)
		cur = b.I64.And(cur, b.I64.Const(^(mask << uint(bf.bitOff))))
		val := b.I64.Shl(b.I64.And(*p, b.I64.Const(mask)), b.I64.Const(bf.bitOff))
		u.storeUnit64(addr, bf.unit, b.I64.Or(cur, val))
		return
	}
	val, ok := v.(ir.I32)
	if !ok {
		// The value reaching here has already been converted to the
		// bit-field's declared type, which promotes to int; anything else
		// is a tree the analyzer would have reported.
		w := u.toI64(v)
		if w == nil {
			return
		}
		val = b.I32.WrapI64(*w)
	}
	cur := u.loadUnit(addr, bf.unit)
	cur = b.I32.And(cur, b.I32.Const(^(mask << uint(bf.bitOff))))
	val = b.I32.Shl(b.I32.And(val, b.I32.Const(mask)), b.I32.Const(bf.bitOff))
	u.storeUnit(addr, bf.unit, b.I32.Or(cur, val))
}

// wideBitField reports whether the shifting is done at sixty-four bits.
//
// Two reasons, and either is enough. The value may not fit in an int — see
// bitFieldWide — or the field may sit past bit 32 of its allocation unit, as
// the second of two thirty-two-bit fields in a doubleword does: reaching it
// at all means loading the whole unit.
func (u *unit) wideBitField(bf bitField) bool {
	return u.bitFieldWide(bf) || bf.bitOff+bf.width > 32
}

// bitFieldWide reports whether the field's *value* needs sixty-four bits,
// which is a question about its declared type: §6.3.1.1 promotes a bit-field
// narrow enough for an int to an int, and leaves the rest in its own type.
func (u *unit) bitFieldWide(bf bitField) bool {
	r, ok := u.reg(bf.typ)
	return ok && r == ir.TypeI64
}

// bitMask is the low w bits set, without the shift that overflows at 64.
func bitMask(w int64) int64 {
	if w >= 64 {
		return -1
	}
	return int64(1)<<uint(w) - 1
}

// loadUnit64 and storeUnit64 are the same as their narrow twins for a field
// whose value does not fit in an int.
func (u *unit) loadUnit64(p ir.Ptr, unit int64) ir.I64 {
	b := u.fn.cur
	switch unit {
	case 1:
		return b.I64.ULoad8(p)
	case 2:
		return b.I64.ULoad16(p)
	case 4:
		return b.I64.ULoad32(p)
	}
	return b.I64.Load(p)
}

func (u *unit) storeUnit64(p ir.Ptr, unit int64, v ir.I64) {
	b := u.fn.cur
	switch unit {
	case 1:
		b.I64.Store8(v, p)
	case 2:
		b.I64.Store16(v, p)
	case 4:
		b.I64.Store32(v, p)
	default:
		b.I64.Store(v, p)
	}
}

// loadUnit and storeUnit read and write an allocation unit of one, two or
// four bytes. A unit is never a bit-field's declared type — it is whatever
// the layout rule opened — so the width is a number here and not a type.
func (u *unit) loadUnit(p ir.Ptr, unit int64) ir.I32 {
	b := u.fn.cur
	switch unit {
	case 1:
		return b.I32.ULoad8(p)
	case 2:
		return b.I32.ULoad16(p)
	}
	return b.I32.Load(p)
}

func (u *unit) storeUnit(p ir.Ptr, unit int64, v ir.I32) {
	b := u.fn.cur
	switch unit {
	case 1:
		b.I32.Store8(v, p)
	case 2:
		b.I32.Store16(v, p)
	default:
		b.I32.Store(v, p)
	}
}

// signedBitField reports whether the field's value is sign-extended on the
// way out. Plain `int x : 3` is signed and plain `unsigned x : 3` is not;
// plain *char* as a bit-field takes the target's answer for plain char,
// which is the same question §6.2.5p15 asks about every char.
func (u *unit) signedBitField(bf bitField) bool { return u.signed(bf.typ) }

// bitFieldMember resolves a member expression to the bit-field it names and
// the address of the record holding it, or reports that it is not one.
func (u *unit) bitFieldMember(e *ast.MemberExpr) (ir.Ptr, bitField, bool) {
	if u.info.Props[e] != nil {
		return ir.Ptr{}, bitField{}, false
	}
	xt := u.typeOf(e.X)
	if o := types.AsObject(xt); o != nil {
		// An instance variable that is a bit-field. The runtime writes an
		// ivar's offset in *bytes*, so packing several into one word means
		// agreeing with clang about which bits each one has — a second
		// layout question, with the non-fragile ABI on the other side of
		// it. Until that is answered, saying so beats reading the whole
		// word and calling it the field.
		if o.Base != nil {
			if iv, _ := o.Base.FindIvar(u.name(e.Sel)); iv != nil && iv.BitField {
				u.unsupported(e, "an instance variable declared as a bit-field")
			}
		}
		return ir.Ptr{}, bitField{}, false
	}

	// Which record, and whether the member is a bit-field, are questions
	// about types: they are answered before the operand is evaluated, and
	// the operand only for a member that is one. Evaluating it to find out
	// ran `f().x`'s call twice -- once here, once for the ordinary read.
	var rec *types.Record
	if e.Op == token.ARROW {
		if pt := types.AsPointer(xt); pt != nil {
			rec = types.AsRecord(pt.Elem)
		}
	} else {
		rec = types.AsRecord(xt)
	}
	if rec == nil {
		return ir.Ptr{}, bitField{}, false
	}
	bf, ok := u.bitFieldOf(rec, u.name(e.Sel))
	if !ok {
		return ir.Ptr{}, bitField{}, false
	}
	var base ir.Ptr
	if e.Op == token.ARROW {
		p, ok := u.rvalue(e.X).(ir.Ptr)
		if !ok {
			return ir.Ptr{}, bitField{}, false
		}
		base = p
	} else {
		addr, _ := u.lvalue(e.X)
		if addr == nil {
			return ir.Ptr{}, bitField{}, false
		}
		base = *addr
	}
	return base, bf, true
}

// assignBitField lowers `s.f = v` and the compound forms.
//
// A compound assignment reads the field, applies the operator, and writes
// it back — the same shape as any other lvalue's, except that each half is
// a shift and a mask rather than a load and a store.
func (u *unit) assignBitField(e *ast.AssignExpr, base ir.Ptr, bf bitField) ir.Value {
	rhs := u.rvalue(e.Rhs)
	if rhs == nil {
		return nil
	}
	if e.Op == token.ASSIGN {
		v := u.convert(rhs, u.typeOf(e.Rhs), bf.typ)
		u.storeBitField(base, bf, v)
		// The value of the assignment is what was stored — read back
		// through the field, because a value too wide for the bits is
		// truncated by the store and §6.5.16p3 says the result is the
		// stored one.
		return u.loadBitField(base, bf)
	}
	old := u.loadBitField(base, bf)
	if old == nil {
		return nil
	}
	res := u.arith(compoundOp(e.Op), old,
		u.convert(rhs, u.typeOf(e.Rhs), bf.typ), u.signedBitField(bf))
	if res == nil {
		return nil
	}
	u.storeBitField(base, bf, res)
	return u.loadBitField(base, bf)
}

// incDecBitField is `s.f++` and its three relatives.
func (u *unit) incDecBitField(base ir.Ptr, bf bitField, op token.Kind, postfix bool) ir.Value {
	old := u.loadBitField(base, bf)
	if old == nil {
		return nil
	}
	arith := token.ADD
	if op == token.DEC {
		arith = token.SUB
	}
	next := u.arith(arith, old, u.constOf(1, bf.typ), u.signedBitField(bf))
	if next == nil {
		return nil
	}
	u.storeBitField(base, bf, next)
	if postfix {
		return old
	}
	return u.loadBitField(base, bf)
}

// fillBitField initializes one bit-field member from a braced initializer.
//
// The object was zeroed before this — §6.7.9p21 — so a member the list does
// not reach is already zero, and one it does reach is written the same way
// an assignment writes it.
func (u *unit) fillBitField(addr ir.Ptr, r *types.Record, i int, c *initCursor, at ast.Node) {
	places, ok := u.model.BitPlaces(r)
	if !ok {
		u.unsupported(at, "an initializer for a bit-field")
		return
	}
	f := r.Fields[i]
	bf := bitField{off: places[i].Off, bitOff: places[i].BitOff,
		unit: places[i].Unit, width: f.Width, typ: f.Type}

	it := c.peek()
	if it == nil {
		return
	}
	c.i++
	if it.Value == nil {
		return
	}
	v := u.rvalue(it.Value)
	if v == nil {
		return
	}
	u.storeBitField(addr, bf, u.convert(v, u.typeOf(it.Value), f.Type))
}

// ivarIsBitField reports whether a bare instance-variable name inside a
// method body names a bit-field, which lower does not read yet.
func (u *unit) ivarIsBitField(st *storage) bool {
	k := u.classNamed(st.class)
	if k == nil {
		return false
	}
	iv, _ := k.FindIvar(st.ivar)
	return iv != nil && iv.BitField
}

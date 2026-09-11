package lower

import (
	"sort"

	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/types"
)

// The storage shape of a C type, as VIR declares it.
//
// A register type (type.go's reg) says how a value is held; an ftype says
// how an *object* is laid out, and a global needs one because §19.10 checks
// its initializer against the declared type. A run of bytes would print but
// would not verify, and would tell the assembler nothing about where a
// relocation lands.
//
// Padding is stated rather than computed. Every field carries the offset the
// model gave it and a struct shorter than sizeof gets an explicit tail, so
// the layout VIR sees is the layout the analyzer measured — not a second
// opinion arrived at independently.

// ftype is the storage type of a C type, if VIR can describe one.
func (u *unit) ftype(t types.Type) (ir.FType, bool) {
	t = types.Unqualify(t)
	switch {
	case types.IsArray(t):
		a := types.AsArray(t)
		elem, ok := u.ftype(a.Elem)
		if !ok {
			return ir.FType{}, false
		}
		n := a.Len
		if a.Form != types.FixedArray {
			// An array whose length the initializer decided: the model
			// already sized the object, so the count comes back out of it.
			esz, _ := u.sizeAlign(a.Elem)
			total, _ := u.sizeAlign(t)
			if esz == 0 {
				return ir.FType{}, false
			}
			n = int64(total / esz)
		}
		return ir.Array(uint64(n), elem), true

	case types.IsRecord(t):
		r := types.AsRecord(t)
		st, ok := u.recordType(r)
		if !ok {
			return ir.FType{}, false
		}
		return st.FType(), true
	}
	if s, ok := u.store(t); ok {
		return s.FType(), true
	}
	return ir.FType{}, false
}

// recordType interns a struct or union as a named VIR type.
//
// A union is declared as a union, whose fields all begin at zero; a struct
// states every offset, because C's padding is the model's answer and not
// something a second layout algorithm should be asked to reproduce.
func (u *unit) recordType(r *types.Record) (*ir.Type, bool) {
	if t, ok := u.records[r]; ok {
		return t, t != nil
	}
	// Recording the miss first: a struct that contains a pointer to itself
	// would otherwise recurse forever.
	u.records[r] = nil
	if !r.Complete {
		return nil, false
	}
	offs, ok := u.model.FieldOffsets(r)
	if !ok {
		return nil, false
	}

	// A type name is its own namespace and takes no symbol prefix: nothing
	// links against it, and the '.' a C tag would carry is not an
	// identifier character.
	name := "struct_" + r.Name
	if r.Union {
		name = "union_" + r.Name
	}
	if r.Name == "" {
		name = u.uniqType("anon")
	} else if u.mod.LookupType(name) != nil {
		name = u.uniqType(name)
	}

	lay, ok := u.virLayout(r, offs)
	if !ok {
		return nil, false
	}
	fields := lay.fields

	// Declared once, as what it is. Declaring a struct and then a union of
	// the same name over it is two types with one name, which the module
	// refuses — and refuses by the name alone, so the diagnostic points at a
	// type nobody wrote.
	var t *ir.Type
	if r.Union {
		t = u.mod.Union(name)
	} else {
		t = u.mod.Struct(name)
	}
	t.Internal()
	for _, f := range fields {
		if r.Union {
			t.Field(f.Name, f.Type)
			continue
		}
		t.FieldAt(f.Name, f.Type, f.Offset)
	}
	// The padding after the last member, which sizeof counts and the field
	// list does not: without it an array of this struct has the wrong
	// stride.
	if !r.Union && lay.end > 0 {
		size, _ := u.sizeAlign(r)
		if int64(size) > lay.end {
			t.FieldAt("_tail",
				ir.Array(uint64(int64(size)-lay.end), ir.StoreI8.FType()), uint64(lay.end))
		}
	}
	if r.Packed {
		t.Pack()
	}
	u.records[r] = t
	return t, true
}

// uniqType numbers a type name until the module has no such type.
//
// The separator is '_' where uniq's is '.', because these two names live in
// different namespaces with different rules: a symbol is written through sym,
// which maps '.' to the '$' a linker accepts, and a type name is written bare
// in the IR text and has to be an identifier there. A type that reached VIR
// as "anon.2" was rejected by the module, not by the assembler.
func (u *unit) uniqType(prefix string) string {
	for {
		u.anon++
		name := prefix + "_" + itoa(u.anon)
		if u.mod.LookupType(name) == nil {
			return name
		}
	}
}

// virLayout is how a C record becomes a VIR struct: the fields, in offset
// order, and where each member's value goes in them.
//
// The order is the requirement. VIR wants a struct's fields in increasing
// offset order, and a bit-field's bytes are not where its declaration is —
// `struct { unsigned a : 3; unsigned d; unsigned e : 1; }` has its two bit
// ranges on either side of d. So the list is built and then sorted, and the
// mapping is kept because an initializer has to put each member's value in
// the field it ended up as.
type virLayout struct {
	fields []ir.Field
	slot   []int // per C member: its index in fields, or -1 for a bit-field
	ranges [][2]int64
	rslot  []int // per byte range: its index in fields

	// end is where the last field ends, which is not the last member's end:
	// a bit-field's bytes may come after it, and a bit-field has no offset
	// of its own to compute from. It is what the tail padding is measured
	// against.
	end int64
}

func (u *unit) virLayout(r *types.Record, offs []int64) (*virLayout, bool) {
	lay := &virLayout{slot: make([]int, len(r.Fields)), ranges: u.bitRanges(r)}
	type entry struct {
		f     ir.Field
		cIdx  int // the C member, or -1
		rgIdx int // the byte range, or -1
	}
	var es []entry
	for i, f := range r.Fields {
		lay.slot[i] = -1
		if f.BitField {
			// A bit-field is not a field here. §6.7.2.1p13 gives it no
			// address, so there is nothing for VIR to name, and what the
			// type says instead is where the bytes are.
			continue
		}
		ft, ok := u.ftype(f.Type)
		if !ok {
			return nil, false
		}
		fname := f.Name
		if fname == "" {
			fname = "f" + itoa(i)
		}
		es = append(es, entry{cIdx: i, rgIdx: -1, f: ir.Field{
			Name: fname, Type: ft, Offset: uint64(offs[i]), HasOffset: true}})
		fsz, _ := u.sizeAlign(f.Type)
		if end := offs[i] + int64(fsz); end > lay.end {
			lay.end = end
		}
	}
	for k, rg := range lay.ranges {
		es = append(es, entry{cIdx: -1, rgIdx: k, f: ir.Field{
			Name:      "bits_" + itoa(int(rg[0])),
			Type:      ir.Array(uint64(rg[1]-rg[0]), ir.StoreI8.FType()),
			Offset:    uint64(rg[0]),
			HasOffset: true,
		}})
		if rg[1] > lay.end {
			lay.end = rg[1]
		}
	}
	sort.SliceStable(es, func(i, j int) bool { return es[i].f.Offset < es[j].f.Offset })

	lay.rslot = make([]int, len(lay.ranges))
	lay.fields = make([]ir.Field, len(es))
	for i, e := range es {
		lay.fields[i] = e.f
		if e.cIdx >= 0 {
			lay.slot[e.cIdx] = i
		} else {
			lay.rslot[e.rgIdx] = i
		}
	}
	return lay, true
}

// bitRanges is the byte ranges a record's bit-fields occupy, in order.
//
// One per merged allocation unit, because that is what the bytes are: two
// bit-fields of different declared types may open units at the same offset
// and of different widths, and two fields of a struct type may not overlap.
// recordType appends one array field per range, after every ordinary member,
// and constinit.go packs constants into the same ranges — so both read this
// and the two cannot describe different bytes.
func (u *unit) bitRanges(r *types.Record) [][2]int64 {
	places, ok := u.model.BitPlaces(r)
	if !ok {
		return nil
	}
	var out [][2]int64
	for i, f := range r.Fields {
		if !f.BitField || f.Width == 0 {
			continue
		}
		// The bytes the *bits* occupy, not the allocation unit's. A unit
		// may overlap an ordinary member — `struct { char c; int b : 3; }`
		// opens a four-byte unit at zero and puts b in the byte after c —
		// and two fields of a struct type may not overlap. The bits are
		// where the field actually is, and they never overlap a member,
		// because the layout put them after one.
		start := places[i].Off*8 + places[i].BitOff
		out = append(out, [2]int64{start / 8, (start + f.Width + 7) / 8})
	}
	return mergeRanges(out)
}

// mergeRanges sorts byte ranges and joins the ones that touch.
//
// Two bit-fields of different declared types may open allocation units at
// the same offset and of different widths — `unsigned a : 3; unsigned short
// b : 3;` is one unit of four bytes and one of two, both at zero — and two
// fields of a struct type may not overlap. One range covering both is the
// only description that is true of the bytes.
func mergeRanges(in [][2]int64) [][2]int64 {
	if len(in) == 0 {
		return nil
	}
	sorted := append([][2]int64(nil), in...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i][0] < sorted[j][0] })
	out := [][2]int64{sorted[0]}
	for _, rg := range sorted[1:] {
		last := &out[len(out)-1]
		if rg[0] <= last[1] {
			if rg[1] > last[1] {
				last[1] = rg[1]
			}
			continue
		}
		out = append(out, rg)
	}
	return out
}

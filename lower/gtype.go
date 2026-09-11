package lower

import (
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

	fields := make([]ir.Field, 0, len(r.Fields))
	for i, f := range r.Fields {
		if f.BitField {
			// A bit-field has no field of its own: its bits live inside an
			// allocation unit shared with its neighbours, and describing
			// that would mean describing the packing. Nothing needs it yet.
			return nil, false
		}
		ft, ok := u.ftype(f.Type)
		if !ok {
			return nil, false
		}
		fname := f.Name
		if fname == "" {
			fname = "f" + itoa(i)
		}
		fields = append(fields, ir.Field{Name: fname, Type: ft,
			Offset: uint64(offs[i]), HasOffset: true})
	}

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
	if !r.Union && len(r.Fields) > 0 {
		size, _ := u.sizeAlign(r)
		last := r.Fields[len(r.Fields)-1]
		lsz, _ := u.sizeAlign(last.Type)
		end := offs[len(offs)-1] + int64(lsz)
		if int64(size) > end {
			t.FieldAt("_tail", ir.Array(uint64(int64(size)-end), ir.StoreI8.FType()), uint64(end))
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
